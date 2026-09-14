# Running AUVC

AUVC is two programs, and it helps to know which is which before choosing how to
run them.

| Program | Runs on | Does |
| --- | --- | --- |
| **Capture** | The Windows PC playing Among Us | Reads the game and reports what happens |
| **Bot** | Anywhere with internet | Talks to Discord: mutes, deafens, moves people |

Capture has to be on the gaming PC, because that is where the game is. The bot
can be anywhere, and there are two sensible places:

- **On the same PC.** Simplest: two programs, no network to think about. The bot
  is online while that PC is on, which for one person playing with friends is
  usually exactly when it needs to be.
- **On a server, NAS or Raspberry Pi.** The bot stays online when the gaming PC
  is off, and somebody else can start a game without you.

The bot is a single executable with nothing to install. Pick whichever suits
you; the setup inside Discord is identical.

## On the same PC

Download `AUVC-bot-win-x64.zip`, unpack it, and start the bot with your token:

```powershell
$env:DISCORD_BOT_TOKEN = "your-token"
.\auvc.exe
```

It keeps its database under `%LOCALAPPDATA%\AUVC` and listens on
`127.0.0.1:8123` for the capture app on the same machine. Nothing is exposed to
the network. To keep it running without a console window, register it as a
scheduled task or a service.

Then jump to [Set it up in Discord](#set-it-up-in-discord).

## In a container

One container, one SQLite file, one Discord bot token. No Redis, no PostgreSQL,
no Galactus, no worker-bot pool: those belonged to the hosted service AUVC is
not, and phase 14 removed them.

You need Docker with Compose, a Discord bot token from an application you create
at [discord.com/developers](https://discord.com/developers/applications), and a
Windows PC running Among Us for the capture app.

### Start it

```sh
cd deploy
cp .env.example .env      # then put your bot token in it
docker compose up -d --wait
```

`--wait` holds until the health check passes, so a broken start fails here
rather than leaving a container that looks running and is not.

## Invite the bot

The bot needs five permissions on the voice channels it manages: **View
Channel**, **Connect**, **Move Members**, **Mute Members** and **Deafen
Members**. Without Move Members it cannot run ghost chat; without Deafen Members
the living can hear the dead during tasks. `/au doctor` names any that are
missing and what each one breaks.

## Install the capture app

On the Windows PC that plays Among Us, either:

- **`AmongUsVoiceCapture-Setup-win-x64.exe`** — installs for you only, no
  administrator needed, with a Start Menu entry and an uninstaller.
- **`AmongUsVoiceCapture-win-x64.zip`** — unpack and run, installs nothing.

Both contain the same application and neither needs .NET installed.

**Windows will warn you.** The installer is not signed yet, so SmartScreen says
*"Windows protected your PC — Unknown publisher"*. Click **More info**, then
**Run anyway**. If you would rather check the download first, `SHA256SUMS` on the
release page lists the expected hash of every file; that proves the file matches
what the release says, not who built it. Signing is planned and tracked in #24.

## Set it up in Discord

```text
/au setup channels     main, ghost and control channels
/au capture pair       a code that works once and expires in ten minutes
/au doctor             checks everything and says what is still missing
```

Type the pairing code into the capture app on the Windows PC. Then
`/au session start`, or set `auto_start` so a connecting capture begins on its
own.

## The connection from capture

The compose file publishes port 8123 on `127.0.0.1` only. Change that to the
address the Windows PC can reach.

**If that path is not local, terminate TLS.** Set `AUVC_CAPTURE_TLS_CERT` and
`AUVC_CAPTURE_TLS_KEY`, or put a reverse proxy in front that does. Without
either, capture credentials travel in the clear, and the bot says so in its log
on every start.

## What survives a restart

The volume at `/data` holds the guild configuration, the persistent player links
and the capture credentials. Everything else is rebuilt: a reconnecting capture
sends a complete snapshot of the round, so there is nothing about a running game
worth keeping across a restart.

Restarting the bot mid-round is therefore safe. Capture reconnects, sends the
snapshot, and AUVC picks the round up where it is — not where it was.

## Upgrading

```sh
docker compose pull        # or: docker compose build --pull
docker compose up -d --wait
```

Database migrations run at start and are applied in order. AUVC refuses to open
a database newer than the binary, so a downgrade stops with an error instead of
quietly working against a schema it does not understand. Back the volume up
before a major upgrade:

```sh
docker run --rm -v auvc_auvc-data:/data -v "$PWD":/backup alpine \
    tar czf /backup/auvc-data.tgz -C /data .
```

## Health

`GET /healthz` answers `ok`, or `503` naming what is wrong. It checks the two
things the bot cannot work without: the Discord connection and the database. A
check that only proved the process was running would be worth little, because a
bot that lost either is as useless as one that crashed — and only the crash
restarts itself.

## What is not here yet

No image has been published to `ghcr.io/tabsi1998/amongus-voice-controller`. The
compose file builds from this repository until one is.

The Windows capture app has an installer, but it is unsigned, and there is no
self-update. The
[Windows installation and release contract](../docs/windows-installation-and-releases.md)
defines what those need. Signing and updates wait on publisher identity and code
signing (#24), which is an owner decision rather than a technical one.
