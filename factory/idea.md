# Idea
lies das PRD von https://claude.ai/artifact/4FSUSazTkkEPLHN2uLc4HZ#d314adb5-4e3d komplett und setz das bitte um im Alleingang. ich komme dann am Ende, um es auf MacOs zu testen. DAs Github Repo soll dann public available sein, also können wir auch alle pipelines da nutzen für test und build.

## Orchestrator notes
- The PRD below is a full transcription of the linked doc (as of 2026-10-31), made by
  the orchestrator because agents cannot open the doc. Diagrams are rendered as text.
- The GitHub repo `rokde/nudged` is public now. GitHub Actions (incl. `macos-latest`
  runners) can be used for test and build.
- The human works alone until the end, then tests by hand on macOS. No human
  checkpoint in between. Prefer decisions that keep moving; record them.
- The factory runs on macOS (Darwin) locally, so macOS-only code can be built and
  unit-tested locally too.
- Scope for this run: Phase 0 (spike), Phase 1 (MVP), Phase 2 (release). Phase 3
  (other agents, Linux/Windows, signing) is out.

---

# PRD: nudged (transcribed)

Date: 2026-10-30 · Author: Robert Kummer

## Überblick & Ziele
nudged ist ein Open-Source-Tray-Tool, das alle laufenden Claude-Code-Sessions beobachtet und sich meldet, sobald eine Session Eingabe braucht. Freigaben und Rückfragen lassen sich direkt im eigenen Dialog beantworten, ohne das passende Terminal suchen zu müssen.

- **Problem:** Bei mehreren parallelen Sessions bleibt eine Session oft minutenlang unbemerkt an einer Freigabe oder Frage hängen.
- **Zielgruppe:** Entwickler, die mehrere Claude-Code-Sessions parallel in Terminal.app, iTerm2 oder Ghostty betreiben.
- **Ziel v1:** Keine Session wartet mehr unbemerkt; Freigaben und Fragen sind in Sekunden beantwortet; alle Sessions mit Projekt, Modell, Effort und Kontextfüllung auf einen Klick sichtbar.
- **Name:** nudged — „angestupst“ und zugleich die Unix-Konvention für einen Daemon (`sshd`, `launchd`).

## Scope und Nicht-Ziele (v1)
v1 ist macOS-first und unterstützt ausschließlich Claude Code.

| | In v1 | Nicht in v1 |
|---|---|---|
| Plattform | macOS (Must) | Linux, Windows (später, nice to have) |
| Agenten | Claude Code | Codex und andere Harnesses |
| Interaktion | Eigene Dialoge im macOS-Look | OS-Notification-Center |
| Verbrauch | Tokens und Kosten aus lokalen Logs | Abo-Limits über inoffizielle APIs |
| Widgets | — | macOS-, Windows-, Linux-Widgets |
| Fokus-Sprung | Terminal.app, iTerm2, Ghostty (best effort) | VS Code, JetBrains |
| Verteilung | Eigener Homebrew-Tap, unsigniert | Signierung, Notarisierung, offizielles homebrew/cask |

## Ereignisse & Dialoge
nudged kennt vier Ereignisse; Freigabe und Frage können direkt beantwortet werden, die anderen beiden sind reine Hinweise.

| Ereignis | Auslöser (Claude-Code-Hook) | Dialog zeigt | Aktionen | Ausblenden (Default) |
|---|---|---|---|---|
| Freigabe | `PermissionRequest` | Tool, Befehl/Parameter, Beschreibung, Projekt, Pfad, Terminal, Session, Modell, Wartezeit, Countdown | Erlauben (⏎), Ablehnen (esc), Immer erlauben … | Bleibt stehen |
| Frage | `PreToolUse` auf `AskUserQuestion` | Reiter je Frage, Frage, Optionen mit Beschreibung, Freitextfeld | Auswahl (Single/Multi), Weiter, Überspringen, Im Terminal öffnen | Bleibt stehen |
| Session fertig | `Stop` | Projekt, Session, Dauer | Zum Terminal springen | Nach 5 Sek. |
| Session wartet | `Notification` (idle) | Projekt, Session, seit wann | Zum Terminal springen | Nach 10 Sek. |

**Verhaltensregeln**
- **Modus:** „Nur benachrichtigen“ zeigt den Dialog ohne Antwortmöglichkeit (Klick springt ins Terminal); „Direkt beantworten“ erlaubt die Antwort im Dialog.
- **Timeout:** Ohne Antwort übernimmt nach einstellbarer Zeit wieder das Terminal; Default 30 Minuten.
- **Antwort im Terminal:** Wird im Terminal geantwortet, schließt der Dialog sofort (Pflicht, nicht abschaltbar).
- **Fallback:** Ist nudged nicht erreichbar, gibt der Hook sofort an Claude Code zurück; die Session verhält sich wie ohne nudged.
- **Mehrere Anfragen:** Dialoge stapeln sich; der älteste liegt oben, ein Zähler zeigt weitere wartende.
- **Pausieren:** Über Tray-Menü oder ⌥⌘P; während der Pause greift nur der Fallback.
- **Look:** Eigene schwebende Panels nah am macOS-Stil (hell/dunkel), kein OS-Notification-Center.

