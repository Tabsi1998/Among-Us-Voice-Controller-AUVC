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

Capture targets `net5.0` / `net5.0-windows`; SDK 8.0.424 is pinned for baseline
builds and the new .NET 8 test host. Run the full solution and the actual
AUVC.Capture.Tests project as described in [baseline-build.md](baseline-build.md).
Application migration to supported .NET LTS belongs to phase 11.

Use Python 3.11+ for the standard-library-only bootstrap verifier and Gitleaks
8.30.1 for the initial scan. Tool archives/caches/logs are local-only and must
not be committed. Root CI checks Go vet/format/tests/build, Windows restore/format/
build/tests, Docker, provenance and secrets. Legacy dependency warnings are reported.

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
