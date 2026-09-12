# Changelog

All notable AUVC changes will be documented here using Semantic Versioning.

## Unreleased

### Added

- Phase 13 completed: the capture side of the connection. `capture/AUVC.Transport`
  builds and numbers protocol messages, runs the handshake, redeems a pairing
  code against the bot and keeps the credential it gets back. The connection
  sits behind an interface, so the handshake, a refusal and a reconnect are
  tested without opening a socket.
- The credential is stored with the Windows Data Protection API, tied to the
  current user account: the file is unreadable to another account on the same
  machine and to anyone who copies it elsewhere. A test asserts that the bytes
  on disk do not contain the credential, which is the part that matters.
- Capture refuses its own mistakes locally rather than learning about them
  from a refusal mid-round: an event before the snapshot, a message before the
  session is open, or authentication without a credential all fail where the
  mistake was made.
- A bot that cannot be reached is reported as unreachable, not as a bad
  pairing code. Telling someone to ask for a new code when the address is
  wrong sends them down entirely the wrong path.

### Added

- Phase 13, second half: the direct capture connection. `bot/pkg/transport`
  serves `POST /capture/pair`, which exchanges a typed pairing code for a
  credential, and `GET /capture/link`, the WebSocket that carries the protocol.
  Both are off unless `AUVC_CAPTURE_ADDR` names an address.
- Capture never sees a guild id: a pairing code is found by its hash across
  guilds, because asking a player to copy a Discord snowflake out of a
  developer menu is not a setup flow anyone finishes.
- A failed credential closes the connection rather than answering and carrying
  on. The protocol receiver has already recorded that an authentication
  message arrived in the right order by the time the credential is checked, so
  a session left open would believe it was authenticated when it was not.
  A missing snapshot, by contrast, keeps the connection: the session repairs
  it by sending one, and closing would turn a recovery into a reconnect loop.
- Only requests without an `Origin` header are upgraded, so a web page cannot
  reach the handshake. `AUVC_CAPTURE_TLS_CERT` and `AUVC_CAPTURE_TLS_KEY` make
  the listener terminate TLS itself; without them it warns on every start that
  it needs a reverse proxy in front of it.

### Added

- Phase 13, first half: capture pairing and credentials. `/au capture pair`
  issues a code that works once and expires after ten minutes, `/au capture
  status` reports whether a capture is paired and when it was last seen, and
  `/au capture revoke` withdraws every credential and cancels any outstanding
  code. All three replies are ephemeral, which is what makes it acceptable to
  put a pairing code in one.
- Only hashes are stored, so no secret can be recovered from the database, a
  backup or a support bundle. `credential.Secret` redacts itself from fmt and
  encoding/json, so the rule that credentials never reach a log is enforced by
  the type instead of by everyone remembering it. Comparison is constant time.
- A pairing code is single use and expires; a wrong code does not consume the
  real one, so a typing error does not cost an administrator a new code.
  Requesting a new code replaces the outstanding one, which is how a code read
  out to the wrong person is cancelled.

### Added

- Phase 12: the capture-to-bot protocol, as a transport-independent contract.
  `protocol/README.md` defines the versioned envelope, all eleven message
  types, the handshake order and the sequence rules; `bot/pkg/protocol`
  receives and `capture/AUVC.Protocol` sends. Neither implementation is the
  reference: both are tested against the shared fixtures in
  `protocol/fixtures`, so a message type cannot be added, renamed or reshaped
  on one side alone without failing the other side's tests.
- The receiver refuses what it cannot safely act on: an incompatible protocol
  version, game data before authentication, and events before a complete
  snapshot. A gap in the sequence costs the session its snapshot and blocks
  events until a new one arrives, because applying only the events that did
  arrive would leave the bot confidently wrong about who is alive. Duplicates
  and late messages are ignored silently rather than answered with an error,
  since retransmission is ordinary on a reconnecting transport.

### Changed

