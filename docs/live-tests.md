# Live-Tests: Protokoll zum Abhaken

Was nur am echten PC mit Among Us und Discord geprüft werden kann, an einer
Stelle. Die Nummern sind die Issues, in die das Ergebnis gehört. Die
ausführliche Liste für die laufende Testversion bleibt in
[#157](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/157);
hier steht, was zum ersten fertigen Release
([#18](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/18))
noch fehlt.

> **Das Repository ist öffentlich.** Kein Token, keine Diagnose-Zip, keine
> Discord-IDs in Issues. Ein Satz, was passiert ist, und ein Screenshot ohne
> Namen reichen.

Zu jedem Test kurz notieren:

| Datum | AUVC-Version | Among-Us-Version / Prüfsumme | Windows | Spieler | Ergebnis |
| --- | --- | --- | --- | --- | --- |
| | | | | | |

## Neue Among-Us-Version (#198)

Erst wenn ein Eintrag für den Build im lokalen Index steht
([offsets.md](offsets.md)).

- [ ] Status wechselt nach **Offsets neu laden** von *nicht lesen* weg.
- [ ] Hauptmenü wird erkannt.
- [ ] Eigene Lobby: Code, Karte, Region, eigener Name und Farbe stimmen.
- [ ] Karte in der Lobby wechseln: Bild und Name auf der Crewmate-Nachricht folgen.
- [ ] Mitspieler treten bei und gehen: Kacheln kommen und gehen, niemand doppelt.
- [ ] Rundenstart: alle Lebenden in einem Rutsch stumm.
- [ ] Kill: Toter bleibt bis zum Meeting, wo er ist.
- [ ] Meeting: Lebende laut, Tote im Geisterkanal.
- [ ] Rauswurf: der Rausgeworfene wechselt sofort.
- [ ] Rundenende: nach etwa drei Sekunden alle laut im Hauptkanal; Sieger stimmen.

## Smoke-Test in einer echten Runde (#141)

Die Schritte stehen in [acceptance.md](acceptance.md). Abhaken, wenn eine ganze
Runde von Lobby bis Rundenende ohne Eingriff lief.

- [ ] Lobby → Aufgaben → Kill → Meeting → Rauswurf → Rundenende
- [ ] Ein Spieler verlässt mitten in der Runde Discord und kommt zurück
- [ ] AUVC wird mitten in der Runde beendet: niemand bleibt stumm (#116)

## Volle Lobby: Name, Farbe, lebend oder tot (#144)

Mit 12 bis 15 Spielern, mindestens einer davon lädt beim Beitreten langsam.

- [ ] Jeder Name in der App stimmt mit dem Spiel überein
- [ ] Jede Farbe stimmt, auch nach einem Farbwechsel in der Lobby
- [ ] Wer stirbt, ist in der App tot, und zwar erst ab dem Kill
- [ ] Ein Spieler mit Sonderzeichen oder Emoji im Namen
- [ ] PlayStation-Spieler im Hauptkanal: bleibt drin (#160); fliegt er raus,
      die Zeilen davor aus `%LOCALAPPDATA%\AUVC\logs\logs.txt` sichern

## Installieren, aktualisieren, deinstallieren (#142)

Auf einem **frischen Windows-Konto** ohne .NET.

- [ ] Setup aus dem Release startet, SmartScreen-Meldung wie in der Anleitung
- [ ] Installation per PowerShell aus der Anleitung kommt ohne Warnung aus
- [ ] Erster Start: Einrichtungsfenster öffnet von selbst
- [ ] Update über die ältere Version: Token, Einstellungen und Verknüpfungen bleiben
- [ ] Deinstallieren: Programm weg; was bleibt, steht in [privacy.md](privacy.md)
- [ ] Portable Zip entpacken und starten geht ebenso

## Bedienungsabnahme mit einem frischen Nutzer (#136)

Jemand, der AUVC nicht kennt, nur mit der App und [anleitung.md](anleitung.md).
Nicht helfen, nur zusehen und notieren.

- [ ] Token anlegen und einfügen
- [ ] Bot einladen, Server und Kanäle wählen
- [ ] Sich selbst mit `/au link` oder der Crewmate-Auswahl verknüpfen
- [ ] Statuszeile zeigt *bereit*
- [ ] Zeit bis dahin: ___ Minuten. Wo hing es? ___

## Danach

- Ergebnisse in die Issues eintragen; was nicht passt, als neues Issue.
- Wenn alles grün ist: `v1.0.0-rc.1`, siehe [release-checklist.md](release-checklist.md).
