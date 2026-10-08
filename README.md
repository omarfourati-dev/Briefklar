# Briefklar

Briefklar erklärt deutsche Behördenbriefe – Ausländerbehörde, Finanzamt, Krankenkasse, Beitragsservice – in einfacher
Sprache auf Deutsch, Englisch, Französisch und Arabisch: was die Behörde will, bis wann, was zu tun ist, dazu die Frist
als Kalendereintrag und ein Antwort-Entwurf. **Persönliche Daten werden auf dem Server geschwärzt, bevor die KI den
Text sieht. Briefe werden nicht gespeichert.**

**Live:** https://briefklar.omarfourati.de · Demo-Zugang auf der Anmeldeseite (`demo@briefklar.app` / `demo-briefklar`,
nur Beispielbriefe) · Keine Rechtsberatung.

Ein Portfolio-Projekt von [Omar Fourati](https://omarfourati.de), gebaut mit **Go** und **Angular**.

## Datenschutz-Ablauf

```
Browser ──► POST /api/letters/preview   (Foto | PDF | Text, max. 10 MB, Fotos vorher im Browser gedreht/verkleinert)
              ├─ Foto        → Tesseract (deu) im Container ─┐
              ├─ PDF mit Text → pdftotext ────────────────────┤   Temp-Dateien nur in tmpfs, sofort gelöscht
              └─ Text        ─────────────────────────────────┘
                              ▼
              Schwärzen: Namen, Adressen, IBAN, Steuer-ID, Aktenzeichen, Telefon, E-Mail, Geburtsdatum → [NAME_1] …
◄──────────── Vorschau = exakt der Text, den die KI bekommt; Wörter per Klick zusätzlich schwärzen
Browser ──► POST /api/letters/explain   (nur geschwärzter Text; Tageslimit; Server schwärzt noch einmal)
              ▼
              OpenAI mit festem JSON-Schema → Behörde, Frist, Dringlichkeit, Zusammenfassung ×4, Checkliste, Antwort
◄──────────── Ergebnis mit Platzhaltern → der Browser setzt die echten Werte wieder ein; .ics entsteht im Browser
```

Die Erkennung ist bewusst konservativ und nicht perfekt – deshalb die Vorschau. In der Datenbank liegen nur Konten und
ein Zähler pro Tag (`internal/store/migrations/001_init.sql`).

## Go-Konzepte im Projekt

| Konzept | Wo |
|---|---|
| `net/http` mit Routing-Mustern (`"POST /api/letters/preview"`, `{id}`) ohne Framework | `internal/server/server.go` |
| Middleware als `func(http.Handler) http.Handler` (Auth, Sicherheits-Header, Request-Log) | `internal/auth/middleware.go`, `internal/server/server.go` |
| `context` für Timeouts (OCR 45 s, KI 60 s) und Abbruch von Unterprozessen | `internal/letters/handler.go`, `internal/extract/extract.go` |
| Interfaces für austauschbare Teile (Explainer, Extractor, Quota) – Fakes in Tests | `internal/explain/explain.go`, `internal/letters/handler.go` |
| `os/exec` mit `CommandContext`, `WaitDelay`, begrenzten Puffern | `internal/extract/extract.go` |
| Typed Errors (`errors.Is/As`, `ToolError` ohne stderr im Text) | `internal/extract/extract.go` |
| Nebenläufigkeit: Semaphore für OCR, Mutex in der Login-Drosselung | `internal/letters/handler.go`, `internal/auth/throttle.go` |
| `embed` – Landingpage und Angular-App im Binary | `static/static.go`, `internal/server/static.go` |
| `log/slog` (JSON, ohne Brieftext) | `cmd/briefklar/main.go` |
| `pgx` + eingebettete Migrationen unter Advisory-Lock, atomares Tageslimit (`INSERT … ON CONFLICT … RETURNING`) | `internal/store/store.go` |
| Graceful Shutdown, `-healthcheck`-Flag für Docker | `cmd/briefklar/main.go` |
| Tabellen-Tests, `testcontainers-go`, `httptest` | `internal/*/*_test.go` |

## Angular-Konzepte im Projekt

| Konzept | Wo |
|---|---|
| Standalone Components, Lazy Loading der Seiten | `web/src/app/app.routes.ts` |
| Signals, `computed`, `effect`, `input()` | `web/src/app/pages/letter.page.ts`, `web/src/app/result/result-view.ts` |
| Neuer Control Flow `@if` / `@for` / `@switch` / `@let` | Templates der Seiten |
| Funktionaler `HttpInterceptor` (Token, 401-Behandlung) | `web/src/app/core/auth.interceptor.ts` |
| Funktionale Route Guards (Login, Admin, Demo) | `web/src/app/core/guards.ts` |
| Reactive Forms | `web/src/app/pages/login.page.ts`, `account.page.ts`, `admin.page.ts` |
| Rechts-nach-links für Arabisch (`dir`, `lang`) | `web/src/app/result/result-view.ts` |
| Reine Logik getrennt und getestet (Wiedereinsetzen, Schwärzen, .ics, Ampel, Bild-Drehung) | `web/src/app/lib/` |
| PWA: Manifest, eigener Service Worker, Update-Hinweis, „Teilen → Briefklar“ (Web Share Target) | `web/public/sw.js`, `web/src/app/core/pwa.ts`, `web/src/app/lib/shared.ts` |

## Lokal entwickeln

Kein lokales Go nötig – Go läuft im Container (`dev.Dockerfile`, mit Tesseract und poppler):

```bash
scripts/go.sh test ./...                       # Backend-Tests (startet Postgres per testcontainers)
docker compose up -d postgres                  # nur die Datenbank
cd web && npm ci && npx ng serve               # Angular auf :4200, /api → :8080
JWT_SECRET=$(openssl rand -hex 32) docker compose --profile app up --build   # alles, KI im Testmodus (EXPLAINER=fake)
cd web && npx playwright test                  # E2E gegen den Stack (E2E_BASE_URL, Standard :8080)
```

## Tests

- Go: Schwärzen (Namen, Adressen, Behörden, Nummern, Platzhalter-Nummerierung), Texterkennung mit echten Tools,
  KI-Client gegen `httptest`, Login/Drosselung/Rollen, Admin-API, Brief-API (u. a. „Brieftext nie im Log“), Server
- Angular (Vitest): Auth, Guards, Interceptor, Vorschau = gesendeter Text, Ergebnis, Kalender, Ampel, Bild
- Playwright: Landingpage, Demo mit Arabisch und .ics, Nutzer mit Schwärzung und Erklärung

## App installieren (PWA)

Briefklar lässt sich auf dem Handy installieren („Zum Startbildschirm hinzufügen“). Auf Android erscheint die App
danach im Teilen-Menü: Foto oder PDF eines Briefs teilen → Briefklar öffnet direkt die Vorschau. Der Service Worker
speichert nur das App-Gerüst (HTML, JS, CSS, Icons) – **nie** etwas unter `/api`. Ein geteilter Brief liegt nur
kurz im Browser-Cache und wird beim Öffnen sofort gelöscht. Jeder Deploy erzeugt einen neuen Service Worker
(`web/scripts/stamp-sw.mjs`), die App bietet dann „Neu laden“ an.

## Betrieb

GitHub Actions (`.github/workflows/`): Tests auf GitHub-Runnern, danach Deploy auf myvps (Self-hosted-Runner),
Docker Compose (`docker-compose.prod.yml`, tmpfs für `/tmp`), Caddy mit HTTPS, Kennzahlen unter `/metrics` nur im
Docker-Netz (Prometheus/Grafana mit Alarmen).
