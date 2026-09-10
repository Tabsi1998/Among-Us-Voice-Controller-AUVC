# Workflow rollout

The imported workflows remain unchanged under `bot/.github/workflows/` and
`capture/.github/workflows/`; GitHub does not execute workflows nested there.

`baseline.yml` runs Go formatting/vet/race tests/build, Windows locked restore/
format/full solution build/11 executed regression tests, a Docker baseline build,
provenance verification and Gitleaks on every PR to main and main/codex pushes.
NuGet vulnerability findings are reported; the legacy dependency tree is not
represented as vulnerability-free. Dependabot covers Go, NuGet, Actions and Docker.
See [the baseline guide](../../docs/baseline-build.md) for local equivalents.

The [release trigger contract](../../docs/windows-installation-and-releases.md#automated-release-trigger-contract)
adds release-preparation PRs, tag-triggered alpha/beta/rc Pre-Releases and Stable
releases, EXE/MSI/ZIP packaging, signing, checksums and channel manifests.
Ordinary PR/main builds never publish user releases; all release gates must pass
before publication and feeds update last. Manual retries cannot bypass those gates.

Release/installer automation remains phase 17 work. No failing baseline test/check
may be silenced, removed or marked successful without resolving its cause.
