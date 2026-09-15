# Development

How to build, test and release AUVC. For using it, see the [user guide](guide.md).

## Tools

| Tool | Version | Used for |
| --- | --- | --- |
| Go | 1.27.1 | The bot in `bot/` |
| .NET SDK | 10.0.401, pinned in `capture/global.json` | The app in `capture/` |
| Python | 3.11 or newer | The local check, repository checks and release notes in `scripts/` |
| A C compiler | gcc, or clang from llvm-mingw on Windows | `go test -race` |
| Gitleaks | 8.30.1, downloaded and checksum-verified by the local check | Secret scanning |
| Inno Setup | 6 | The installer, built by the release workflow |

## Build and test

One command runs every check the CI runs, on your own computer:

```sh
python scripts/local_check.py
```

| Group | Checks |
| --- | --- |
| `repository` | `verify_repository.py`, `check_upstream_references.py`, the tests of the scripts, Gitleaks over the history and over uncommitted and new files |
| `bot` | `gofmt`, `go vet`, `go test -race`, `go build`, the Windows build of the bot, `govulncheck` |
| `capture` | Locked restore, whitespace format check, Release build, the tests (at least 99 must run), the self-contained publish, vulnerable NuGet packages. Windows only |
| `release` | Only with `--release`: release notes, the app with the bot inside, the portable zip, `SHA256SUMS`, and the installer when Inno Setup 6 is installed. Nothing is published |

`--only bot,capture` runs some groups and `--list` shows every step. Results go
to `.local-testing/`, which Git ignores: `local-check.json`, `local-check.log`,
`go-events.jsonl` and `dotnet/local.trx`.

- Tools come from `PATH`. With the VS Code test setup on the PC, its pinned Go,
  .NET SDK and clang are used first.
- Gitleaks 8.30.1 is downloaded on first use and checked against its published
  SHA-256; govulncheck v1.8.0 is installed with `go install`. Both go to
  `.local-testing/tools/`.
- Environment variables whose names look like credentials, such as tokens,
  passwords, keys, and database or mail settings, are withheld from every step.
  Only their names are printed.
- A publish for `win-x64` adds that runtime to `packages.lock.json`; the check
  puts the lock files back afterwards.

Run it before every push. The CI then confirms the same checks on clean
machines.

### The same checks by hand

The bot, from `bot/`:

```sh
gofmt -l .
go vet ./...
go test -race ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o auvc.exe .
```

The app, from `capture/`, exactly as CI runs it:

```sh
dotnet restore AmongUsCapture.sln --locked-mode
dotnet format whitespace AmongUsCapture.sln --verify-no-changes --no-restore
dotnet build AmongUsCapture.sln --configuration Release --no-restore
dotnet test AUVC.Capture.Tests/AUVC.Capture.Tests.csproj --configuration Release --no-build --no-restore
```

The repository, from the root:

```sh
python scripts/verify_repository.py
python scripts/check_upstream_references.py
gitleaks detect --redact --no-git --config .gitleaks.toml
```

To run the app with its bot from a local build, put the built `auvc.exe` into a
`bot` folder beside `AUVC.exe`. The app looks for `bot\auvc.exe` next to
itself. The bot's texts are compiled into it (`bot/pkg/text`), so nothing else
goes with it.

## Running the bot on its own

The app starts the bot itself. For a bot on another computer, build `auvc.exe`
(or a Linux binary with `GOOS=linux`) and start it with at least
`DISCORD_BOT_TOKEN` set. Then pair the app with `/au capture pair`, as described
in the user guide.

| Variable | Default | Meaning |
| --- | --- | --- |
| `DISCORD_BOT_TOKEN` | — | Required. Treat it like a password. |
| `AUVC_DATABASE_PATH` | `%LOCALAPPDATA%\AUVC\amongus.db` on Windows | The SQLite file |
| `AUVC_CAPTURE_ADDR` | `127.0.0.1:8123` | Where the app connects; `off` disables it |
| `AUVC_CAPTURE_TLS_CERT`, `AUVC_CAPTURE_TLS_KEY` | — | Terminate TLS in the bot. Without them, put a TLS proxy in front whenever the app is not on the same machine |
| `SLASH_COMMAND_GUILD_IDS` | global | Register `/au` in named servers only, which takes effect at once. `*` registers it in every server the bot is in |
| `AUVC_LOCAL_CONTROL_SECRET` | — | Set by the app when it starts the bot. Leave it unset otherwise |
| `LOG_PATH`, `DISABLE_LOG_FILE` | `./`, off | Where `logs.txt` goes, or no file at all |
| `AUVC_LISTENING` | `/au` | The activity Discord shows |

