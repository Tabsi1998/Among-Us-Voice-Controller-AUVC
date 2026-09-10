# Changelog

All notable AUVC changes will be documented here using Semantic Versioning.

## Unreleased

### Added

- Phase 8: `bot/pkg/voice` maps a session and its guild configuration to the
  `DesiredVoiceState` of every managed player, following the ghost-chat table in
  `docs/requirements.md`. Pure and fully tested; not wired into the bot yet, the
  reconciler follows.
- Phase 7: `bot/pkg/au` defines the complete `/au` command tree with real
  Discord option types, the authorization rules for each subcommand, and
  configuration validation. A test asserts the tree against the command list in
  `docs/requirements.md`, so a documented command cannot quietly disappear.
- `/au` is registered and routed before the Redis-backed legacy command path.
  Setup, settings, persistent link/unlink, basic doctor and version handlers use
  a Discord-independent application service with typed inputs and authorization.
  Capture and session handlers report their staged status until their services
  exist.
- Phase 6: SQLite persistence in `bot/pkg/storage/sqlite` for guild
  configuration and Discord player links, with versioned embedded migrations,
  a gapless-version check and a refusal to open a database newer than the
  binary. Uses the pure-Go `modernc.org/sqlite` driver so the `CGO_ENABLED=0`
  Docker build keeps working. The bot now opens `/data/amongus.db`, and the
  baseline image provides the writable persistent volume; see
  [docs/persistence.md](docs/persistence.md).
- Phase-2 Windows test project with 11 offset/CLI regression cases, locked NuGet
  restores, root Go/Windows/Docker/provenance/secret CI and Dependabot.
- Branding and product-design roadmap with asset/license inventory, multiple
  visual concepts, owner approval gates and staged Capture/Discord/installer work.

### Added

- Phase 5: `bot/pkg/session`, the Discord-free projection of a running session
  (phase, player links, alive/dead) that the voice policy will consume in phase
  8. `(*GameState).SessionState()` is the only seam between the Discord/Redis
  side and that domain.
- Discord bot accounts are now recorded (`User.IsBot`) and reported as unmanaged,
  so the voice policy and channel enforcement can never act on a music bot.

### Changed

- Dependabot now groups minor and patch updates per ecosystem and ignores the
  Redis/PostgreSQL packages that phase 14 deletes, plus the .NET majors that
  need the phase 11 LTS migration. Both sets are documented with the phase that
  re-enables them. `docs/development.md` records why NuGet bumps fail with
  `NU1004` and the exact commands that resolve it.

### Changed

- Map images are embedded in the bot binary instead of being fetched from the
  upstream GitHub repository. `/map` now answers from the bundled image with no
  network access; `BASE_MAP_URL` has no default, so nothing points at a foreign
  host unless an operator configures one. The decorative game-state thumbnail
  appears only when `BASE_MAP_URL` is set. See
  [docs/upstream-independence.md](docs/upstream-independence.md).

### Changed

- Capture no longer needs a third-party host to read the game. The offset index
  in `capture/Offsets.json` was already committed and used as a test fixture but
  never read at runtime; it is now embedded in `AUOffsetManager` and used as the
  always-present base layer. `IndexURL` defaults to empty, so a remote refresh is
  opt-in, and the hardcoded fallback to a second foreign repository is removed.
  Remote and cached entries merge on top of the bundled index instead of
  replacing it. See [docs/upstream-independence.md](docs/upstream-independence.md).

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

### Changed

- The bot Go module is now
  `github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot` instead of
  `github.com/automuteus/automuteus/v8`, so no import path in AUVC source points
  at a repository AUVC does not own. The `/v8` suffix is dropped because the
  release sequence restarts at `v0.1.0-alpha.1`. Provenance and license
  documents keep their upstream references unchanged.

### Removed

- The four unused backup project files under `capture/`. None was in
  `AmongUsCapture.sln` or referenced by any `ProjectReference`; they were older
  snapshots targeting netcoreapp3.1 and outdated package versions. Dependabot
  was maintaining projects nobody builds.
- The inert upstream CI configuration nested at `bot/.github/` and
  `capture/.github/`: four Docker/goreleaser/build workflows, a second
  Dependabot config, upstream issue templates and a `FUNDING.yml` whose custom
  sponsor link pointed at `automute.us/premium`. GitHub only reads the
  repository-root `.github/`, which already provides all of these, so the nested
  copies were dead weight that named another project as owner and publisher.
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
