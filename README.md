# Among Us Voice Controller (AUVC)

AUVC is an independent community rework of AutoMuteUs and AmongUsCapture.
It is not an official AutoMuteUs project or an official new AutoMuteUs version.

Repository: https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC

## Project status

The repository contains the original upstream imports plus a build/test baseline.
AUVC is not yet a working standalone replacement. The applications retain legacy
behavior and dependencies; the new runtime features below are planned.

The full capture solution now builds locally and its 11 regression tests pass.
Root CI checks Go, Windows capture, Docker, provenance and secrets. See the
[current build guide](docs/baseline-build.md) for commands, merge procedure and
remaining legacy warnings, and the [historical import report](docs/bootstrap-validation.md)
for the original findings. Current Among Us/Discord gameplay has not been verified.

Development follows the [20-phase roadmap](docs/roadmap.md) and
[v1.0.0 milestone](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/milestone/1).
Each phase uses a separate feature branch and PR.

AUVC will receive an independent logo, visual system and complete Capture/Discord/
installer styling rework. The [branding plan](docs/branding-and-design.md) requires
multiple previews and explicit owner approval before existing assets are replaced.

## How it works

Target: Among Us → AmongUsVoiceCapture.exe → authenticated WSS → AUVC Bot →
Discord API. Only one Windows computer runs capture. Other players need no mods
or additional software. The bot is self-hosted.

The [architecture](docs/architecture.md) separates protocol validation, Game State,
Voice Policy and the Discord Reconciler. Configuration will persist in SQLite.
Galactus, Redis, PostgreSQL and premium/public-worker infrastructure are scheduled
for removal in separate phases.

## Repository layout

| Path | Responsibility |
| --- | --- |
| `bot/` | Unmodified AutoMuteUs baseline; future Discord/server application |
| `capture/` | AmongUsCapture import with baseline build repairs and regression tests |
| `protocol/` | Planned versioned schemas and compatibility fixtures |
| `deploy/` | Planned Docker and self-hosting configuration |
| `docs/` | Requirements, architecture, development and roadmap |
| `LICENSES/` | Exact copies of original upstream licenses |
| `scripts/` | Bootstrap verification |

## Prerequisites

Current baseline checks use Go 1.19.13, Windows and .NET SDK 8.0.424 to build
the imported .NET 5 projects. Python 3.11+ runs the bootstrap verifier.
These are baseline observations, not the final supported AUVC toolchains.
See [development](docs/development.md) for commands and limitations.

## Discord Bot Setup

AUVC-specific setup is not implemented yet. The target bot needs a Discord
application and effective View Channel, Connect, Move Members, Mute Members and
Deafen Members permissions in the configured channels. Administrative commands
will be limited to authorized users/roles.

Do not commit a real bot token or populated environment file. Historical setup
instructions in [bot/README.md](bot/README.md) describe upstream, not a supported
AUVC installation.

## Capture Setup

The planned recommended download is a graphical
`AmongUsVoiceCapture-Setup-win-x64.exe` installer, with MSI and self-contained
portable ZIP alternatives. Install, follow the first-run guide, pair using an
expiring `/au capture pair` code and play. No BAT, terminal or manual .NET
installation should be needed. Long-term credentials will be securely stored on
Windows and revocable. See the [installation/release plan](docs/windows-installation-and-releases.md).
This flow and these artifacts are not available yet. The imported source and historical
instructions are in [capture/](capture/README.md).

## Ghost Channel behavior

The planned default preset is `ghost-chat`:

| Phase | Living players | Ghosts |
| --- | --- | --- |
| Lobby | Main, open | Reset to main, open |
| Tasks | Main, muted and deafened | Ghost, open |
| Discussion / meeting / voting | Main, open | Ghost, open |
| Ended | Main, open | Main, open |

Ghosts can talk together throughout the active round. Channel enforcement
defaults to enabled. Only linked human players are managed by default.
Capture timeout defaults to unmute/undeafen, pause and warn.

## Slash Commands

Planned groups: `/au setup`, `/au settings`, `/au capture`, `/au session`.
Planned individual commands: `/au link`, `/au unlink`, `/au doctor`, `/au version`.
The [complete command contract](docs/requirements.md#discord-commands-and-permissions)
defines typed options and authorization. These commands are not implemented yet.

## Docker Installation

No supported AUVC image or Compose installation exists yet.
The [deployment plan](deploy/README.md) uses a persistent `/data` volume for
`/data/amongus.db` and direct authenticated WSS.

## Upgrade

There is no released AUVC version to upgrade yet. Future releases must provide
versioned migration and backup/recovery instructions. The planned GUI updater
defaults to Stable; Preview is opt-in. Verified updates preserve settings/pairing
and must not interrupt active rounds. Release preparation and tag-triggered
EXE/MSI/ZIP builds, signing and publication will be automated. Never replace a
runtime database with source-controlled configuration.

## Troubleshooting and /au doctor

For build setup and known limitations see [the baseline guide](docs/baseline-build.md).
`/au doctor` is planned to diagnose Discord, SQLite/migrations, channels,
permissions, capture/protocol/heartbeat, game-state detection and build version
using ✅ / ⚠️ / ❌. Report problems with redacted diagnostics and exact versions.

## Security

See [SECURITY.md](SECURITY.md). Pairing, revocation, WSS and fail-safe behavior
are requirements awaiting implementation and acceptance tests.

## Privacy

The target design requires guild settings, player links and game/session data
to manage Discord voice; audio recording is not required. Retention and deletion
behavior must be documented before release. Imported upstream privacy statements
are historical references, not a finished AUVC privacy policy.

## Upstream and Credits

- [AutoMuteUs](https://github.com/automuteus/automuteus), MIT,
  Copyright (c) 2020 Denver Quane.
- [AmongUsCapture](https://github.com/automuteus/amonguscapture), MIT,
  Copyright (c) 2020 Denver Quane.

Original notices remain in `bot/LICENSE` and `capture/LICENSE`, with byte-for-byte
copies under `LICENSES/`. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
and [UPSTREAM.md](UPSTREAM.md) for attribution, exact SHAs, dates and sync rules.
Thanks to the upstream authors and contributors.

## License

New AUVC components: MIT, Copyright (c) 2026 IT-Tabelander. See [LICENSE](LICENSE).
Upstream components retain their original notices and applicable licenses.
