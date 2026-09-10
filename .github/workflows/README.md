# Workflow rollout

The imported workflows remain unchanged under `bot/.github/workflows/` and
`capture/.github/workflows/`; GitHub does not execute workflows nested there.

Phase 2 will establish reproducible build/test commands and expose baseline
failures. Phase 17 will complete root PR/release CI and Dependabot. Required
checks are Go formatting/vet/tests/build, .NET restore/build/tests, Docker build,
secret scanning and appropriate dependency scanning. See
[the roadmap](../../docs/roadmap.md).

This placeholder is not a passing CI check. No failing baseline check may be
silenced, removed or marked successful without resolving its cause.
