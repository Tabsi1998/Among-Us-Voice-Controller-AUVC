# AUVC – Arbeitsstand

## Aufgabe
Repo Tabsi1998/Among-Us-Voice-Controller-AUVC analysieren und weiterbauen (Owner: Fabian, Deutsch).

## Runde 1
- Analysebericht docs/analyse-rework.md, scripts/offsets_from_il2cpp.py, docs/offsets.md, live-tests.md, release-checklist.md, winget-Entwürfe
- Leser-Fixes (StructBytes, ReadString-Grenze), Diagnose nennt Spielversion

## Runde 2 (Okt 2026)
- Offsets für Among Us 2026.9.29 x64 Steam (Hash 615A95C9…) in capture/Offsets.json, aus Il2CppDumper-Dump der Owner-Dateien
- Offsets ohne Release: OffsetManager.FetchMissingAsync lädt capture/Offsets.json von main bei unbekanntem Spiel (Setting loadOffsetsFromGitHub)
- API-Server nur Loopback
- Kosmetik als Text unter Spielern (PlayerCosmetics)
- Altlasten entfernt (.vs, Video, AUOffsetHelper, verify_bootstrap.py, ...)

## Offen
- Live-Test mit 64-Bit-Build (docs/live-tests.md)
- MeetingStates-Verschiebung (Results=4) prüfen
- Kosmetik-Bilder (Originale aus Spieldateien?)
