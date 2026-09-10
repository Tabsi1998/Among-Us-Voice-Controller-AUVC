# Third-party notices

Among Us Voice Controller (AUVC) contains code derived from the following
independent upstream projects. AUVC is a community rework and is not an
official AutoMuteUs project or an official new AutoMuteUs version.

| Project | Repository | License | Original copyright | Imported path | Original license copy |
| --- | --- | --- | --- | --- | --- |
| AutoMuteUs | https://github.com/automuteus/automuteus | MIT | Copyright (c) 2020 Denver Quane | `bot/` | [AutoMuteUs-MIT.txt](LICENSES/AutoMuteUs-MIT.txt) |
| AmongUsCapture | https://github.com/automuteus/amonguscapture | MIT | Copyright (c) 2020 Denver Quane | `capture/` | [AmongUsCapture-MIT.txt](LICENSES/AmongUsCapture-MIT.txt) |

The original [bot license](bot/LICENSE) and [capture license](capture/LICENSE)
are retained. Copies under `LICENSES/` are byte-for-byte copies of the
imported Git blobs. Original file-level notices and attribution remain intact.
See [UPSTREAM.md](UPSTREAM.md) for pinned commits, dates and the sync procedure.

The root [LICENSE](LICENSE) covers new AUVC contributions:
Copyright (c) 2026 IT-Tabelander. It does not replace upstream copyright notices.

`capture/Offsets.json` is an unchanged upstream data file from AmongUsCapture and
is embedded into the shipped `AUOffsetManager` assembly so capture can read the
game without contacting a third-party host. It carries the AmongUsCapture MIT
license and copyright listed above.

Dependency manifests and upstream assets remain unchanged in the baseline.
Their presence does not imply that every dependency or asset is MIT-licensed.
Dependency and distributable-asset license review is required before packaging
releases, with additional notices included as necessary.
