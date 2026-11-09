# Fahrrad-Verleih

Verleihsystem für Fahrräder, entwickelt mit Spec-Driven Development (SDD): `CLAUDE.md` und diese `README.md` sind die Spezifikation.

**Starten:** `go run .` und http://localhost:8080 öffnen. Tests: `go test ./...`
Daten liegen nur im Speicher und sind nach einem Neustart weg (12 Demo-Fahrräder werden beim Start angelegt, darunter ein E-Bike, das schon lädt, und eines mit nur 23 % Akku zum Testen).
Zeitraffer für den Akku (zum Testen): `BATTERY_SPEEDUP=60 go run .` (1 Minute entspricht dann 1 Stunde).

## Spezifikation

### Entitäten
| Entität | Felder |
|---|---|
| **Fahrrad** (Item) | Nr (eindeutig, z.B. `BIKE-001`), Bezeichnung, Kategorie (Normales Fahrrad, Mountainbike, E-Bike), Preis (je Kategorie), nur E-Bike: Akkustand in %, Zustand (verfügbar, ausgeliehen, lädt, außer Betrieb) |
| **Person** | ID, Name, Geburtstag, Ausweisart (Personalausweis, Reisepass, Führerschein), Ausweisnummer, optionaler Ausweis-Upload (Bild/PDF, max. 2 MB) |
| **Ausleihe** (Rental) | ID, Fahrrad, Person, **Verleihbeginn**, **Verleihdauer (Stunden)**, Zubehör, Preis, Kaution, Rückgabezeitpunkt |

### Funktionen
1. **Dashboard** mit Kacheln: Verfügbare Fahrräder, Ausgeliehene Fahrräder, Gesamt Fahrräder, Registrierte Personen (zusätzlich: Lädt gerade, Überfällig, Außer Betrieb, Umsatz) und Schnellzugriff-Buttons (Fahrräder verwalten, Personen verwalten, Neue Ausleihe).
2. **Fahrrad-Verwaltung:** anlegen, auflisten (mit Status verfügbar/ausgeliehen/außer Betrieb), löschen.
3. **Personen:** anlegen mit Ausweisart + Nummer, Ausweis wahlweise **eingeben** (Nr. + Typ) und/oder **hochladen**; auflisten, löschen.
4. **Ausleihe:** Die Person wählt keine einzelnen Fahrräder aus, sondern nur die **Kategorie** (Normales Fahrrad, Mountainbike oder E-Bike); das System weist automatisch ein freies Fahrrad dieser Kategorie zu (bei E-Bikes das mit dem vollsten Akku). Dazu **Person**, **Beginn** (Datum/Uhrzeit, Standard: jetzt), **Dauer in Stunden** und optional Zubehör. Der Preis und die Kaution werden live angezeigt. Rückgabe per Button.
5. **Banner:** Oben auf der Website wirbt ein Banner für den Gutscheincode `Willkommen10`. Ein Klick auf „Mehr erfahren“ zeigt alle weiteren Gutscheine mit Beschreibungen, die auf ansprechende, auffällige Weise gestaltet sind.
6. **Fußzeile:** Unter jeder Seite stehen „© 2026“ mit dem Namen des Verleihs und die Social-Media-Links (Instagram, Facebook, X, YouTube, TikTok) als kleine Logos. Die Konten existieren nicht; die Links führen nur zur Startseite der jeweiligen Plattform und dienen der Optik.

### Preise (flexibel)
Jede Kategorie hat einen eigenen Basispreis pro Stunde: Normales Fahrrad 3,00 €, Mountainbike 4,50 €, E-Bike 7,00 €.

Der Preis ist dynamisch: Es gibt einen Basispreis und einen Höchstpreis (150 % des Basispreises, also z.B. beim Mountainbike höchstens 6,75 € pro Stunde).

Wenn mindestens 50 % der Fahrräder einer Kategorie gebucht sind, steigt der Preis langsam und dynamisch an (linear). Der Höchstpreis beträgt 150 % des Basispreises. Er soll erreicht werden, wenn in der Kategorie alles außer einem Fahrrad gebucht ist. Der Preis darf nie über 150 % steigen.

Wenn jemand zweimal ein Fahrrad bucht, bekommt die Person bei der dritten Buchung 50 % Rabatt auf eine beliebige Kategorie. Das gilt bei jeder dritten Buchung. Die rabattierte dritte Buchung gilt dabei als letzte.

Man kann einen Gutscheincode angeben, pro Buchung aber nur einen. Zu Beginn gibt es drei:
- `Willkommen10`: 10 % Rabatt auf die erste Buchung einer Person
- `Birthday10`: 10 % Rabatt am Geburtstag der Person (jede Person hat dafür einen Geburtstag)
- `LOYALTY50`: 50 % Rabatt auf die dritte Buchung

