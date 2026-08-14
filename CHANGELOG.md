# Changelog

## 1.0.9

Das An- und Ausschalten einzelner Aktionen wirkte wirkungslos, ohne dass irgendwo etwas dazu
stand. Drei Ursachen:

- **Ohne Twitch-Verbindung wurde gar nichts protokolliert.** Der Log-Aufruf lag innerhalb der
  Bedingung „Twitch verbunden" — man konnte nicht einmal sehen, ob der Klick angekommen war.
  Jetzt wird jeder Umschaltvorgang festgehalten, samt Verbindungszustand und Reward-ID.
- **Fehler beim Anlegen eines Rewards wurden verschluckt** (`if err == nil`). Betrifft
  besonders Rewards, die gleichnamig schon einer anderen App gehören: der Schalter ging an,
  auf Twitch entstand nichts, und niemand erfuhr davon. Jetzt kommt die Meldung mit der
  nötigen Handlung.
- **Der Schalterzustand wird zuerst gespeichert**, unabhängig davon, ob Twitch gerade
  erreichbar ist. Probleme auf Twitch-Seite werden gemeldet, führen aber nicht mehr dazu, dass
  der Schalter zurückspringt.

Dazu lädt die Oberfläche die Aktionsliste jetzt in jedem Fall neu. Vorher verhinderte ein
Fehler das Neuladen, und der Schalter sprang optisch zurück — was aussah, als sei er kaputt.

## 1.0.8

Im Live-Betrieb aufgefallen: Einlösungen liefen ins Leere, ohne dass irgendwo etwas dazu stand.
Im Log tauchte nur `Redemption: NO MATCH` auf.

Ursache waren Rewards, die noch von der alten Twitch-App auf dem Kanal lagen. Twitch lässt eine
App nur ihre **eigenen** Rewards verwalten: `only_manageable_rewards` liefert die fremden gar
nicht erst, also versucht SCTroll sie anzulegen — und Twitch lehnt wegen Namensgleichheit mit
`DUPLICATE_REWARD` ab. Die Aktion blieb unverknüpft, jede Einlösung darauf löste nichts aus.
Löschen kann SCTroll sie ebenfalls nicht; das geht nur im Twitch-Dashboard.

- **Doppelte Rewards werden erkannt** (`ErrRewardExists`) statt in einer allgemeinen
  Fehlermeldung unterzugehen.
- **Beim Synchronisieren** wird pro betroffenem Reward gemeldet, was zu tun ist, und am Ende
  gesammelt: *„Einlösungen darauf lösen NICHTS aus."*
- **Bei einer Einlösung ohne Treffer** wird geprüft, ob der Titel zu einer Aktion passt. Wenn
  ja, ist es ein solcher Fremd-Reward — und das steht jetzt im Log und im Twitch-Log der
  Oberfläche, statt nur als `NO MATCH`. Rewards anderer Tools werden weiterhin still ignoriert.

## 1.0.7

### Selbstaktualisierung

SCTroll prüft beim Start still im Hintergrund, ob es eine neue Version gibt, und zeigt sie
als Leiste über der Oberfläche an. Eingespielt wird **nur auf Knopfdruck** — ein Neustart
mitten im Stream wäre ein schlechter Zeitpunkt, den das Programm nicht selbst wählen sollte.
In den Einstellungen lässt sich zusätzlich von Hand suchen.

Vor dem Austausch wird zweifach geprüft:

- **SHA256** gegen die veröffentlichte Prüfsumme — belegt einen heilen Download.
- **Authenticode-Signatur** über `WinVerifyTrust`, dieselbe Prüfung, die Windows beim
  Ausführen macht. Das ist der eigentliche Schutz: die Prüfsumme kommt aus derselben Quelle
  wie die Datei. Ohne Signaturprüfung wäre die Update-Funktion ein bequemer Weg, fremden Code
  auszuführen.

Der Austausch nutzt den üblichen Windows-Weg — die laufende Exe lässt sich nicht
überschreiben, wohl aber umbenennen: alte zur Seite, neue an ihren Platz, Neustart, Reste
beim nächsten Start aufräumen. Schlägt das Umbenennen fehl (etwa unter `C:\Program Files`
ohne Rechte), wird zurückgerollt und die alte Version läuft weiter.