- Phase 11: capture targets .NET 10, the current LTS, instead of .NET 5, and
  `capture/global.json` pins SDK 10.0.401. .NET 8 was deliberately not chosen:
  its support ends in November 2026, which would have meant doing this
  migration twice. Every project builds and all tests pass on the new target
  with no source changes to the memory or offset code.

### Added

- The Windows CI job proves the win-x64 payload is self-contained, so the app
  still starts on a machine with no .NET installed, and fails the build if the
  payload turns framework-dependent.
- Tests for what the framework move actually risks: the shipped assemblies
  target the expected LTS, the test host runs it, and the offset index parses
  identically under a comma-decimal culture. Nineteen cases, up from fifteen.
- Nullable reference types are enabled where the code is already clean under
  them and switched off deliberately elsewhere, each project recording the
  measured warning count and what unblocks it. The table is in
  `docs/development.md`.

### Security

- `System.Drawing.Common` 4.7.0 (critical, GHSA-rxg9-xrhp-64gj) and
  `Tmds.DBus` 0.9.1 (high, GHSA-xrw6-gwf8-vvr9) are gone. The first arrived
  through Config.Net and is cut off by pinning a modern
  `System.Configuration.ConfigurationManager`, which avoids an untested
  Config.Net major. All six capture projects now report no vulnerable packages,
  and CI fails if that changes.

### Added

- Phase 10: `enforce_channels` is honoured. On by default, it returns a player
  who switched channels themselves — a living player who walks into the ghost
  channel goes back to main, a dead one who walks into main goes back to ghost.
  `/au settings ghosts enforce_channels:false` is the administrator override:
  the bot then stops deciding where a living player sits and still mutes and
  deafens them as the phase demands, so wandering into the ghost channel is
  never a way to listen in. Turning it off does not turn ghost chat off with
  it, and does not suppress the return to main at the end of a round.

- The game handlers now apply the AUVC voice policy: `handleTrackedMembers`
  reconciles observed against desired voice states instead of following the
  legacy rules. Because it compares against what Discord reports rather than
  against the intent recorded on the last run, a mute that failed or a player
  somebody unmuted by hand is now corrected instead of staying wrong for the
  rest of the round. A failure reports the missing Discord permissions by name.
  Guilds that are disabled, have not run `/au setup channels`, or whose
  configuration cannot be read keep the legacy behaviour until phase 14.

### Fixed

- The reconciler applies moves before relaxing the living. Dying at the moment
  a meeting started could otherwise undeafen the living while the fresh corpse
  was still in the main channel, giving the round away.
- A delayed voice change now applies the state as it is when the delay ends.
  Someone dying during the delay before a phase change used to be ignored,
  because the change had already been decided from the state as it was when
  the delay started.

### Added

- The Discord side of the voice reconciler: observing server mute and deafen
  from the guild voice states, turning a reconciler change into a member edit,
  and resolving the effective permissions on the configured channels. Not wired
  into the game handlers yet.
- Phase 9: `bot/pkg/permission` names the Discord permissions AUVC is missing
  and what each one breaks, grouped so a permission missing on both channels is
  explained once. Not wired into the bot yet.
- Phase 8: `voice.Diff` and `voice.Reconciler` apply only the differences
  between the observed and desired voice states, idempotently and serialized per
  guild. Not wired into the bot yet.
- Phase 8: `bot/pkg/voice` maps a session and its guild configuration to the
  `DesiredVoiceState` of every managed player, following the ghost-chat table in
  `docs/requirements.md`. Pure and fully tested; not wired into the bot yet, the
  reconciler follows.
- `scripts/check_upstream_references.py` fails CI when a new reference to
  AutoMuteUs-controlled infrastructure appears. The forty that still exist are
  baselined and tracked in #33 and #44; the list may only shrink.
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

### Security

- Updated `jackc/pgx` to v4.18.2, `jackc/pgproto3` to v2.3.3 and
  `gorilla/websocket` to v1.5.3, clearing three advisories the bot calls into,
  two of them SQL injection. `scripts/check_go_vulnerabilities.py` now runs in
  CI and fails on any advisory that has not been assessed; the two remaining
  ones have no fix in their major line and are recorded with the phase that
  removes them.

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
