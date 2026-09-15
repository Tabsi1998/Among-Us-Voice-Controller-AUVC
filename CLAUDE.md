# CLAUDE.md

Handoff for Claude Code sessions working on AUVC. It holds what the code and the
docs do not: how the owner works, the rules, how every change is verified, and
where things stand. Keep it current when that changes.

## Working with the owner

- **Language.** Answer in German. PR, commit and issue titles are English;
  issue bodies and comments may be German. Keep it simple and understandable.
- **Roles.** The owner merges. Deliver complete, verified PRs from branches named
  `codex/<nnn>-<topic>` (last number used: 083), then report CI.
- **Never do these yourself:** merge, push to `main`, rewrite or force-push `main`,
  create tags or releases. Tags and releases happen only when the owner says so.
- **Work from GitHub issues.** Every change belongs to an issue.
  - Prefer several mid-sized issues over one huge one.
  - PR bodies say `Closes #N`, which closes the issue on merge.
  - Labels: app, bot, release, tracking, testing, live test, upstream cleanup,
    needs owner, priority: next, accessibility, documentation.
  - Milestones: `v0.1.4-beta`, `v1.0.0`, `after v1.0.0`.
  - When a batch is done, ask which issues come next.
- **PR body.** A table or list of what changed, **Checked** with only results
  actually seen, and **Not checked**. It ends with
  `🤖 Generated with [Claude Code](https://claude.com/claude-code)`, and commits
  end with a `Co-Authored-By: Claude …` line.
- **Stacked branches.** A branch built on an unmerged one is pushed only after
  its base merges. Rebasing an unpublished branch is fine. If both change
  `CHANGELOG.md`, say "Merge after #N" in the PR.

## Rules that are never bent

- **Secrets never go to GitHub.** No real Discord tokens, capture credentials,
  pairing secrets, private keys, filled env files or databases. The repository
  is **public**: no diagnostics zips or tokens in issues either.
- **Other projects' secrets.** `.vscode/testing.json`, `testing.env` and
  `settings.json` hold env values and keys of other projects. Never print or
  commit them.
- **Push protection.** Secret scanning push protection is on. Never use its
  unblock URL.
  - **Test values:** build token-shaped test values at runtime (see
    `capture/AUVC.Capture.Tests/DiagnosticsTests.cs`).
  - **Before every push,** scan the commit for these patterns:
    `[A-Za-z0-9_-]{24,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{27,}` and
    `\b[0-9a-fA-F]{16}\.[A-Za-z2-7]{52,}\b`.
- **Gitleaks** runs with `--redact` and `--config .gitleaks.toml`.
  `scripts/local_check.py` does this.
