# Changelog

## 1.0.15

### Optionale Obergrenzen pro Aktion

Zwei neue Felder je Aktion: **maximal pro Stream** und **maximal pro Zuschauer und Stream**.
Null heißt unbegrenzt, wie bisher.

Durchgesetzt wird das von Twitch selbst, nicht von SCTroll. Das ist der wichtige Unterschied:
Twitch blendet die Belohnung aus, sobald die Grenze erreicht ist, und zwar **bevor** Punkte
abgebucht werden. Der Zuschauer sieht die Grenze in der Oberfläche, und ein Neustart von
SCTroll setzt nichts zurück. Selbst mitzuzählen wäre in jeder Hinsicht schlechter gewesen --
der Zuschauer hätte bezahlt und die Punkte erst hinterher zurückbekommen.

Die Grenzen pro Stream zählt Twitch ab Beginn der Übertragung; offline greifen sie nicht.

### Reward-Felder an einer Stelle

Anlegen und Ändern einer Belohnung bauten ihre Felder getrennt zusammen und liefen dadurch
auseinander -- die Beschreibung etwa wurde beim Anlegen anders behandelt als beim Bearbeiten.
Beides geht jetzt durch dieselbe Abbildung (`RewardSpec`).

Dabei eine Falle aus der Twitch-Dokumentation: **die Anfrage nimmt flache Felder**
(`is_max_per_stream_enabled`, `max_per_stream`), verschachtelt (`max_per_stream_setting` mit
`is_enabled`) ist ausschließlich die *Antwort*. Wer die Antwortform zurückschickt, bekommt
keinen Fehler -- Twitch ignoriert unbekannte Felder --, und die Grenze wirkt schlicht nie. Drei
Tests halten das fest.

## 1.0.14

### Beschreibung für die Zuschauer

Twitch zeigt zu jeder Belohnung einen Beschreibungstext an. Bisher stand dort bei allen
derselbe Platzhalter. Jetzt wird die **Beschreibung der Aktion** dorthin übernommen — der
Zuschauer sieht also beim Einlösen, was er anrichtet.

Alle mitgelieferten Beschreibungen wurden dafür neu geschrieben — aus Zuschauersicht statt als
technische Notiz. In acht standen noch Einrichtungshinweise wie „im Spiel ab Werk unbelegt“,
die jeder Zuschauer zu sehen bekommen hätte; bestehende Konfigurationen werden davon bereinigt,
selbst geschriebene Texte bleiben unangetastet.

Gilt auch für bereits angelegte Rewards: beim nächsten Synchronisieren wird die Beschreibung
nachgetragen. Twitch begrenzt den Text, längere Beschreibungen werden gekürzt statt den Aufruf
scheitern zu lassen.

### Selbstzerstörung wird 15 Sekunden gehalten

Star Citizen will die Taste dafür lange gedrückt sehen. Die 900 ms, die sich aus dem
`activationMode` ergaben, lösten schlicht nichts aus. Bestehende Konfigurationen werden einmalig
angehoben — nur nach oben, wer bewusst länger eingestellt hat, behält seinen Wert.

Lange Halteaktionen werden jetzt protokolliert, und die Taste wird per `defer` losgelassen. Ohne
das konnte sie bei einem Abbruch gedrückt hängen bleiben — bei fünfzehn Sekunden ein spürbarer
Unterschied.

### Zwei neue Aktionen

- **Nachladen** (`r`) — mitten im Gefecht besonders unpraktisch.
- **Granate werfen** — zweistufig, wie im Spiel: `G` zieht die Granate, die linke Maustaste
  wirft sie. Der Wurf wird gehalten, weil `throw_overhand` eine Halteaktion ist. Standardmäßig
  aus.

### Maustasten zählen jetzt wie im Spiel

`mouse1` ist ab sofort die **linke** Taste, `mouse2` die rechte, `mouse3` die mittlere — so wie
Star Citizen es in der `defaultProfile.xml` schreibt. Vorher zählte diese Tabelle ab null und
`mouse1` war die rechte Taste: wer eine Belegung aus dem Spiel abschrieb, bekam die falsche.

Die Tastennamen folgten dem Spiel schon lange, die Maustasten jetzt auch. `lmouse`, `rmouse`
und `mmouse` funktionieren unverändert und sind eindeutig. **Wer in einer eigenen Aktion
`mouse1` benutzt hat, sollte sie prüfen** — sie zeigt jetzt auf die linke statt auf die rechte
Taste.

## 1.0.13

Aus dem Log eines anderen Nutzers.

### Not-Aus blendet Rewards aus, statt sie zu löschen

Der Globalschalter hat bei „Aus" **alle Rewards vom Kanal gelöscht** und bei „An" neu angelegt.
Im Log waren das 134 Erstellungen und 111 Löschungen an drei Tagen.

Das war aus drei Gründen falsch: es kostet pro Umschaltung ein Dutzend API-Aufrufe, es verwirft
Bild, Farbe und Reihenfolge der Belohnungen auf dem Kanal, und beim Wiederanlegen kann es an
Twitchs Obergrenze scheitern — dann wären die Rewards weg und ließen sich nicht zurückholen.

