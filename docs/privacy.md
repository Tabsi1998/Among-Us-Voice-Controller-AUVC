# Privacy

What AUVC stores, where, for how long, and how to remove it.

This describes the code in this repository, not a hosted service. Whoever runs
the bot is responsible for the data it keeps, and nothing leaves the machines
involved except what Discord itself needs to carry out a mute or a move.

## What AUVC does not collect

- **Audio.** AUVC never joins a voice channel to listen. It changes mute, deafen
  and channel membership through the Discord API, and that is all it does with
  voice.
- **Message content.** The bot requests only the Guilds and Guild Voice States
  gateway intents.
- **Match history, statistics or play time.** Those belonged to the upstream
  PostgreSQL layer, which was removed rather than carried.

## On the machine running the bot

Stored in the SQLite database at `AUVC_DATABASE_PATH` — by default
`/data/amongus.db` in the container, or `%LOCALAPPDATA%\AUVC\amongus.db` when the
Windows executable runs without the variable set.

| Table | Contents | Kept until |
| --- | --- | --- |
| `guild_config` | Channel ids, the admin role id, and the voice, ghost and safety settings of each server | `/au setup reset` restores the defaults; the row itself remains |
| `player_link` | In-game name and Discord user id, per server | `/au unlink` |
| `capture_pairing` | SHA-256 hash of an outstanding pairing code, and the Discord user id of whoever requested it | The code is redeemed, replaced, revoked, or found expired |
| `capture_credential` | Random identifier and SHA-256 hash of each capture credential, with creation, last-use and revocation times | Not deleted automatically. A revoked credential stays as a hash, so a later attempt to use it can be recognised as revoked rather than unknown |

No secret is stored in a form that can be read back: pairing codes and
credentials are kept only as hashes.

**Held only in memory:** the running round — in-game names, colours, who is
alive and who disconnected. It is gone when the bot stops, and a reconnecting
capture sends it again.

**Logs** go to standard output and, unless `DISABLE_LOG_FILE` is set, to
`logs.txt` under `LOG_PATH`. They can contain Discord server and user ids,
capture session ids and error messages. They never contain pairing codes or
credentials; the transport and credential tests assert that on both the success
and the failure paths.

## On the Windows PC running capture

| Location | Contents |
| --- | --- |
| `%AppData%\AmongUsCapture\AmongUsGUI\Settings.json` | Capture app settings |
| `%AppData%\AmongUsCapture\logs` | Capture app logs |
| `%LOCALAPPDATA%\AUVC\credential.bin` | The paired credential, encrypted with the Windows Data Protection API for the current user account |

The credential location belongs to the AUVC transport in
`capture/AUVC.Transport`. The capture application does not use that transport
yet; see [architecture.md](architecture.md).

Capture reads the memory of the Among Us process to find the game phase and the
players.

## Removing data

| To remove | Do |
| --- | --- |
| One player link | `/au unlink` |
| A capture install's access | `/au capture revoke` — takes effect at once: open connections are closed and new ones refused |
| One server's settings | `/au setup reset` — restores defaults, keeps links |
| Everything | Stop the bot and delete the database file or the Docker volume |

Deleting the credential on the capture PC — the uninstaller offers to — does
**not** revoke it. The bot keeps accepting it until `/au capture revoke` runs.
The two are easy to confuse, and only the second one closes the door.

Two gaps are known and stated rather than hidden. There is no single command
that removes every trace of one server while keeping the others. And removing
the bot from a Discord server does not delete what it stored about that server.

## Upstream

The [privacy statement under bot/](../bot/PRIVACY.md) was written for the hosted
AutoMuteUs service and describes data collection this code no longer performs.
It is kept as part of the upstream import and is not an AUVC policy.
