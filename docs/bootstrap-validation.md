# Bootstrap validation

Historical phase-1 report. Phase 2 repairs and current commands are documented
in [baseline-build.md](baseline-build.md); the failures below describe the original
import inspection and are intentionally retained as evidence.

Date: 2026-09-10. Scope: unchanged imports and first-party bootstrap files on
`codex/001-bootstrap`. This is an initial inspection, not completion of the
separate build-baseline or runtime modernization phases.

## Environment

Windows x64; Git for Windows; Python 3.13.7; Go 1.19.13; .NET SDK 8.0.424;
Gitleaks 8.30.1. Docker is not installed/available on this workstation.
Go and Gitleaks were downloaded from their official release locations and their
archives checked against the published SHA256 checksums. Portable tools,
dependency caches and redacted logs are local-only under `.git/`.

## Results

| Check | Command / method | Result |
| --- | --- | --- |
| Bot import identity | Compare `HEAD:bot` with pinned upstream tree | PASS: `d9ae52ffc0e804e0211154ace579425fc5ef9f4f` |
| Capture import identity | Compare `HEAD:capture` with pinned upstream tree | PASS: `d05d7cd8f300b4368d3872337d2a9f5059c76ee6` |
| Original license copies | Compare bytes against imported Git blobs | PASS; both original Denver Quane notices retained |
| Bootstrap files | `python scripts/verify_bootstrap.py` | PASS: tree/license identity, structure, links and text format |
| First-party diff | `git diff --cached --check` before scaffold commit | PASS |
| Staged first-party secrets | Staged diff piped to `gitleaks stdin --redact --config .gitleaks.toml` | PASS |
| Committed history secrets | `gitleaks git . --redact --config .gitleaks.toml --log-opts=HEAD` | PASS; no findings in reachable AUVC history |
| Go formatting | `gofmt -l .` in `bot/` | FAIL: 109 existing files need formatting; no import rewritten |
| Go lint | `go vet ./...` in `bot/` | PASS, exit 0 |
| Go tests | `go test ./...` in `bot/` | PASS, seven packages have passing tests; remaining packages report no tests |
| Go build | `go build ./...` in `bot/` | PASS, exit 0 |
| Capture restore | `dotnet restore capture/AmongUsCapture.sln --verbosity minimal` | PASS with legacy-framework/package warnings |
| Capture solution build | `dotnet build capture/AmongUsCapture.sln --no-restore --verbosity minimal` | FAIL: 14 CS0117 errors in AUOffsetHelper; 105 warnings |
| Capture WPF isolation | `dotnet build capture/AUCapture-WPF/AUCapture-WPF.csproj --no-restore --verbosity minimal` | PASS, three warnings on the isolated build |
| Capture formatting | `dotnet format whitespace capture/AmongUsCapture.sln --verify-no-changes --no-restore --verbosity quiet` | FAIL, exit 2; 1,645 existing whitespace diagnostics |
| Capture tests | `dotnet test capture/AmongUsCapture.sln --no-build --no-restore --verbosity normal` | NO COVERAGE: exit 0, but no test projects or tests discovered/executed |
| Docker build | Docker CLI/daemon prerequisite check | NOT RUN: Docker unavailable |
| Upstream diff whitespace | `git diff main --check -- bot capture` | FAIL: inherited whitespace/line-ending findings; pristine imports retained |

Root scaffold verification and final committed-history secret scan are run using:

```sh
python scripts/verify_bootstrap.py
git diff --cached --check -- . ':(exclude)bot/**' ':(exclude)capture/**'
gitleaks git . --redact --config .gitleaks.toml --log-opts=HEAD
```

The first-party verifier checks text format, local document links, exact license
copies, scaffold files and pristine import identities. It does not replace
application tests or a secret scanner. Nested upstream CI workflows are retained
for provenance but are not active root workflows. Full AUVC CI is not established.

Git for Windows initially converted local import files to CRLF. Their committed
objects were unchanged. The working files were restored from the same Git objects
with automatic conversion disabled, and the index refreshed without blob changes.
Root attributes now preserve upstream and copied license bytes while requiring LF
for new AUVC documentation, scripts and configuration. The final worktree is clean.

## Capture failure analysis

`capture/AUOffsetHelper/Program.cs:63-84` constructs
`PlayerInfoStructOffsets` and `WinningPlayerDataStructOffsets` using obsolete
properties (including PlayerNameOffset, ColorIDOffset, DeadOffset and
ImposterOffset). The corresponding classes in
`capture/AUOffsetManager/OffsetManager.cs:158-178` have a different shape,
including IsDeadOffset and outfit-related indirection.

The helper remains included and enabled in the solution. The WPF project does
not depend on the helper and builds independently. This isolates the reported
solution failure to stale helper code; it does not prove current Among Us
memory compatibility or correct capture behavior.

Do not delete the helper, alter offsets speculatively or exclude its project just
to make the solution green. Phase 2 must determine intended helper behavior and
fix/validate the baseline separately. Memory offsets require relevant game fixtures
or runtime evidence before modification.

Restore also reports end-of-support frameworks, WebSocketSharp framework
compatibility warnings and a NuGet vulnerability warning for Tmds.DBus 0.9.1
(GHSA-xrw6-gwf8-vvr9). Record and address these in controlled dependency work;
the import does not claim production readiness.

## Secret review

Gitleaks initially flagged the constant `steamapi64_orig_hash` in
`capture/AmongUsCapture/Verification/GameVerifier.cs:28`.
The value is a SHA-1 file checksum; line 96 compares it with the computed
`steam_api64.dll` hash. It is not an API credential.

The root Gitleaks configuration retains default detectors and exempts only that
exact checksum value in that exact relative source path. The reviewed
upstream snapshot scan passes with no findings. The embedded
`AutoMuteUs_PK.asc` begins with `PGP PUBLIC KEY BLOCK`; it is a public
verification key, not a private key. No runtime credentials were configured.

## Review and next-phase boundary

The bootstrap PR stays draft and must not merge while checks fail.
The import requirement is satisfied without changing upstream source. Resolving
formatting, the solution failure and missing test coverage is tracked in
[build baseline #2](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/2).
Starting that separate phase while phase 1 remains blocked requires explicit
owner direction under the project workflow. No later phase has begun.

No live Discord, Among Us capture, WSS, recovery or E2E behavior was tested.
No migration, image, capture ZIP or release was produced.
