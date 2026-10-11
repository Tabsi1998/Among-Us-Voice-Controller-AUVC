# Offsets für eine neue Among-Us-Version

Among Us liest AUVC über feste Adressen im Spiel, die **Offsets**. Jede Version
von `GameAssembly.dll` hat eigene. AUVC sucht sie über die SHA-256-Prüfsumme
dieser Datei in `capture/Offsets.json`. Fehlt die Version, zeigt die Statuszeile
*AUVC kann diese Among-Us-Version nicht lesen*, und im Log steht
`No offsets found for hash: …`.

Seit 2026 ist Among Us auf Steam ein **64-Bit-Programm**
([#198](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/198)).
Der Leser kann das schon: Zeiger sind so breit wie das Spiel, und ein Test liest
eine Lobby im 64-Bit-Layout (`RecordedMemoryTests.A64BitGameIsReadWithTheSameReader`).
Was fehlt, sind die Zahlen für den Build.

## Was sich mit 64 Bit ändert

Ein Teil der Zahlen gehört nicht zu Among Us, sondern zu IL2CPP selbst. Sie
setzt das Skript abhängig von der Architektur ein:

| | 32 Bit | 64 Bit |
| --- | --- | --- |
| Zeigerbreite (`AddPlayerPtr`) | `0x4` | `0x8` |
| statische Felder einer Klasse (zweiter Schritt jeder Kette) | `0x5C` | `0xB8` |
| erstes Element eines Arrays (`PlayerListPtr`) | `0x10` | `0x20` |
| `List<T>._items` / `._size` | `0x8` / `0xC` | `0x10` / `0x18` |
| Länge / Zeichen eines Strings (`StringOffsets`) | `0x8` / `0xC` | `0x10` / `0x14` |
| `m_CachedPtr` eines Unity-Objekts | `0x8` | `0x10` |
| Outfit hinter dem Dictionary | `[…, 0xC, 0x1C]` | `[…, 0x18, 0x30]` |

Der Rest – wo die Klassen in `GameAssembly.dll` liegen und an welcher Stelle
ihre Felder stehen – kommt aus dem Spiel selbst.

## Werte ermitteln

Du brauchst den PC mit Among Us, Python 3.11 und
[Il2CppDumper](https://github.com/Perfare/Il2CppDumper/releases) (kostenlos,
läuft nur lokal; nichts davon kommt in AUVC).

1. Im Spielordner, z. B. `D:\SteamLibrary\steamapps\common\Among Us`:
   - `GameAssembly.dll`
   - `Among Us_Data\il2cpp_data\Metadata\global-metadata.dat`
2. Il2CppDumper starten, erst `GameAssembly.dll`, dann `global-metadata.dat`
   wählen. Er schreibt unter anderem `dump.cs` und `script.json`.
   - Meldet er eine unbekannte Metadaten-Version, die neueste Version von
     Il2CppDumper nehmen und das Ergebnis in #198 eintragen.
3. Im Repository:

   ```powershell
   python scripts/offsets_from_il2cpp.py `
     --dump C:\il2cpp\dump.cs --script C:\il2cpp\script.json `
     --game-assembly "D:\SteamLibrary\steamapps\common\Among Us\GameAssembly.dll" `
     --description "v2026.9.29s x64 (build num: 25134893)" `
     --merge "$env:APPDATA\AmongUsCapture\index.json"
   ```

   - Das Skript erkennt 32 oder 64 Bit am Kopf der DLL und rechnet die
     Prüfsumme selbst.
   - Fehlt eine Klasse oder ein Feld, nennt es sie mit `MISSING:` und schreibt
     **nichts**. Dann den Namen in `dump.cs` suchen und in der Liste im Skript
     ergänzen; die Namen stehen oben in `scripts/offsets_from_il2cpp.py`.
   - Ohne `--merge` gibt es den Eintrag nur aus, im Stil von `Offsets.json`.

## Ohne neuen Build ausprobieren

`%AppData%\AmongUsCapture\index.json` ist der **lokale Index**. Er hat Vorrang
vor dem eingebauten und braucht keinen neuen Build.

1. Among Us starten, AUVC starten.
2. In AUVC: Zahnrad → **Fehlersuche** → **Offsets neu laden**. Die Statuszeile
   wechselt von *nicht lesen* zum nächsten Schritt.
3. Prüfen, was allein geht:
   - Hauptmenü: Status *im Menü*.
   - Eigene Lobby: Lobby-Code, Karte und Region in der Crewmate-Nachricht, dein
     Name und deine Farbe in der App. Karte wechseln: Bild wechselt mit.
4. Mit Mitspielern: Rundenstart, Kill, Meeting, Rauswurf, Rundenende – die
   Liste steht in [live-tests.md](live-tests.md#neue-among-us-version-198).

Passt alles, kommt der Eintrag aus der Ausgabe des Skripts nach
`capture/Offsets.json`, mit einer Beschreibung im Stil der anderen. Erst dann
App und Installer neu bauen.

## Wenn etwas nicht stimmt

| Was du siehst | Wahrscheinlich |
| --- | --- |
| Status bleibt *im Menü*, obwohl du in der Lobby bist | `GameStateOffsets`: Feld `GameState` von `InnerNetClient` |
| Keine Spieler in der App | `AllPlayerPtrOffsets`, `PlayerInfoStructOffsets` |
| Namen leer, Farben richtig | `PlayerOutfitStructOffsets.PlayerNameOffset` |
| Kein Lobby-Code | `GameCodeOffsets`: `GameRoomNameCode` und `TMP_Text.m_text` |
| Falsche Region | `PlayRegionOffsets` (liest die Länge des Regionsnamens) |
| Meeting wird nicht erkannt | `MeetingHudPtr`, `MeetingHudStateOffsets` |
| Rauswurf ohne Folgen | `ExiledPlayerIdOffsets` |
| Sieger falsch | `RawGameOverReasonOffsets`, `WinningPlayerDataStructOffsets` |

Die Diagnose-Zip (Zahnrad → **Fehlersuche** → **Diagnose exportieren**) nennt jetzt oben in `summary.txt` die
Prüfsumme, 32 oder 64 Bit und welche Offsets geladen sind. Sie enthält aber
Spielernamen: nicht ins öffentliche Repository hochladen.

## Später: Offsets ohne neues Release

Heute kommt jede neue Spielversion nur mit einem neuen AUVC. Zwei Wege, die das
lösen würden, stehen in [analyse-rework.md](analyse-rework.md#vorschläge):
ein eigener, abgesicherter Offset-Index aus diesem Repository (die Einstellung
`IndexURL` gibt es schon, sie ist leer) und Signatursuche im Speicher, wie sie
BetterCrewLink für die Klassenadressen nutzt.
