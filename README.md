# Among Us Voice Controller (AUVC)

AUVC is an independent community rework of AutoMuteUs and AmongUsCapture.
It is not an official AutoMuteUs project or an official new AutoMuteUs version.

Repository: https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC

AUVC mutes, deafens and moves Discord players to match an Among Us round. One
Windows PC runs capture, which reads the game; a self-hosted bot does the Discord
side. Nobody else needs mods or extra software.

## Status

**AUVC does not work end to end yet.** The bot is complete for its v1 scope and
the capture side has every piece it needs as a tested library — but the capture
application itself has not been connected to those pieces. Read this section
before trying it.

### Works and is tested

**Bot**

- `/au` for setup, settings, player links, capture pairing, session control and
  diagnostics.
- Ghost chat that does not give a kill away: a victim stays put and silenced
  until a meeting announces the death, then moves to the ghost channel.
- Channel enforcement, with an administrator override.
- Pairing codes that work once and expire, and revocable credentials stored only
  as hashes.
- An authenticated WebSocket for capture, speaking a versioned protocol.
- A fail-safe that releases everyone when capture stops responding.
- Recovery from capture reconnects and from bot restarts mid-round.
- `/au doctor`, and `/healthz` for container health checks.
- No Redis, PostgreSQL, Galactus or worker bots: one process and one SQLite file.

**Delivery**

- A container image and a compose file.
- The bot as a plain executable for Windows x64 and Linux amd64/arm64.
- A release workflow that builds everything from a tag.
- The capture app as a self-contained portable zip and an unsigned installer.

**Capture libraries** (`capture/AUVC.Protocol`, `capture/AUVC.Transport`)

- The protocol, the authenticated WebSocket client, the pairing client and
  Windows credential storage.

### Not done yet

- **The capture app does not use the new transport.** `capture/AUCapture-WPF`
  still contains the upstream socket connection, which the bot no longer serves.
  A capture app built from this repository therefore cannot reach the bot. This
  is the next piece of work, and until it lands AUVC cannot run a real round.
