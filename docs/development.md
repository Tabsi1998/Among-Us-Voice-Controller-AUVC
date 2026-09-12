# Development workflow

## Workspace and branches

Clone https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC.git into a
directory named `amongus-voice-controller`. Restore optional upstream remotes
using [UPSTREAM.md](../UPSTREAM.md).

Never develop directly on main. Use `codex/<number>-<short-topic>`, one phase
per branch and PR. Before each phase inspect current code, identify affected
files and risks, and select meaningful tests. Finish the current phase before
starting another unless the owner explicitly changes that ordering.

For this initially empty repository, the root seed commit was created on
`codex/001-bootstrap`. Main points only to that shared seed; the import and
scaffolding commits belong to the feature branch and its PR. Do not rewrite
or force-push main.

## Commit and review discipline

Use small Conventional Commits. Keep imports, formatting and refactors separate.
Preserve original notices, tests and the ability to compare memory/offset code
with upstream. Diagnose failures before changes; never delete/weaken tests for
a green build.

Before every push:

1. Format new/changed AUVC files. For pristine imports, report existing formatting
   differences without rewriting the imported trees.
2. Run the relevant linter, tests and build; record precise failures and unavailable
   tooling. See [bootstrap validation](bootstrap-validation.md).
3. Run `python scripts/verify_repository.py` for provenance, licensing and documents.
4. Review `git diff`, `git diff --cached` and `git status`.
5. Run `gitleaks git . --redact --config .gitleaks.toml` for committed changes.
   Also scan staged/new files before committing. Never print unredacted findings.
6. Commit logically, push only the intended branch, and open an issue-linked PR
   against main with purpose, validation, migrations and limitations.
7. Do not merge while required tests/checks fail. A draft PR can document a
   blocked baseline; it is not completion of the phase.

## Baseline tools

The bot module declares Go 1.27.0 and is verified with Go 1.27.1, a supported
release. See [go-modernization.md](go-modernization.md) for the selected
toolchain, the dependencies that were updated and the ones deliberately left at
baseline versions because their code is scheduled for removal.

Capture targets `net10.0` / `net10.0-windows`; SDK 10.0.401 is pinned in
`capture/global.json` with `rollForward: disable`, and CI installs exactly that
SDK from the same file. .NET 10 is the current LTS, supported until November
2028; .NET 8 was not chosen because its support ends in November 2026. Run the
full solution and the AUVC.Capture.Tests project as described in
[baseline-build.md](baseline-build.md).

Nullable reference types are enabled where the code is already clean under them
and switched off deliberately elsewhere, with the reason recorded in each
project file. The backlog is measurable rather than implied:

| Project | Nullable | Warnings if enabled today |
| --- | --- | --- |
| AUVC.Capture.Tests | enabled | 0 |
| AUOffsetHelper | enabled | 0 |
| AUOffsetManager | disabled | 56 |
| AmongUsCapture | disabled | 168 |
| AUCapture-WPF | disabled | 556 |

A full rebuild reports around 128 compiler warnings, none of them errors. The
largest groups are `CS8632` (nullable annotations in code compiled without the
nullable context), `CS0168`/`CS0169` (unused locals and fields), `SYSLIB0021`
and `SYSLIB0014` (APIs the newer framework marks obsolete) and `CA1416`
(Windows-only APIs). They are reported rather than suppressed, and CI does not
gate on them; the transport rewrite in phases 12 and 13 removes a large part of
the code they come from. `NU1701` covers `WebSocketSharp`, a .NET Framework
package restored through the compatibility fallback, which the same rewrite
replaces.

Reproduce a column with
`dotnet build <project> -c Release -t:Rebuild -p:Nullable=enable`. Annotating
the memory and offset code is held back on purpose: the phase 11 contract keeps
it comparable with upstream, and phases 12 and 13 replace the legacy transport
that makes up much of the rest.

The container image sets `AUVC_CAPTURE_ADDR` itself, because the same listener
carries `/healthz` and a container nobody can check is a container nobody
notices has died. Publishing the port stays the operator's decision; see
[deploy/README.md](../deploy/README.md).

The direct capture connection is off unless `AUVC_CAPTURE_ADDR` names a listen
address such as `:8123`. `AUVC_CAPTURE_TLS_CERT` and `AUVC_CAPTURE_TLS_KEY` make
it terminate TLS itself; without them it serves plain HTTP and warns on every
start, because a plain listener is only safe behind a reverse proxy that
terminates TLS for it. See [protocol/README.md](../protocol/README.md).

