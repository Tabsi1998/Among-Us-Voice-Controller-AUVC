# AUVC-Anleitung

Alles, was du zum Einrichten und Benutzen von AUVC brauchst. **English:** [guide.md](guide.md).

- [Was du brauchst](#was-du-brauchst)
- [1. Herunterladen](#1-herunterladen)
- [2. Installieren oder entpacken](#2-installieren-oder-entpacken)
- [3. Ersteinrichtung](#3-ersteinrichtung)
- [4. Spieler verknüpfen](#4-spieler-verknüpfen)
- [5. Spielen](#5-spielen)
- [6. Einstellungen](#6-einstellungen)
- [7. Discord-Befehle](#7-discord-befehle)
- [8. Aktualisieren](#8-aktualisieren)
- [9. Probleme lösen](#9-probleme-lösen)
- [10. Deinstallieren und Daten löschen](#10-deinstallieren-und-daten-löschen)
- [11. Ein Bot auf einem anderen Rechner](#11-ein-bot-auf-einem-anderen-rechner)

## Was du brauchst

- **Den Windows-PC, auf dem Among Us läuft**, mit 64-Bit Windows 10 oder 11. AUVC
  und sein Discord-Bot laufen beide dort. .NET oder sonst etwas musst du nicht
  installieren.
- **Ein Discord-Konto**, das Bots zu deinem Discord-Server hinzufügen darf. Dafür
  braucht es die Berechtigung *Server verwalten*.
- **Zwei Sprachkanäle** auf diesem Server: einen für alle Lebenden und einen für
  die Geister. Ein Textkanal für Hinweise von AUVC ist optional.
- **Alle anderen** brauchen nur Discord und installieren nichts.

Der Bot von AUVC ist online, solange AUVC auf diesem PC läuft, und offline, wenn
du es schließt.

## 1. Herunterladen

Öffne die [Release-Seite](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/releases)
und wähle die neueste Version. Solange das Repository privat ist, musst du bei
GitHub mit einem Konto angemeldet sein, das Zugriff hat.

| Datei | Was sie ist |
| --- | --- |
| `AmongUsVoiceCapture-Setup-win-x64.exe` | Der Installer. Empfohlen. |
| `AmongUsVoiceCapture-win-x64.zip` | Die portable Version: entpacken und starten, installiert nichts |
| `SHA256SUMS` | Prüfsummen, um zu bestätigen, dass eine Datei die aus dem Release ist |

Versionen mit `-beta` am Ende sind Vorabversionen zum Testen.

**Windows warnt dich.** Die Dateien sind nicht signiert, deshalb zeigt Windows
*„Der Computer wurde durch Windows geschützt“*. Klicke auf **Weitere
Informationen** und dann auf **Trotzdem ausführen**.

## 2. Installieren oder entpacken

**Installer.** Starte ihn und folge ihm. Er installiert nur für dein
Windows-Konto, ohne Administratorrechte, und legt **AUVC Capture** im Startmenü
an. Auf der letzten Seite kannst du AUVC gleich starten.

**Portabel.** Entpacke die Zip-Datei in einen eigenen Ordner, zum Beispiel
`Dokumente\AUVC`, und starte `AUCapture-WPF.exe`. Lass den Ordner vollständig:
Der Bot liegt im Unterordner `bot`.

Beide Versionen speichern ihre Einstellungen am selben Ort, du kannst also
wechseln.

## 3. Ersteinrichtung

Beim ersten Start öffnet sich das Einrichtungsfenster von selbst. Du kannst es
jederzeit über den Knopf **Einrichten** oben im Hauptfenster wieder öffnen.

### Schritt 1 von 4: der Bot-Token

AUVC braucht einen eigenen Discord-Bot. Das kostet nichts und dauert zwei Minuten.

1. Klicke auf **Developer Portal öffnen** oder öffne
   [discord.com/developers/applications](https://discord.com/developers/applications)
   und melde dich mit deinem Discord-Konto an.
2. Klicke auf **New Application**, gib einen Namen ein, zum Beispiel *AUVC*,
   akzeptiere die Bedingungen und klicke auf **Create**.
3. Öffne links **Bot**. Klicke auf **Reset Token**, bestätige und klicke dann auf
   **Copy**.
4. Füge den Token in AUVC ein und klicke auf **Token prüfen**.

AUVC fragt bei Discord nach, ob der Token funktioniert, und zeigt den Namen des
Bots. Den Token speichert AUVC verschlüsselt, nur für dein Windows-Konto und nur
auf diesem PC.

- **Der Token ist ein Passwort.** Wer ihn hat, steuert deinen Bot. Schick ihn
  niemandem und poste ihn nie mit einem Screenshot oder Log.
- **Eine Redirect- oder Callback-URL brauchst du nicht**, und auch keine
  *Privileged Gateway Intents*. Lass diese Einstellungen, wie sie sind.
- Meldet AUVC, dass **Requires OAuth2 Code Grant** eingeschaltet ist, schalte die
  Option auf der Seite **Bot** aus. Sonst klappt die Einladung nicht.

Klicke auf **Weiter**. AUVC startet jetzt den Bot im Hintergrund.

### Schritt 2 von 4: Bot einladen und Server wählen

1. Klicke auf **Bot einladen**. Im Browser öffnet sich die Einladungsseite von
   Discord.
2. Wähle unter *Zu Server hinzufügen* deinen Server und klicke auf
   **Autorisieren**.
3. In AUVC erscheint der Server nach ein paar Sekunden in der Liste. Falls nicht,
   klicke auf **Aktualisieren**.
4. Wähle den Server aus und klicke auf **Weiter**.

Die Einladung fordert genau das an, was AUVC benutzt: *Kanäle ansehen*,
*Nachrichten senden*, *Links einbetten*, *Externe Emojis verwenden*, *Verbinden*,
*Mitglieder stummschalten*, *Mitglieder taubschalten* und *Mitglieder
verschieben*. Mit den ersten vier zeigt AUVC die Crewmate-Auswahl im Textkanal.

### Schritt 3 von 4: die Kanäle

| Feld | Wähle |
| --- | --- |
| **Hauptkanal (Sprachkanal)** | Den Sprachkanal, in dem alle spielen |
| **Geisterkanal (Sprachkanal)** | Den Sprachkanal, in den die Toten kommen |
| **Textkanal für die Crewmate-Auswahl und Hinweise (empfohlen)** | Wo die Spieler ihre Figur auswählen und AUVC Warnungen schreibt, zum Beispiel wenn das Spiel nicht mehr erkannt wird |

**AUVC automatisch starten, sobald ein Spiel erkannt wird** ist eingeschaltet.
Lass es an, dann legt AUVC los, sobald ihr spielt. Klicke auf **Weiter**, um zu
speichern.

Sind die Listen leer, hat der Server noch keine Sprachkanäle. Lege zwei in Discord
an und klicke auf **Aktualisieren**.

### Schritt 4 von 4: fertig

AUVC verbindet sich mit seinem Bot und zeigt dessen Prüfungen:

- ✅ alles in Ordnung
- ⚠️ gut zu wissen, meist ein Schritt, der noch aussteht, zum Beispiel *no game
  data yet*, bevor jemand gespielt hat
- ❌ muss behoben werden, was zu tun ist, steht hinter dem Pfeil

Klicke auf **Fertig**.

## 4. Spieler verknüpfen

AUVC muss wissen, welches Discord-Mitglied welche Figur spielt. Deshalb sagt es
jeder Spieler einmal. Die Auswahl der Figur aus einem Menü gibt es ab
`v0.1.1-beta`; mit `v0.1.0-beta` tippst du deinen Namen wie unter **Mit einem
Befehl** beschrieben.

**Im Textkanal.** Sobald die App eine Lobby sieht, schreibt AUVC eine Nachricht in
den Textkanal aus der Einrichtung. Sie zeigt jede Figur in der Lobby, wer schon
welche gewählt hat, und ein Menü **Choose your crewmate**. Wähle dort deine
Figur. Wählst du eine andere, wandert deine Verknüpfung mit; **Unlink me**
entfernt sie. Die Antwort von AUVC siehst nur du.

**Mit einem Befehl.** Tippe in einem beliebigen Textkanal des Servers nur
`/au link`, dann bekommst du dasselbe Menü, sichtbar nur für dich. Oder tippe
deinen Namen genau so, wie er in Among Us steht, mit Groß- und Kleinschreibung:

```text
/au link player:Name
```

**In der App.** Am PC, auf dem AUVC läuft, verknüpft **Bot** → **Spieler** jede
Figur mit jemandem aus einem Sprachkanal des Servers, zum Beispiel für jemanden,
der nicht selbst wählen kann.

Die Verknüpfung bleibt gespeichert und muss nur erneuert werden, wenn jemand
seinen Namen im Spiel ändert. Eine Verknüpfung während einer Runde wirkt sofort.

Server-Administratoren können jemand anderen verknüpfen:
`/au link player:Name user:@Person`. `/au unlink` entfernt deine eigene
Verknüpfung.

Die Nachricht verrät nichts: Eine getötete Figur wird dort erst zum Geist, wenn
ein Meeting den Tod bekannt gibt. Schließt du AUVC, wird die Nachricht gelöscht;
mit der nächsten Lobby kommt sie wieder. Beim ersten Mal lädt AUVC die
Figurenbilder zu deinem Bot hoch, das kann eine Minute dauern. Bis dahin zeigt
das Menü Namen und Farben. Ohne Textkanal gibt es keine Nachricht, dann nehmen
die Spieler `/au link`.

Spieler ohne Verknüpfung lässt AUVC in Ruhe: Es schaltet sie nie stumm und
verschiebt sie nie. Bots wie Musik-Bots fasst es ebenfalls nicht an.

## 5. Spielen

1. Starte AUVC. Im Hauptfenster steht *Waiting for Among Us*.
2. Starte Among Us. Sobald du in einer Lobby bist, zeigt AUVC die Spieler.
3. Alle gehen in den Hauptkanal.

Ab dann folgt AUVC dem Spiel von selbst:

| Phase | Lebende | Tote |
| --- | --- | --- |
| Lobby | Hauptkanal, können reden | Hauptkanal, können reden |
| Aufgaben, Tod noch nicht bekannt | Hauptkanal, stumm und taub | Bleiben, wo sie sind, stumm |
| Aufgaben, Tod bekannt | Hauptkanal, stumm und taub | Geisterkanal, können reden |
| Meeting und Abstimmung | Hauptkanal, können reden | Geisterkanal, können reden |
| Runde vorbei | Hauptkanal, können reden | Hauptkanal, können reden |

- **Kills werden nicht verraten.** Wenn jemand in einen anderen Kanal wechselt,
  sieht das jeder in Discord. Deshalb bleibt ein getöteter Spieler, wo er ist,
  stumm, bis das nächste Meeting den Tod bekannt macht.
- **Wer rausgewählt wird**, kommt sofort in den Geisterkanal. Das haben ohnehin
  alle gesehen.
- **Wenn du AUVC schließt**, gibt es alle frei und der Bot geht offline.
- **Stürzt AUVC ab**, stoppt sein Bot mit und kann in dem Moment niemanden
  freigeben. Beim nächsten Start von AUVC gibt der Bot alle frei, die er stumm,
  taub oder in den Geisterkanal geschickt hatte, und sonst niemanden. Wer dann
  nicht im Sprachkanal ist, wird freigegeben, sobald er wieder beitritt. Sofort
  geht es in Discord: Rechtsklick auf die Person und *Server-Stummschaltung* und
  *Server-Taubschaltung* ausschalten.

## 6. Einstellungen

### Bot-Einstellungen: der Knopf „Bot“

Ist AUVC eingerichtet, heißt der Knopf oben im Hauptfenster **Bot**. Er öffnet
alles, was die Einrichtung festgelegt hat, in fünf Bereichen, die du einzeln
ändern kannst:

| Bereich | Was du tun kannst |
| --- | --- |
| **Token** | Neuen Token einfügen und **Token prüfen** klicken. Der Bot startet damit neu. |
| **Server** | Anderen Server wählen und **Diesen Server verwenden** klicken, danach unter **Kanäle** dessen Kanäle wählen. **Bot einladen** fügt den Bot vorher einem anderen Server hinzu. |
| **Kanäle** | Haupt-, Geister- und Textkanal sowie den automatischen Start ändern und **Speichern** klicken. |
| **Status** | Die Prüfungen des Bots. **Bot neu starten** startet ihn neu. **Bot auf diesem PC ausschalten** sorgt dafür, dass AUVC ihn nicht mehr startet. Token und Einstellungen bleiben gespeichert, und **Einrichten** schaltet ihn wieder ein. |
| **Spieler** | Jede Figur der aktuellen Lobby, jeweils mit einem Menü der Mitglieder in den Sprachkanälen des Servers. Ein Mitglied wählen verknüpft es, *(niemand)* entfernt die Verknüpfung. Das wirkt sofort. |

### App-Einstellungen: das Zahnrad

Die App-Einstellungen sind bisher nur auf Englisch beschriftet.

| Reiter | Einstellung | Was sie tut |
| --- | --- | --- |
| General | Language | Die Sprache der App |
| General | Always copy game code | Kopiert den Lobby-Code in die Zwischenablage, sobald du einer Lobby beitrittst |
| General | Startup memes | Zeigt ab und zu beim Start einen Spaß-Startbildschirm mit Ton. Der Ton wird vom Server des ursprünglichen AutoMuteUs geladen. Schalte das aus, wenn du das nicht willst |
| General | Focus window on connect | Holt das Fenster nach vorn, wenn ein Kopplungslink AUVC öffnet |
| General | API Server | Startet eine lokale Schnittstelle für Overlay-Programme. Lass sie aus, solange kein Programm sie braucht |
| General | Always on top | Hält das AUVC-Fenster über anderen Fenstern |
| Debug | Debug mode | Öffnet beim nächsten Start ein zusätzliches Konsolenfenster mit technischen Ausgaben |
| Debug | Open log folder | Öffnet den Ordner mit den Log-Dateien |
| Debug | Reload offsets | Lädt die Speicher-Offsets neu, mit denen AUVC das Spiel liest. Hilft nach einem Among-Us-Update |
| Debug | Reset Config | Löscht die App-Einstellungen und bietet einen Neustart an. Der Bot-Token bleibt gespeichert. Danach **Einrichten** erneut durchgehen |
| About | App version | Die installierte AUVC-Version |
| About | Latest version | Zeigt noch die neueste Version des ursprünglichen AmongUsCapture, nicht von AUVC ([#44](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/44)) |

Tastenkürzel im Hauptfenster: **Strg+L** öffnet den Log-Ordner, **F2** kopiert das
neueste Log in die Zwischenablage, **Strg+R** startet die App neu.

### Spiel-Einstellungen in Discord

Diese Einstellungen änderst du mit `/au settings` in Discord. Das dürfen der
Server-Eigentümer, Discord-Administratoren und die Rolle aus
`/au setup permissions`. Discord schlägt die Optionen beim Tippen vor.

| Einstellung | Standard | Was sie tut |
| --- | --- | --- |
| `auto_move_ghosts` | an | Tote in den Geisterkanal verschieben |
| `enforce_channels` | an | Spieler zurückholen, die selbst den Kanal wechseln |
| `capture_timeout_seconds` | 60 | Wie lange der Bot wartet, wenn die App ihm nichts mehr meldet, bevor er handelt (10–600) |
| `capture_timeout_action` | `fail-open` | `fail-open` schaltet alle laut und holt sie zurück, `pause` lässt alle, wie sie sind |
| `auto_start` | nach der Einrichtung an | Loslegen, sobald ein Spiel erkannt wird |

`/au settings show` zeigt die aktuellen Einstellungen.

## 7. Discord-Befehle

Jede Antwort sieht nur, wer den Befehl eingegeben hat.

| Befehl | Was er tut |
| --- | --- |
| `/au link` | Deine Figur aus einem Menü wählen, das nur du siehst |
| `/au link player:<Name>` | Dich mit deinem Namen in Among Us verknüpfen |
| `/au unlink` | Deine Verknüpfung entfernen |
| `/au session status` | Ob AUVC gerade die Sprachkanäle steuert |
| `/au session start`, `stop`, `pause`, `resume` | Das Steuern von Hand starten oder beenden. `stop` gibt alle frei, `pause` lässt alle, wie sie sind |
| `/au doctor` | Alles prüfen und sagen, was fehlt |
| `/au settings show`, `voice`, `ghosts`, `safety`, `preset`, `export` | Spiel-Einstellungen ansehen und ändern |
| `/au setup channels` | Die Kanäle in Discord statt in der App wählen |
| `/au setup permissions` | Eine Rolle, die AUVC zusätzlich zu den Administratoren ändern darf |
| `/au setup reset` | Standard-Einstellungen wiederherstellen, Verknüpfungen bleiben |
| `/au capture status`, `pair`, `revoke` | Die Verbindung der App zum Bot. Nur für einen Bot auf einem anderen Rechner nötig |
| `/au version` | Die Version des Bots |

## 8. Aktualisieren

**Installer.** Lade die neue Setup-Datei herunter und starte sie. Sie aktualisiert
AUVC. Einstellungen, Token und Verknüpfungen bleiben erhalten. Schließe AUVC
vorher.

**Portabel.** Schließe AUVC, lösche den alten Ordner und entpacke die neue
Zip-Datei. Einstellungen und Daten liegen nicht in diesem Ordner, es geht also
nichts verloren.

## 9. Probleme lösen

### Einrichtung

| Problem | Was zu tun ist |
| --- | --- |
| *Discord akzeptiert diesen Token nicht* | Kopiere den Token im Developer Portal auf der Seite **Bot** noch einmal, notfalls nach **Reset Token** |
| *Der Bot hat sich gleich wieder beendet* | Fast immer liegt es am Token. Geh zurück zu Schritt 1 und prüfe ihn. Sonst die Internetverbindung prüfen |
| Der Server erscheint nicht | Lade den Bot mit **Bot einladen** ein und klicke dann auf **Aktualisieren** |
| Die Einladungsseite zeigt einen Fehler | Schalte **Requires OAuth2 Code Grant** auf der Seite **Bot** aus |
| *Das Bot-Programm fehlt* | Installiere AUVC neu oder entpacke die portable Version vollständig |

### In Discord

| Problem | Was zu tun ist |
| --- | --- |
| `/au` erscheint nicht | Drücke in Discord **Strg+R**. Erscheint es dann noch nicht, lade den Bot mit **Bot einladen** erneut ein |
| Niemand wird stummgeschaltet | Läuft AUVC und zeigt es das Spiel? Prüfe `/au session status` und den Bereich **Status** hinter **Bot** |
| Ein Spieler wird nicht stummgeschaltet | Er ist nicht verknüpft oder mit einer anderen Figur verknüpft. Die Crewmate-Nachricht im Textkanal prüfen oder `/au link` erneut ausführen |
| Die Crewmate-Nachricht erscheint nicht | Hinter **Bot** → **Kanäle** einen Textkanal wählen. Der Rolle des Bots dort *Kanäle ansehen*, *Nachrichten senden* und *Links einbetten* geben. Die Nachricht erscheint, sobald die App eine Lobby sieht |
| AUVC meldet fehlende Rechte | Gib der Rolle des Bots auf beiden Sprachkanälen *Kanäle ansehen*, *Verbinden*, *Mitglieder stummschalten*, *Mitglieder taubschalten* und *Mitglieder verschieben* |
| Jemand bleibt stumm | Solange AUVC läuft, gibt `/au session stop` alle frei. Nach einem Absturz AUVC neu starten: Es gibt alle frei, die es stumm gelassen hat. Oder in Discord mit Rechtsklick auf die Person *Server-Stummschaltung* und *Server-Taubschaltung* ausschalten |

### Im Spiel

| Problem | Was zu tun ist |
| --- | --- |
| *Waiting for Among Us* verschwindet nicht | Läuft Among Us auf diesem PC und unter demselben Windows-Konto? |
| Das Spiel wird erkannt, aber keine Spieler angezeigt | Among Us wurde vielleicht aktualisiert. **Zahnrad → Debug → Reload offsets**, dann AUVC neu starten |

### Log-Dateien

Wenn du um Hilfe fragst, schick diese Dateien und die Ausgabe von `/au doctor`
mit. Keins davon enthält den Token.

| Datei | Was sie ist |
| --- | --- |
| `%AppData%\AmongUsCapture\logs\latest.log` | Das Log der App |
| `%LOCALAPPDATA%\AUVC\logs\logs.txt` | Das Log des Bots |

Füge den Pfad in die Adressleiste des Windows-Explorers ein, um ihn zu öffnen.

## 10. Deinstallieren und Daten löschen

**Installer.** In Windows **Einstellungen → Apps → Installierte Apps → AUVC
Capture → Deinstallieren**. Dabei wird gefragt, ob die AUVC-Daten mit gelöscht
werden sollen: Einstellungen, Bot-Token, die Datenbank des Bots mit Kanälen und
Verknüpfungen, und die Logs. Wähle **Nein**, um sie für eine spätere
Neuinstallation zu behalten.

**Portabel.** Lösche den Ordner. Um auch die Daten zu löschen, lösche
`%LOCALAPPDATA%\AUVC` und `%AppData%\AmongUsCapture`.

Das Löschen der Daten löscht den Bot in Discord nicht. Damit der Token sicher nie
wieder funktioniert, klicke im Developer Portal auf der Seite **Bot** auf
**Reset Token**. Um den Bot von deinem Server zu entfernen, wirf ihn über die
Mitgliederliste raus.

Was AUVC wo speichert, steht in [privacy.md](privacy.md) (Englisch).

## 11. Ein Bot auf einem anderen Rechner

Normalerweise läuft der Bot in AUVC. Läuft ein Bot schon woanders, zum Beispiel
auf einem Server, der immer an ist, kann sich AUVC stattdessen damit verbinden:

1. Schließe das Einrichtungsfenster.
2. In Discord führt ein Administrator `/au capture pair` aus und bekommt einen
   Code.
3. Klicke in AUVC auf den Knopf mit dem Hinweis **Pair with the AUVC bot**, gib
   die Adresse des Bots und den Code ein und klicke auf **Pair**.

Releases enthalten nur die App. Wie der Bot allein läuft, steht in
[development.md](development.md) (Englisch).
