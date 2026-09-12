# Windows installation, updates and release delivery

Status: planned work, added to the v1.0.0 roadmap on 2026-09-10.
No installer, updater, signing integration or release workflow is implemented yet.

## User journey

For the capture host: **Download Setup EXE -> Install -> Pair -> Play**.
The graphical installer is the recommended entry point. No BAT, terminal,
manual runtime installation, developer tools or hand-edited configuration should
be necessary. Only one Windows PC runs capture; installing the Discord bot on a
self-hosted server remains a separate administrator task.

First launch guides the user through language, bot address, one-time pairing code
and connection verification. Explain what to do when Among Us is not running,
a code expires, a protocol is incompatible or the bot is unreachable.
Use plain German/English labels, keyboard navigation, high-DPI support and
status indicators that do not rely on color alone.

The main screen shows game detection, bot connection, pairing/session state,
installed version and any actionable problem. Settings, release channel, update
progress and redacted diagnostic export belong in the GUI. Autostart, tray
behavior and shortcuts are deliberate choices. Never show permanent credentials.

## Deliverables and installer behavior

| Artifact | Purpose |
| --- | --- |
| `AmongUsVoiceCapture-Setup-win-x64.exe` | Recommended graphical installer |
| `AmongUsVoiceCapture-win-x64.msi` | Windows Installer package for direct/managed installation |
| `AmongUsVoiceCapture-win-x64.zip` | Self-contained portable application |
| `AUVC-bot-win-x64.zip` | The bot as a Windows executable, for running it on the same PC |
| `AUVC-bot-linux-amd64.tar.gz`, `AUVC-bot-linux-arm64.tar.gz` | The bot for a server, a NAS or a Raspberry Pi |
| `SHA256SUMS` | Checksums of the downloadable artifacts |
| Channel-specific signed update manifest | Version, platform, protocol compatibility and verified asset metadata |
| Release notes, SBOM and provenance | Changes, migration instructions, dependencies and source/build identity |
| Bot Docker image | The same bot, for anyone who would rather run a container |

AUVC is two programs. Capture reads the game on the Windows PC that plays it;
the bot talks to Discord and can run anywhere. They are released together and
must match, because the protocol between them is versioned and a mismatch is
refused at the handshake.

The bot ships as a plain executable as well as a container, because the two
answer different situations. One person playing on one PC wants to run the bot
there and be done; somebody with a server or a Raspberry Pi wants it to stay
online when the gaming PC is off. Neither is more correct, and the bot is a
single static binary with no runtime to install, so offering both costs a build
matrix entry rather than a second architecture.

The bot needs no installer and no signing to be useful: it is started by whoever
runs it, not double-clicked by somebody who downloaded it expecting a program to
install. Capture is the one that faces that problem, and that is what the
signing and publisher-identity work is for.

EXE and MSI use the same tested capture payload and coordinated installation
identity, not two competing installations. Prefer per-user installation and
normal operation without elevation; document and test an optional machine-wide
installation separately. Validate supported Windows versions, architecture,
disk space, running processes and file locks before changing anything.

Provide install, cancellation, repair, upgrade and uninstall behavior.
Retain settings and protected pairing credentials during upgrades. Uninstall
offers an explicit local-data deletion choice; explain that deleting local
credentials and revoking remote access are distinct actions. Portable mode must
define its data location and update behavior explicitly.

The installer framework is Inno Setup, chosen and justified in
[installer-decision.md](installer-decision.md) against WiX/MSI, MSIX and
Squirrel. The MSI is deferred rather than dropped: its three-field
`ProductVersion` has no direct representation for `1.2.3-rc.1`, and that
mapping should be designed and tested when an MSI is actually needed rather
than guessed now.

**The first releases are unsigned, by owner decision.** SmartScreen shows
"Windows protected your PC — Unknown publisher", and the person has to click
**More info** and then **Run anyway**. That is stated in the download
instructions rather than left as a surprise: somebody who was not expecting
the warning concludes the download is broken or malicious, and somebody who
was expecting it clicks through anything that looks similar. `SHA256SUMS`
published beside the artifacts is what a careful person can check instead,
which proves the file matches the release page and not who built it.
Signing is tracked in #24 and changes nothing about the installer except
that it is signed.