## Tray & Session-Liste
Ein Klick aufs Tray-Icon zeigt alle laufenden Sessions; ein Badge am Icon signalisiert wartende Sessions.

| Feld | Quelle | Status |
|---|---|---|
| Projekt, Pfad | `SessionStart`-Hook (cwd) | gesichert |
| Status (wartet / arbeitet / fertig) | Hook-Ereignisse | gesichert |
| Terminal | Prozessbaum / TTY | zu prüfen |
| Modell | Statusline-JSON | gesichert |
| Effort | Statusline-JSON oder Settings | zu prüfen |
| Kontextfüllung (%) | Statusline-JSON | zu prüfen (Feldverfügbarkeit) |
| Tokens & Kosten heute | Lokale Session-Logs (`~/.claude/projects`) | gesichert, Kosten als „≈ API-Preise“ |

- **Sortierung:** Wartende Sessions zuerst, dann nach letzter Aktivität.
- **Fokus-Sprung:** Klick auf eine Session holt das Terminalfenster nach vorn — Terminal.app und iTerm2 per AppleScript über die TTY, Ghostty best effort; Fehlschläge werden still ignoriert.
- **Menü:** Benachrichtigungen pausieren (⌥⌘P), Einstellungen … (⌘,), Beenden (⌘Q).

## Einstellungen & Töne
Alle Töne sind standardmäßig aus; jeder lässt sich einzeln einschalten und probehören.

| Einstellung | Werte | Default |
|---|---|---|
| Modus | Nur benachrichtigen / Direkt beantworten | Direkt beantworten |
| Timeout bis Terminal übernimmt | Minuten | 30 |
| Dialog schließen bei Antwort im Terminal | fest an | an |
| Anzeigen je Ereignis | an / aus | an |
| Ton je Ereignis | an / aus + Probe-Button | aus |
| Ausblenden je Ereignis | Bleibt stehen / 5 Sek. / 10 Sek. | siehe Ereignistabelle |
| Hooks | Installieren / Entfernen in `~/.claude/settings.json` | — |
| Beim Login starten | an / aus | an |

**Töne:** selbst synthetisiert, liegen unter der Repo-Lizenz, keine Fremd-Samples; Lautstärke folgt dem System.
- **Freigabe:** zwei weiche, aufsteigende Töne.
- **Frage:** ein sanfter Glockenton.
- **Session fertig / wartet:** ein kurzer, tiefer Ton.

Konfiguration liegt als Datei unter `~/.config/nudged/config.toml` und ist auch per Hand editierbar.

## Architektur & Technik
Diagram "Architektur · ein Binary, drei Rollen" (as text):
- Claude-Code-Sessions (hooks: PermissionRequest, PreToolUse, Stop, Notification, SessionStart/End; statusline command)
  ↔ `nudged hook` (blocks until answer or timeout) ↔ nudged App (daemon)
  → `nudged statusline` (model, effort, context fill) → nudged App
- nudged App (daemon) contains: Session-Registry (Unix socket, event queue) · Dialoge (Freigabe, Frage, Hinweise) · Tray-Menü (session list, badge) · Einstellungen · Log-Leser (tokens and cost today)
- App → Terminals (Terminal.app, iTerm2, Ghostty): Fokus-Sprung (AppleScript)
- Session-Logs (`~/.claude/projects`) → Log-Leser

Claude Code ruft `nudged hook` und `nudged statusline` auf; beide sprechen über einen lokalen Unix-Socket mit der laufenden App, die Dialoge, Tray und Fokus-Sprung übernimmt.

- **Sprache:** Go; UI mit Wails v3 (Go + Webview, Oberfläche in Vue 3/TypeScript). Fallback: Tauri v2.
- **Ein Binary, drei Rollen:** `nudged` (App), `nudged hook` (von Claude Code pro Ereignis aufgerufen, liest JSON von stdin), `nudged statusline` (liefert Sessiondaten; bestehende Statusline des Nutzers wird weitergereicht).
- **Kommunikation:** Unix-Socket unter `~/.config/nudged/`, nur Nutzerrechte, kein Netzwerk-Port.
- **Hook-Installation:** Einträge in `~/.claude/settings.json` per Einstellungsdialog setzen und wieder entfernen; vorhandene Hooks bleiben erhalten.
- **Fokus-Sprung:** AppleScript über die TTY der Session.

