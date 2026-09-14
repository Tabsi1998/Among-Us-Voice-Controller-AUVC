# Changelog

All notable AUVC changes will be documented here using Semantic Versioning.

## Unreleased

### Changed

- Development: `python scripts/local_check.py` runs every check the CI runs on
  the developer's own PC: repository guards, Gitleaks, `go test -race`, the
  Windows build of the bot, `govulncheck`, the C# build and tests, the
  self-contained publish and vulnerable NuGet packages. `--release` adds a
  release dry run that publishes nothing. Environment variables that look like
  credentials are withheld from every step
  ([#111](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/111)).
- CI: the checks run once per change, for the pull request and again on main
  after a merge. Pushing a branch used to start them twice.
- Releases: pre-releases such as `v1.2.3-beta` are built and published from the
  maintainer's PC with `scripts/local_release.py`, after every check has passed
  against a fresh copy of the commit. Releases such as `v1.2.3` are still built
  and published by GitHub; pre-release tags no longer start that workflow.

### Fixed

- A crash no longer leaves players muted
  ([#21](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/21)).
  The bot writes down every server mute, server deafen and move into the ghost
  channel before making it, and removes the record once it has lifted it. On
  the next start it releases exactly what a crashed run left behind, members
  who are not in voice as soon as they join, and never touches a member an
  administrator muted by hand. A running round keeps its players muted.

## v0.1.1-beta — 2026-09-14

The second pre-release: players choose their crewmate instead of typing their
in-game name. Like `v0.1.0-beta`, it has not yet been through a round against a
real game and a real Discord server.

**Update.** Install over the previous version, or unpack the new portable zip;
the token and the settings stay. The crewmate message needs two permissions a
bot invited with `v0.1.0-beta` does not have yet, *Embed Links* and *Use
External Emojis*: give them to the bot's role in Discord, or invite the bot into
the same server again. Then choose a text channel behind **Bot** → **Channels**.

**Known limitations.** Not yet tried against a real game. Unsigned, so Windows
warns about an unknown publisher. The first start uploads the crewmate pictures,
which can take a minute.

### Added

- Choosing a crewmate in Discord
  ([#105](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/105)).
  Once the app sees a lobby, the bot posts a message in the text channel with
  every crewmate in the lobby, their pictures, who has picked which, and a menu
  to pick your own; **Unlink me** removes the link. `/au link` without a name
  shows the same menu to you alone. The message is edited in place, deleted when
  the bot stops, found again after a crash, and never shows a death before the
  meeting that announces it. A link made during a round is applied at once.
- Linking players in the app: **Bot** → **Players** lists every crewmate in the
  lobby with a menu of the members in the server's voice channels, for somebody
  who cannot pick for themselves. Only crewmates in the lobby and people in
  voice can be linked, never bots.
- The crewmate pictures from AutoMuteUs are built into the bot and uploaded once
  as emojis of your own Discord application, so they take no emoji slot on the
  server.
- `/au doctor` and the app's checks report whether the crewmate message can be
  posted and whether every picture is uploaded.

### Changed

- The invite also asks for *Embed Links* and *Use External Emojis*, which the
  crewmate message needs. A bot invited with `v0.1.0-beta` does not have them
  yet: give them to the bot's role in Discord, or invite the bot into the same
  server again.
- `/au link` no longer requires a name, and the text channel in the setup is now
  recommended rather than optional, because that is where players choose their
  crewmate.

## v0.1.0-beta — 2026-09-14

The first pre-release of AUVC, for trying it on one Windows PC and reporting
what does not work. It has not yet been through a round against a real game
and a real Discord server.

**Try it.** Download `AmongUsVoiceCapture-Setup-win-x64.exe`, or the portable
`AmongUsVoiceCapture-win-x64.zip`, start AUVC and follow the setup window. It
asks for a Discord bot token, invites the bot, and lets you choose the server
and the channels; the bot then runs with the app. Everything chosen there can
be changed later behind the **Bot** button. Windows warns about an unknown
publisher because the files are not signed: choose **More info**, then
**Run anyway**. `SHA256SUMS` lists the hash of every file.

**Known limitations.** Not yet tried against a real game. Unsigned. Players
link themselves with `/au link`; choosing a crewmate in Discord follows in a
later pre-release.

### Removed

- Docker: the image, the compose file, the container build in CI and the image in
  releases. AUVC runs on the Windows PC that plays Among Us, with the bot inside
  the app.
- The separate bot downloads for Windows and Linux. A release is the setup EXE and
  the portable zip, both carrying the bot, with `SHA256SUMS` and release notes.
- Outdated planning and phase documents, and the README and privacy files
  inherited from upstream, which described the hosted AutoMuteUs service.

### Changed

- Documentation for people using AUVC: a user guide in English
  (`docs/guide.md`) and German (`docs/anleitung.md`) with download, setup,
  every setting, the Discord commands, troubleshooting, updating and
  uninstalling. The README is a short entry point; building and releasing moved
  to `docs/development.md`.
- The capture app connects to the AUVC bot. It pairs from its window with the
  address of the bot and a code from `/au capture pair`, keeps the credential
  encrypted for the Windows user, and reports the round over the authenticated
  connection: a snapshot on every connection, then events in order, with a
  heartbeat every five seconds. It reconnects by itself, answers a request for
  a snapshot on the same connection, and stops retrying, with a message saying
  why, when the bot refuses the credential or the build.
- Removed from capture: the upstream Socket.IO connection and its connect code,
  and the Discord bot token setting and handler, which only existed to carry
  out mute tasks for the hosted service. `SocketIOClient` and `Discord.Net` are
  no longer dependencies.
- `CaptureLink` replaces `CaptureConnection`. The connection tests carry over as
  `CaptureLinkTests`, next to new tests for the record of the round, bot
  addresses, and the mapping from memory-reader events to reports.
- The README describes AUVC as it is: what works and is tested, and what has
  not been tried yet. It had described a much earlier baseline: legacy services
  still running, pairing and session control staged, and the old ghost rule
  that announced every kill.
- `SECURITY.md` lists the protections that exist and how they work, and the
  known limitations, instead of a plan for protections to come.
- `docs/architecture.md` and `docs/service-removal.md` no longer say Redis and
  PostgreSQL are still in use.

### Fixed

- The release workflow no longer fails before building anything. Its first job
  checked out a shallow clone, in which `verify_repository.py` cannot find the
  original upstream imports it verifies. A dry run found this; a tag would have
  failed the same way.
- The container build retries `go mod download` up to three times. A single
  reset connection to the Go module proxy failed a pull request build that
  changed no Go code at all.
- An exiled player now goes to the ghost channel. Capture reports an exile
  before the phase leaves the meeting, and the bot treated that death as a
  secret, so the player stayed silenced in the main channel until the next
  meeting. A death reported during a meeting is now public at once.
- An update for an announced ghost no longer makes the death secret again.
  Capture reports the same death more than once, and each report reset it,
  which silenced the ghost in the ghost channel.

### Added

- Bot settings in the Windows app, in both the installed and the portable
  version. Once the bot is set up, the **Bot** button opens everything the
  setup chose as separate sections: a new token restarts the bot with it, a
  different server can be chosen and capture follows it, the channels and
  automatic start can be changed and saved, and the status section shows the
  bot's checks with buttons to restart the bot or stop running it on this PC.
  Nothing needs reinstalling or setting up again from the start.
- The Windows app sets up and runs the bot itself. On first start a setup
  window asks for the bot token and checks it with Discord, opens an invite link
  with exactly the permissions AUVC needs, lets you choose the server and the
  channels from lists, and connects capture to the bot without a pairing code.
  From then on the bot starts invisibly with the app and is asked to release
  everyone and stop when the app closes; a Windows job object ends it even if
  the app crashes. The token is stored with the Windows Data Protection API.
  The installer and the portable zip now carry `bot\auvc.exe`.
- The capture window's connection indicators have names again. The resource
  keys for "AUVC bot" were missing, so the indicator had no label.
- A local control interface for the AUVC Windows app, the groundwork for a
  Windows setup without PowerShell or server ids. When the app starts the bot
  with `AUVC_LOCAL_CONTROL_SECRET`, the bot answers `/local/...` requests from
  this computer that carry the secret: status and servers, the channels of a
  server, saving the channel setup and auto start, the doctor's checks, a
  capture credential without a pairing code, and a clean stop. Without the
  variable nothing changes.
- `SLASH_COMMAND_GUILD_IDS=*` registers `/au` in every server the bot is in,
  including servers it is invited to while running, so the commands appear at
  once.
- A stopping bot releases the players of every running session: unmuted,
  undeafened and back in the main channel. Before, stopping the bot mid-round
  left the living server-muted until somebody fixed it by hand.
- `docs/privacy.md`: every table and file AUVC keeps, what is in it, how long it
  stays and how to remove it, what AUVC does not collect, and the two gaps that
  are known rather than hidden.

### Security

- `/au capture revoke` now ends capture connections that are already open.
  The credential was checked only at the handshake, so a capture connected at
  the time of a revoke kept working, and its heartbeats kept it connected
  indefinitely. It is now told its credential was revoked and disconnected.
  A connection whose credential check was still running when the revoke
  landed is refused as well. Writing the privacy page found the gap: the
  first draft said revoking took effect at once, and checking that claim
  showed it did not.

### Added

- The required scenarios from `docs/requirements.md` run as tests, one per
  scenario, driving a real lobby through the real session, links, policy and
  reconciler. The simulated Discord applies what the reconciler decides, so
  each step is judged against the state the previous one produced.
- `docs/acceptance.md` maps every scenario to where it is proved and says
  plainly what none of it proves: that Discord does what it was asked, that
  capture reads the game correctly, that the halves connect over a real
  network, or that the installer installs. It carries the manual smoke test
  that answers those.

### Fixed

- Scenario 3 in `docs/requirements.md` still described the old behaviour, where
  a death moved the player to the ghost channel immediately. The ghost-chat
  table had been corrected and the scenario list had not.
- The capture path no longer panics when Discord is unavailable. Checking
  whether a user is a bot account dereferenced the session without guarding
  it, which the acceptance tests found the moment they ran without one.

### Added

- A Windows installer for the capture app, built by the release workflow from
  the same payload the portable zip contains, so the two are one application
  offered two ways rather than two builds that can disagree.
- It installs per user with no elevation, refuses to install over a running
  capture, and keeps settings and the paired credential through an upgrade. An
  upgrade that made a working install pair again is an upgrade people avoid.
- Uninstalling asks separately whether to delete local data, and says plainly
  that removing the credential from the PC is not the same as revoking it:
  the bot still accepts it until somebody runs `/au capture revoke`. The two
  are easy to confuse and only one of them closes the door.
- `docs/installer-decision.md` records why Inno Setup rather than WiX, MSIX or
  Squirrel. MSIX was decisive: Windows refuses to install one unsigned at all.
- The installer is unsigned, and what that looks like is written down rather
  than left as a surprise. Somebody who is not expecting the SmartScreen
  warning concludes the download is broken; somebody who is expecting it clicks
  through anything that looks similar.

### Added

- A release workflow. A `v*` tag builds the bot for Windows x64 and Linux
  amd64/arm64, the self-contained capture payload and the container image,
  publishes the image to GHCR and creates the GitHub release with checksums.
  Running it on demand builds everything and publishes nothing, so the pipeline
  can be exercised without producing a release nobody asked for.
- The gates run again against the exact commit being released. Trusting the
  pull request that led there would mean releasing a combination nothing ever
  tested.
- A pre-release tag never becomes `:latest`. Somebody pulling `latest` is asking
  for the version they are meant to run.
- Release notes come from `CHANGELOG.md` through `scripts/release_notes.py`,
  which fails when the version has no section. Writing them is part of
  preparing a release, and a silent fallback is how that step gets skipped
  forever.
- Licences travel with every artifact. A downloaded executable with no notices
  beside it is what those files exist to prevent.

### Added

- The bot ships as a plain executable as well as a container: Windows x64 for
  running it on the same PC as the game, Linux amd64 and arm64 for a server, a
  NAS or a Raspberry Pi. It is a single static binary with nothing to install,
  which the pure-Go SQLite driver chosen in phase 6 is what makes possible. CI
  cross-compiles all three, so a change that only builds on Linux fails there
  rather than in somebody's download.
- The database lands somewhere sensible when nothing says otherwise: under
  `%LOCALAPPDATA%\AUVC` on Windows rather than at the root of the current
  drive, which is where the container default would have put it.
- The capture listener defaults to `127.0.0.1:8123` instead of being off. It is
  the only way capture can reach the bot now that the legacy transport is gone,
  so a bot without it does nothing at all; localhost covers the same-PC case
  and exposes nothing. `AUVC_CAPTURE_ADDR=off` turns it off.
- Self-hosting is one `docker compose up`. `deploy/docker-compose.yml` runs a
  single service with a volume for the SQLite file, `deploy/.env.example`
  documents every setting without holding a token, and `deploy/README.md` walks
  through the first run, the Discord permissions, upgrading and backups.
- `GET /healthz` answers whether AUVC can actually work: it checks the Discord
  connection and the database and names what failed. A check that only proved
  the process was running would be worth little, because a bot that lost
  either is as useless as one that crashed and only the crash restarts itself.
- The image declares a `HEALTHCHECK`, and CI fails if a future change drops it.

### Fixed

- The bot exits with a failing status code when it fails to start. It logged
  the error and exited zero, which tells a container manager, a service
  supervisor and a shell script alike that everything went fine.

### Changed

- `deploy/Dockerfile.baseline` is now `deploy/Dockerfile`. It described itself
  as a build-only baseline that still ran the legacy services; those services
  are gone, so it is simply the image.

### Removed

- The capture app's self-updater. It downloaded releases from AutoMuteUs's
  GitHub and verified them against an embedded AutoMuteUs public key, so it
  could never have updated AUVC correctly: it checked that a download came
  from a project this one is no longer part of.
- `PgpCore` and the embedded `AutoMuteUs_PK.asc` went with it. That was the
  only path to a vulnerable `BouncyCastle`, which is why the pending NuGet
  update failed the vulnerability check rather than the packages it bumped.
- The contributor list stays, and so does Octokit. Crediting the upstream
  authors is something this project wants to keep doing.

  Capture has no self-update until phase 17 provides one with AUVC signing
  and publisher identity (#24). Updating means downloading the new release.

### Added

- `/au doctor` is a real diagnosis. It reports the Discord connection, SQLite
  and its migration state, the three configured channels, the five effective
  voice permissions, capture pairing, heartbeat freshness, protocol version,
  what AUVC believes about the running game, and the build version.
- Each finding says what to do about it. A warning without a next step makes a
  reader feel worse without helping, and a fresh server is incomplete rather
  than broken.
- Passing checks are listed too, so the reader can see what was checked rather
  than guess whether the rest was skipped.
- A configured channel that no longer exists is reported. It is the failure
  nobody thinks to look for, because the configuration still names it.

### Fixed

- A capture that reconnected while its previous socket was still draining
  could have both connections writing the same guild, and an event from the
  old one would undo the snapshot that had just rebuilt the round. The
  protocol receiver lives per connection and cannot know it has been replaced,
  so the guild now follows one capture session at a time and a snapshot is what
  takes over. The end-to-end recovery tests found this.

### Added

- Recovery scenarios driven through the real WebSocket server, the real
  protocol receiver and the real handler: reconnect rebuilding a round, stale
  events from a replaced connection, duplicates, a bot restart recovered by the
  next snapshot, a capture crash failing open and resuming on reconnect, and a
  reconnect that never sends a snapshot being unable to change anything.

### Removed

- `AUCapture-Console`, the Linux D-Bus console host. It shipped in no release
  artifact and spoke the socket transport whose bot-side counterpart phase 14
  deleted, so it talked to something that no longer exists.

### Fixed

- Every NuGet update pull request failed the Windows job, whatever it bumped.
  `AUCapture-Console` depended on `Mono.Posix`, which made NuGet record a
  runtime-identifier target in its lock file — and the identifier is whatever
  host ran the restore. Dependabot restores on Linux, CI restores on Windows,
  and `--locked-mode` refused the mismatch with `NU1004`. The lock files are
  platform independent now.

### Added

- The capture fail-safe. A capture that dies mid-round used to leave every
  living player server-muted and deafened with no way out, because nothing in
  Discord expires a server mute. A watchdog now notices when a running session
  has not heard from its capture within `capture_timeout_seconds` and applies
  `capture_timeout_action`: `fail-open` releases everyone and pauses the
  session, `pause` suspends it and leaves players where they are.
- The control channel is warned once per stall rather than once per check, and
  told again when capture comes back.
- A session interrupted by the timeout returns to the mode the administrator
  had chosen when capture reconnects. A pause caused by a fault is undone; a
  pause somebody asked for is not.
- Any accepted protocol message counts as a life sign, not only a heartbeat: a
  session sending game events is evidently running.

### Fixed

- A kill is no longer announced by Discord. Dead players used to be moved to
  the ghost channel the moment they died, and a channel change is visible to
  every member of the server: the ghost channel filled up in plain sight while
  the survivors were still meant to be guessing who was missing. A freshly
  killed player is now silenced and left where they are, and moves only once a
  meeting has made the death public.
- Once announced, ghosts keep the ghost channel for the rest of the round,
  including through later task phases. Sending them back would be the same leak
  in reverse and would cost them ghost chat for the rest of the game.

### Removed

- Redis is gone, and with it the game state store, the event queue, the
  distributed locks, the rate limiter, the username cache and the per-guild
  settings. A self-hosted bot is one process: the session lives in it, the
  locks are mutexes, and the configuration is a SQLite file.
- The Galactus token provider and its worker-bot pool are gone. It existed to
  spread Discord rate limits across several bot tokens for a hosted service
  running many guilds; a self-hosted bot has one token and nothing to spread.
- Thirteen legacy commands are gone. `/au` is the whole surface: `/new`,
  `/refresh`, `/pause`, `/end`, `/link`, `/unlink`, `/settings`, `/info`,
  `/map`, `/debug`, `/help`, `/stats` and `/download` were built around a
  hosted service AUVC is not.
- The Go module is down to six direct dependencies. `REDIS_ADDR`, `REDIS_PASS`,
  `POSTGRES_*`, `HOST`, `EMOJI_GUILD_ID` and `ACK_TIMEOUT_MS` are no longer
  read; `DISCORD_BOT_TOKEN` and `AUVC_DATABASE_PATH` are what the bot needs.

### Added

- `/au session start`, `stop`, `pause`, `resume` and `status` work. They were
  registered and reported themselves as staged; they now control whether the
  bot acts on what capture reports.
- Stopping and pausing are deliberately different. Stopping releases everyone
  and returns them to the main channel; pausing leaves them exactly where they
  are and keeps following the game, so resuming acts on the round as it stands
  rather than as it stood when the pause began.
- `auto_start` decides whether a connecting capture starts managing voice on
  its own. A guild that left it off has said it wants to decide, so a snapshot
  does not quietly take over; a configuration that cannot be read answers no
  for the same reason.

### Removed

- PostgreSQL is gone, with everything that only existed to feed it: match
  history, per-event recording, the cached user data, and the `/stats`,
  `/download` and `/privacy` commands. AUVC is a self-hosted voice controller
  for one server, and a match-history warehouse was part of the hosted service
  it is not.
- `jackc/*`, `scany` and `pgxmock` left the module graph entirely: nineteen
  lines out of `go.mod` and two hundred and ten out of `go.sum`.

### Security

- Both accepted Go vulnerabilities are gone, because the code that reached them
  is gone. `scripts/check_go_vulnerabilities.py` now reports that no vulnerable
  code is reachable at all, and its list of accepted advisories is empty. The
  guard earned its keep on the way out: it failed the build on the two entries
  that had become stale, which is exactly what it was written to do.

### Added

- Phase 14 begins: a connected capture now drives Discord voice without Redis.
  `session.Live` holds the running session, `(*Bot).HandleCapture` applies the
  protocol messages to it, in-game names resolve through the SQLite player
  links, and the existing voice policy and reconciler do the rest. This is the
  first path from a game to a muted player that touches neither Redis nor
  Postgres; removing the legacy one follows.
- A snapshot replaces the player set instead of merging into it. After a
  reconnect the bot cannot know which of the players it remembers are still in
  the game, and keeping a stale one would mean managing the voice of somebody
  who left.
- Only messages that can change the voice picture cause a reconciliation. A
  heartbeat, or a phase change to the phase already in effect, does not, so
  the bot does not talk to Discord every few seconds for no reason.

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
  docs/persistence.md.
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
  docs/upstream-independence.md.

### Changed

- Capture no longer needs a third-party host to read the game. The offset index
  in `capture/Offsets.json` was already committed and used as a test fixture but
  never read at runtime; it is now embedded in `AUOffsetManager` and used as the
  always-present base layer. `IndexURL` defaults to empty, so a remote refresh is
  opt-in, and the hardcoded fallback to a second foreign repository is removed.
  Remote and cached entries merge on top of the bundled index instead of
  replacing it. See docs/upstream-independence.md.

### Changed

- Phase 3: Go toolchain modernized from the end-of-life 1.19.13 to the supported
  1.27.1 in the module directive, CI and the Docker build stage together, which
  is what the pinned baseline previously made impossible.
- DiscordGo updated to v0.29.0 and its pointer-based `ComponentEmoji` /
  `MessageEdit.Components` API adopted at five call sites, behaviour unchanged.
- Localization (`go-i18n`, `BurntSushi/toml`, `golang.org/x/text`) and the shared
  `golang.org/x/*` libraries updated. Redis, PostgreSQL, premium, metrics and
  Swagger dependencies deliberately stay at baseline versions until the phases
  that delete them. Rationale in docs/go-modernization.md.

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
  docs/service-removal.md.
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
