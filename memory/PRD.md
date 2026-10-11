# AUVC – Arbeitsstand

## Aufgabe
Repo Tabsi1998/Among-Us-Voice-Controller-AUVC analysieren, Rework-Vorschläge, Among-Us-64-Bit-Update (#198), Issues abarbeiten (#194, #21, #131, #18, Live-Tests).

## Erledigt (Okt 2026)
- Analysebericht docs/analyse-rework.md (Altlasten nur gelistet)
- scripts/offsets_from_il2cpp.py + Tests, docs/offsets.md (64-Bit-Offsets aus Il2CppDumper)
- Leser: StructBytes-Aufrundung (v18 _object-Bug), ReadString-Längenschutz, 64-Bit- und v18-Aufzeichnungstests
- Diagnose nennt GameAssembly-Hash/Bitbreite/Offsets
- verify_bootstrap.py repariert (#194) + Tests
- docs/live-tests.md, docs/release-checklist.md, installer/winget/ Entwürfe

## Offen
- Echte Offsets für 2026.9.29 x64 am PC ermitteln (#198)
- API-Server auf Loopback (Sicherheit)
- Offset-Index ohne Release / Signatursuche
- Live-Tests #141 #142 #144 #136 #157