- **Tests.** Never weaken or remove tests to make a build pass.
- **Among Us pictures.** Only originals, never generated or recoloured.
- **Distribution.** Betas are published on GitHub only. winget and the Microsoft
  Store come after the first stable release (#131). Free options are preferred.

## How every change is verified

1. **Full local check:** `python scripts/local_check.py`, 17 steps. It covers
   repository checks, Gitleaks, Go with the race detector, govulncheck, the C#
   format check, the release build, the C# tests and the self-contained publish.
   - `--only repository` runs a subset.
   - `--release --version vX.Y.Z` adds a release dry run.
2. **Mutation proof** for new tests. A small script applies each mutation,
   builds the break back in, and runs the tests. A test must fail; a build
   failure does not count. Every file is restored byte for byte. The PR reports
   "caught n/n".
3. **Secret scan** of the commit, with the patterns above.
4. **CI.** Push, wait about 20 s, then `gh pr checks <branch> --watch`. Check
   that `mergeStateStatus` is `CLEAN`.

**Toolchain on the original PC:** `C:\GIT Privat\.codex-tools\local-testing\`
holds `go\go\bin\go.exe`, `dotnet\dotnet.exe` and `bootstrap\Scripts\python.exe`.
On another machine you need Go 1.27, the .NET 10 SDK, Python 3, the `gh` CLI, and
Inno Setup 6 for the installer (`winget install JRSoftware.InnoSetup`).

**PowerShell pitfalls:**
- Write commit messages to a file and use `git commit -F <file>`.
- `Remove-Item` on paths with spaces was blocked by the harness.

## Architecture facts worth knowing

- **Layout.** `bot/` is the Go bot (discordgo, sqlite). `capture/` holds the .NET 10
  WPF app `AUCapture-WPF`, whose program file is `AUVC.exe`. The app uses
  `AUVC.Transport` (pure logic, tested), `AUVC.Protocol`, `AmongUsCapture`
  (memory reader) and `AUOffsetManager`.
- **Tests.** `AUVC.Capture.Tests` (xUnit) cannot reference WPF.
  - It compiles `AUCapture-WPF/AppLanguage.cs` directly.
  - It reads the resx files, XAML and `docs/` from the source tree by walking up
    from `AppContext.BaseDirectory`.
- **App texts.**
  - `Properties/Resources.resx` (neutral English), `Resources.en.resx`,
    `Resources.de.resx` and `Resources.Designer.cs` must list the same keys
    (`AppTextTests`).
  - Setup texts live in `SetupText.cs` as `T(german, english)`.
  - The guides' settings table and pairing steps are held to the resx texts
    (`GuideTextTests`).
  - Every button needs a screen-reader name (`XamlAccessibilityTests`).
- **Bot texts** live in `bot/pkg/text/english.go` and `german.go`. The crewmate
  message uses the server's language. The bot on this PC writes its checks in the
  app's language.
- **Bot inside the app.**
  - `BotHost` starts `bot\auvc.exe` in a job object with a random secret on a
    loopback port, so the bot ends when the app ends.
  - After a crash, the next start releases everyone held (`voice_hold`).
  - A remote bot is used through pairing (`/au capture pair`).
- **Revoke.** `/au capture revoke` ends connections with `unauthenticated`. The app
  replaces a refused local credential once per run
  (`MainWindow.OnLinkStatusChanged`). A second revoke leaves it refused. Whether
  "Bot neu starten" then helps is being checked in #157.
- **Kept for compatibility on purpose:**
  - the `aucapture://` scheme, re-registered on every start
  - the mutex `AmongUsCapture`
  - `%AppData%\AmongUsCapture` (settings, logs)
  - `%LOCALAPPDATA%\AUVC` (token, credential, database, bot logs)
- **Versions.** `-p:Version` stamps the version in `local_check.publish_command`
  and `release.yml`; the csproj default is `0.0.0-dev`. `ReleaseFeed`: a
  pre-release hears about newer pre-releases and releases, a release only about
  releases.
- **Upstream guard.** `scripts/check_upstream_references.py` checks against
  `scripts/upstream_references_baseline.txt`. Only `capture/.all-contributorsrc`
  is left (#44). Removing a reference means running `--update`.
- **Cosmetics.** The memory reader sets hat, pet and skin ids to 0, because Among
  Us now uses string ids. The display and SharpVectors were removed (#155).
  Showing cosmetics again would be a new feature that starts in the reader.
- **Network calls of the app** (documented in `docs/privacy.md`):
  - Discord, for the token check and the invite
  - the GitHub release list
  - the GitHub contributors list, when **Contributors** is opened

  The offset index loads nothing from the network by default (`IndexURL` is
  empty).

## Release flow

1. **Changelog PR.** Rename `## Unreleased` to `## vX.Y.Z-beta — YYYY-MM-DD` and
   add an intro, **Update.** and **Known limitations.** Keep an empty
   `## Unreleased` above it. The owner merges.
2. **Dry run.** On a clean, updated `main`, run
   `python scripts/local_release.py vX.Y.Z-beta`. It builds everything and
   publishes nothing.
3. **Publish.** Only on the owner's OK, run the same command with `--publish`. It
   tags, pushes and runs `gh release create --prerelease` with the installer,
   the zip and `SHA256SUMS`.
4. **Verify.** Check `gh release view vX.Y.Z-beta`, confirm the tag commit is
   `main`, and compare the downloaded `SHA256SUMS` with the built files.

A PR merged after the changelog PR but before publishing needs its changelog
entry moved into the version section first (see #156).

## Where things stand (2026-09-15)

- **Published.** `v0.1.4-beta` is out, tag at `d38e24e`: the name AUVC,
  diagnostics export, status line for bot problems, screen-reader names, and the
  cosmetics removal.
- **Merged since the beta:** #158 (guides fixed, `GuideTextTests`, closes #143).
- **Next: the owner's live test, [#157](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/157).**
  It gathers every open live test in one checklist. After it:
  1. Copy the results into #145, #141, #142, #144 and #136, and the keyboard,
     DPI and screen-reader results onto #135's checklist. #135 itself was closed
     by #153.
  2. Open one bug issue per deviation.
  3. Close what passed.
- **Open issues, milestone `v1.0.0`:** #18 (release), #21 (UI tracking), #44
  (upstream), #136, #141, #142, #144.
- **Open issues, milestone `after v1.0.0`:** #131.
- **Open issues without a milestone:** #146, #147, #148.
- **Proposed to the owner, not yet decided:**
  - an issue for `capture/.vs/` (Visual Studio's cache, tracked in git since the
    AmongUsCapture import)
  - a feature issue to show cosmetics again (string ids plus original pictures)
- **Not possible:** a "join lobby" button. Among Us has no official deep link,
  and Discord link buttons allow only http, https and discord.
