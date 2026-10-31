# Product: nudged

## Goal
Developers who run many Claude Code sessions in parallel on macOS get a tray app. The app tells them when a session waits for input, and they can answer permission requests and questions in a small panel, so that no session waits without notice.

## Scope
- In: one Go binary with three roles: `nudged` (tray app and daemon), `nudged hook` (Claude Code calls it for each event), `nudged statusline` (sends model, effort and context fill, and wraps the existing statusline of the user).
- In: four events: Permission (`PermissionRequest`), Question (`PreToolUse` on `AskUserQuestion`), Session done (`Stop`), Session waits (`Notification` idle).
- In: floating panels in macOS style (light and dark), stacked, oldest on top, with a counter. Modes "notify only" and "answer directly". Timeout (default 30 min). The panel closes when the user answers in the terminal.
- In: fallback. When the daemon is not running or is paused, `nudged hook` returns at once with no output, and Claude Code continues as without nudged.
- In: tray menu with the session list (waiting first, then last activity), a badge, tokens and cost today (≈ API prices), Pause (⌥⌘P), Settings (⌘,), Quit (⌘Q).
- In: settings window and the file `~/.config/nudged/config.toml`. Hook install and removal in `~/.claude/settings.json`. Start at login. Synthesized sounds, off by default.
- In: focus jump to Terminal.app and iTerm2 by TTY (AppleScript). Ghostty is best effort: the app comes to the front.
- In: Phase 0 spike report, CI on GitHub Actions (`macos-latest`), release with GoReleaser to the tap `rokde/homebrew-tap` (unsigned), README, MIT license.
- Out: Linux, Windows (Phase 3).
- Out: Codex and other agents (Phase 3; Codex cannot answer permission requests).
- Out: signing and notarization, the official homebrew/cask (Phase 3, costs money).
- Out: OS Notification Center (it cannot show multi-select and free text).
- Out: subscription limits through unofficial APIs (not stable).
- Out: widgets (too much work, platform specific).
- Out: focus jump for VS Code and JetBrains (not used by the target group).
- Out: Tauri v2 (Wails v3.0.0 is stable now; Tauri is only the fallback if the spike fails).
- Out: frontend unit tests (the views stay dumb; Go owns the logic and its tests).

## Stack
- Go 1.27 (installed: go1.27.3 darwin/arm64). Test runner: `go test ./...`. CGO on, because Wails needs it.
- UI: Wails v3.0.0. Frontend: Vue 3 + TypeScript + Vite in `frontend/` (Node 26, npm 11). The Go binary embeds `frontend/dist`. Go and JS talk only through Wails events (`@wailsio/runtime`). Thus, no `wails3` CLI and no generated bindings.
- Frontend check: `npm --prefix frontend run build` (runs `vue-tsc --noEmit` and `vite build`).
- Build: `make build` (frontend build, then `go build -o bin/nudged .`). Start: `make run`, or `./bin/nudged`. Headless daemon for tests: `nudged daemon`.
- CI: GitHub Actions on `macos-latest`: `go vet`, `go test ./...`, frontend build.
- Release: GoReleaser in GitHub Actions on a `v*` tag. Universal binary (arm64 + amd64) in `nudged.app`, cask update in `rokde/homebrew-tap`.

## Dependencies
- github.com/wailsapp/wails/v3 v3.0.0: tray icon and frameless webview panels; the Go standard library has no GUI.
- github.com/BurntSushi/toml: the config file is TOML; the standard library has no TOML parser.
- vue, vite, @vitejs/plugin-vue, typescript, vue-tsc: the PRD picks Vue 3/TS for the panels; Vite builds them into one embedded bundle.
- @wailsio/runtime: JS side of the Wails events between panels and Go.
- GoReleaser (CI only, not a Go module): GitHub release, universal binary and cask update in one config.
- Sounds, AppleScript, Login item: none. Go writes the WAV data, `afplay` plays it, `osascript` runs AppleScript, a LaunchAgent plist starts the app at login.

