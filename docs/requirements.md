# AUVC v1.0.0 requirements

This is the implementation contract preserved from the project brief.
Features below are planned unless explicitly recorded as implemented in the
changelog. Phase 1 only establishes provenance and the repository foundation.

## Product and deployment

- Independent community rework of AutoMuteUs and AmongUsCapture, never described
  as an official AutoMuteUs successor or release.
- Preserve original MIT notices; new code: Copyright (c) 2026 IT-Tabelander.
- One Windows capture PC, no mods or extra software for other players.
- Capture -> authenticated WSS -> self-hosted AUVC bot -> Discord.
- SQLite preferred; remove Galactus, Redis, PostgreSQL, public workers, premium
  infrastructure and unnecessary sharding in the staged roadmap.
- Docker volume at `/data`, database `/data/amongus.db`.

## Persistent guild configuration

Versioned, non-destructive reviewed migrations must store at least:

```text
guild_id
enabled
main_voice_channel_id
ghost_voice_channel_id
control_text_channel_id
admin_role_id
voice_policy
auto_move_ghosts
enforce_channels
capture_timeout_seconds
capture_timeout_action
auto_start
config_version
created_at
updated_at
```

Persist Discord/player links needed for restart recovery as well.
Configuration must survive bot/container restarts. Destructive migrations need
explicit examination and a documented backup/recovery plan.

## Discord commands and permissions

```text
/au setup channels
/au setup permissions
/au setup reset
/au settings show
/au settings preset
/au settings voice
/au settings ghosts
/au settings safety
/au settings export
/au capture pair
/au capture status
/au capture revoke
/au session start
/au session stop
/au session pause
/au session resume
/au session status
/au link
/au unlink
/au doctor
/au version
```

Use actual Discord channel, role, boolean and integer option types.
Do not flatten everything to strings. Restrict administration to authorized
roles/users and validate configuration before saving.

## Default voice policy: ghost-chat

| Phase | Living players | Dead players |
| --- | --- | --- |
| Lobby | Main, unmuted, undeafened | All managed players reset to main/open |
| Tasks | Main, muted, deafened | Ghost, unmuted, undeafened |
| Meeting/discussion | Main, unmuted, undeafened | Ghost, unmuted, undeafened |
| Voting | Main, unmuted, undeafened | Ghost, unmuted, undeafened |
| Ended | Main, unmuted, undeafened | Main, unmuted, undeafened |

Central policy produces `DesiredVoiceState` with `TargetChannelID`, `Muted`
and `Deafened`. Game handlers never directly mutate Discord voice state.
The reconciler applies only differences, idempotently, with concurrency control.

On death: update alive state, resolve the Discord link, move to ghost, clear
server mute/deafen, then recheck observed state. Ghosts remain in ghost throughout
the active round and may speak during meetings and voting.

Default `enforce_channels = true`: linked living players entering ghost return
to main; linked dead players entering main return to ghost. Provide a configurable
admin override. Never accidentally manage bots/music bots or unlinked users.

Required permissions include Move Members, Mute Members, Deafen Members,
View Channel and Connect, considering effective permissions/overwrites.

## Capture modernization

Migrate to a currently supported .NET LTS in its dedicated phase. Update NuGet
deliberately, introduce tests and enable nullable reference types through a
controlled migration. Confirm non-use before removing backup projects/artifacts.
Keep memory/offset detection easy to compare with upstream. Produce a
self-contained Windows x64 `AmongUsVoiceCapture-win-x64.zip`.

## Protocol, pairing and recovery

Version the capture-to-bot schema. Example envelope:

```json
{
  "protocol": 1,
  "type": "snapshot",
  "seq": 123,
  "phase": "tasks",
  "players": []
}
```

Required message types: hello, authentication, heartbeat, snapshot,
game_state_changed, player_joined, player_left, player_changed, player_died,
game_ended and error. Validate protocol compatibility with understandable errors.

`/au capture pair` creates an expiring single-use pairing code. Successful
pairing issues a long-term revocable credential, securely stored on Windows.
Do not log credentials or permanent tokens in plaintext.
`/au capture revoke` invalidates existing access/pairing.

Capture sends a complete snapshot after every reconnect. Bot reconstructs phase,
players, alive/dead state, persisted Discord links and desired voice state, then
reconciles Discord. Support both capture and bot restarts during a round.
Incremental events alone are insufficient. Define sequence ordering, duplicate
handling and reconnect/session identity in protocol tests.

Capture sends heartbeats. On configurable timeout, default fail-open:
unmute/undeafen all managed players, optionally return them to main, pause session
and send a Discord warning. Recovery must not leave players indefinitely server
muted after a crash; test failure and restart scenarios explicitly.

## Doctor and diagnostics

`/au doctor` reports understandable ✅ / ⚠️ / ❌ results for:

- Discord connection; SQLite access and migration state.
- Main, ghost and text control channels.
- Move Members, Mute Members, Deafen Members, View Channel and Connect permissions.
- Capture connection, protocol compatibility and heartbeat freshness.
- Current game-state detection.
- Docker/build version when available.

Logs must aid troubleshooting without exposing credentials or unnecessary personal
data. `/au version` exposes build/protocol version information.

## Required automated scenarios

1. Lobby: all managed players open in main.
2. Tasks: living players muted and deafened.
3. Death during tasks: linked player moves to ghost.
4. Ghost is unmuted and undeafened.
5. A second ghost joins; both remain in ghost.
6. Meeting: living players unmuted/undeafened.
7. Ghosts remain in ghost during meeting.
8. Ghosts can continue talking during meeting.
9. Meeting ends: living players muted/deafened again.
10. Dead player enters main: returned to ghost.
11. Living player enters ghost: returned to main.
12. Game ends: all managed players in main, open.
13. Capture disconnect/timeout: fail-safe.
14. Capture reconnect: full state recovery.
15. Bot restart: guild settings survive.
16. Bot restart mid-match: snapshot restores state and persisted links.
17. Configured channel deleted: doctor reports error.
18. Missing Move Members: doctor reports error.
19. Unlinked Discord user is untouched by default.

Also cover duplicate events, stale work, protocol incompatibility, credential
revocation, race conditions and bot exclusion.

## CI, releases and documentation

Each PR: gofmt check, go vet, go test, Go build; dotnet restore/build/test;
Docker build; secret scan and meaningful dependency scan. Dependabot:
Go Modules, NuGet, GitHub Actions and Docker. Keep tests intact when failures occur.

Semantic Versioning: early `v0.1.0-alpha.1`, `v0.1.0-alpha.2`, then
`v0.5.0`, stable `v1.0.0`. Releases include a bot Docker image, capture ZIP,
checksums, release notes and upgrade/migration instructions. Intended GHCR image:
`ghcr.io/tabsi1998/amongus-voice-controller` (registry paths are lowercase).

README must cover purpose, data flow, prerequisites, Discord setup, capture
setup, ghost behavior, slash commands, Docker, upgrade, troubleshooting, doctor,
security, privacy, credits and license. Label planned behavior accurately until
implemented. Architecture explains Capture -> Protocol -> Game State ->
Voice Policy -> Discord Reconciler.

Every phase gets its own feature branch and issue-linked PR. No new phase before
the current one is cleanly complete unless the owner explicitly directs otherwise.