Die Version steht jetzt nur noch in `internal/version` und wird von dort ins Frontend
gereicht. Ein Test wacht darüber, dass sie zur `productVersion` in `wails.json` passt —
liefen die auseinander, würde die App Updates auf sich selbst anbieten.

### Neue Twitch-App vom Typ „Public"

Die bisher mitgelieferte App stammte aus dem Tarkov-Vorläufer und war als **Confidential**
registriert. Solche Apps dürfen den Access Token nicht ohne Client Secret erneuern — und ein
Secret in einer ausgelieferten Anwendung ist keins. Damit war das Problem für alle Nutzer
strukturell nicht lösbar.

Die neue App ist als **Public** registriert. Für die existiert gar kein Secret, und der
Refresh funktioniert ohne. Bestehende Installationen werden beim Start automatisch umgestellt:
Client-ID getauscht, Tokens und Reward-Verknüpfungen der alten App entfernt, weil beide immer
zu genau einer App gehören.

**Einmalig nötig:** neu anmelden und die Rewards neu anlegen. Die unter der alten App
erstellten Rewards bleiben auf dem Kanal stehen und sollten vorher gelöscht werden.

### Twitch-Anmeldung hielt weiterhin nicht

Die Anmeldung ging trotz 1.0.6 weiterhin nach ein paar Stunden verloren. Im Debug-Log stand
der Grund:

```
RefreshAccessToken: status=400 body={"status":400,"message":"missing client secret"}
```

Twitch verlangt beim Erneuern ein Client Secret, **wenn die App als „Confidential"
registriert ist**. Nur Apps vom Typ **„Public"** dürfen ohne erneuern. Die mitgelieferte App
ist eine Confidential-App — der Login per Device Code Flow klappt damit, der Refresh nicht.
Die Annahme in 1.0.6, ein Secret sei bei diesem Flow generell entbehrlich, galt also nur für
die halbe Wahrheit.

- **Client Secret optional hinterlegbar** (`twitch.client_secret` bzw. im Twitch-Tab). Wird
  nur mitgeschickt, wenn gesetzt — ein leeres Feld ließe Public-Apps scheitern. Nichts davon
  wird mitgeliefert.
- **Eigene Twitch-App im Twitch-Tab einstellbar**: Client-ID und optional Secret. Ein Wechsel
  der Client-ID setzt die Anmeldung zurück und löst die Verknüpfung zu bestehenden Rewards,
  weil beides immer zu genau einer App gehört — darauf wird hingewiesen.
- **Eigener Fehler `ErrClientSecretRequired`** statt „Anmeldung abgelaufen". Das ist ein
  Einrichtungsproblem, kein abgelaufener Token: eine Neuanmeldung hätte es nur bis zum
  nächsten Ablauf verdeckt und den Nutzer in eine Endlosschleife geschickt. Die Oberfläche
  zeigt stattdessen, was zu tun ist.
- Schrittnummerierung im Twitch-Tab korrigiert (stand auf 1 und 3).

Drei neue Tests: fehlendes Secret wird nicht als Ablauf behandelt, ein hinterlegtes Secret
wird mitgeschickt, ein leeres gar nicht erst.

## 1.0.6

Die Anmeldung hielt nicht, weil der Token-Refresh bei **jedem** Fehler die gespeicherten
Zugangsdaten gelöscht hat. `RefreshAccessToken` prüfte den HTTP-Status nicht: kam von Twitch
eine Fehlerantwort — abgelaufener Token, HTTP 500, Rate Limit oder nur ein kurzer Netzausfall
beim Systemstart — waren `access_token` und `refresh_token` in der Antwort leer, wurden **so
gespeichert**, und die Funktion meldete Erfolg. Danach war der Refresh Token endgültig weg.

- **Fehler löschen keine Anmeldung mehr.** Nur HTTP 400/401 gelten als endgültige Ablehnung
  (`ErrLoginRequired`); alles andere ist vorübergehend und lässt die Zugangsdaten stehen.
- **Refresh-Token-Rotation korrekt behandelt.** Liefert Twitch keinen neuen mit, gilt der alte
  weiter, statt überschrieben zu werden.
