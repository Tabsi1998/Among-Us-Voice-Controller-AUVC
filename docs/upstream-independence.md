# Independence from upstream infrastructure

Record for
[issue #44](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/44).

AUVC is self-hosted. It must keep working when the AutoMuteUs project changes,
moves or removes something on its own servers. This document tracks each runtime
dependency on infrastructure AUVC does not control, and how it was resolved.

This is not about hiding provenance. `UPSTREAM.md`, `LICENSES/` and
`THIRD_PARTY_NOTICES.md` keep every upstream reference, and the MIT license is
unchanged. It is about being able to run without asking anyone for permission.

## Resolved

### Game memory offsets

Capture reads the Among Us process using offsets keyed by the SHA-256 of the game
binary. Before this change `OffsetManager` obtained that index over HTTP only:

- the configured `IndexURL`, defaulting to
  `raw.githubusercontent.com/automuteus/amonguscapture/master/Offsets.json`
- on failure, a **hardcoded** fallback to
  `raw.githubusercontent.com/denverquane/amonguscapture/master/Offsets.json`,
  a second repository outside AUVC's control
- on failure of both, a cache under
  `%APPDATA%\AmongUsCapture\indexCache.json` that only exists after an earlier
  successful download

A fresh installation with no network access, or one made after either file
disappeared, therefore had **no offsets at all** and could not read the game —
even though `capture/Offsets.json` was already committed to this repository and
used as a test fixture. It was built, but never read at runtime.

`capture/Offsets.json` is now embedded in `AUOffsetManager` as
`AUOffsetManager.Offsets.json` and loaded as the base layer. The resolution order
is:

1. `LocalOffsetIndex` — a user-supplied `%APPDATA%\AmongUsCapture\index.json`,
   unchanged and still highest priority
2. entries merged on top of the bundled index: a successful remote refresh, or
   otherwise the cache from an earlier run
3. the embedded index, always present

Remote refresh is now **opt-in**: `IndexURL` defaults to empty, so no request is
made unless an operator configures one. The hardcoded third-party fallback is
gone; a failed refresh logs and keeps the bundled index instead of reaching for
someone else's server.

Merging rather than replacing matters: a remote index that is a subset can no
longer remove game versions AUVC already knows.

`BundledOffsetIndexTests` fails if the resource stops being embedded, if it drifts
from `capture/Offsets.json`, or if a known hash stops resolving with an empty
`IndexURL`.

Note on the file format: `Offsets.json` uses hexadecimal literals such as
`0x22F4F10`. Newtonsoft.Json accepts these; strict RFC 8259 parsers, including
`python -m json.tool`, do not. The file is kept byte-identical to the imported
upstream blob rather than reformatted.

## Open

| Dependency | Where | Planned handling |
| --- | --- | --- |
| Map images | `bot/pkg/game/map.go`, `DefaultMapsUrl` | Images already exist under `bot/assets/maps/`, but Discord renders embed images from a URL, so serving them locally means uploading attachments and referencing `attachment://`. Own change. |
| Hats, pets, pants, sounds | `capture/AUCapture-WPF/Converters/*`, `App.xaml.cs` | Fetched from `CDN.automute.us`. Needs the asset licence review that [#33](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/33) already requires; then embed, replace or drop. |
| Capture download link | `bot/bot/command/commands.go`, `CaptureDownloadURL` | Points at the upstream download site. Can only move once AUVC has its own release channel, see [#24](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/24). |
| Product links | `command/help.go`, `info.go`, `privacy.go`, `setting/language.go` | Website, invite, Discord, privacy policy and Crowdin of the upstream project. Product-facing text belongs to #33. |
