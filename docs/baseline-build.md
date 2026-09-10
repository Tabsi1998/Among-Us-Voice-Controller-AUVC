# Build and test baseline (phase 2)

This is a buildable development baseline, not a standalone AUVC release.
The owner authorized advancing phase 2 over the open bootstrap on 2026-09-10.
Work is on `codex/002-baseline-build`, based on `codex/001-bootstrap`.
The owner performs the merge; no main merge is automated.

## What changed

- Mechanical C# whitespace formatting is isolated from functional changes.
  The earlier Go formatting findings came from Windows CRLF checkout conversion;
  the byte-preserving checkout and gofmt check now agree without changing Go code.
- AUOffsetHelper still belongs to the full solution. Its old 2020 initializer
  cannot use today's outfit/role model, so it exports its preserved historical
  shape independently and only with explicit `--legacy-sample`. It warns on
  stderr, keeps JSON on stdout and no longer waits for console input.
- No replacement memory addresses or speculative modern offset mappings were added.
  Capture memory algorithms and the bundled `Offsets.json` are unchanged apart
  from mechanical C# whitespace formatting.
- An actual test project executes 11 cases on .NET 8: historical golden output,
  explicit CLI behavior, both bundled 2024 records and malformed input.
  This is regression coverage, not live game or full capture coverage.
- SDK 8.0.424 is pinned in `capture/global.json`. Application targets remain
  .NET 5 until phase 11. The .NET 8 test host does not upgrade the application.
- NuGet lock files include all solution projects; CI uses locked restore.
- Root CI checks provenance/secrets, Go format/vet/race tests/build, Windows
  restore/format/full solution build/executed tests and a Docker baseline build.
- Dependabot covers Go, NuGet, Actions and the deployment Dockerfile.

## Local commands

From the repository root, with Python 3.11+ and Gitleaks 8.30.1:

```sh
python scripts/verify_repository.py
gitleaks git . --redact --config .gitleaks.toml --log-opts=HEAD
```

From `bot/`, using Go 1.27.1:

```sh
gofmt -l .
go vet ./...
go test ./...
go build ./...
```

An empty gofmt listing is success. Linux CI additionally uses `go test -race ./...`.
The Go toolchain and the dependencies AUVC keeps were modernized in phase 3; see
[go-modernization.md](go-modernization.md). The capture .NET SDK and its NuGet
dependencies deliberately remain baseline versions until phase 11.

From `capture/`, using SDK 8.0.424 (honored by global.json):

```sh
dotnet restore AmongUsCapture.sln --locked-mode
dotnet format whitespace AmongUsCapture.sln --verify-no-changes --no-restore
dotnet build AmongUsCapture.sln --configuration Release --no-restore
dotnet test AUVC.Capture.Tests/AUVC.Capture.Tests.csproj --configuration Release --no-build --no-restore --logger "trx;LogFileName=capture.trx" --results-directory ../artifacts/tests
```

CI checks that at least 11 tests executed and none failed; a no-op `dotnet test`
cannot turn this gate green. Test reports are retained as short-lived artifacts.
When intentionally updating a package, update/review the associated lock file
with an unlocked restore, then prove a locked restore succeeds.

The helper's historical CLI is `AUOffsetHelper --legacy-sample` when run with
its required runtime. No argument or unknown arguments exit 2; `--help` exits 0.
It is not an offset generator for current Among Us versions.

## Docker baseline

From the repository root:

```sh
docker build -f deploy/Dockerfile.baseline -t auvc:baseline .
```

This recipe builds the existing Go module without assuming a nested `.git`
directory or a local upstream release tag. It packages locales and license notices
and uses a non-root runtime user. It does not publish an image. The root Docker
context excludes local tooling, repository metadata and credentials.

The original `bot/Dockerfile` remains as an upstream reference. The new recipe
does not remove the application's existing Redis/PostgreSQL/Galactus dependencies
or establish final self-hosting. Docker execution is verified by Linux CI because
the workstation has no Docker daemon.

## Evidence and limitations

Local Windows validation: locked restore, .NET whitespace verification, full
Release solution build and 11/11 tests pass. Go formatting, vet, tests, Windows
build and Linux cross-build are checked locally. Actionlint validates the root
workflow; repository/link/license checks and redacted Gitleaks scans are required
before push. GitHub check results for the exact PR SHA are the final CI evidence.

The legacy solution still reports 105 build warnings on a full build, including
obsolete frameworks, upstream compiler warnings and package compatibility warnings.
CI exposes the NuGet vulnerability report (including Tmds.DBus 0.9.1); it does not
claim a vulnerability-free dependency tree. Dependency modernization remains in
the dedicated issues. No tests or warning settings were weakened to obtain success.

No current-version Among Us or live Discord session was tested. The WPF application
still targets .NET 5 and needs its corresponding runtime to run; building with SDK 8
does not make the existing app self-contained. New AUVC commands, SQLite, ghost
policy, secure WSS, recovery, installers and release automation remain planned.

## Provenance and merge procedure

`scripts/verify_bootstrap.py` remains the strict original phase-1-only verifier.
It correctly rejects current-tree changes after the import. CI now uses
`scripts/verify_repository.py`, which checks the immutable original import commits,
their tree IDs, ancestry and exact license bytes while allowing reviewed
implementation commits. This replaces an obsolete pristine-worktree invariant
with provenance checks plus actual application tests; it does not discard the
original check or ignore failing tests.

Use **Create a merge commit** when integrating the initial import/build PR.
Squashing or rebasing that import chain would discard the commit ancestry needed
for subtree synchronization and provenance verification. The phase-2 PR includes
the bootstrap ancestors so it can establish the complete baseline on main in one
normal merge. The earlier bootstrap PR remains available as the import review
record and should not be merged separately ahead of this tested baseline.
