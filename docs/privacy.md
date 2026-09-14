# Privacy

What AUVC stores, where, for how long, and how to remove it.

AUVC is not a hosted service. It runs on your PC, and whoever runs it is
responsible for the data it keeps. Nothing leaves that PC except what Discord
needs: the mutes and moves, the crewmate message in your text channel, and the
crewmate pictures, which are uploaded to your own bot.

## What AUVC does not collect

- **Audio.** The bot never joins a voice channel to listen. It changes mute,
  deafen and channel membership through the Discord API, and that is all it does
  with voice.
- **Message content.** The bot requests only the Guilds and Guild Voice States
  gateway intents.
- **Match history, statistics or play time.** Nothing about past rounds is kept.

## Where it is stored

Everything is stored for the Windows account that runs AUVC.

| Location | Contents |
| --- | --- |
| `%AppData%\AmongUsCapture\AmongUsGUI\Settings.json` | App settings, including whether the bot runs on this PC and which Discord server it manages |
| `%AppData%\AmongUsCapture\logs` | App logs. They can contain in-game names; the token and credentials are never written to them |
| `%LOCALAPPDATA%\AUVC\bot-token.bin` | The Discord bot token, encrypted with the Windows Data Protection API |
| `%LOCALAPPDATA%\AUVC\credential.bin` | The app's credential for the bot, encrypted the same way |
| `%LOCALAPPDATA%\AUVC\amongus.db` | The bot's database, described below |
| `%LOCALAPPDATA%\AUVC\logs` | Bot logs. They can contain Discord server and user ids and error messages; never the token, pairing codes or credentials |

The bot's database:

| Table | Contents | Kept until |
| --- | --- | --- |
| `guild_config` | Channel ids, the admin role id, and the voice, ghost and safety settings of each server | `/au setup reset` restores the defaults; the row itself remains |
| `player_link` | In-game name and Discord user id, per server | `/au unlink` |
| `capture_pairing` | SHA-256 hash of an outstanding pairing code, and the Discord user id of whoever requested it | The code is redeemed, replaced, revoked, or found expired |
| `capture_credential` | Random identifier and SHA-256 hash of each app credential, with creation, last-use and revocation times | Not deleted automatically. A revoked credential stays as a hash, so a later attempt to use it is recognised as revoked |
| `crewmate_board` | Channel id and message id of the crewmate message, per server | The bot stops and deletes the message, or the text channel is removed from the setup |
| `voice_hold` | Server id, member id, and which of server mute, server deafen and the move into the ghost channel AUVC set on that member | AUVC lifts it again: during the round, when it stops, or on its next start after a crash |

No secret is stored in a form that can be read back from the database: pairing
codes and credentials are kept only as hashes.

**Held only in memory:** the running round — in-game names, colours, who is alive
and who disconnected. It is gone when AUVC closes.

**Shown in Discord:** while AUVC runs, the crewmate message in your text channel
lists the in-game names and colours of the lobby and which members picked them.
Everyone who can read that channel sees it. It is deleted when AUVC closes.

The app reads the memory of the Among Us process to find the game phase and the
players.

## Removing data

| To remove | Do |
| --- | --- |
| One player link | `/au unlink` |
| One server's settings | `/au setup reset` — restores defaults, keeps links |
| The app's access to the bot | `/au capture revoke` — takes effect at once |
| Everything | Uninstall and choose to delete the data, or delete `%LOCALAPPDATA%\AUVC` and `%AppData%\AmongUsCapture` |
| The bot token | Click **Reset Token** on the **Bot** page of the Discord Developer Portal |

Deleting local files does not change anything in Discord: the bot stays in your
server until you remove it, and its token keeps working until you reset it.

Two gaps are known and stated rather than hidden. There is no single command that
removes every trace of one server while keeping the others. And removing the bot
from a Discord server does not delete what it stored about that server.
