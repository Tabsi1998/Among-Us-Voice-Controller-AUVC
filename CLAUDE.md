# CLAUDE.md

Build, test and code rules for AUVC. What the app does and how it is run is in
`README.md` and `docs/`; building and releasing in detail is in
`docs/development.md`. Answer the owner (Tabsi1998, Fabian) in German. Commit,
PR and issue titles are English, bodies may be German. A PR body says
`Closes #N` — and "close", "fix" or "resolve" before an issue number closes that
issue even inside a sentence saying it does not, so write `Refs #N` for an issue
that must stay open.

## The local check is the gate

```bash
python scripts/local_check.py                      # all 17 steps, about 40 s
python scripts/local_check.py --only repository    # one group
python scripts/local_check.py --list               # the steps, without running them
python scripts/local_check.py --release --version vX.Y.Z-beta
```

The report lands in `.local-testing/local-check.json` (ignored by Git) and ends
with the Go coverage per package.

| Group | Runs |
| --- | --- |
| repository | provenance, licences, document links, no new AutoMuteUs references, the repository scripts' own tests, Gitleaks over the history and over uncommitted files |
| bot | `gofmt`, `go vet`, Go tests with the race detector, `go build`, the bot as the app ships it, known Go vulnerabilities |
| capture | locked NuGet restore, C# whitespace format, release build, the C# tests (at least 99 must run), the self-contained publish, vulnerable packages |

Two more things before every push:

1. **Mutation proof** for new tests. Apply one break per test, run the tests,
   restore every file byte for byte. A test must *fail*; a build failure does
   not count. The PR reports "caught n/n".
2. **Secret scan** of the commit for
   `[A-Za-z0-9_-]{24,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{27,}` and
   `\b[0-9a-fA-F]{16}\.[A-Za-z2-7]{52,}\b`.

Tools: Go 1.27, the .NET 10 SDK, Python 3.11+, the `gh` CLI, Gitleaks, and Inno
Setup 6 for the installer (`winget install JRSoftware.InnoSetup`). A terminal or
editor started before a PATH change does not see new tools until it restarts.

## Rules that are never bent

- **Never merge, push to `main`, force-push, or create a tag or release.** The
  owner merges; tags and releases happen when he says so.
- **Secrets never go to GitHub.** No real Discord tokens, capture credentials,
  pairing secrets, private keys, filled env files or databases. The repository
  is **public**, so no diagnostics zips or tokens in issues either. Build
  token-shaped test values at runtime (see
  `capture/AUVC.Capture.Tests/DiagnosticsTests.cs`). Secret scanning push
  protection is on; never use its unblock URL.
- **Never weaken or remove a test** to make a build pass.
- **Among Us pictures:** originals only, never generated or recoloured.
- Files outside `bot/`, `capture/` and `LICENSES/` need LF endings, a final
  newline and no trailing whitespace, and their local links have to resolve
  (`scripts/verify_repository.py`).
- `scripts/check_upstream_references.py` holds the repository to
  `scripts/upstream_references_baseline.txt`. Removing a reference means running
  it with `--update`.

## Tests and texts

- `AUVC.Capture.Tests` (xUnit) cannot reference WPF. It compiles
  `AUCapture-WPF/AppLanguage.cs` directly and reads the resx files, the XAML and
  `docs/` out of the source tree by walking up from `AppContext.BaseDirectory`.
- `Properties/Resources.resx` (neutral English), `Resources.en.resx`,
  `Resources.de.resx` and `Resources.Designer.cs` must list the same keys
  (`AppTextTests`).
- Setup texts live in `SetupText.cs` as `T(german, english)`.
- The guides' settings table and pairing steps are held to the resx texts
  (`GuideTextTests`), so `docs/guide.md` and `docs/anleitung.md` change with the
  app.
- Every button needs a screen-reader name (`XamlAccessibilityTests`).
- Bot texts live in `bot/pkg/text/english.go` and `german.go`. What one person
  sees comes in their Discord language, what everybody reads in the server's.
- `Recorded/` in the test project replays recorded game memory, so the reader
  can be tested without a running game. A recording has to answer with whole
  structs, not single fields.

## Architecture facts worth knowing

- `bot/` is the Go bot (discordgo, SQLite). `capture/` holds the .NET 10 WPF app
  `AUCapture-WPF`, whose program file is `AUVC.exe`, with `AUVC.Transport` (pure
  logic, fully tested), `AUVC.Protocol`, `AmongUsCapture` (the memory reader)
  and `AUOffsetManager`.
- `bot/pkg/voice` is a pure mapping from game state to voice state. The
  reconciler sends only the differences, moves before mutes, so a kill is never
  revealed by a channel change before the meeting announces it.
- `BotHost` starts `bot\auvc.exe` in a job object with a random secret on a
  loopback port, so the bot ends when the app ends. After a crash the next start
  releases everyone left held (`voice_hold`). A bot on another PC is reached
  through pairing (`/au capture pair`).
- `/au capture revoke` ends connections with `unauthenticated`. The app replaces
  a refused local credential once per run (`MainWindow.OnLinkStatusChanged`).
- **Kept for compatibility on purpose:** the `aucapture://` scheme, the mutex
  `AmongUsCapture`, `%AppData%\AmongUsCapture` (settings, logs) and
  `%LOCALAPPDATA%\AUVC` (token, credential, database, bot logs).
- Among Us stores four-byte pointers, so struct pointers are read through
  `ReadPointer(..., is64Bit)` and never with `Marshal.ReadIntPtr`.
- `-p:Version` stamps the version in `local_check.publish_command` and
  `release.yml`; the csproj default is `0.0.0-dev`. `ReleaseFeed`: a pre-release
  hears about newer pre-releases and releases, a release only about releases.
- The memory reader sets hat, pet and skin ids to 0, because Among Us now uses
  string ids (#155).
- The app reaches the network only for Discord's token check and invite, the
  GitHub release list, and the GitHub contributors list when **Contributors** is
  opened. The offset index loads nothing by default (`IndexURL` is empty). All
  of it is documented in `docs/privacy.md`.