Keep user-facing SemVer and installer versions consistent. Define a deterministic,
collision-free mapping for preview/stable packages, installation identity and
upgrade ordering. Windows Installer uses three numeric ProductVersion fields
and ignores a fourth; directly inserting `-alpha.1` is not a solution.
See [Microsoft ProductVersion](https://learn.microsoft.com/en-us/windows/win32/msi/productversion).
Test the mapping rather than choosing it speculatively in this planning phase.

## Update contract

Stable is the default channel. Preview is opt-in and visibly labeled; alpha,
beta and release-candidate builds must never silently reach Stable users.

Check for updates automatically, present release notes and allow the user to
install, defer, retry or cancel. Do not interrupt a running match, restart capture
silently during a round or force a computer restart. Coordinate the updater with
session state and fail-safe recovery.

Use HTTPS and authenticated manifests plus signature verification before
execution. A checksum downloaded beside an untrusted binary is not sufficient
publisher verification. Define trusted keys, rotation, metadata freshness,
anti-replay behavior, supported OS/architecture and capture/bot protocol ranges.
Reject corrupted or incompatible packages with an actionable message.

Stage downloads before replacing a working installation. Preserve configuration
and protected credentials. Recover from interruption, offline operation, locked
files and partial installs. Rollback is allowed only when data/protocol versions
remain compatible; never silently downgrade or destructively migrate settings.
Switching from Preview back to Stable requires explicit version/path handling.

No shared GitHub PAT, private signing key or download credential may be embedded
in the client, installer, manifest, logs or repository.

## Automated release trigger contract

All rows below describe future automation, not active workflows.

| Trigger | Result |
| --- | --- |
| Pull request | Format/lint/test/build plus installer checks when available; temporary test artifacts, no user release |
| Ordinary push/merge to main | Integration checks; no Stable/Preview publication |
| Merge of an explicitly reviewed release-preparation PR | Controller validates version/changelog and creates the intended protected SemVer tag |
| `vX.Y.Z-alpha.N`, `vX.Y.Z-beta.N`, `vX.Y.Z-rc.N` tag | Full pipeline; GitHub pre-release and Preview feed after all gates pass |
| `vX.Y.Z` tag | Full pipeline; stable release, Stable feed and appropriate stable image tags after all gates pass |
| Authorized manual retry/dry-run | Resume/test the same validated release intent and SHA; no bypass of gates |

Automate preparation of version/changelog PRs from reviewed changes, while
requiring an explicit release intent. Ordinary dependency merges must not create
releases. Tags are the single publishing entry point; do not run competing
publishers for tag, release and workflow-completed events.

Reject malformed tags, version mismatches, unapproved source revisions and
failed checks. Validate the exact tagged SHA against the allowed main history.
Use one version source for application metadata, installer mapping, update
manifests, image tags and release notes.

Release sequence:

1. Validate source/tag/version and run mandatory format, lint, tests, builds,
   dependency/secret scans and Windows installation tests.
2. Build the capture payload once for the release. Sign and verify application
   executables before packaging that signed payload into MSI/ZIP and the setup EXE.
3. Sign and verify installer packages using protected signing infrastructure;
   sign any contained MSI before embedding it in the setup EXE. Calculate checksums
   only after all signing/packaging steps, over the final downloadable bytes.
4. Produce signed manifests, SBOM/provenance, release notes and explicit
   upgrade/migration instructions. Build and verify the matching bot image.
5. Assemble a draft release and immutable versioned artifacts. Verify the complete
   asset set and access from a clean client context before making it discoverable.
6. Publish the complete release; update channel feeds and mutable image aliases
   last. Preview never moves Stable or `latest`.

Limit concurrency by release identity. Retry idempotently using recorded source
and artifact identity. Do not move tags, overwrite a published version or replace
signed bytes under the same version silently. Cross-service publication can
partially fail: keep existing feeds pointing to their last verified version and
provide a documented recovery path.

Pin toolchains/actions appropriately, limit workflow permissions and prevent
untrusted PR code from accessing signing/publishing credentials. Signing service
access should use scoped short-lived identity where supported.

## Signing and download access

Before supported public releases, establish publisher identity, signing provider,
timestamping, key rotation and CI access. Track any account verification and costs
as external prerequisites. This plan does not purchase a certificate, provision
paid services, publish artifacts or change repository visibility.

A valid signature identifies the publisher; it does not promise the absence of
every Windows reputation warning. Do not make bypassing OS warnings the normal
installation guide.

The source repository is currently private. GitHub releases are visible to people
with repository read access; a private release URL alone cannot serve ordinary
community users. See [GitHub release access](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases).
Define a private pilot access path and an explicitly approved public download
channel/artifact repository/hosting arrangement before community distribution.
Keep source visibility unchanged unless the owner requests a change.

The download page should clearly offer the recommended EXE, optional MSI/ZIP,
installed/available version guidance, Stable/Preview choice, system requirements
and checksum/signature information. Test the actual intended audience's access,
including missing/expired authorization, without embedding a shared token.

## Acceptance matrix

| Scenario | Required result |
| --- | --- |
| Fresh supported Windows VM, no .NET/SDK, standard user | EXE and MSI install and launch; portable ZIP launches |
| Paths containing spaces/Unicode, high DPI, keyboard-only use | Setup and first-run flow remain usable |
| Install twice, repair, cancel, uninstall | No duplicate/broken installation; clear data-retention choices |
| First pairing, expired code, unavailable bot/game | Clear state, next action and retry path |
| alpha -> alpha, alpha -> Stable, Stable -> Stable | Correct ordering; settings and pairing retained |
| Preview opt-out or incompatible bot version | No silent downgrade or incompatible update |
| Offline, truncated/corrupt download, invalid signature, stale manifest | Update rejected; working version remains available |
| Crash/file lock during installation | Deterministic recovery; no lost credentials/configuration |
| Update available during an active match | Round continues; installation deferred |
| Prerelease publication, failed signing/test/upload, duplicate trigger | No Stable feed contamination or partial release exposed through feeds |
| Download with a fresh intended-user account/context | Artifacts accessible through the documented channel |
| Published package contents | Correct version, valid signatures/checksums, licenses, notices and no secrets |

Use automated clean-VM installation/update tests plus explicit GUI usability
acceptance. Distinguish simulated protocol tests from real Among Us/Discord
checks. The supported Windows/game-version matrix must be validated before release.

## Work packages and sequencing

| Issue | Scope | Dependencies / phase |
| --- | --- | --- |
| [#20](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/20) | EXE/MSI/ZIP installer | .NET modernization (#10); packaging phase 17 |
| [#21](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/21) | First-run and GUI UX | Design in phase 11; pairing phase 13; diagnostics phase 16 |
| [#22](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/22) | Secure updater | Protocol/recovery (#11/#13), signing and feeds; phase 17 |
| [#23](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/23) | Release automation | CI (#15), installer/signing/metadata; phase 17, acceptance 20 |
| [#24](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/24) | Signing and distribution | Plan external access early; implement/verify in phase 17 |

Each work package uses a separate feature branch and PR when its prerequisites
are ready. Phase 18 (#17) covers the acceptance matrix; phase 19 (#16) documents
tested installation/updates; phase 20 (#18) gates the release candidate.
The 20 main phases remain in order; phase 17 is divided into these smaller
deliverables instead of one large installer/release rewrite.

## Historical reference

Reviewed [capture-install](https://github.com/automuteus/capture-install) at
`c08906336fb60fb2e8feb0ac2e033f76c17bfb0a`, default branch `main`,
on 2026-09-10. Its BAT installs .NET Desktop Runtime 5.0.1, downloads the capture
ZIP, extracts and launches it, then deletes itself. It also uses a VBS helper
and an MD5 check for the runtime download. These observations inform requirements,
not an implementation template.

No capture-install source, scripts or assets were imported or executed.
Its license is MIT, Copyright (c) 2021 automuteus. If code/assets are reused later,
preserve that original license text and notice, record the exact commit/path and
extend LICENSES/THIRD_PARTY_NOTICES before the import.
