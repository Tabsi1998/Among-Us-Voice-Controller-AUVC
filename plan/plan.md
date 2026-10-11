# AUVC – Analyse, Versions-Update & Issue-Abarbeitung

AUVC (Among Us Voice Controller) ist eine Windows-App mit eingebautem Discord-Bot, die Spieler in Discord passend zur Among-Us-Runde stummschaltet, taub stellt und verschiebt.
Diese Runde: Das Repo wird komplett importiert und analysiert, auf die aktuelle Steam-Version von Among Us vorbereitet und die offenen Issues werden – soweit ohne echten Windows-PC möglich – abgearbeitet.

## Für wen
- Den Repo-Owner (Tabsi1998), der AUVC weiterentwickelt und Richtung v1.0.0 bringen will.
- Indirekt: Discord-Gruppen, die Among Us mit automatischem Voice-Handling spielen.

## Kernumfang & Ergebnis
1. **Kompletter Import** von https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC (capture/ .NET, bot/ Go, protocol/, installer/, docs/, scripts/).
2. **Analyse-Bericht** (Deutsch, als Datei im Repo, z. B. `docs/analyse-rework.md`):
   - Architektur-Überblick und Zustand jedes Teils (Capture, Bot, Protokoll, Installer, Docs, CI).
   - Ehrliche Einschätzung: Stärken, Schwachstellen, technische Schulden, Risiken.
   - Liste veralteter, ungenutzter oder halbfertiger Teile (z. B. Upstream-Reste wie `.vs`, `.goreleaser.yml`, alte Offset-Werkzeuge, gelöschte Dateien, die noch referenziert werden) – **nur aufgelistet**, nichts wird ohne Freigabe entfernt.
   - Priorisierte Vorschläge für Rework, Erweiterungen und Verbesserungen (Funktionen, UX, Stabilität, Release/Verteilung).
3. **Among-Us-Versions-Update** (Issue „Offsets für den aktuellen 64-bit-Build 2026.9.29“):
   - Prüfen, wie die App die Spielversion erkennt und Offsets lädt (`Offsets.json`, AUOffsetManager).
   - Recherche öffentlich verfügbarer Offsets/Strukturdaten für den aktuellen Build; wo gefunden, werden sie eingetragen.
   - Wo Werte nur am laufenden Spiel ermittelt werden können: klare Schritt-für-Schritt-Anleitung bzw. Hilfsskript, damit du die Werte auf deinem PC auslesen und eintragen kannst.
   - Verständliche Meldung in der App, wenn eine unbekannte Spielversion erkannt wird.
4. **Issues abarbeiten**, sortiert nach Machbarkeit:
   - Direkt umsetzbar: `verify_bootstrap.py` verlangt vier im September gelöschte Dateien → Prüfskript an den aktuellen Stand anpassen; geführter Erststart & verständlicher Betriebsstatus in der Capture-Oberfläche (Konzept + Umsetzung, soweit ohne Windows-Test möglich).
   - Vorbereitbar: winget-/Microsoft-Store-Verteilung (Manifest-Entwurf, Release-Checkliste), v1.0.0-Release-Checkliste.
   - Nur manuell testbar (Live-Tests v0.1.3/v0.1.7, volle Lobby, Smoke-Test, Install/Update/Uninstall, Usability-Test): Testprotokolle/Checklisten werden aufbereitet, Durchführung liegt bei dir.
5. **Build- und Testlauf** im Container: Go-Bot bauen und testen, .NET-Projekte bauen und Unit-Tests ausführen, Protokoll-Fixtures prüfen.

## Ablauf für dich
1. Plan freigeben.
2. Repo wird importiert, gebaut, getestet, analysiert.
3. Du bekommst den Analyse-Bericht + Liste der Änderungen pro Issue.
4. Du entscheidest, welche gelisteten Altlasten entfernt und welche Vorschläge umgesetzt werden.
5. Live-Tests am eigenen Windows-PC mit Among Us; Rückmeldung fließt in Phase 2.

## Stil der Ergebnisse
- Bericht auf Deutsch, knapp, priorisiert (Muss / Sollte / Kann), mit Datei-Verweisen.
- Code-Änderungen im bestehenden Stil des Repos, kleine nachvollziehbare Schritte, bestehende Lizenzhinweise bleiben unangetastet.

## Phasen
**Phase 1 (jetzt, MVP)**
- Import, Build & Tests, vollständiger Analyse-Bericht mit Altlasten-Liste und Verbesserungsvorschlägen.
- Offsets-Update vorbereitet bzw. eingetragen (soweit recherchierbar) + Anleitung zum Selbst-Ermitteln.
- `verify_bootstrap.py` repariert; Erststart-/Status-Verbesserungen in der Oberfläche; Checklisten für alle manuellen Test-Issues.

**Phase 2**
- Umsetzung der von dir freigegebenen Bereinigungen und Rework-Punkte.
- Korrekturen aus deinen Live-Tests (Namen, Farben, Lebend/Tot, Mute-Timing).

**Phase 3**
- v1.0.0-Release vorbereiten, winget/Microsoft Store, automatische Offset-Updates ohne neues App-Release.

## Annahmen
- Kein Repo-Anhang vorhanden; es wird der genannte öffentliche GitHub-Link verwendet (Branch `main`).
- Die Arbeitsumgebung ist Linux ohne Among Us und ohne Windows-Oberfläche: WPF-UI und Spiel-Speicherauslesen können gebaut, aber nicht live getestet werden.
- Exakte Offsets für Build 2026.9.29 lassen sich ohne laufendes Spiel nur übernehmen, wenn sie öffentlich verfügbar sind; sonst wird nur die Ermittlung vorbereitet.
- Altlasten werden ausschließlich aufgelistet, nicht gelöscht (deine Vorgabe).
- Keine Discord-Bot-Tokens oder Secrets nötig, solange nichts live gegen Discord getestet wird.
- Änderungen landen im importierten Code; Hochladen nach GitHub erfolgt durch dich über „Save to GitHub“.
- Bericht und Kommunikation auf Deutsch.