- **Ablaufzeitpunkt wird gespeichert** (`expires_in` wurde bisher weggeworfen) und der Token
  15 Minuten vorher im Hintergrund erneuert. Vorher fiel ein Ablauf erst auf, wenn mitten im
  Stream keine Einlösung mehr ankam — die EventSub-Verbindung meldet keinen 401, sie wird
  still widerrufen.
- **Auto-Login hängt am Refresh Token**, nicht mehr am Access Token. Der ist nach ein paar
  Stunden ohnehin abgelaufen; die Prüfung `AccessToken != ""` hat den Auto-Login danach gar
  nicht erst versucht.
- **Wiederholversuche beim Start** (0/3/10/30/60 s). Beim Systemstart ist das Netz oft noch
  nicht da — das war bisher sofort ein Fehlschlag.
- **EventSub-Anmeldung läuft über `doAuthorized`.** Sie baute ihre Anfrage selbst und ging an
  der Token-Erneuerung vorbei; ein abgelaufener Token liess den ganzen Verbindungsaufbau
  scheitern.
- `Disconnect` beendet die Hintergrund-Erneuerung sauber, erneutes Verbinden legt einen
  frischen Client an.

Acht neue Tests gegen einen Stub-Server decken die Token-Behandlung ab, darunter der
ursprüngliche Fehler: HTTP 500 darf die Zugangsdaten nicht anfassen.

## 1.0.5

Behebt einen Fehler, den 1.0.4 selbst eingebaut hat: die Umstellung auf Star Citizens echte
Standardbelegungen hat auch die Tasten von Aktionen überschrieben, die im Spiel **ab Werk
unbelegt** sind — Emotes und Türen. Deren Auslieferungswert ist leer, und damit hat die
Migration bereits aus dem Spielprofil übernommene Belegungen gelöscht.

- **Migration leert keine Tasten mehr.** Ist der Auslieferungswert leer, bleibt eine
  vorhandene Belegung stehen.
- **Fehlende Tasten werden beim Start aus dem Spielprofil ergänzt.** Betrifft genau die
  Aktionen ohne Standardbelegung. Bereits gesetzte Tasten bleiben unangetastet; der
  vollständige Abgleich läuft weiter nur über den Knopf in den Einstellungen.
- **Die Bindliste zeigt jetzt alle Aktionen**, nicht nur die aktiven. Emotes und Türen sind
  ab Werk abgeschaltet und waren dadurch in der Liste unsichtbar — ausgerechnet die, deren
  Belegung man nachsehen will. Abgeschaltete Zeilen sind ausgegraut.

Drei neue Tests sichern das ab: die Migration darf keine Taste leeren, umbenannte Aktionen
müssen ihre Reward-ID mitnehmen, und die Emote-Belegungen müssen aus dem Profil ankommen.

## 1.0.4

Ein Abgleich aller mitgelieferten Aktionen gegen die Standardbelegung hat fünf gefunden, die
im Spiel ins Leere zeigten — teils weil Star Citizen sie umgebaut hat, teils weil sie ab Werk
nie eine Taste hatten.

- **Quantum-Modus ersetzt durch Master Mode.** Seit den Master Modes ist
  `v_toggle_quantum_mode` unbelegte Altlast. Der Moduswechsel läuft über
  `v_master_mode_cycle_long` — Taste `b`, `activationMode="delayed_press"`, also **langer
  Druck**. Neue Haltezeit 500 ms.
- **Energie:** `v_power_set_off` ist unbelegt, richtig ist `v_power_toggle` auf `u`. Dazu neu
  Schilde (`o`) und Waffen (`p`), die es vorher gar nicht gab.
- **Decoy ist eine Halteaktion** (`onHold="1"`). Mit den bisherigen 80 ms wäre auch mit
  korrekter Taste nichts passiert — jetzt 800 ms.
- **Nachtsicht und Speed Limiter entfernt.** `v_light_amplification_toggle` steht so nicht
  mehr im Standardprofil, `v_ifcs_speed_limiter_toggle` ist ab Werk unbelegt.