Twitch kann Belohnungen ausblenden (`is_enabled`), genau dafür ist das gedacht. Der Not-Aus
benutzt jetzt das. Der Knopf *Alle Rewards löschen* bleibt als bewusste Handlung erhalten.

### Obergrenze für Kanalpunkt-Belohnungen

Twitch begrenzt die Zahl eigener Belohnungen pro Kanal. Ist sie erreicht, antwortete die API
mit `TOO_MANY_REWARDS`, und SCTroll zeigte die rohe Fehlermeldung — 13-mal hintereinander, für
jede weitere Aktion erneut.

Der Fall wird jetzt erkannt (`ErrTooManyRewards`), im Klartext erklärt, und das Synchronisieren
bricht danach ab, statt es für jede weitere Aktion zu wiederholen.

### Bestätigt

Der Fehler beim Selbstupdate aus 1.0.11 ließ sich im Log nachvollziehen: der Nachfolger startete
um 14:02:43.216 und war 456 ms später wieder weg, der Nutzer startete 70 Sekunden später von
Hand. Seit 1.0.11 behoben.

Ebenfalls im Log: 48 Tastendrücke, alle angenommen, keiner blockiert — auf einem fremden Rechner.

## 1.0.12

### Eigenes Programmsymbol

Bisher lief noch das Symbol des Tarkov-Vorläufers mit. Das neue zeigt eine abgeschrägte
Tastenkappe mit Blitz — Zuschauer lösen per Tastendruck etwas aus — in derselben Formensprache
wie die Oberfläche.

Erzeugt wird es aus Code (`build/make-icon.ps1`), damit es reproduzierbar bleibt: vierfach
vergrößert gezeichnet und heruntergerechnet, sonst fransen die schrägen Kanten aus. Sieben
Größen von 16 bis 256 px, wobei die kleinen die Kachel stärker ausfüllen und einen kräftigeren
Blitz bekommen — sonst verschwindet die Form zwischen Rand und Fase.

### Mit Windows starten

Neuer Schalter in den Einstellungen. Eintrag unter `HKEY_CURRENT_USER`, also ohne
Administratorrechte, und über die Registry statt einer Verknüpfung im Autostart-Ordner — so
entsteht keine zweite Datei, die nach einem Verschieben ins Leere zeigt.

Nach einem Update oder einem verschobenen Ordner wird ein bestehender Eintrag beim Start
automatisch auf die aktuelle Programmdatei nachgezogen. Zeigt er auf etwas anderes, gilt der
Autostart als aus, statt „an" zu melden und beim Anmelden nichts zu tun.

### Twitch-App-Abschnitt entfernt

Die Felder für Client-ID und Secret sind aus der Oberfläche verschwunden. Seit die
mitgelieferte App vom Typ „Public" ist, braucht sie niemand mehr. Wer eine eigene App benutzen
will, trägt sie weiterhin in der `config.json` ein; der Hinweis bei fehlendem Secret bleibt und
erklärt jetzt den Weg dorthin.

## 1.0.11

**Das Selbstupdate aus 1.0.7 konnte nicht funktionieren.** Es brach mit „SCTroll läuft
bereits!" ab.

Beim Update wird die neue Programmdatei gestartet, während die alte noch läuft — die beendet
sich erst kurz danach. Der Nachfolger traf also auf die Einzelinstanz-Sperre der Vorgängerin,
meldete pflichtgemäß „läuft bereits" und beendete sich. Kurz darauf endete auch die alte
Instanz planmäßig. Ergebnis: Datei ausgetauscht, aber nichts lief mehr.

Zweifach abgesichert:

- Die alte Instanz **gibt die Sperre frei**, bevor sie den Nachfolger startet. Schlägt der
  Austausch fehl, übernimmt sie die Sperre wieder — das Programm läuft ja weiter.
- Der Nachfolger bekommt beim Start `--updated` mit und **wartet bis zu 15 Sekunden** auf die
  Sperre, statt sofort abzubrechen. Ein normaler Doppelstart wird weiterhin sofort abgewiesen.

Geprüft: zweiter Start ohne Kennzeichen wird abgewiesen; mit Kennzeichen wartet er, und sobald
die Vorgängerin endet, übernimmt er und öffnet sein Fenster.

## 1.0.10

Aus dem Log von 1.0.9 aufgefallen: rund ein Dutzend gespeicherter Reward-IDs zeigten auf
Rewards, die es auf Twitch nicht mehr gab.

```
UpdateReward: status=404 "The custom reward specified in the id query parameter was not found."
```

Das passiert im Alltag ständig — es reicht, im Twitch-Dashboard aufzuräumen. Bisher wurde der
Fehler nur protokolliert und aufgegeben: die Aktion galt als aktiv, hatte aber keinen Reward
mehr auf dem Kanal und war damit **dauerhaft nicht auslösbar**. Ohne vollständigen Sync gab es
keinen Weg zurück.

- **Veraltete Verknüpfungen heilen sich selbst.** Antwortet Twitch mit 404, wird die
  gespeicherte Reward-ID verworfen und der Reward neu angelegt (`ErrRewardNotFound`).
- Andere Fehler bleiben davon unberührt — ein vorübergehender Serverfehler darf nicht dazu
  führen, dass ein zweiter Reward entsteht.

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