- A guided first run and clear status in the capture app
  ([#21](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/21)).
- A published release. No tag has been created and no image has been pushed.
- A manual test against a real Discord server and a real game. The automated
  suites simulate Discord; [acceptance.md](docs/acceptance.md) lists what they do
  not prove and the smoke test that does.
- Signed artifacts
  ([#24](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/24)).
  The installer shows an "Unknown publisher" warning.
- Replacing or clearing upstream graphics
  ([#33](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/33),
  [#44](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/44)),
  and an independent AUVC visual identity, which the
  [branding plan](docs/branding-and-design.md) puts behind owner approval.
- An MSI package. Deferred; see [installer-decision.md](docs/installer-decision.md).

Development follows the [20-phase roadmap](docs/roadmap.md) and the
[v1.0.0 milestone](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/milestone/1).

## How it works

AUVC is two programs.

| Program | Runs on | Does |
| --- | --- | --- |
| **Capture** | The Windows PC playing Among Us | Reads the game and reports what happens |
| **Bot** | Anywhere with internet | Talks to Discord: mutes, deafens, moves |

```text
Among Us → capture → authenticated WebSocket → bot → Discord
```

Capture has to be on the gaming PC, because that is where the game is. The bot
can run on the same PC or on a server, a NAS or a Raspberry Pi; the Discord side
is identical either way.

Inside the bot, protocol validation, the game session, the voice policy and the
Discord reconciler are separate layers: the policy decides what every player
should hear and say, and the reconciler sends only the difference from what
Discord reports. The [architecture](docs/architecture.md) explains each layer
and the [protocol](protocol/README.md) defines the connection.

## Running the bot

Two ways, both described step by step in [deploy/README.md](deploy/README.md):

- **On the gaming PC.** Unpack `AUVC-bot-win-x64.zip`, set `DISCORD_BOT_TOKEN`,
  run `auvc.exe`.
- **In a container.** `docker compose up -d --wait` in `deploy/`.

The bot needs a Discord application with a bot token, invited with **View
Channel**, **Connect**, **Move Members**, **Mute Members** and **Deafen Members**
on the voice channels it manages.

### Environment

| Variable | Default | Meaning |
| --- | --- | --- |
| `DISCORD_BOT_TOKEN` | — | Required. Treat it like a password. |
| `AUVC_DATABASE_PATH` | `%LOCALAPPDATA%\AUVC\amongus.db` on Windows, `/data/amongus.db` in the image | The SQLite file |
| `AUVC_CAPTURE_ADDR` | `127.0.0.1:8123`, `:8123` in the image | Where capture connects; `off` disables it |
| `AUVC_CAPTURE_TLS_CERT`, `AUVC_CAPTURE_TLS_KEY` | — | Terminate TLS in the bot. Without them, put a TLS proxy in front whenever capture is not on the same machine |
| `SLASH_COMMAND_GUILD_IDS` | global | Register `/au` in named servers only, which takes effect immediately |
| `LOG_PATH`, `DISABLE_LOG_FILE` | `./`, off | Where `logs.txt` goes, or no file at all |
| `BOT_LANG`, `LOCALE_PATH` | English | Language of bot messages |
| `AUVC_LISTENING` | `/au` | The activity Discord shows |

## Setting it up in Discord

```text
/au setup channels       main voice, ghost voice and control text channels
/au setup permissions    optionally, a role allowed to administer AUVC
/au capture pair         a code for the capture app — once, ten minutes
/au link                 tie each Discord user to their in-game name
/au doctor               check everything and see what is still missing
```

The guild owner and Discord administrators can always administer AUVC; the role
from `/au setup permissions` is added to them, not instead of them.

### Commands

| Command | Does |
| --- | --- |
| `/au setup channels`, `permissions`, `reset` | First-time setup; `reset` restores defaults and keeps links |
| `/au settings show`, `export` | See the configuration |
| `/au settings preset`, `voice`, `ghosts`, `safety` | Change it |
| `/au capture pair`, `status`, `revoke` | Manage the capture connection |
| `/au session start`, `stop`, `pause`, `resume`, `status` | Control whether AUVC acts on the game |
| `/au link`, `/au unlink` | Player links |
| `/au doctor` | Diagnose |
| `/au version` | Build and commit |

Every reply is visible only to whoever ran the command.

### Settings

| Setting | Default | Meaning |
| --- | --- | --- |
| `auto_move_ghosts` | on | Move the dead to the ghost channel |
| `enforce_channels` | on | Return players who switch channels themselves |
| `capture_timeout_seconds` | 60 | Silence from capture before the fail-safe runs (10–600) |
| `capture_timeout_action` | `fail-open` | `fail-open` releases everyone; `pause` leaves players as they are |
| `auto_start` | off | Start managing voice when capture sends a game, without `/au session start` |

## What players hear and see

The default `ghost-chat` preset:

| Phase | Living players | Dead players |
| --- | --- | --- |
| Lobby | Main, open | Main, open |
| Tasks, death not yet announced | Main, muted and deafened | Stay where they are, muted |
| Tasks, death announced | Main, muted and deafened | Ghost channel, open |
| Meeting and voting | Main, open | Ghost channel, open |
| Round over | Main, open | Main, open |

A channel change is visible to everyone in Discord, so moving a player the moment
they die would announce the kill. The move waits for the meeting, which is when
the game tells everyone anyway. From then on the ghost keeps the ghost channel
for the rest of the round and can talk to the other ghosts, including during
tasks.

Only linked human players are managed. Bot accounts, unlinked users and players
who disconnected are left alone.

If capture stops responding, the default `fail-open` action unmutes and
undeafens everyone, returns them to the main channel, pauses the session and
warns the control channel. The session resumes on its own when capture returns.
Nothing in Discord ever expires a server mute by itself, which is why this is
the default.

## Upgrading

Back up the database first: the Docker volume, or the `amongus.db` file.

- **Container:** `docker compose pull` (or `build --pull`), then
  `docker compose up -d --wait`.
- **Executable:** stop the bot, replace `auvc.exe` or `auvc`, start it again.

Migrations run on start. AUVC refuses to open a database newer than itself, so a
downgrade stops with an error rather than working against a schema it does not
understand. Restarting mid-round is safe: capture reconnects and sends the state
of the round again.

## Troubleshooting

Run `/au doctor` first. It checks the Discord connection, SQLite and its
migration, the three channels and whether they still exist, the five voice
permissions, capture pairing, how recently capture was heard from, the protocol
version, what AUVC believes about the game, and the build. Each finding says what
to do about it.

`GET /healthz` on the capture port answers `ok`, or names what is wrong.

When asking for help, include `/au version` and the `/au doctor` output. Neither
contains a token or a credential.

## Security and privacy

- [SECURITY.md](SECURITY.md) — how to report a vulnerability, what is protected
  and how, and the known limitations.
- [docs/privacy.md](docs/privacy.md) — exactly what is stored, where, for how
  long, and how to remove it. AUVC records no audio, no messages and no match
  history.

## Repository layout

| Path | Contains |
| --- | --- |
| `bot/` | The Discord bot (Go) |
| `capture/` | The capture app and its protocol and transport libraries (.NET) |
| `protocol/` | The protocol contract and fixtures shared by both languages |
| `deploy/` | Container, compose file and self-hosting guide |
| `installer/` | The capture installer script |
| `docs/` | Requirements, architecture, decisions and guides |
| `scripts/` | Repository checks and release helpers |
| `LICENSES/` | Exact copies of the upstream licences |

## Developing

Go 1.27.1, .NET SDK 10.0.401 and Python 3.11 or newer. See
[development.md](docs/development.md) for the commands CI runs and
[baseline-build.md](docs/baseline-build.md) for building locally.

## Upstream and Credits

- [AutoMuteUs](https://github.com/automuteus/automuteus), MIT,
  Copyright (c) 2020 Denver Quane.
- [AmongUsCapture](https://github.com/automuteus/amonguscapture), MIT,
  Copyright (c) 2020 Denver Quane.

Original notices remain in `bot/LICENSE` and `capture/LICENSE`, with byte-for-byte
copies under `LICENSES/`. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
and [UPSTREAM.md](UPSTREAM.md) for attribution, exact SHAs, dates and how
upstream changes are reviewed and ported.
Thanks to the upstream authors and contributors.

## License

New AUVC components: MIT, Copyright (c) 2026 IT-Tabelander. See [LICENSE](LICENSE).
Upstream components retain their original notices and applicable licenses.
