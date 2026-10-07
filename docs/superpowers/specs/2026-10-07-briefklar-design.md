# Briefklar – Design

Stand: 07.10.2026 · Status: abgestimmt mit Omar Fourati, wartet auf Review dieser Datei

## 1. Ziel

Briefklar erklärt deutsche Behördenbriefe (Ausländerbehörde, Finanzamt, Krankenkasse, Rundfunkbeitrag …):
Was will die Behörde, bis wann, was ist zu tun – in Deutsch, Englisch, Französisch und Arabisch –,
dazu die Frist als Kalendereintrag (.ics) und ein Antwort-Entwurf.

Zweck: Portfolio-Projekt für die Jobsuche. Es soll zwei Technologien zeigen, die im Lebenslauf noch fehlen:
**Go** (Backend) und **Angular** (Frontend). Code und README erklären die Go- und Angular-typischen Konzepte,
damit Omar das Projekt im Vorstellungsgespräch vertreten kann.

Erfolgskriterien:
- Live unter `https://briefklar.omarfourati.de`, Demo ohne Kosten nutzbar
- Personenbezogene Daten verlassen den Server nicht ungeschwärzt; nichts wird gespeichert
- CI mit Unit-, Integrations- und Playwright-Tests, automatischer Deploy, Monitoring mit Alarmen

Nicht im Umfang (YAGNI): Selbstregistrierung, Speichern von Briefen oder Verläufen, OCR anderer Sprachen als
Deutsch, mobile App, Rechtsberatung (die App weist darauf hin, dass sie keine Rechtsberatung ist).

## 2. Architektur

Ein Go-Programm (ein Container) liefert API und die gebaute Angular-App aus (`embed`), wie Spring Boot bei Belegfluss.

```
Angular ──► Go: POST /api/letters/preview  (Foto | PDF | Text, max. 10 MB)
              ├─ Foto → Tesseract (deu) im Container ─┐
              ├─ PDF mit Textebene → Text ─────────────┤
              └─ Text ────────────────────────────────┘
                              ▼
              Schwärzen → Text mit Platzhaltern [NAME_1], [IBAN_1] … + Liste der Funde
◄──────────── Vorschau: geschwärzter Text + Funde (Werte bleiben nur im Browser)
Angular: Nutzer prüft, markiert weitere Stellen
Angular ──► Go: POST /api/letters/explain  (nur geschwärzter Text; Tageslimit)
              ▼
              OpenAI (JSON-Schema) → Behörde, Frist, Dringlichkeit, Zusammenfassung ×4,
                                       Checkliste, Antwort-Entwurf
◄──────────── Ergebnis mit Platzhaltern
Angular: setzt die Originalwerte lokal wieder ein; .ics wird im Browser aus Frist erzeugt
```

Entscheidungen:
- **Zwei Schritte (Vorschau, dann Erklären):** Der Nutzer sieht vor dem KI-Aufruf, was geschwärzt wird, und kann
  ergänzen. Die Originalwerte der Funde gehen in der Vorschau-Antwort an den Browser zurück und werden nie an die KI
  gesendet; der Server speichert zwischen den Schritten nichts.
- **OCR auf dem Server** mit dem Tesseract-CLI (`exec.CommandContext`, Timeout), nicht per cgo – einfacher Build.
- **PDF-Text** mit einer reinen Go-Bibliothek (z. B. `ledongthuc/pdf`); PDFs ohne Textebene werden in Bilder
  gerendert (`pdftoppm` aus poppler-utils) und durch Tesseract geschickt (max. 3 Seiten).
- **.ics im Browser** erzeugt (kein Server-Endpunkt nötig, keine Daten zurück zum Server).

## 3. Schwärzen

Erkennung per Regex und Heuristik, jede Regel einzeln getestet:
- IBAN (mit Prüfziffer mod 97), Steuer-ID (11 Ziffern mit Prüfziffer), Steuernummer, Aktenzeichen/Geschäftszeichen
  („Az.“, „Aktenzeichen:“, „Unser Zeichen:“ …), Kunden-/Beitragsnummern, Geburtsdatum (Datum nach „geb.“),
  Telefonnummern, E-Mail-Adressen, Postleitzahl + Ort, Straße + Hausnummer
- Namen: Zeile nach „Herrn/Frau“ im Adressfeld und Anrede „Sehr geehrte(r) Frau/Herr X“
- Gleiche Werte bekommen denselben Platzhalter (`[NAME_1]` bleibt `[NAME_1]`)
- Fristen und Behördennamen werden **nicht** geschwärzt (die KI braucht sie)

Grenzen werden offen genannt: Erkennung ist nicht perfekt, deshalb die Vorschau mit manueller Ergänzung.

## 4. KI-Aufruf

- OpenAI Chat Completions mit `response_format: json_schema` (strict), Modell per Konfiguration (Standard `gpt-4o-mini`)
- Antwort-Struktur: `authority`, `letterType`, `deadline` (ISO-Datum oder null), `deadlineText`,
  `urgency` (`red|yellow|green`), `summary` {de,en,fr,ar}, `actions` [{de,en,fr,ar}], `replyDraft` (Deutsch),
  `missingInfo` (was im Brief unklar ist)
