# Workflows

The workflows imported with upstream remain under `bot/.github/workflows/` and
`capture/.github/workflows/`. GitHub does not run workflows nested there.

## baseline.yml

Runs once per change: on every pull request, whatever branch it targets, and on
every push to main. A branch push without a pull request starts nothing, so a
push to a pull request no longer runs everything twice. Before pushing, run the
same checks locally with `python scripts/local_check.py`.

- **Provenance and secrets:** `verify_repository.py`, `check_upstream_references.py`,
  the tests of the repository scripts and a Gitleaks scan of the history.
- **Go:** formatting, vet, tests with the race detector, a build, a Windows
  cross-compile and `govulncheck`.
- **Windows:** locked NuGet restore, formatting, the full solution build, the
  tests (at least 99 must run), a self-contained publish, and a failure on any
  vulnerable NuGet package.

No failing check may be silenced, removed or marked successful without fixing its
cause.

## release.yml

Runs on a `v*` tag, or on demand as a dry run that builds everything and publishes
nothing.

1. Checks the exact commit again: Go checks and the repository checks.
2. Builds the app as a self-contained Windows payload, builds the bot into it,
   and packages the portable zip and the installer.
3. On a tag, publishes a GitHub release with `SHA256SUMS` and the notes for that
   version from `CHANGELOG.md`. Tags with `-alpha`, `-beta` or `-rc` become
   pre-releases.

`scripts/release_notes.py` fails when the changelog has no section for the
version: writing the notes is part of preparing a release. See
[development.md](../../docs/development.md#releases).