`GET /healthz` on the capture port answers `ok`, or names what is wrong.

## Branches, commits and pull requests

- Never work on main. Use a `codex/<number>-<topic>` branch and one pull request
  per topic.
- Small Conventional Commits. Keep formatting and refactoring apart from
  behaviour changes.
- Never delete or weaken a test to make a build pass. Diagnose the failure.
- Before pushing: run `python scripts/local_check.py` and review the diff.
- Merge only when every check is green. Never rewrite or force-push main.

## Releases

A release is the Windows app: `AmongUsVoiceCapture-Setup-win-x64.exe` and
`AmongUsVoiceCapture-win-x64.zip`, both with the bot inside, plus `SHA256SUMS` and
release notes.

1. In `CHANGELOG.md`, rename `## Unreleased` to the version, for example
   `## v1.2.3-beta — 2026-09-20`, and add a new empty `## Unreleased` above it.
   Merge that change, switch to `main` and pull.
2. Publish it. Who builds it depends on the kind of version:

| Version | Example | Built and published by |
| --- | --- | --- |
| Pre-release | `v1.2.3-beta`, `v1.2.3-alpha`, `v1.2.3-rc.1` | `scripts/local_release.py` on your PC |
| Release | `v1.2.3` | `.github/workflows/release.yml` on GitHub, started by pushing the tag |

### Pre-releases

```sh
python scripts/local_release.py v1.2.3-beta             # builds and checks, publishes nothing
python scripts/local_release.py v1.2.3-beta --publish   # also tags, pushes the tag and uploads
```

The script refuses unless the working folder is clean, the commit is on
`origin/main`, `CHANGELOG.md` has a section for the version, and neither the
release nor the tag on another commit exists yet. It then:

- builds from a fresh worktree of the commit, not from the working folder, so
  nothing uncommitted or ignored can end up in a download;
- runs every check of `local_check.py` there, with the release dry run;
- with `--publish`, needs Inno Setup 6 for the installer
  (`winget install JRSoftware.InnoSetup`), creates and pushes the tag, and
  uploads only the installer, the zip and `SHA256SUMS`, marked as a pre-release,
  with the notes from `CHANGELOG.md`.

If the upload fails after the tag was pushed, the same command finishes it. The
files stay in `.local-testing/release-<version>/`.

### Releases

Tag the merge commit on `main` and push the tag, for example `v1.2.3`.
`release.yml` checks the commit again on a clean runner, builds everything and
publishes the release. Pre-release tags do not start it. **Actions → Release →
Run workflow** is a dry run that builds everything on GitHub and publishes
nothing.

Version numbers follow Semantic Versioning: a pre-release carries `-alpha`,
`-beta` or `-rc` and comes before the release of the same number, so
`v1.2.3-beta` comes before `v1.2.3`.

## Dependencies

Dependabot updates Go modules, NuGet packages and GitHub Actions.

CI restores NuGet packages with `--locked-mode`, so `packages.lock.json` has to
match the project files. Dependabot updates the lock file of the project it
bumps, but not of the projects that reference it, which fails the Windows job
with `NU1004`. Regenerate the lock files on the Dependabot branch:

```sh
cd capture
dotnet restore AmongUsCapture.sln --force-evaluate
```

Do not run a `dotnet publish --runtime` before that: it adds a runtime target to
the lock files.

`scripts/check_go_vulnerabilities.py` runs `govulncheck` and fails when the bot
reaches an advisory that has not been assessed.

## Provenance

`scripts/verify_repository.py` checks that the original upstream imports are
still in the history with their exact trees and licence files, that the required
files exist, that text files use LF, and that local Markdown links resolve.
`scripts/check_upstream_references.py` fails on any new reference to AutoMuteUs
infrastructure; the known ones are listed in
`scripts/upstream_references_baseline.txt` and tracked in
[#44](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/44).
[UPSTREAM.md](../UPSTREAM.md) describes how upstream changes are reviewed and
ported.

The Gitleaks exception matches one known upstream checksum in `GameVerifier.cs`,
by file and exact value. It does not exclude anything else.