- Prompt verlangt: Platzhalter unverändert übernehmen, nichts erfinden, Frist nur aus dem Text
- `context`-Timeout 60 s; Fehlermeldungen des Anbieters werden nie an den Client durchgereicht (Lehre aus Belegfluss)
- Interface `Explainer` – im Test ersetzt durch eine Fake-Implementierung, die CI braucht kein Guthaben

## 5. Benutzer, Login, Limits

Modell wie Belegfluss:
- Konten legt der Admin an (Rollen `user`, `admin`), Konten sperren/entsperren, Passwort selbst ändern
- Login liefert ein JWT (HS256, 8 h); bcrypt; Login-Drosselung (5 Fehlversuche pro Konto+IP / 20 pro IP in 15 min)
- **Demo-Konto** (`demo@briefklar.app`, öffentliches Passwort auf der Login-Seite): sieht nur die Beispielbriefe,
  `preview` und `explain` liefern 403 (keine Uploads, keine KI-Kosten)
- **Tageslimit** pro Konto (Standard 20 Erklärungen), gezählt in Postgres; Überschreitung → 429
- „Zugang anfragen“ per E-Mail an info@omarfourati.de
- Admin-Konto aus Umgebungsvariablen beim Start (wie Belegfluss)

Datenbank (Postgres auf `zentrades-postgres`, eigene Rolle/DB `briefklar`, Migrationen mit `golang-migrate`
oder eingebetteten SQL-Dateien): `app_user`, `usage_day (user_id, day, count)`. Keine Brief-Tabellen.

## 6. Oberfläche (Angular, aktuelle Version, Standalone, Signals)

1. **Landingpage `/`** (statisch, ohne JS lesbar): Erklärung, Beispiel, FAQ, SEO (Meta, Open Graph, JSON-LD),
   `robots.txt`, `sitemap.xml`, `llms.txt`, Link zur App
2. **App `/app/`**: Login (mit „Als Demo anmelden“), Neuer Brief (Upload/Text → Vorschau mit gelben Balken, Wort
   anklicken = schwärzen → „Erklären lassen“), Ergebnis (Behörde, Ampel, Frist + .ics, Sprachumschalter DE/EN/FR/AR
   mit `dir="rtl"` für Arabisch, Checkliste, Antwort-Entwurf kopieren), Beispielbriefe, Admin, Konto
3. Beispielbriefe: 3 erfundene Briefe (Ausländerbehörde – Verlängerung Aufenthaltstitel, Finanzamt – fehlende
   Belege, Rundfunkbeitrag – Zahlungserinnerung) mit fertigem Ergebnis als JSON im Frontend
4. Hinweis überall: „Keine Rechtsberatung. Im Zweifel bei der Behörde nachfragen.“

## 7. Fehler

RFC 9457 Problem Details. OCR ohne brauchbaren Text → 422 mit Hinweis „schärfer fotografieren“ und erkanntem Text;
falscher Typ/zu groß → 400/413; KI-Timeout/-Fehler → 502 mit neutraler Meldung; Limit → 429; Demo → 403.

## 8. Tests

- Go: Tabellen-Tests für alle Schwärzungsregeln und das Wiedereinsetzen, Problem-Details, JWT, Drosselung,
  Limits; Integrationstests mit `testcontainers-go` (Postgres) und Fake-`Explainer`; OCR-Test mit einem
  generierten Bild (läuft im Docker-Build bzw. CI, wo Tesseract installiert ist)
- Angular: Komponententests (Vorschau, Sprachumschalter/RTL, .ics-Erzeugung)
- Playwright gegen den Docker-Stack: Landingpage, Demo-Login, Beispielbrief, Sprachwechsel, .ics-Download,
  Admin legt Nutzer an, Nutzer erklärt einen Text-Brief (Fake-KI über Umgebungsvariable `EXPLAINER=fake` nur in CI)

## 9. Betrieb

- Dockerfile mehrstufig: Node (Angular) → Go (statisches Binary) → Debian slim mit `tesseract-ocr`,
  `tesseract-ocr-deu`, `poppler-utils`, nicht-root
- GitHub Actions: `ci.yml` (Build, Tests, E2E) und `deploy.yml` (Self-hosted-Runner der Organisation, wie Belegfluss);
  Secrets: `OPENAI_API_KEY` (vorhanden), JWT-Secret, Admin-Passwort, DB-Passwort werden im Deploy erzeugt bzw. gesetzt
- Caddy: `briefklar.omarfourati.de` mit `import access_log`, `/metrics` öffentlich 404, `request_body` 11 MB
- Prometheus-Kennzahlen: `briefklar_letters_total{input,outcome}`, `briefklar_ocr_duration_seconds`,
  `briefklar_explain_duration_seconds`, `briefklar_redactions_total{kind}`, `briefklar_logins_total{outcome}`
- Monitoring-Repo: Job, Probe, Alarme (Kennzahlen weg, KI-Fehler gehäuft, Login-Drosselung), Dashboard „Briefklar“
- Portfolio, Lebenslauf, LinkedIn danach ergänzen (Go, Angular)
