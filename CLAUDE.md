# CLAUDE.md

Handoff for Claude Code sessions working on AUVC. It holds what the code and the
docs do not: how the owner works, the rules, how every change is verified, and
where things stand. Keep it current when that changes.

## Working with the owner

- **Language.** Answer in German. PR, commit and issue titles are English;
  issue bodies and comments may be German. Keep it simple and understandable.
- **Roles.** The owner merges. Deliver complete, verified PRs from branches named
  `codex/<nnn>-<topic>` (last number used: 087), then report CI.
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

**Toolchain on the second PC** (`C:\Programmieren`, since 2026-09-15): Go 1.27.1
and the llvm-mingw clang for `go test -race` live in `~/.local-toolchain` and are
on the user PATH. The .NET SDK 10.0.401 and Docker Desktop are installed
system-wide; gitleaks, `gh` and Inno Setup 6 (`%LOCALAPPDATA%\Programs\Inno Setup 6`,
found by `local_check.inno_setup`) come from winget for the user. No
`.vscode/testing.json` exists there, so the tools come from PATH. A terminal or
VS Code started before a PATH change does not see new tools until it restarts.
Machine-local, not in Git:
- `.ci-panel/test_checks.py` shows every step of `scripts/local_check.py` in the
  VS Code Testing panel (hidden through `.git/info/exclude`).
- `C:\Programmieren\check-all.py --serve` is a live dashboard over all five
  repositories; `C:\Programmieren\Programmieren.code-workspace` opens them.

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

## Where things stand (2026-09-27)

- **Published.** `v0.1.5-beta` is out, tag at `5282e03` (the merge of #162): the
  choice to keep the dead muted in the main channel (#161), the voice log (#160)
  and the fixed guides (#143). `main` is at `d0e5957` (#164, local setup notes,
  closes #163) and has not moved since 2026-09-15. CI on `main` is green.
- **#160 was closed by mistake and is open again.** The body of #162 said "it
  does not close #160"; GitHub read "close #160" in it as a closing keyword and
  closed the issue on merge.
  - **Rule from this:** in a PR body, "close", "fix" or "resolve" followed by an
    issue number closes that issue, even inside "does not close". Write only
    `Refs #N` for an issue the PR must leave open.
- **#160, console players get dropped.** The owner reported on #157 that with
  v0.1.5-beta PlayStation players are still dropped, less often than before.
  - **Symptom:** an unlinked PlayStation player who joins the main channel in
    the lobby while AUVC runs is removed from voice. Players who were in the
    channel before AUVC started stay. PC players are fine.
  - **Code reading found no cause.** The bot's only write to members is
    `GuildMemberEdit` in `bot/bot/voice_adapter.go`. It acts only on linked
    players (`bot/pkg/voice`) or on holds left by a crash (`voice_hold.go`),
    never sends `channel_id: null`, sets no permissions and joins no channel.
  - **Known Discord limitation:** Discord disconnects console voice users whom a
    bot or moderator moves ([Discord community report](https://support.discord.com/hc/en-us/community/posts/12286453822231-Discord-Voice-Channels-Disconnect-on-Xbox-When-User-Is-Moved)).
  - **Since v0.1.5-beta** the bot log names every voice change AUVC makes and
    every join, leave and switch in its channels (`bot/bot/voice_changes.go`).
  - **Next step:** read `%LOCALAPPDATA%\AUVC\logs\logs.txt` around a real drop.
    A change logged by AUVC for that member just before means that path gets
    fixed. Nothing logged means Discord dropped the player on its own.
- **The live tests became issues on 2026-09-27**, one per point the owner
  reported on #157 and #145:
  - **#168** the crewmate message keeps the first map when the host picks
    another one in the lobby. Cause: the memory reader read code and map only
    when the state changed into the lobby.
  - **#169** wait about 3 s before muting after a meeting and before unmuting at
    round end. `bot/pkg/game/delay.go` still holds AutoMuteUs' unused waits.
  - **#170** muting everyone at a phase change takes too long. The reconciler
    sends one `GuildMemberEdit` per player, one after another.
  - **#171** mute only the microphone of the living during tasks, instead of
    muting and deafening (`bot/pkg/voice/policy.go`). Proposed as a setting that
    is off by default, because deafening also hides unlinked speakers.
  - **#172** the settings flyout is cut off and cramped; it is 275 px wide.
  - **#173** move **Bot** and pairing out of the title bar into the settings.
    Pairing stays reachable for a bot on another PC.
  - **#174** this file, kept current.
- **Live tests.** #157 gathers every open live test for v0.1.5-beta and is in
  milestone `v0.1.4-beta`, like #145. After it:
  1. Copy the results into #145, #141, #142, #144 and #136, and the keyboard,
     DPI and screen-reader results onto #135's checklist.
  2. Open one bug issue per deviation.
  3. Close what passed.
- **Dependabot** keeps three open PRs: #165 (Microsoft.NET.Test.Sdk 18.10.1) and
  #167 (modernc.org/sqlite 1.59.0) are green; #166 (NLog 6.2.1) fails the
  Windows job because only one project's lock file was updated, so the locked
  restore refuses. It needs `dotnet restore --force-evaluate` and a check that
  NLog 6 still builds.
- **Open issues, milestone `v0.1.4-beta`:** #145, #157.
- **Open issues, milestone `v1.0.0`:** #18 (release), #21 (UI tracking), #44
  (upstream), #136, #141, #142, #144, #168, #169, #170, #171, #172, #173.
- **Open issues, milestone `after v1.0.0`:** #131.
- **Open issues without a milestone:** #146, #147, #148, #160, #174.
- **Proposed to the owner, not yet decided:**
  - an issue for `capture/.vs/` (Visual Studio's cache, 4 files tracked in git
    since the AmongUsCapture import)
  - a feature issue to show cosmetics again (string ids plus original pictures)
  - deleting the remote branches `codex/061` to `codex/085`, which are all
    merged into `main`
- **Not possible:** a "join lobby" button. Among Us has no official deep link,
  and Discord link buttons allow only http, https and discord.
