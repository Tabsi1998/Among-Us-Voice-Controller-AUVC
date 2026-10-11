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

## Runde 3 (Okt 2026)
- Meeting-Timing: MeetingProceedingState pro Eintrag (5 für 2026.9.29), GameMemReader.RoundState
- Epic/Store: Signatursuche (scripts/signatures_from_build.py, capture/AUOffsetManager/Signatures.json, SignatureIndex) – gegen echte Steam-DLL geprüft
- Rundenstatistik: game_ended.result (Protokoll), Bot postet Sieger/Grund/Impostor in Steuerkanal (bot/round_result.go)
- Offen: Kosmetik-Bilder (braucht StreamingAssets/aa-Bundles + Unity-Bundle-Leser), Live-Tests

## Runde 4 (Okt 2026)
- /au settings round (round_results, Standard an), /au stats (+ reset für Admins), Migration 0007 (player_stats)
- scripts/cosmetics_inventory.py: kleine JSON-Übersicht des aa-Ordners zum Hochladen
- Test-Lockfile ohne AUOffsetHelper neu erzeugt (sonst schlägt --locked-mode im Release fehl)
- EXE: Owner pusht per Save to GitHub, dann Actions → Release → Run workflow → Artefakt capture-payload
