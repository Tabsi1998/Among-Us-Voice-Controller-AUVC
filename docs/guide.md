# AUVC user guide

Everything you need to set up and use AUVC. **Deutsch:** [anleitung.md](anleitung.md).

- [What you need](#what-you-need)
- [1. Download](#1-download)
- [2. Install or unpack](#2-install-or-unpack)
- [3. First setup](#3-first-setup)
- [4. Link the players](#4-link-the-players)
- [5. Play](#5-play)
- [6. Settings](#6-settings)
- [7. Discord commands](#7-discord-commands)
- [8. Updating](#8-updating)
- [9. Troubleshooting](#9-troubleshooting)
- [10. Uninstalling and removing data](#10-uninstalling-and-removing-data)
- [11. A bot on another computer](#11-a-bot-on-another-computer)

## What you need

- **The Windows PC that plays Among Us**, with 64-bit Windows 10 or 11. AUVC and
  its Discord bot both run there. No .NET or anything else needs installing.
- **A Discord account** that may add bots to your Discord server, which needs the
  *Manage Server* permission.
- **Two voice channels** on that server: one for everyone alive and one for the
  ghosts. A text channel for AUVC's notices is optional.
- **Everyone else** needs only Discord. Other players install nothing.

AUVC's bot is online while AUVC is running on that PC, and offline when it is
closed.

## 1. Download

Open the [releases page](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/releases)
and pick the newest version. While the repository is private, you have to be
signed in to GitHub with an account that has access.

| File | What it is |
| --- | --- |
| `AmongUsVoiceCapture-Setup-win-x64.exe` | The installer. Recommended. |
| `AmongUsVoiceCapture-win-x64.zip` | The portable version: unpack and run, installs nothing |
| `SHA256SUMS` | Checksums, to confirm a download is the file the release lists |

Versions ending in `-beta` are pre-releases for testing.

**Windows will warn you.** The files are not signed, so Windows shows *"Windows
protected your PC"*. Click **More info**, then **Run anyway**.

## 2. Install or unpack

**Installer.** Run it and follow it. It installs for your Windows account only,
without administrator rights, and adds **AUVC Capture** to the Start menu. The
last page can start AUVC straight away.

**Portable.** Unpack the zip into a folder of its own, for example
`Documents\AUVC`, and start `AUCapture-WPF.exe`. Keep the folder complete: the
bot lives in its `bot` subfolder.

Both versions keep their settings in the same place, so you can switch between
them.

## 3. First setup

The first time AUVC starts, the setup window opens by itself. You can open it
again at any time with the **Set up** button at the top of the main window.

### Step 1 of 4: the bot token

AUVC needs a Discord bot of its own. It is free and takes two minutes.

1. Click **Open the Developer Portal**, or open
   [discord.com/developers/applications](https://discord.com/developers/applications),
   and sign in with your Discord account.
2. Click **New Application**, give it a name such as *AUVC*, accept the terms and
   click **Create**.
3. Open **Bot** on the left. Click **Reset Token**, confirm, then click **Copy**.
4. Back in AUVC, paste the token and click **Check token**.

AUVC asks Discord whether the token works and shows the bot's name. It stores the
token encrypted, for your Windows account and on this PC only.

- **The token is a password.** Anyone who has it controls your bot. Never send it
  to anyone, and never post it with a screenshot or a log.
- **No redirect or callback URL is needed**, and no *Privileged Gateway Intents*
  either. Leave those settings as they are.
- If AUVC says **Requires OAuth2 Code Grant** is on, switch that option off on the
  **Bot** page, otherwise the invite fails.

Click **Next**. AUVC now starts the bot in the background.

### Step 2 of 4: invite the bot and choose the server

1. Click **Invite the bot**. Your browser opens Discord's invite page.
2. Choose your server under *Add to server* and click **Authorize**.
3. Back in AUVC, the server appears in the list within a few seconds. Click
   **Refresh** if it does not.
4. Select the server and click **Next**.

The invite asks for exactly what AUVC uses: *View Channels*, *Send Messages*,
*Embed Links*, *Use External Emojis*, *Connect*, *Mute Members*, *Deafen Members*
and *Move Members*. The first four let AUVC post the crewmate menu in the text
channel.

### Step 3 of 4: the channels

| Field | Choose |
| --- | --- |
| **Main channel (voice)** | The voice channel everyone plays in |
| **Ghost channel (voice)** | The voice channel the dead move to |
| **Text channel for choosing crewmates and for notices (recommended)** | Where players choose their crewmate, and where AUVC posts warnings, for example when the game is no longer detected |

**Start AUVC automatically when a game is detected** is on. Leave it on, and AUVC
starts working as soon as you play. Click **Next** to save.

If the lists are empty, the server has no voice channels yet: create two in
Discord and click **Refresh**.

### Step 4 of 4: done

AUVC connects to its bot and lists the bot's checks:

- ✅ fine
- ⚠️ something to know, usually a step not done yet, such as *no game data yet*
  before anyone has played
- ❌ something to fix, with what to do after the arrow

Click **Finish**.

## 4. Link the players

AUVC has to know which Discord member plays which crewmate, so every player tells
it once. Choosing a crewmate from a menu is new in `v0.1.1-beta`; with
`v0.1.0-beta`, type your name as described under **With a command**.

**In the text channel.** As soon as the app sees a lobby, AUVC posts a message in
the text channel chosen during setup. It shows the map, the phase and the lobby
code, unless the host hides it, then every crewmate in the lobby, who
has already picked which one, and a menu **Choose your crewmate**. Pick your own
figure there. Picking another one moves your link, and **Unlink me** removes it.
Only you see AUVC's answer.

**With a command.** In any text channel of the server, type `/au link` on its own
to get the same menu, visible only to you. Or type your name exactly as it
appears in Among Us, including capital letters:

```text
/au link player:Name
```

**In the app.** On the PC that runs AUVC, **Bot** → **Players** links any crewmate
to anybody in a voice channel of the server, for somebody who cannot pick for
themselves.

A link is remembered, so it only has to be done again when someone changes their
name in the game. A link made during a round takes effect at once.

Server administrators can link someone else with `/au link player:Name user:@Person`.
`/au unlink` removes your own link.

The message never gives anything away: a killed crewmate turns into a ghost on it
only when a meeting announces the death. When AUVC closes, the message is deleted,
and it comes back with the next lobby. The first time, AUVC uploads the crewmate
pictures to your bot, which can take a minute; until then the menu shows names
and colours. Without a text channel there is no message, and players use
`/au link`.

Players without a link are left alone: AUVC never mutes or moves them. Neither
does it touch bots such as music bots.

## 5. Play

1. Start AUVC. The line at the top of the main window always says what AUVC is
   waiting for and what to do next, for example *Waiting for Among Us: Start
   Among Us on this PC*. Once everything is in place it says *Ready* and how many
   players in the lobby are linked.
2. Start Among Us. As soon as you are in a lobby, AUVC shows the players. Under
   each name it says whether the player is linked and whether AUVC has muted or
   deafened them or moved them into the ghost channel, for example *linked ·
   muted · deafened*.
3. Everyone joins the main voice channel.

From then on AUVC follows the game on its own:

| Phase | Living players | Dead players |
| --- | --- | --- |
| Lobby | Main channel, can talk | Main channel, can talk |
| Tasks, death not yet announced | Main channel, muted and deafened | Stay where they are, muted |
| Tasks, death announced | Main channel, muted and deafened | Ghost channel, can talk |
| Meeting and voting | Main channel, can talk | Ghost channel, can talk |
| Round over | Main channel, can talk | Main channel, can talk |

- **Kills are not given away.** Moving a player to another channel is visible to
  everyone in Discord, so a killed player stays where they are, muted, until the
  next meeting announces the death.
- **Being voted out is public anyway**, so that player moves to the ghost channel
  straight away.
- **Closing AUVC** releases everyone and takes the bot offline.
- **If AUVC crashes**, its bot stops with it and cannot release anyone at that
  moment. The next time AUVC starts, the bot releases everyone it had left
  muted, deafened or in the ghost channel, and nobody else. People who are not
  in voice then are released as soon as they join. To release someone straight
  away, right-click them in Discord and switch off *Server Mute* and
  *Server Deafen*.

## 6. Settings

### Bot settings: the Bot button

Once AUVC is set up, the button at the top of the main window reads **Bot**. It
opens everything the setup chose, as five sections you can change one at a time:

| Section | What you can do |
| --- | --- |
| **Token** | Paste a new token and click **Check token**. The bot restarts with it. |
| **Server** | Choose another server and click **Use this server**, then choose its channels under **Channels**. **Invite the bot** adds the bot to another server first. |
| **Channels** | Change the main, ghost and text channel and automatic start, then click **Save**. |
| **Status** | The bot's checks. **Restart the bot** restarts it. **Stop running the bot on this PC** keeps AUVC from starting it; the token and the settings stay stored, and **Set up** switches it back on. |
| **Players** | Every crewmate in the current lobby, each with a menu of the members in the server's voice channels. Choose a member to link them, or *(nobody)* to remove the link. It takes effect at once. |

### App settings: the gear button

AUVC uses the display language of Windows when that is German or English, and
English otherwise. Choose another under **General → Language**; **Same as
Windows** goes back.

| Tab | Setting | What it does |
| --- | --- | --- |
| General | Language | **Same as Windows**, **Deutsch** or **English** |
| General | Always copy game code | Copies the lobby code to the clipboard whenever you join a lobby |
| General | Startup memes | Now and then shows a joke splash screen with a sound on start. The sound is downloaded from the original AutoMuteUs server; switch this off if you do not want that |
| General | Focus window on connect | Brings the window to the front when a pairing link opens AUVC |
| General | API Server | Starts a local interface for overlay tools. Leave it off unless a tool asks for it |
| General | Always on top | Keeps the AUVC window above other windows |
| Debug | Debug mode | Opens an extra console window with technical output on the next start |
| Debug | Open log folder | Opens the folder with AUVC's log files |
| Debug | Reload offsets | Reloads the memory offsets AUVC reads the game with, after an Among Us update |
| Debug | Reset Config | Deletes the app's settings and offers to restart. The bot token stays stored; run **Set up** again afterwards |
| About | App version | The installed AUVC version |
| About | Latest version | Still shows the newest version of the original AmongUsCapture, not of AUVC ([#44](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/44)) |

Keyboard shortcuts in the main window: **Ctrl+L** opens the log folder, **F2**
copies the newest log to the clipboard, **Ctrl+R** restarts the app.

### Game settings in Discord

These are changed with `/au settings` in Discord, by the server owner, a Discord
administrator, or the role set with `/au setup permissions`. Discord suggests the
options as you type.

| Setting | Default | What it does |
| --- | --- | --- |
| `auto_move_ghosts` | on | Move the dead to the ghost channel |
| `enforce_channels` | on | Send players back when they switch channels themselves |
| `capture_timeout_seconds` | 60 | How long the bot waits when the app stops reporting to it before it acts (10–600) |
| `capture_timeout_action` | `fail-open` | `fail-open` unmutes everyone and brings them back; `pause` leaves everyone as they are |
| `auto_start` | on after setup | Start as soon as a game is detected |
| `language` | Same as Discord | The language of the crewmate message and the notices in the text channel. *Same as Discord* follows the server language set in Discord, which is English on a server without the Community feature |

`/au settings show` lists the current settings.

## 7. Discord commands

Every answer is visible only to whoever typed the command. It comes in your
Discord language: German when your Discord is set to German, English otherwise.
Discord shows the command descriptions the same way. The crewmate message and
the notices in the text channel are read by everyone, so they come in the
server's language (`/au settings language`).

| Command | What it does |
| --- | --- |
| `/au link` | Choose your crewmate from a menu only you see |
| `/au link player:<name>` | Link yourself to your name in Among Us |
| `/au unlink` | Remove your link |
| `/au session status` | Whether AUVC is currently managing voice |
| `/au session start`, `stop`, `pause`, `resume` | Start or stop managing voice by hand. `stop` releases everyone, `pause` leaves everyone as they are |
| `/au doctor` | Check everything and say what is missing |
| `/au settings show`, `voice`, `ghosts`, `safety`, `language`, `preset`, `export` | See and change the game settings |
| `/au setup channels` | Choose the channels in Discord instead of in the app |
| `/au setup permissions` | A role, besides administrators, that may change AUVC |
| `/au setup reset` | Restore the default settings; links are kept |
| `/au capture status`, `pair`, `revoke` | The app's connection to the bot. Only needed for a bot on another computer |
| `/au version` | The bot's version |

## 8. Updating

**Installer.** Download the new setup file and run it. It updates AUVC in place;
settings, token and links stay. Close AUVC first.

**Portable.** Close AUVC, delete the old folder and unpack the new zip. Settings
and data are not kept in that folder, so nothing is lost.

## 9. Troubleshooting

### Setup

| Problem | What to do |
| --- | --- |
| *Discord does not accept this token* | Copy the token again on the **Bot** page of the Developer Portal, after **Reset Token** if needed |
| *The bot stopped right after starting* | Almost always the token. Go back to step 1 and check it. Otherwise check the internet connection |
| The server does not appear | Invite the bot with **Invite the bot**, then click **Refresh** |
| The invite page shows an error | Switch off **Requires OAuth2 Code Grant** on the **Bot** page |
| *The bot program is missing* | Reinstall AUVC, or unpack the portable version completely |

### In Discord

| Problem | What to do |
| --- | --- |
| `/au` does not appear | Press **Ctrl+R** in Discord to reload it. If it still does not appear, invite the bot again with **Invite the bot** |
| Nobody is muted | Is AUVC running and does it show the game? Check `/au session status`, and the **Status** section behind **Bot** |
| One player is not muted | They are not linked, or linked to another crewmate. Check the crewmate message in the text channel, or run `/au link` again |
| The crewmate message does not appear | Choose a text channel behind **Bot** → **Channels**. Give the bot's role *View Channels*, *Send Messages* and *Embed Links* in it. The message appears once the app sees a lobby |
| AUVC reports missing permissions | Give the bot's role *View Channels*, *Connect*, *Mute Members*, *Deafen Members* and *Move Members* on both voice channels |
| Someone stays muted | While AUVC runs, `/au session stop` releases everyone. After a crash, start AUVC again: it releases everyone it left muted. Or right-click the person in Discord and switch off *Server Mute* and *Server Deafen* |

### In the game

| Problem | What to do |
| --- | --- |
| *Waiting for Among Us* does not go away | Make sure Among Us is running, on this PC and under the same Windows account |
| The game is detected but no players appear | Among Us may have been updated. **Settings → Debug → Reload offsets**, then restart AUVC |

### Log files

If you ask for help, include these files and the output of `/au doctor`. Neither
contains the token.

| File | What it is |
| --- | --- |
| `%AppData%\AmongUsCapture\logs\latest.log` | The app's log |
| `%LOCALAPPDATA%\AUVC\logs\logs.txt` | The bot's log |

Paste the path into the Windows Explorer address bar to open it.

## 10. Uninstalling and removing data

**Installer.** Windows **Settings → Apps → Installed apps → AUVC Capture →
Uninstall**. It asks whether to delete AUVC's data as well: settings, bot token,
the bot's database with channels and links, and the logs. Choose **No** to keep
them for a later reinstall.

**Portable.** Delete the folder. To remove the data too, delete
`%LOCALAPPDATA%\AUVC` and `%AppData%\AmongUsCapture`.

Deleting the data does not delete the bot in Discord. To be sure the token can
never be used again, click **Reset Token** on the **Bot** page of the Developer
Portal. To remove the bot from your server, kick it from the member list.

What AUVC stores, and where, is described in [privacy.md](privacy.md).

## 11. A bot on another computer

Normally the bot runs inside AUVC. If a bot already runs elsewhere, for example on
a server that is always on, AUVC can connect to it instead:

1. Close the setup window.
2. In Discord, an administrator runs `/au capture pair` and gets a code.
3. In AUVC, click the button with the tooltip **Pair with the AUVC bot**, enter
   the bot's address and the code, and click **Pair**.

Releases contain only the app. Running the bot on its own is described in
[development.md](development.md).