## Architecture
- `main.go` dispatches the subcommands: none (app), `daemon`, `hook`, `statusline`, `status`, `answer`, `install-hooks`, `uninstall-hooks`, `usage`, `focus`, `sound`, `version`. Each subcommand lives in its own file `cmd_<name>.go` and registers itself in a map (no edit of `main.go` needed).
- `internal/proto`: the socket path `~/.config/nudged/nudged.sock` (mode 0600), the message types (one JSON line request, one JSON line reply), the event kinds.
- `internal/hookclient` and `internal/statusline`: short-lived clients. They read the Claude Code JSON from stdin, send it to the socket, and print the reply. On any error they return at once with no output.
- `internal/daemon`: socket server, session registry (project, path, status, terminal, TTY, model, effort, context %, last activity), queue of pending requests (block, answer, timeout, cancel, pause). It publishes changes on a Go channel.
- `internal/config` (TOML), `internal/installer` (`~/.claude/settings.json`), `internal/usage` (`~/.claude/projects/**/*.jsonl`), `internal/focus` (AppleScript), `internal/sound` (WAV synthesis), `internal/loginitem` (LaunchAgent): plain Go libraries with tests.
- `internal/app`: the Wails app. It runs the daemon in the same process, owns the tray and the panel stack, and sends the registry changes to the views as Wails events. Each panel kind registers itself in `internal/app/panels.go` by `init()` in its own file.
- `frontend/src/main.ts` routes by URL hash to `frontend/src/views/*.vue` through `import.meta.glob`. A new view is a new file only.
- Data flow: Claude Code → `nudged hook` → socket → daemon → panel → answer → daemon → `nudged hook` stdout → Claude Code.