- **Alle Tasten auf die echten Standardbelegungen umgestellt.** Vorher standen dort erfundene
  RAlt-Kombinationen. Licht `l`, Fahrwerk `n`, VTOL `k`, Decoupled `c`, Scan `v`,
  Umschauen `comma`, Taschenlampe `t`, Boost `lshift`, Aufstehen `y`.
- **Ab Werk unbelegte Aktionen** (Türen, Türschlösser, G-Comp, alle Emotes) werden ohne Taste
  ausgeliefert und sind aus. Sie zeigen in der Liste *unbelegt*; wer sie im Spiel gebunden hat,
  holt sie mit *Aus dem Spiel übernehmen*.
- **Einmalige Migration** beim ersten Start: Tasten werden auf die Standardwerte gesetzt,
  umbenannte Aktionen ziehen samt Reward-ID um. Eigene Belegungen holt der Abgleich zurück.

Zwei neue Tests hätten das verhindert und laufen ab jetzt bei jedem Build: jede Aktion muss in
der `defaultProfile.xml` existieren, und ihre Haltezeit muss zum `activationMode` passen.

## 1.0.3

Star Citizens komplette Standardbelegung liegt jetzt bei. Damit kennt SCTroll auch die Tasten
von Aktionen, die nicht in der `actionmaps.xml` stehen — und das sind die allermeisten, weil
die Datei nur Abweichungen enthält.

