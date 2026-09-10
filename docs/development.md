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
3. Run `python scripts/verify_bootstrap.py` while phase 1 imports remain pristine.
4. Review `git diff`, `git diff --cached` and `git status`.
5. Run `gitleaks git . --redact --config .gitleaks.toml` for committed changes.
   Also scan staged/new files before committing. Never print unredacted findings.
6. Commit logically, push only the intended branch, and open an issue-linked PR
   against main with purpose, validation, migrations and limitations.
7. Do not merge while required tests/checks fail. A draft PR can document a
   blocked baseline; it is not completion of the phase.

## Baseline tools

The imported bot declares Go 1.19. Baseline verification uses Go 1.19.13
without modifying its module files. Later modernization must select a supported
toolchain and update dependencies in controlled commits.

Capture targets `net5.0` / `net5.0-windows`. The initial workstation has
.NET SDK 8.0.424. A successful no-op `dotnet test` without test projects is
not test coverage. Reproducible baseline builds and actual capture tests belong
to phase 2; supported .NET LTS migration belongs to phase 11.

Use Python 3.11+ for the standard-library-only bootstrap verifier and Gitleaks
8.30.1 for the initial scan. Tool archives/caches/logs are local-only and must
not be committed. There is no root application linter or root CI yet.

## License and sync checks

`python scripts/verify_bootstrap.py` verifies pinned tree hashes, exact license
copies, directory scaffolding and formatting for new text files. It is specific
to the pristine phase 1 baseline. Later implementation PRs must deliberately
replace full-tree equality checks with relevant provenance/regression checks
and explain why; never leave a no-longer-valid check silently bypassed.

The Gitleaks exception matches one known upstream checksum value in
`GameVerifier.cs`, with both file-path and exact-value conditions. It does not
exclude arbitrary keys, the capture subtree or the file as a whole.
