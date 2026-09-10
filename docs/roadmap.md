# Roadmap to v1.0.0

Milestone: [v1.0.0](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/milestone/1).

Each phase gets its own feature branch and PR against main. The branch numbers
below are a sequence proposal; issue numbers are independent. Do not start the
next phase until the current one is cleanly complete unless the owner explicitly
directs otherwise. Existing failing checks block merge, including baseline failures.

Phase 1 and the authorized phase-2 build repairs are prepared on separate branches.
The phase-2 PR includes the bootstrap ancestors for a normal merge into main.
The owner performs the merge after reviewing its checks. No new AUVC runtime
feature is complete yet. See [the build baseline](baseline-build.md).
[Requirements](requirements.md) define the acceptance contract.

| Phase | Scope | Proposed branch | Tracking issue |
| --- | --- | --- | --- |
| 1 | Repository, licenses, unchanged upstream imports | `codex/001-bootstrap` | [#1](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/1) |
| 2 | Reproducible build and test baseline | `codex/002-baseline-build` | [#2](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/2) |
| 3 | Go/DiscordGo toolchain and dependencies | `codex/003-modernize-go` | [#3](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/3) |
| 4 | Remove unnecessary public/premium infrastructure | `codex/004-service-cleanup` | [#4](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/4) |
| 5 | Domain and Game State separation | `codex/005-game-state` | [#7](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/7) |
| 6 | SQLite configuration and migrations | `codex/006-persistent-config` | [#5](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/5) |
| 7 | Typed, authorized /au slash commands | `codex/007-discord-commands` | [#6](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/6) |
| 8 | Voice Policy and Discord Reconciler | `codex/008-voice-policy` | [#7](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/7) |
| 9 | Ghost channel and Move Members | `codex/009-ghost-channel` | [#8](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/8) |
| 10 | Channel enforcement and overrides | `codex/010-channel-enforcement` | [#9](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/9) |
| 11 | Supported .NET LTS and capture dependencies | `codex/011-capture-modernization` | [#10](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/10) |
| 12 | Versioned transport-independent protocol | `codex/012-protocol` | [#11](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/11) |
| 13 | Direct authenticated WSS and pairing | `codex/013-secure-websocket` | [#12](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/12) |
| 14 | Final Galactus/Redis/PostgreSQL removal | `codex/014-remove-legacy-services` | [#4](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/4) |
| 15 | Reconnect, snapshot recovery and fail-safe | `codex/015-recovery` | [#13](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/13) |
| 16 | Doctor, logging and diagnostics | `codex/016-doctor` | [#16](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/16) |
| 17 | Docker, CI/CD, dependency automation | `codex/017-deploy-ci` | [#14](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/14), [#15](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/15) |
| 18 | Full E2E acceptance tests | `codex/018-e2e` | [#17](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/17) |
| 19 | Validated user/operator/developer documentation | `codex/019-documentation` | [#16](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/16) |
| 20 | v1.0.0 release candidate and release readiness | `codex/020-release-candidate` | [#18](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/18) |

Use `Refs #...` for partial work on issues spanning phases. Use `Closes #...`
only once that issue's complete acceptance criteria are met. PRs must record
changes, purpose, exact validation results, migrations and known limitations.

## Release progression

### Windows usability and automated delivery work packages

The [Windows installation and release contract](windows-installation-and-releases.md)
extends the original ZIP-only target with a graphical EXE installer, MSI, guided
first-run setup, secure updates and fully automated release delivery after an
explicit release trigger. These are planned features, not available downloads.

| Issue | Deliverable | Placement |
| --- | --- | --- |
| [#20](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/20) | Self-contained EXE/MSI/ZIP, repair/upgrade/uninstall | After .NET modernization; phase 17 |
| [#21](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/21) | Guided pairing, clear status and accessible German/English GUI | Phases 11/13/16 |
| [#22](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/22) | Verified Stable/Preview updates and recovery | After protocol/recovery; phase 17 |
| [#23](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/23) | Release preparation, tag-triggered Pre-Releases/Stable, complete artifacts | Phase 17; release gates in phase 20 |
| [#24](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/24) | Signing/publisher identity and accessible downloads | Plan prerequisites early; phase 17 |

Use a separate `codex/<number>-<topic>` branch and PR per work package when its
prerequisites are ready. Phase 17 is split into small deliverables. Extend #17
with clean-Windows installation/update tests, #16 with user guides and #18 with
all installer/signing/distribution gates. The main phase sequence is unchanged.

### Version sequence

Early development: `v0.1.0-alpha.1`, `v0.1.0-alpha.2`.
Broader integration: `v0.5.0`. Stable: `v1.0.0` after acceptance.

Required artifacts: bot Docker image, signed Windows x64 Setup EXE/MSI and
self-contained portable ZIP, checksums, signed update metadata, SBOM/provenance,
release notes and upgrade/migration instructions. Preview never updates the Stable
feed or latest image alias. No release is created by the bootstrap phase.
