# Verleihsystem – Fahrrad-Verleih (Prototyp)

Nur Prototyp. Priorität: schnell verständlich, leicht änderbar.
Fachliche Anforderungen stehen in `README.md` (die Spezifikation). Bei Änderungen zuerst die Spec anpassen, dann den Code.

## Stack
- Backend: Go 1.22+ ("objektorientiert": Structs + Methoden, Interfaces wo sinnvoll), nur Standardbibliothek
- DB: In-Memory (kein Postgres/SQLite, kein ORM)
- Frontend: reines HTML/CSS/JS in `web/` (per `go:embed` vom Go-Server ausgeliefert)
- Versionierung: lokales Git, bei Bedarf GitHub

## Commands
- Starten: `go run .` → http://localhost:8080
- Tests: `go test ./...`
- Format: `go fmt ./...`
- Lint (falls eingerichtet): `golangci-lint run`

## Architektur
- `main.go` → Einstieg, HTTP-Server, Routing, Seed-Daten
- `internal/domain/` → Kern-Structs + Business-Logik (Item, Person, Rental, dynamische Preise, Gutscheine)
- `internal/domain/config.go` → Verleih-Typ: Name, Kategorien (Preis, Kaution, Akku), Zubehör, Akku-Regeln, Demo-Daten
- `internal/store/` → thread-safe In-Memory-Store (Map + Mutex) hinter dem `Store`-Interface
- `internal/handler/` → HTTP-Handler (JSON-API unter `/api/...`)
- `internal/upload/` → Prüfung aller Uploads (Endung, Magic Bytes, Größe), nur PDF/JPG/PNG
- `web/` → Frontend (`index.html`, `app.js`, `style.css`)

## Konventionen
- Methoden an Structs hängen statt freier Funktionen, wo es Sinn ergibt
- Interfaces nur bei echtem Bedarf (hier: `Store`)
- Kein Dependency-Injection-Framework, keine neuen Dependencies ohne Nachfrage
- In-Memory-Store muss thread-safe sein
- Fehler mit `fmt.Errorf("%w", err)` wrappen, nicht verschlucken
- Keine globalen Variablen außer `AppConfig`
- Keine Secrets im Code

## Frontend
- Minimales JS, klare Trennung vom Backend: Zugriff immer über die Go-Endpoints (`/api/...`)
- Benutzerfreundlich: Formulare mit Vorbelegung, Preisvorschau, klare Statusanzeigen
- Alle Texte aus Nutzereingaben HTML-escapen

## Akku
- Akkustand wird beim Zugriff lazy berechnet (`tickLocked` im Store): ausgeliehen = sinkt, lädt = steigt; Zeitraffer über `BATTERY_SPEEDUP`

## Sicherheit
- Nur JSON akzeptieren (kein XML), unbekannte Felder ablehnen, alle Eingaben serverseitig validieren
- Uploads nur über `internal/upload` annehmen, nie auf die Festplatte schreiben, mit geprüftem MIME-Typ + `nosniff` ausliefern
- Frontend: Nutzerdaten immer mit `esc()` escapen, kein Inline-JS/-CSS (Content-Security-Policy aktiv)

## Regeln
- Die README.md ist die Spezifikation: bei Änderungen zuerst README, dann Code und Tests anpassen
- Commit-Messages: kurz und imperativ
- Bei Unklarheit fragen statt raten
- Prototyp-Fokus: lieber einfach und klar als elegant und komplex
