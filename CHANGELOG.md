# Changelog

All notable AUVC changes will be documented here using Semantic Versioning.

## Unreleased

### Added

- Phase-2 Windows test project with 11 offset/CLI regression cases, locked NuGet
  restores, root Go/Windows/Docker/provenance/secret CI and Dependabot.

### Changed

- Phase 3: Go toolchain modernized from the end-of-life 1.19.13 to the supported
  1.27.1 in the module directive, CI and the Docker build stage together, which
  is what the pinned baseline previously made impossible.
- DiscordGo updated to v0.29.0 and its pointer-based `ComponentEmoji` /
  `MessageEdit.Components` API adopted at five call sites, behaviour unchanged.
- Localization (`go-i18n`, `BurntSushi/toml`, `golang.org/x/text`) and the shared
  `golang.org/x/*` libraries updated. Redis, PostgreSQL, premium, metrics and
  Swagger dependencies deliberately stay at baseline versions until the phases
  that delete them. Rationale in [docs/go-modernization.md](docs/go-modernization.md).

### Removed

- Phase 4, first step: the public HTTP API and its generated Swagger package,
  the Prometheus metrics endpoint, the Kubernetes liveness/readiness probes,
  the request telemetry at twelve call sites, and the secondary worker bot token
  pool including its per-tier allowance and guild-membership enforcement.
  Behaviour is preserved for a self-hosted bot; mutes now go to the capture bot
  and otherwise to the primary session. Inventory and rationale in
  [docs/service-removal.md](docs/service-removal.md).
- Dependencies dropped as a result: gin, gorilla/mux, prometheus/client_golang,
  the swaggo family and golang.org/x/exp, plus about 29 indirect modules.
- Phase 4, second step: premium tiers and the `/premium` command, the top.gg
  vote integration, the `AUTOMUTEUS_OFFICIAL` switch and gateway sharding.
  Self-hosters already received the full feature set unconditionally, so every
  gate resolved to the branch they already took and no feature is lost; the
  settings embed is now one flat list instead of a free/premium split.
  `TOP_GG_TOKEN`, `NUM_SHARDS`, `SHARD_ID` and `SHARDS` are gone, and
  `github.com/top-gg/go-dbl` left the module.

### Fixed

- Full capture solution build: the old offset helper now exports its historical
  2020 format independently and requires explicit `--legacy-sample` selection.
- C# baseline whitespace formatting, without memory algorithm/offset changes.

### Bootstrap foundation

- Independent AUVC repository foundation and MIT license for new contributions.
- Unmodified squash-subtree imports of AutoMuteUs and AmongUsCapture.
- Original license copies, credits and pinned upstream provenance.
- Monorepo boundaries, contribution workflow, security guidance and v1.0.0 roadmap.
- Bootstrap verification and a narrowly scoped, reviewed secret-scan exception.
- Windows installation and release roadmap: graphical EXE/MSI installers, guided
  first-run UX, secure Stable/Preview updates, automated release triggers, signing
  and download distribution (planned; issues #20–#24).

No AUVC application release has been produced. Planned early tags are
`v0.1.0-alpha.1`, `v0.1.0-alpha.2`, then `v0.5.0` and eventually `v1.0.0`.