- **`defaultProfile.xml` eingebettet** (1097 Aktionen, 50 Actionmaps). Die Datei steckt im
  Spiel in der `Data.p4k`; die beiliegende Kopie stammt aus
  [ltmajor42/StarCitizen_Streamdeck_Plugin](https://github.com/ltmajor42/StarCitizen_Streamdeck_Plugin),
  das sie aus der p4k entpackt. Eine eigene Datei unter `%APPDATA%\SCTroll\defaultProfile.xml`
  hat Vorrang — so lässt sich nach einem Patch aktualisieren, ohne neu zu bauen.
- **Tastenauflösung in zwei Stufen**: eigene Belegung aus der `actionmaps.xml`, sonst
  Standardbelegung. Aktionen ohne jede Belegung werden als *unbelegt* geführt statt still
  eine erfundene Taste zu drücken. Türen und Emotes sind ab Werk unbelegt — deshalb hat sie
  jeder selbst gebunden.
- **Haltezeiten aus `activationMode`.** Das Standardprofil sagt pro Aktion, ob getippt,
  gedrückt oder gehalten werden muss. `pl_exit` (aus dem Sitz) ist `onHold`, `v_self_destruct`
  ist `delayed_press_medium` — mit 80 ms passiert da nichts. Beim Abgleich wird die Haltezeit
  nur verlängert, nie verkürzt.
- Die Bindliste unterscheidet jetzt *Eigene* / *Standard* / *unbelegt*.

Korrigierte Standardbelegungen, die vorher geraten waren: Licht `l`, Fahrwerk `n`,
Flight Ready `ralt+r`, Helm `lalt+h`, Schleudersitz `ralt+y`, Selbstzerstörung `backspace`,
Aus dem Sitz `y`, Decoupled `c`.

## 1.0.2

Bis hierhin gab es keine Möglichkeit, einen Tastendruck auszulösen, ohne dass ein Zuschauer
einen Reward einlöst. Damit ließ sich auch nicht feststellen, ob überhaupt etwas beim Spiel
ankommt — im Debug-Log stand entsprechend keine einzige `sendKey`-Zeile.

- **Test-Knopf pro Aktion.** 5 Sekunden Vorlauf zum Wechseln ins Spiel, dann wird die Taste
  gedrückt. Ohne Twitch, ohne Cooldown, aber mit Vordergrundprüfung. Das Debug-Log zeigt
  danach pro Tastendruck `ok` oder `BLOCKIERT`.
- **Sendeverfahren umschaltbar**: Scancode (bisher fest verdrahtet), Virtual-Key oder beides.
  Der bisherige Scancode-Zwang (`Vk=0`) stammte aus der Tarkov-Vorlage und war für Star Citizen
  nie geprüft. Das Stream-Deck-Plugin für Star Citizen benutzt InputSimulator, also
  Virtual-Keys — Software-Injection funktioniert dort nachweislich.
- Log-Ausgabe pro Tastendruck nennt jetzt das benutzte Verfahren.

## 1.0.1

**Korrektur eines Denkfehlers in 1.0.0.** SCTroll hat eigene `ralt+…`-Belegungen in die
`actionmaps.xml` geschrieben. Das war falsch: die Datei enthält nur Abweichungen vom Standard,
ein Eintrag *überschreibt* also Star Citizens Standardtaste. Bei Aktionen, die vorher auf der
Standardtaste liefen, hat SCTroll damit genau die weggenommen — die gewohnte Taste tat nichts
mehr. Die Annahme dahinter ("wer per Joystick bindet, hat keine Tastaturbelegung mehr") stimmt
nicht: beide Belegungen existieren nebeneinander.

- **Es wird nicht mehr geschrieben.** `WriteSCBinds` ist raus.
- **Neu: Aus dem Spiel übernehmen** (`ImportSCBinds`) — liest alle `kb1_`-Belegungen aus der
  `actionmaps.xml` und setzt sie als Tasten der Aktionen.
- **Neu: Von sctroll gesetzte Belegungen entfernen** (`RemoveSCBinds`) — nimmt die von 1.0.0
  geschriebenen Einträge zurück, sodass die Standardbelegung wieder greift. Entfernt nur, was
  exakt einer mitgelieferten SCTroll-Taste entspricht; eigene Belegungen bleiben.
- Die Bindliste zeigt jetzt pro Aktion die Herkunft der Taste (*Profil* / *Standard*) und
  markiert Abweichungen zwischen Aktion und Profil rot.

## 1.0.0

Erste Version. Portiert von [tarkovtroll](https://github.com/miwidot/tarkovtroll) auf Star Citizen.

### Neu

- **Installationssuche** über RSI-Launcher-Log, alle lokalen Festplatten und manuelle Auswahl.
  Das Spiel muss nicht auf `C:` liegen.
- **`actionmaps.xml` schreiben statt lesen**: SCTroll trägt seine eigenen Tastenbelegungen ins
  Spielprofil ein. Joystick-, HOTAS- und Gamepad-Belegungen bleiben erhalten, vorher wird eine
  Sicherung mit Zeitstempel angelegt. Geblockt, solange das Spiel läuft — Star Citizen würde
  die Änderungen beim Beenden sonst überschreiben.
- **30 Star-Citizen-Aktionen** in den Kategorien Schiff, Flug, Energie, Sicht, Spieler, Emotes
  und Gefährlich.
- **Erstattung von Kanalpunkten**, wenn eine Aktion nicht ausgeführt werden konnte (Spiel nicht
  im Vordergrund, Cooldown, Aktion deaktiviert, Queue voll).

### Geändert gegenüber tarkovtroll

- **Kein Client Secret mehr.** Der Device Code Flow ist für öffentliche Clients gedacht, auch
  der Token-Refresh kommt ohne aus. Ein Secret in einer ausgelieferten Desktop-App ist keins.
- **Reconnect**: `session_reconnect` wurde bisher nur geloggt. Jetzt wird die neue Session-URL
  übernommen, ohne die Subscriptions doppelt anzulegen. Dazu eine Read-Deadline anhand des
  Keepalive-Intervalls, damit tote Verbindungen überhaupt auffallen.
- **Reward-Anlage**: `should_redemptions_skip_queue` war ein Feldname, den Twitch nicht kennt.
  Korrekt ist `should_redemptions_skip_request_queue` — und er muss `false` sein, sonst gilt eine
  Einlösung sofort als erledigt und lässt sich nicht mehr erstatten.
- **Key Lock** ohne Tarkov-Sonderfall: die Zwangsfreigabe von Shift/Strg/WASD vor Waffenaktionen
  gab es nur, weil Tarkov beim Sprinten die Waffe senkt. Jetzt zählt nur die Konfiguration
  pro Aktion.
- **Tastennamen** folgen der `actionmaps.xml`-Schreibweise (`np_5`, `lbracket`, `ralt`, …), damit
  Spielprofil und Konfiguration dieselbe Sprache sprechen. Ziffernblock, Satzzeichen und die
  rechten Modifier sind neu dazugekommen.
- **F13–F24 bewusst nicht unterstützt**: Star Citizen kann sie nicht binden.