Der Preis wird bei der Ausleihe berechnet und fest gespeichert.

### Akku & Laden (E-Bikes)
Nur E-Bikes haben einen Akku. Solange ein E-Bike ausgeliehen ist, gilt es als in Fahrt und der Akku sinkt gleichmäßig (10 % pro Stunde). Erreicht der Akku 20 % oder weniger, kann das E-Bike nicht mehr ausgeliehen werden und geht in den Zustand „Lädt“ (eine laufende Ausleihe läuft normal weiter, nach der Rückgabe wird geladen). Beim Laden steigt der Akku um 25 % pro Stunde; bei 100 % ist das E-Bike wieder verfügbar. Zum Testen wird beim Start ein E-Bike im Zustand „Lädt“ angelegt.

### Kaution & Zubehör
Jede Kategorie hat eine Kaution: Normales Fahrrad 20 €, Mountainbike 40 €, E-Bike 100 €. Wer zusätzlich Zubehör nimmt, zahlt mehr Kaution: Helm +10 €, Schloss +15 €. Die Kaution wird bei der Rückgabe erstattet; bei einem Schadenbericht wird sie einbehalten.

### Regeln
- Ein Fahrrad kann nur einmal gleichzeitig ausgeliehen sein. Sind alle Fahrräder einer Kategorie belegt (ausgeliehen, lädt oder außer Betrieb), wird der Fehler „Keine … verfügbar“ angezeigt.
- Eine Person kann mehrere Fahrräder gleichzeitig buchen.
- Ausgeliehene Fahrräder und Personen mit aktiver Ausleihe können nicht gelöscht werden.
- Dauer mindestens 1 Stunde; Ausleihe nach Ablauf der Dauer ohne Rückgabe gilt als überfällig.
- Ein Fahrrad ist außer Betrieb, wenn bei der Rückgabe ein Schadenbericht hinzugefügt wird. Er wird dann zur Prüfung weitergeleitet. Man kann den Schaden beschreiben oder ein Bild hochladen; beides ist optional. Nach der Prüfung („Prüfung abgeschlossen“) ist das Fahrrad wieder im Betrieb.

### Eingabeprüfung & Sicherheit
Die API vertraut keiner Benutzereingabe und übernimmt Text nicht direkt in den Quellcode des Frontends (Schutz gegen XSS und HTML-Injection).

Beim Upload muss sichergestellt sein, dass die Datei wirklich ein PDF oder Foto ist (und z.B. keine .php-Datei oder etwas anderes, das auf dem Server ausgeführt werden kann). Das beginnt mit der Prüfung der Dateiendung; einfache Umgehungen wie `.pdf.php` dürfen ebenfalls nicht funktionieren.

Es wird kein XML verarbeitet (Schutz gegen XXE-Injection).

### API (JSON)
| Methode | Pfad | Zweck |
|---|---|---|
| GET | `/api/config`, `/api/dashboard` | Beschriftungen, Kennzahlen |
| GET | `/api/categories` | Kategorien mit Verfügbarkeit, aktuellem Preis, Kaution und Zubehör |
| GET | `/api/vouchers` | Gutscheine mit Beschreibung (Banner, Gutschein-Seite) |
| POST | `/api/quote` | Preis vorab berechnen (inkl. Aufschlag und Gutschein) |
| GET/POST | `/api/items` | Fahrräder auflisten / anlegen |
| DELETE | `/api/items/{nr}` | Fahrrad löschen |
| POST | `/api/items/{nr}/review-done` | Prüfung abgeschlossen (wieder im Betrieb) |
| GET | `/api/items/{nr}/damage-photo` | Schadenfoto |
| GET/POST | `/api/persons` | Personen auflisten / anlegen |
| DELETE | `/api/persons/{id}` | Person löschen |
| GET | `/api/persons/{id}/id-scan` | Ausweis-Upload |
| GET/POST | `/api/rentals` | Ausleihen auflisten / anlegen |
| POST | `/api/rentals/{id}/return` | Rückgabe (optional mit Schadenbericht) |

## Herkunft & Erweiterbarkeit (Teil d)
Dieses Projekt (Teil c) wurde aus dem Ski-Verleih abgeleitet: Personen, Ausweis-Upload, Gutscheine, Banner, Dashboard, Schadenbericht und der dynamische Preis sind gleich geblieben. Fahrrad-spezifisch sind die Kategorien mit Preis und Kaution, die automatische Zuweisung, Akku und Laden sowie das Zubehör (siehe oben). Eine echte Plugin-Architektur, bei der mehrere Verleih-Typen gleichzeitig laufen (Teil d), ist nicht umgesetzt.