## Verteilung & Release
nudged wird über einen eigenen Homebrew-Tap verteilt, zunächst unsigniert.
- **Tap:** `rokde/homebrew-tap`; Installation `brew install --cask rokde/tap/nudged`.
- **Warum eigener Tap:** Das offizielle homebrew/cask nimmt seit längerem keine neuen unsignierten Casks an und hat am 2026-09-01 Gatekeeper-fehlschlagende Casks deaktiviert (https://github.com/orgs/Homebrew/discussions/7050).
- **Quarantäne:** README beschreibt `xattr -d com.apple.quarantine`; ein `postflight` im Cask wird im Spike getestet.
- **Ein Binary:** App und Hook-Client sind dasselbe Binary (`nudged hook …` als Unterbefehl).
- **Release:** GitHub Releases per GoReleaser, Universal Binary (arm64 + amd64), Cask-Update automatisch.
- **Lizenz:** MIT.
- **Später:** Signierung und Notarisierung (Apple Developer, 99 $/Jahr), danach offizielles homebrew/cask.

## Offene Technikfragen (Spike)
Der Spike klärt diese Punkte, bevor Phase 1 startet; jeder hat einen definierten Fallback.

| # | Frage | Fallback, falls nein |
|---|---|---|
| 1 | Lässt sich `AskUserQuestion` per Hook beantworten (Antworten zurückgeben)? | Nur anzeigen, Klick springt ins Terminal |
| 2 | Zeigt Claude Code den Freigabe-Dialog im Terminal parallel, während der Hook blockiert? | Terminal ist während der Wartezeit blockiert; Timeout entscheidet |
| 3 | Wie erkennt nudged eine Antwort im Terminal, um den Dialog zu schließen? | Schließen beim nächsten Hook-Ereignis der Session |
| 4 | Ist ein Hook-Timeout von 1800 s zulässig? | Kürzeres Maximum als Default dokumentieren |
| 5 | Liefert die Statusline-JSON Effort und Kontextfüllung? | Aus Logs schätzen oder Feld ausblenden |
| 6 | Ist Wails v3 stabil genug für Tray + rahmenlose Panels? | Tauri v2 mit Go-Hook-Client |
| 7 | Fokus-Sprung in Ghostty per TTY möglich? | App aktivieren statt Fenster |
| 8 | Funktioniert ein `postflight` gegen die Quarantäne im eigenen Tap? | README-Hinweis |
| 9 | Ist `nudged` auf GitHub/Homebrew frei? Eine Websuche fand keinen Konflikt, die Homebrew-Prüfung war hier nicht möglich. | Anderen Paketnamen wählen |

## Phasen & Meilensteine
Roadmap (4 Phasen, 3 Tore):
1. Phase 0 · Spike — Technikfragen 1–5, Wails v3 prüfen, Name prüfen. Gate: Spike-Ergebnis liegt vor.
2. Phase 1 · MVP — Freigabe + Frage, Tray-Liste, Einstellungen, Töne, Hook-Installer. Gate: 1 Woche Eigennutzung.
3. Phase 2 · Release — Homebrew-Tap, GoReleaser, README, Fokus-Sprung. Gate: Install per brew klappt.
4. Phase 3 · Ausbau — Codex und Co., Linux, Windows, Signierung.

Der Spike entscheidet, ob Phase 1 mit Wails v3 und beantwortbaren Fragen startet; Termine legen wir nach dem Spike fest.

## Entscheidungslog
| Datum | Entscheidung | Begründung |
|---|---|---|
| 2026-10-30 | Name `nudged` | „angestupst“ + Daemon-Konvention; Abstand zum macOS-Admin-Tool „Nudge“ |
| 2026-10-30 | Verteilung über eigenen Homebrew-Tap, unsigniert | Offizielles homebrew/cask nimmt keine unsignierten Casks an |
| 2026-10-30 | Töne selbst synthetisiert, standardmäßig aus | Keine Lizenzfragen, nicht nervig |
| 2026-10-30 | Timeout 30 Min. (einstellbar), Dialog schließt bei Antwort im Terminal | Vorgabe PO |
| 2026-10-30 | Fokus-Sprung für Terminal.app, iTerm2, Ghostty; Fehlschläge still | Genutzte Terminals |
| 2026-10-30 | Keine inoffiziellen Limit-APIs | Stabilität, nur einfach erhältliche Daten |
| 2026-10-30 | Keine Widgets | Aufwand, plattformabhängig |
| 2026-10-30 | Nur eigene Dialoge, kein OS-Notification-Center | Mehrfachauswahl und Freitext nötig |
| 2026-10-30 | Go mit Wails v3 (Fallback Tauri v2) | Ein Binary, schneller Hook-Start, Vue/TS-Oberfläche |
| 2026-10-30 | v1 nur Claude Code, nur macOS | Fokus; Codex kann Freigaben nicht beantworten |
