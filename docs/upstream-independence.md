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

### Map images

`FormMapUrl` fell back to `DefaultMapsUrl`, a hardcoded
`raw.githubusercontent.com/automuteus/automuteus/.../assets/maps/`. The images
were already committed under `bot/assets/maps/`, but nothing read them, and the
runtime Docker image never copied that directory, so reading from disk was not an
option either.

The eleven images moved to `bot/pkg/game/maps/` and are embedded with `//go:embed`.
That is the only delivery that works for both the container and a bare binary,
and it costs about 3.8 MB in a 26 MB binary. `DefaultMapsUrl` is gone: `FormMapUrl`
now returns an empty string unless an operator sets `BASE_MAP_URL`.

`/map` answers from the embedded image, attached to the response as a file. It
needs no network and no configuration.

**The game-state thumbnail is deliberately not converted.** It is decorative, and
the message carrying it is edited on nearly every game event —
`DispatchRefreshOrEdit` is called from more than ten places in `eventHandler.go`.
Attaching an image up to 1 MB on every edit would be wasteful where Discord
currently caches one URL server-side, and reworking attachment retention across
edits touches the most exercised path in the bot without any way to verify it
against a live guild here.

So the thumbnail now appears only when `BASE_MAP_URL` is configured, and is
omitted otherwise. The silent dependency is gone either way; what remains is an
opt-in. Restoring it as an attachment belongs with the game-state message rework
in [#33](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/33),
where it can be tested against a real guild.

Incidentally, `airship.png` and `airship_detailed.png` are byte-identical
upstream. Both are kept so the naming scheme stays predictable.

### Guarding against regression

`scripts/check_upstream_references.py` runs in CI and fails when a new reference
to `automute.us`, `raw.githubusercontent.com/automuteus`,
`raw.githubusercontent.com/denverquane` or the upstream Crowdin project appears
in the source.

The references that still exist are recorded in
`scripts/upstream_references_baseline.txt`, so the check passes today while
refusing anything new. Forty entries at the time of writing, twenty-five of them
translated product text in `bot/locales`.

The list may only shrink. Removing a reference makes the check fail on the stale
baseline entry, which keeps the file honest; regenerate it with:

```sh
python scripts/check_upstream_references.py --update
```

Provenance is deliberately exempt. `UPSTREAM.md`, `LICENSES/`,
`THIRD_PARTY_NOTICES.md` and the documentation name upstream on purpose. This
check is about what the running product reaches for, not about hiding where the
code came from.

## Open

| Dependency | Where | Planned handling |
| --- | --- | --- |
| Hats, pets, pants, sounds | `capture/AUCapture-WPF/Converters/*`, `App.xaml.cs` | Fetched from `CDN.automute.us`. Needs the asset licence review that [#33](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/33) already requires; then embed, replace or drop. |
| Capture download link | `bot/bot/command/commands.go`, `CaptureDownloadURL` | Points at the upstream download site. Can only move once AUVC has its own release channel, see [#24](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/24). |
| Product links | `command/help.go`, `info.go`, `privacy.go`, `setting/language.go` | Website, invite, Discord, privacy policy and Crowdin of the upstream project. Product-facing text belongs to #33. |
