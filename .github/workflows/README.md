# Workflow rollout

The imported workflows remain unchanged under `bot/.github/workflows/` and
`capture/.github/workflows/`; GitHub does not execute workflows nested there.

`baseline.yml` runs on every pull request to main and every push to main or a
codex branch: Go formatting, vet, tests and a cross-compile for all three
shipped targets; Windows locked restore, format, full solution build and the
executed regression tests; the Docker image build and its runtime contract;
provenance verification and Gitleaks. Vulnerable NuGet packages fail the
build rather than being reported. Dependabot covers Go, NuGet, Actions and
Docker. See [the baseline guide](../../docs/baseline-build.md) for local
equivalents.

The [release trigger contract](../../docs/windows-installation-and-releases.md#automated-release-trigger-contract)
adds release-preparation PRs, tag-triggered alpha/beta/rc Pre-Releases and Stable
releases, EXE/MSI/ZIP packaging, signing, checksums and channel manifests.
Ordinary PR/main builds never publish user releases; all release gates must pass
before publication and feeds update last. Manual retries cannot bypass those gates.

`release.yml` runs only on a `v*` tag, or on demand as a dry run that builds
every artifact and publishes none. It re-runs the gates against the exact
commit being released rather than trusting the pull request that led there,
then builds the bot for Windows x64 and Linux amd64/arm64, the self-contained
capture payload, and the container image. A pre-release tag never becomes
`:latest`. Release notes come from `CHANGELOG.md` through
`scripts/release_notes.py`, which fails when the version has no section:
writing the notes is part of preparing a release, and a silent fallback is how
that step gets skipped forever.

The Windows installer is not built yet. No failing baseline test or check may
be silenced, removed or marked successful without resolving its cause.
