# AUVC – Analyse, Rework- und Erweiterungsvorschläge

Stand: Oktober 2026, `main` bei `08fab77` (#197). Ein Blick von außen auf das
ganze Repository, mit dem, was in dieser Runde geändert wurde, und dem, was ich
als Nächstes tun würde. Nichts in der Liste der Altlasten ist gelöscht; das
entscheidest du.

## Kurz

- **Der Zustand ist gut.** Klare Aufteilung (App, Bot, Protokoll), ehrliche
  Doku, eine lokale Prüfung mit 17 Schritten, CI auf Linux und Windows. Der Bot
  ist reine Logik mit sehr guter Testabdeckung (etwa so viele Zeilen Tests wie
  Code).
- **Das größte Produktrisiko ist der Spielspeicher.** Jede neue Among-Us-Version
  macht AUVC blind, bis jemand Offsets einträgt und ein neues Release baut. Das
  ist genau der Fall von #198. Dafür gibt es jetzt ein Werkzeug; die Abhängigkeit
  vom Release sollte als Nächstes weg.
- **Die App ist der schwächere Teil.** Sie trägt noch viel Code aus
  AmongUsCapture (Singletons, `while (true)`, Code-behind von fast 1000 Zeilen,
  ein offener WebSocket-Server) und ist außerhalb von `AUVC.Transport` kaum
  testbar.
- **Bis v1.0.0 fehlen nur Live-Tests**, und die kann nur ein Mensch machen.

## Was im Container geprüft wurde

Linux (arm64), ohne Among Us, ohne Discord, ohne Windows-Oberfläche.

| Prüfung | Ergebnis |
| --- | --- |
| `go vet ./...`, `go test ./...` (Go 1.27.2) | alle 21 Pakete grün |
| `go build` des Bots | gebaut |
| `dotnet build AmongUsCapture.sln -c Release` (.NET SDK 10.0.401, `EnableWindowsTargeting`) | gebaut, 0 Fehler, nur alte Warnungen aus der WPF-App |
| `dotnet test AUVC.Capture.Tests` | 371 von 372 grün |
| `dotnet format whitespace --verify-no-changes` | sauber |
| Python-Skripttests (`unittest discover -s scripts`) | 62 grün, davon 16 neu |
| `verify_repository.py`, `verify_bootstrap.py`, `check_upstream_references.py` | PASS |

Der eine rote C#-Test, `BotHostTests.TheEnvironmentLeavesNothingToChance`,
erwartet einen Windows-Pfad (`C:\data\amongus.db`) und bekommt auf Linux
`C:\data/amongus.db`. Das ist eine Eigenheit der Umgebung, kein Fehler: die CI
führt die C#-Tests auf `windows-2022` aus. Drei weitere Tests dieser Art
(`DiagnosticsTests`) waren auch rot; sie zeigten aber eine echte, kleine Lücke
(siehe unten) und sind jetzt grün.

Nicht geprüft werden konnte: die WPF-Oberfläche selbst, das Lesen eines
laufenden Among Us, Discord, der Installer (Inno Setup).

## Architektur und Zustand je Teil

| Teil | Was es ist | Zustand |
| --- | --- | --- |
| `bot/` | Go-Bot (discordgo, SQLite ohne cgo), läuft als `auvc.exe` in einem Job-Objekt der App | Stark. `pkg/voice` ist eine reine Abbildung Spielzustand → Sprachzustand, der Reconciler schickt nur Unterschiede. Abnahmetests für 19 Szenarien, Wiederanlauf, Absturz-Freigabe (`voice_hold`) |
| `capture/AUVC.Transport`, `AUVC.Protocol` | Neue, reine Logik der App: Verbindung, Kopplung, Status, Diagnose, Release-Hinweis | Stark, gut getestet |
| `capture/AmongUsCapture` | Speicherleser aus AmongUsCapture | Funktioniert, aber alter Stil: globale Singletons, eine Endlosschleife mit `Thread.Sleep`, Offsets nur über die Prüfsumme. Mit #146 (Aufzeichnungen) erstmals testbar |
| `capture/AUOffsetManager`, `Offsets.json` | 60 Einträge, neuester `v18s/e/m`, alle 32 Bit | Kein Eintrag für die 64-Bit-Builds (#198) |
| `capture/AUCapture-WPF` | WPF-App (MahApps), Einrichtung, Einstellungen, Statuszeile | Bedienung inzwischen gut durchdacht (#21). Code-behind groß (`MainWindow.xaml.cs` 943, `SetupWindow.xaml.cs` 836 Zeilen), nicht testbar, weil der Testprojekt WPF nicht referenzieren kann |
| `protocol/` | JSON-Vertrag mit Fixtures, die Go und C# beide testen | Sehr gut: ein Bruch fällt auf beiden Seiten auf |
| `installer/` | Inno-Setup-Skript, pro Benutzer | Schlank und passend |
| `scripts/` | Lokale Prüfung, Release, Herkunft, Lizenzen | Gut, mit eigenen Tests. `verify_bootstrap.py` lief nirgends (#194, behoben) |
| `docs/` | Anleitung de/en, Architektur, Datenschutz, Abnahme | Ehrlich und aktuell, durch `GuideTextTests` an die App gebunden |
| `.github/` | Baseline-CI (Linux + Windows), Release aus Tag, Dependabot | Gut |

## Stärken

- **Sicherheit von Anfang an mitgedacht:** zufälliges Geheimnis auf Loopback
  für den eingebauten Bot, Kopplung mit Code, Entziehen (`/au capture revoke`),
  DPAPI für Token, Redaktion der Diagnose, Gitleaks über die ganze Geschichte.
- **Kein Kill wird verraten:** Verschieben vor Stummschalten und Tote erst beim
  Meeting verschieben ist sauber getestet (`secrecy_test.go`).
- **Absturzfest:** Nach einem Absturz gibt der nächste Start alle frei.
- **Herkunft sauber:** Importe mit Baum-IDs, Lizenzbytes, Prüfung gegen neue
  Upstream-Verweise.

## Schwachstellen, technische Schulden, Risiken

1. **Offsets nur per Release.** Jede Spielversion braucht einen Eintrag in
   `Offsets.json`, eingebaut in die App. Among Us aktualisiert mehrmals im Jahr,
   AUVC ist dann bis zum nächsten Release unbrauchbar. Mit 64 Bit kommen zwei
   Plattformen (Steam, Epic, Microsoft Store) in je eigener Version dazu.
2. **Der optionale API-Server lauscht auf allen Netzwerkkarten.**
   `ServerSocket` startet `new WebSocketServer(42069)`; websocket-sharp bindet
   das an `IPAddress.Any`, ohne Anmeldung. Wer die Einstellung einschaltet,
   gibt Spielzustand und Spielernamen an jeden im Netz. Nur aus, wenn der
   Schalter aus ist – aber er steht in den allgemeinen Einstellungen.
3. **Speicherleser alt und global.** `GameMemReader`, `ProcessMemory`,
   `ServerSocket` und `IPCAdapter` sind Singletons; `RunLoop` läuft ewig und
   fängt alles. Fehler beim Lesen werden zu leeren Werten, was wie ein
   verlassener Spieler aussieht.
4. **WPF-Code nicht testbar.** Was in `MainWindow`/`SetupWindow` entschieden
   wird, ist ohne Windows nicht prüfbar. Was schon nach `AUVC.Transport`
   gewandert ist (Status, Texte, Diagnose), ist es.
5. **Kosmetik ist abgeschaltet** (#155): Hut, Haustier und Skin sind immer 0,
   weil Among Us Zeichenketten statt Zahlen nutzt.
6. **Kleinigkeiten im Leser:** der Cache-Zeiger des Meetings wird als `uint`
   gelesen (auf 64 Bit nur die untere Hälfte, praktisch harmlos);
   `GameVerifier` vergleicht feste SHA-1-Werte alter Steam-DLLs.
7. **Ein Mensch für alle Live-Tests.** Jede Änderung am Leser oder an den
   Discord-Abläufen wartet auf einen Abend mit voller Lobby.

## Altlasten (nur aufgelistet)

| Was | Wo | Warum es weg könnte | Risiko beim Löschen |
| --- | --- | --- | --- |
| Visual-Studio-Ordner, eingecheckt trotz `.gitignore` | `capture/.vs/` (inkl. `slnx.sqlite`, `db.lock`) | IDE-Zustand aus dem Upstream | keins |
| GoReleaser-Konfiguration | `capture/.goreleaser.yml` (`skip: true`) | AUVC baut mit `local_release.py` und `release.yml` | keins |
| All-Contributors-Datei | `capture/.all-contributorsrc` | Mitwirkende kommen in der App von GitHub | keins |
| Video, 15 MB | `capture/AUCapture-WPF/Pirate Video.mp4` | nirgends verwendet, bläht das Repository | keins |
| Ressourcen-Zip, 1 MB | `capture/AUCapture-WPF/Resources/Resources.zip` | nirgends verwendet | gering, vorher Inhalt ansehen |
| Upstream-Bilder des Bots | `bot/assets/DiscordBot_Black.gif`, `AutoMuteUsBanner*.png`, `BotProfilePicture.png` | eingebettet wird nur `emojis/*.png` | keins für den Code; Lizenzhinweise prüfen |
| Sprachen ohne Nutzen | `capture/AmongUsCapture/Properties/Resources.{ja,pt,ru,zh-hans}.resx` | AUVC spricht Deutsch und Englisch | gering |
| WinForms-Rest | `capture/AmongUsCapture/UserForm.resx` (schon aus dem Build genommen) | keine Form mehr | keins |
| Zweites Veröffentlichungsprofil | `capture/AmongUsCapture/Properties/PublishProfiles/FolderProfile1.pubxml` | doppelt | keins |
| Alter Offset-Helfer | `capture/AUOffsetHelper/` | exportiert nur das Muster von 2020; ersetzt durch `scripts/offsets_from_il2cpp.py` | Tests in `OffsetBaselineTests` mit anpassen |
| Offset-Standardwerte aus 2020 | `capture/AmongUsCapture/Settings.cs` (`GameHash`, `Offsets.*`), `AUOffsetManager/GameOffsetsHelper.cs` | ungenutzt, der Index ersetzt sie | Einstellungsdatei alter Nutzer enthält sie, wird ignoriert |
| Ungenutzte Methode | `ProcessMemory.ReadArray` (liest immer 4 Byte) | nirgends aufgerufen, auf 64 Bit falsch | keins |
| `isEpic` | `GameOffsets` | nirgends ausgewertet | keins |
| Bootstrap-Prüfung | `scripts/verify_bootstrap.py` | jetzt repariert, aber `verify_repository.py` deckt fast alles ab | keins; Zeile in `UPSTREAM.md` mit ändern |
| Renovate-Konfiguration | `bot/renovate.json` | Updates kommen über Dependabot | keins |
| Planungsordner dieser Runde | `plan/` | nur für diese Sitzung | keins |

## Among Us 2026.9.29, 64 Bit (#198)

**Was geht:** Der Leser ist für 64 Bit vorbereitet, und das ist jetzt auch
bewiesen: ein neuer Test liest eine ganze Lobby mit Achtbyte-Zeigern oberhalb
von 4 GB, Strings, Outfits hinter dem Dictionary und Rollen
(`A64BitGameIsReadWithTheSameReader`).

**Was nicht geht, und warum:** Die Klassenadressen (`AmongUsClientOffset` und
die sieben anderen) stecken in der `GameAssembly.dll` genau dieses Builds. Ohne
die Datei lassen sie sich nicht bestimmen. Öffentlich gibt es sie nicht:
BetterCrewLink hat 64-Bit-Strukturdaten bis V18 (Stand 12.09.2026), findet die
Klassen aber per Signatursuche und veröffentlicht keine Adressen pro
Prüfsumme. Erfundene Zahlen in `Offsets.json` wären schlimmer als keine: die
App läse Unsinn, statt *nicht unterstützt* zu sagen.

**Was du jetzt tun kannst** – Schritt für Schritt in [offsets.md](offsets.md):

1. Il2CppDumper über `GameAssembly.dll` und `global-metadata.dat` laufen lassen.
2. `python scripts/offsets_from_il2cpp.py … --merge %APPDATA%\AmongUsCapture\index.json`
3. In AUVC **Fehlersuche → Offsets neu laden**, Menü und Lobby prüfen.
4. Passt es, den ausgegebenen Eintrag nach `capture/Offsets.json`.

Die Meldung in der App gab es schon (#125/#132: *AUVC kann diese
Among-Us-Version nicht lesen* mit nächstem Schritt). Neu ist, dass die
Diagnose-Zip oben die Prüfsumme, 32/64 Bit und die geladenen Offsets nennt.

## Issues: Stand nach dieser Runde

| Issue | Was passiert ist | Was bleibt |
| --- | --- | --- |
| #198 64-Bit-Offsets | Werkzeug, Anleitung, 64-Bit-Test, Diagnose nennt die Version, zwei Lesefehler behoben | Werte am PC ermitteln, live prüfen, eintragen |
| #194 `verify_bootstrap.py` | prüft Import-Bäume an den Import-Commits und den heutigen Bestand; eigene Tests laufen in der lokalen Prüfung | entscheiden, ob es bleibt (siehe Altlasten) |
| #21 Bedienoberfläche | alle Kriterien bis auf #136 waren schon erledigt | #136 |
| #131 winget/Store | Manifest-Entwürfe in `installer/winget/`, Ablauf in [release-checklist.md](release-checklist.md) | erst nach v1.0.0, Einreichung nur durch dich |
| #18 v1.0.0 | Checkliste in [release-checklist.md](release-checklist.md) | Live-Tests |
| #141, #142, #144, #136, #145, #157 | ein Protokoll zum Abhaken: [live-tests.md](live-tests.md) | Durchführung |

## Diese Runde geändert

- `capture/AmongUsCapture/Memory/Structs/PlayerInfo.cs`: Die Größe des
  gelesenen Spieler-Structs wurde abgerundet. Bei v18 (letzter Zeiger bei
  `0x58`) las der Leser 88 Byte und holte `_object` von hinter dem Puffer.
  Jetzt wird aufgerundet (`StructBytes`). Gleiches für `WinningPlayerData`.
- `ProcessMemoryWindows/Linux.ReadString`: Eine Länge außerhalb von 1 bis 1024
  Zeichen ist kein String. Mit falschen Offsets – also genau beim Ausprobieren
  einer neuen Version – konnte eine zufällige Länge Gigabytes anfordern.
- `AUVC.Transport/Diagnostics.cs`: verbotene Dateien werden auch mit
  Windows-Pfad auf anderen Systemen erkannt; die Zusammenfassung nennt das Spiel.
- Tests: Lobby im v18- und im 64-Bit-Layout (Mutationsprobe: beide fangen ihren
  Fehler, 2/2), Spielzeile in der Diagnose, 12 Tests für das Offset-Skript,
  4 für `verify_bootstrap.py`.
- Neu: `scripts/offsets_from_il2cpp.py`, `docs/offsets.md`,
  `docs/live-tests.md`, `docs/release-checklist.md`, `installer/winget/`.
- Angepasst: `scripts/verify_bootstrap.py`, `UPSTREAM.md`, `CHANGELOG.md`,
  `docs/development.md`, ein Satz in beiden Anleitungen.

## Vorschläge

### Muss (vor v1.0.0)

1. **64-Bit-Offsets eintragen** (#198), mit dem neuen Werkzeug. Ohne sie gibt
   es nichts live zu testen.
2. **API-Server nur auf Loopback** (`new WebSocketServer(IPAddress.Loopback, 42069, false)`)
   oder ganz entfernen, wenn ihn niemand nutzt. Heute offen ins ganze Netz.
3. **Live-Tests abarbeiten** nach [live-tests.md](live-tests.md), #160
   (PlayStation-Spieler) entscheiden.

### Sollte (bald nach v1.0.0)

4. **Offsets ohne neues Release.** Ein eigener Index im Repository, z. B.
   `offsets/index.json` als Release-Asset mit Signatur; `IndexURL` gibt es schon.
   Datenschutz-Doku anpassen, Standard bleibt *aus* bis zur Zustimmung. Damit
   ist ein Among-Us-Update eine Datei und kein Release.
5. **Signatursuche für die Klassenadressen.** Die acht Adressen pro Build per
   Byte-Muster in `GameAssembly.dll` finden, wie BetterCrewLink es tut. Dann
   bleiben nur Feldoffsets, die sich selten ändern, und viele Updates laufen
   ohne Zutun weiter.
6. **Speicherleser entflechten.** `GameMemReader` als Klasse mit eingespeistem
   `ProcessMemory`, Zustandsmaschine statt `while (true)`, ein Ereignis
   „Lesen fehlgeschlagen“ statt leerer Werte. Die Aufzeichnungen aus #146 machen
   das sicher.
7. **Echte Aufzeichnungen.** Eine Lobby aus dem echten 64-Bit-Spiel als JSON
   aufnehmen (die Form gibt es: `RecordedMemory.ToJson`) und als Test
   einchecken – ohne Namen.
8. **Code-behind verkleinern.** Entscheidungen aus `MainWindow`/`SetupWindow`
   nach `AUVC.Transport` ziehen, wie schon bei Status und Diagnose.
9. **Altlasten aufräumen** nach der Tabelle oben; allein das Video spart 15 MB
   pro Klon.

### Kann (Erweiterungen)

10. **Kosmetik zurück** (#155): Hut-, Skin- und Haustier-Strings lesen und auf
    die vorhandenen Bilder abbilden.
11. **Plattformen:** Epic und Microsoft Store als eigene Einträge, sobald die
    64-Bit-Werte stehen.
12. **Rundenstatistik** in Discord: wer gewonnen hat, Impostor, Dauer – die
    Daten liest AUVC am Rundenende schon.
13. **Proximity-ähnliche Modi** sind mit Discord nicht machbar (keine
    Lautstärke pro Hörer); nicht verfolgen.
14. **Tray und Autostart**, nur nach ausdrücklicher Wahl (steht schon in #21).
15. **winget, dann Store** nach [release-checklist.md](release-checklist.md).