The bot reads AUVC configuration from `AUVC_DATABASE_PATH`. Containers default
to `/data/amongus.db`; for a local development run set it to a writable path such
as `./data/amongus.db`. Never commit the database or use it for secrets.

Use Python 3.11+ for the standard-library-only bootstrap verifier and Gitleaks
8.30.1 for the initial scan. Tool archives/caches/logs are local-only and must
not be committed. Root CI checks Go vet/format/tests/build, Windows restore/format/
build/tests, Docker, provenance and secrets. The Windows job fails on vulnerable
NuGet packages and proves that the win-x64 payload is self-contained, so it
still runs on a machine with no .NET installed.

## Dependency updates

Dependabot is configured in `.github/dependabot.yml` for Go modules, NuGet,
GitHub Actions and Docker. Minor and patch updates are grouped so a quiet week
produces one pull request per ecosystem rather than one per package.

Nothing in the Go module is ignored any more. The Redis and PostgreSQL
packages used to be, because they were reachable only from layers with a
scheduled deletion date; phase 14 deleted those layers instead, and the bot is
down to six direct dependencies.

One set of packages is still ignored:

- **Major bumps of `Discord.Net` and `Config.Net`**, and **major or minor bumps
  of `HandyControl`**, need code changes that belong to the .NET LTS migration in
  phase 11. Until then such a bump only produces a red pull request. HandyControl
  is treated more strictly because it broke on a *minor*: 3.0.0 to 3.5.1 changes
  the `ResourceHelper.GetTheme` signature. Patches still come through.

Remove the corresponding `ignore` entries in those phases, together with the code.

### Go vulnerability scanning

`scripts/check_go_vulnerabilities.py` runs `govulncheck` in CI and fails when the
bot calls into an advisory that has not been assessed. Run it locally with:

```sh
go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
python scripts/check_go_vulnerabilities.py
```

Two advisories are currently accepted, both recorded in the script with the
reason and the phase that removes them: `GO-2026-5004` and `GO-2026-4518`, in
`pgx/v4` and `pgproto3/v2`. Neither has a fix in the major line the upstream code
uses, so clearing them means migrating to `pgx/v5` — a breaking change to code
phase 14 deletes. They are listed so the decision is visible instead of being
implied by a silent scan.

The list may only shrink. An accepted entry that no longer applies fails the
check too, which keeps it describing reality.

Note that `jackc` packages are **not** blanket-ignored in Dependabot despite
being scheduled for removal: the advisories above are reachable, so security
fixes inside the current major line have to keep arriving. Only majors are held
back.

### NuGet updates and the lock files

The lock files are platform independent, which they were not before
`AUCapture-Console` was removed. That project depended on `Mono.Posix`, which
made NuGet record a runtime-identifier target in its lock file — and the
identifier is whatever host ran the restore. Dependabot restores on Linux and
CI restores on Windows, so every NuGet bump failed the Windows job with
`NU1004` no matter what the bump was.

CI restores with `dotnet restore --locked-mode`, so `packages.lock.json` must
match the project files exactly. Dependabot updates the `.csproj` of the package
it bumps and that project's lock file, but **not** the lock files of projects
that reference it. `AUCapture-WPF` references `AmongUsCapture`, so a bump
there fails the Windows job with:

```
error NU1004: The project references amonguscapture whose dependencies has changed.
```

This is not a broken update. Regenerate the lock files and commit them onto the
Dependabot branch:

```sh
cd capture
dotnet restore AmongUsCapture.sln --force-evaluate
git add **/packages.lock.json
```

Then verify the way CI does, from `capture/`:

```sh
dotnet restore AmongUsCapture.sln --locked-mode
dotnet format whitespace AmongUsCapture.sln --verify-no-changes --no-restore
dotnet build AmongUsCapture.sln --configuration Release --no-restore
dotnet test AUVC.Capture.Tests/AUVC.Capture.Tests.csproj --configuration Release --no-build --no-restore
```

## License and sync checks

`python scripts/verify_repository.py` verifies the preserved import commits and
tree IDs, exact license copies, current structure, text format and document links.
The original `verify_bootstrap.py` remains available for the pristine phase-1
checkout. It intentionally fails on later implementation changes. Current CI
uses provenance plus application regression checks, as documented in the baseline
guide. Use **Create a merge commit**, not squash/rebase, for the initial import
chain so its provenance and subtree history remain reachable.

The Gitleaks exception matches one known upstream checksum value in
`GameVerifier.cs`, with both file-path and exact-value conditions. It does not
exclude arbitrary keys, the capture subtree or the file as a whole.