## Slices
1. S1 Walking skeleton: `go run . version` prints the version; the repo has `go.mod`, `main.go` with the subcommand map, `internal/proto` (socket path, message types, event kinds), `Makefile` (`test`, `build`, `run`), a passing test, the CI workflow on `macos-latest`, `LICENSE` (MIT) and `.gitignore`.
2. S2 Spike report: `docs/spike.md` answers the 9 open technical questions of the PRD with evidence (Claude Code docs and tests against the installed Claude Code 2.1.316), names the exact hook input and output JSON for each event, and records each decision or fallback. Needs S1.
3. S3 Config file: `internal/config` loads and saves `~/.config/nudged/config.toml` with all PRD defaults, creates it on first use, and keeps hand edits; `nudged config` prints the path and the values. Needs S1.
4. S4 Hook client: `nudged hook <event>` reads the hook JSON from stdin, adds TTY (from the parent process via `ps`) and `TERM_PROGRAM`, sends it to the socket, waits for the reply, and prints it; with no socket it exits 0 at once with no output (test with a fake server). Needs S1.
5. S5 Statusline wrapper: `nudged statusline [--wrap '<cmd>']` sends model, effort and context % to the socket (short timeout, fire and forget), runs the wrapped command with the same stdin, and prints its output; with no wrapped command it prints a short default line. Needs S1.
6. S6 Daemon and session registry: `nudged daemon` listens on the socket (mode 0600), tracks sessions from `SessionStart`, `SessionEnd`, `UserPromptSubmit`, `PostToolUse`, `Stop`, `Notification` and statusline messages, and `nudged status` prints the sessions sorted (waiting first, then last activity). Needs S1.
7. S7 Pending requests: the daemon holds each `PermissionRequest` and `AskUserQuestion` hook open in a queue (oldest first), `nudged answer <id> <json>` answers it, and a request ends with no output on timeout, on pause, on client disconnect, or on the next event of the same session (answer in the terminal); headless end-to-end test with the real hook client. Needs S4, S6.
8. S8 Hook installer: `nudged install-hooks` adds the nudged hooks (all events of S6 and S7, timeout 1800 s, absolute binary path) and wraps the existing statusline in `~/.claude/settings.json`; `nudged uninstall-hooks` removes only the nudged entries and restores the statusline; both keep other hooks, write a backup, and are idempotent. Needs S1.
9. S9 Usage today: `internal/usage` sums tokens and cost (≈ API prices from a price table per model) of today from `~/.claude/projects/**/*.jsonl`; `nudged usage` prints the totals (tests with fixture logs). Needs S1.
10. S10 Focus jump: `nudged focus --tty <tty> --term <TERM_PROGRAM>` brings the window of that TTY to the front in Terminal.app and iTerm2 by AppleScript, activates Ghostty as best effort, and ignores all errors silently (tests check the generated script). Needs S1.
11. S11 Sounds: `internal/sound` synthesizes the three PRD tones as WAV data (two soft rising tones, a soft bell, a short low tone) and `nudged sound <permission|question|notice>` plays one with `afplay` (tests check the WAV header and length). Needs S1.
12. S12 Tray app shell: `nudged` with no arguments starts the Wails app with no Dock icon, runs the daemon in the same process, shows a template tray icon with the menu (Pause ⌥⌘P, Settings ⌘,, Quit ⌘Q), and serves the Vue frontend with the hash router; Pause sets the daemon pause flag and changes the icon. Needs S7.
13. S13 Global pause hotkey: ⌥⌘P toggles pause from any app (Carbon `RegisterEventHotKey` via cgo in its own file), and the tray menu shows the current state. Needs S12.
14. S14 Panel stack: each pending request opens a frameless floating panel near the top right in light or dark mode; the oldest is on top, a counter shows the others, a panel closes when its request ends, the panel follows the config (show per event, mode, sound per event), and "notify only" mode turns a click into a focus jump; a generic view shows the raw request. Needs S3, S10, S11, S12.
15. S15 Permission panel: the panel shows tool, command or parameters, description, project, path, terminal, session, model, wait time and a countdown, and Allow (⏎), Deny (esc) and "Always allow …" (from `permission_suggestions`) send the right `PermissionRequest` decision back. Needs S14.
16. S16 Question panel: the panel shows one tab per question, the options with descriptions (single or multi select), and a free-text field; Next, Skip and "Open in terminal" work, and the answers go back in the format that the spike found (or, as fallback, the panel only shows and jumps to the terminal). Needs S2, S14.
17. S17 Notice panels: `Stop` shows "Session done" (project, session, duration) and idle `Notification` shows "Session waits" (project, session, since when); a click jumps to the terminal, and the panels hide after the configured time (defaults 5 s and 10 s). Needs S14.
18. S18 Session list and badge: a click on the tray icon opens a list of all sessions (project, path, status, terminal, model, effort, context %) with tokens and cost today at the bottom, the icon shows a badge with the number of waiting sessions, and a click on a session jumps to its terminal. Needs S9, S10, S12.
19. S19 Settings window: ⌘, opens a window for all PRD settings (mode, timeout, show, sound with a test button, and hide time per event), buttons to install and remove the hooks, and a "Start at login" switch (LaunchAgent plist in `~/Library/LaunchAgents`); changes save to `config.toml` and apply at once. Needs S3, S8, S11, S12.
20. S20 Release: a `v*` tag runs GoReleaser on `macos-latest`, builds the universal `nudged.app` (Info.plist with `LSUIElement`) as a zip on GitHub Releases, and updates the cask in `rokde/homebrew-tap` with a `postflight` that removes the quarantine flag; the README describes install (`brew install --cask rokde/tap/nudged`), `xattr -d com.apple.quarantine`, `nudged install-hooks`, settings, and the fallback. Needs S12.

## Open questions
- The cask update needs the repo `rokde/homebrew-tap` and a token secret (for example `HOMEBREW_TAP_TOKEN`) in `rokde/nudged`. Decision for now: S20 creates the config, and the cask step skips when the secret is missing. The human must create the tap repo and the secret before the first tag.
