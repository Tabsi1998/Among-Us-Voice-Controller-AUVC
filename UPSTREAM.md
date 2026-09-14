# Upstream provenance and synchronization

AUVC is an independent community rework, not an official AutoMuteUs release.

| Component | Upstream repository | Default branch at import | Last imported commit | Import date | Local path |
| --- | --- | --- | --- | --- | --- |
| AutoMuteUs | https://github.com/automuteus/automuteus | master | `ad9bfe0d2ab7a570c53e409fc33a98f33a42ff7f` | 2026-09-10 | `bot/` |
| AmongUsCapture | https://github.com/automuteus/amonguscapture | master | `c4877fe07b5374279b4669c21235b9095fac67bd` | 2026-09-10 | `capture/` |

Both branches were discovered using `git ls-remote --symref <remote> HEAD`
and `git remote set-head <remote> --auto`, not assumed.

## Import method

Each component was imported unchanged using `git subtree add --prefix=<path>
--squash <SHA> -m <message>` on `codex/001-bootstrap`. Git subtree creates
a synthetic squash commit and an integration commit for each import. Full upstream
history is not part of AUVC's reachable history.

Integration commits:

- AutoMuteUs: `772c0872d252081a19b702dad7c4a9a673432e82`.
- AmongUsCapture: `197c2a9a4242351ba13279d199685802c5506bca`.

Original tree identities:

- `bot/`: `d9ae52ffc0e804e0211154ace579425fc5ef9f4f`.
- `capture/`: `d05d7cd8f300b4368d3872337d2a9f5059c76ee6`.

The imports retain original licenses, workflow files, backup project files and
tracked IDE artifacts. Nested workflows are historical source files; GitHub
does not run them as root workflows. Do not silently discard them during sync.

## Remotes after a fresh clone

Git remotes other than origin are local metadata and are not cloned automatically:

```sh
git remote add upstream-bot https://github.com/automuteus/automuteus.git
git remote add upstream-capture https://github.com/automuteus/amonguscapture.git
git fetch --no-tags upstream-bot
git fetch --no-tags upstream-capture
git remote set-head upstream-bot --auto
git remote set-head upstream-capture --auto
git symbolic-ref --short refs/remotes/upstream-bot/HEAD
git symbolic-ref --short refs/remotes/upstream-capture/HEAD
```

Origin is https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC.git,
as corrected by the project owner. The local directory remains
`amongus-voice-controller`.

## Reviewed updates

1. Start a dedicated `codex/<number>-<topic>` branch from current main.
2. Fetch the relevant upstream without importing its tags into AUVC's release namespace.
3. Rediscover its default branch. Record the candidate SHA.
4. Compare the last imported SHA with the candidate. Review behavior, licenses,
   dependencies, secrets and relevance before importing.
5. Prefer small ports for capture offsets/memory fixes to keep that code comparable.
   Otherwise use `git subtree merge --prefix=capture --squash <reviewed-SHA>`
   (or `--prefix=bot`). Never merge an unreviewed moving branch.
6. Run relevant format, lint, test, build, recovery and secret checks.
7. Record the SHA, date, rationale and tests here and in the commit body.
8. Push the feature branch and open its issue-linked PR against main.

For a selective port, use `fix(capture): port upstream Among Us offset update`
and `Upstream: automuteus/amonguscapture@<SHA>` in the body. Record selective
ports separately; do not advance the full-import SHA for a partial port.

Review capture upstream when Among Us updates and before each AUVC release.
Never force-push or rewrite main. Preserve original notices when copying code.

## Reviewed local changes after import

Phase 2 (`codex/002-baseline-build`) retains both import commits and their exact
original trees. It separately formats C# whitespace, isolates AUOffsetHelper's
historical 2020 export from current model classes, adds 11 regression tests, pins
the baseline SDK/NuGet resolution and introduces root build CI. Memory algorithms
and bundled offset values are not changed.

The current-tree pristine check remains in `scripts/verify_bootstrap.py` for
historical phase 1 checkouts. Current CI uses `scripts/verify_repository.py` to
verify original import ancestry/tree IDs and license bytes alongside application
tests. Preserve that ancestry with normal merge commits for the initial PRs.

## Installer reference (not imported)

On 2026-09-10, reviewed https://github.com/automuteus/capture-install at
`c08906336fb60fb2e8feb0ac2e033f76c17bfb0a`, default branch `main`.
License: MIT, Copyright (c) 2021 automuteus. No scripts/assets were imported or
executed, so this is a design reference rather than a third subtree.
Before any future reuse, preserve the original license and attribution and add
the copied license/provenance to LICENSES and THIRD_PARTY_NOTICES.
The installer AUVC ships is described in [installer-decision.md](docs/installer-decision.md).
