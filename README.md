# Among Us Voice Controller (AUVC)

AUVC mutes, deafens and moves players in Discord to match an Among Us round. It
runs on the one Windows PC that plays Among Us, with its Discord bot inside the
app. Nobody else installs anything.

AUVC is an independent community rework of AutoMuteUs and AmongUsCapture.
It is not an official AutoMuteUs project or an official new AutoMuteUs version.

**Deutsch:** Die vollständige Anleitung auf Deutsch steht in
[docs/anleitung.md](docs/anleitung.md).

## Get started

1. Download `AmongUsVoiceCapture-Setup-win-x64.exe` from the
   [releases page](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/releases).
2. Install it and start **AUVC Capture**.
3. Follow the setup window: paste a Discord bot token, invite the bot, choose
   the server and the channels.
4. Every player links themselves once in Discord with `/au link`.
5. Play.

The [user guide](docs/guide.md) walks through every step and every setting, and
says what to do when something does not work.

## What players hear and see

| Phase | Living players | Dead players |
| --- | --- | --- |
| Lobby | Main channel, can talk | Main channel, can talk |
| Tasks, death not yet announced | Main channel, muted and deafened | Stay where they are, muted |
| Tasks, death announced | Main channel, muted and deafened | Ghost channel, can talk |
| Meeting and voting | Main channel, can talk | Ghost channel, can talk |
| Round over | Main channel, can talk | Main channel, can talk |

A channel change is visible to everyone in Discord, so a killed player does not
move until the next meeting announces the death. A player who is voted out moves
straight away, because everyone saw it happen.

## Status

The current version is the pre-release `v0.1.0-beta`. It has not yet been tried in
a real round with Among Us and Discord, and the files are not signed, so Windows
warns about an unknown publisher. Choosing a crewmate in Discord instead of typing
`/au link` comes with `v0.1.1-beta`
([#105](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/105)).
`v1.0.0` will be the first finished release
([#18](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/18)).

## Documentation

| For | Read |
| --- | --- |
| Using AUVC | [User guide](docs/guide.md) · [Anleitung (Deutsch)](docs/anleitung.md) |
| What is stored, and how to remove it | [docs/privacy.md](docs/privacy.md) |
| Security, and reporting a vulnerability | [SECURITY.md](SECURITY.md) |
| How it works | [docs/architecture.md](docs/architecture.md), [protocol/README.md](protocol/README.md) |
| What the tests prove, and the manual smoke test | [docs/acceptance.md](docs/acceptance.md) |
| Building, testing and releasing | [docs/development.md](docs/development.md) |
| What changed | [CHANGELOG.md](CHANGELOG.md) |

## Repository layout

| Path | Contains |
| --- | --- |
| `capture/` | The Windows app: game reading, the setup and settings windows, and the connection to the bot (.NET) |
| `bot/` | The Discord bot the app runs (Go) |
| `protocol/` | The contract between app and bot, with fixtures both sides test against |
| `installer/` | The installer script |
| `docs/` | User guides and technical documentation |
| `scripts/` | Repository checks and release helpers |
| `LICENSES/` | Exact copies of the upstream licences |

## Upstream and Credits

- [AutoMuteUs](https://github.com/automuteus/automuteus), MIT,
  Copyright (c) 2020 Denver Quane.
- [AmongUsCapture](https://github.com/automuteus/amonguscapture), MIT,
  Copyright (c) 2020 Denver Quane.

Original notices remain in `bot/LICENSE` and `capture/LICENSE`, with byte-for-byte
copies under `LICENSES/`. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
and [UPSTREAM.md](UPSTREAM.md) for attribution, exact SHAs, dates and how
upstream changes are reviewed and ported. The crewmate look belongs to Innersloth,
the makers of Among Us. Thanks to the upstream authors and contributors.

## License

New AUVC components: MIT, Copyright (c) 2026 IT-Tabelander. See [LICENSE](LICENSE).
Upstream components retain their original notices and applicable licenses.
