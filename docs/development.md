# Development

How to build, test and release AUVC. For using it, see the [user guide](guide.md).

## Tools

| Tool | Version | Used for |
| --- | --- | --- |
| Go | 1.27.1 | The bot in `bot/` |
| .NET SDK | 10.0.401, pinned in `capture/global.json` | The app in `capture/` |
| Python | 3.11 or newer | Repository checks and release notes in `scripts/` |
| Gitleaks | 8.30.1 | Secret scanning |
| Inno Setup | 6 | The installer, built by the release workflow |

## Build and test

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

To run the app with its bot from a local build, put the built `auvc.exe` and
`bot/locales` into a `bot` folder beside `AUCapture-WPF.exe`. The app looks for
`bot\auvc.exe` next to itself.

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
| `BOT_LANG`, `LOCALE_PATH` | English | Language of bot messages |
| `AUVC_LISTENING` | `/au` | The activity Discord shows |

`GET /healthz` on the capture port answers `ok`, or names what is wrong.

## Branches, commits and pull requests

- Never work on main. Use a `codex/<number>-<topic>` branch and one pull request
  per topic.
- Small Conventional Commits. Keep formatting and refactoring apart from
  behaviour changes.
- Never delete or weaken a test to make a build pass. Diagnose the failure.
- Before pushing: format, test, `verify_repository.py`, review the diff, and run
  Gitleaks with `--redact`.
- Merge only when every check is green. Never rewrite or force-push main.

## Releases

A release is the Windows app: `AmongUsVoiceCapture-Setup-win-x64.exe` and
`AmongUsVoiceCapture-win-x64.zip`, both with the bot inside, plus `SHA256SUMS` and
release notes.

1. In `CHANGELOG.md`, rename `## Unreleased` to the version, for example
   `## v0.1.1-beta — 2026-09-20`, and add a new empty `## Unreleased` above it.
   Merge that change.
2. Tag the merge commit on main and push the tag, for example `v0.1.1-beta`.
3. `.github/workflows/release.yml` checks the commit again, builds everything and
   publishes the release. Tags with `-alpha`, `-beta` or `-rc` become
   pre-releases.

Version numbers: `v0.x.y-beta` for pre-releases, `v1.0.0` for the first finished
release. **Actions → Release → Run workflow** is a dry run that builds everything
and publishes nothing.

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
