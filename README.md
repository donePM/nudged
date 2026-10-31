# nudged

nudged is a macOS tray app for developers who run many Claude Code sessions in parallel.
It tells you when a session waits for input.
You can answer permission requests and questions in a small panel, so no session waits without notice.

nudged is not "Nudge", the macOS admin tool that makes users install OS updates.

## Install

```sh
brew install --cask donepm/tap/nudged
```

The cask installs `/Applications/nudged.app` and links `nudged` onto your PATH.

### Quarantine

The app is not signed or notarized yet, so macOS sets a quarantine flag and can block it.
The cask removes this flag after the install.
If macOS still blocks the app, remove the flag yourself:

```sh
xattr -dr com.apple.quarantine /Applications/nudged.app
```

Use this command only on `/Applications/nudged.app`.

## Setup

1. Start the app: run `nudged` with no arguments, or open `nudged.app`.
   The icon shows in the menu bar. The app has no Dock icon.
2. Connect Claude Code:

   ```sh
   nudged install-hooks
   ```

`nudged install-hooks` changes `~/.claude/settings.json`:

- It adds a `nudged hook <event>` entry for `SessionStart`, `SessionEnd`, `UserPromptSubmit`, `PostToolUse`, `Stop`, `Notification`, and `PermissionRequest` (timeout 1800 s, absolute path of the binary).
- It wraps your statusline: `nudged statusline --wrap '<your command>'` runs your command and shows its output.
- It writes a backup to `~/.claude/settings.json.nudged.bak`.
- It keeps your other hooks and settings. You can run it again with no harm.

`nudged uninstall-hooks` removes only the nudged entries and restores your statusline.
The Settings window has the same two actions as buttons.

New Claude Code sessions use the hooks. Restart sessions that run already.

## Panels

A panel opens near the top right of the screen for each event. It follows the light or dark mode of macOS.

- **Permission**: a tool asks for permission. The panel shows the tool, the command or parameters, the project, the terminal, and a countdown. Allow (⏎), Deny (esc), or "Always allow …".
- **Question**: Claude asks a question. The panel shows one tab per question with the options and a free-text field. Next, Skip, and "Open in terminal" are available.
- **Session done**: a session stops. The panel shows the project, the session, and the duration. It hides after 5 s.
- **Session waits**: a session is idle and waits for you. The panel shows the project, the session, and the time since it waits. It hides after 10 s.

A click on a notice panel ("Session done" or "Session waits") jumps to the terminal of the session.

Two modes (key `mode`):

- `answer`: you answer permission requests and questions in the panel.
- `notify`: the panel only tells you. A click on the panel jumps to the terminal, and you answer there.

The focus jump works for Terminal.app and iTerm2 (by TTY). For Ghostty, the app only comes to the front.

The panels stack: the oldest is on top, and a counter shows how many others wait.
A held request ends after the timeout (default 30 min). Claude Code then asks in the terminal.
When you answer in the terminal, the panel closes.

## Tray

- A click on the tray icon opens the session list: project, path, status, terminal, model, effort, and context fill. Waiting sessions come first. A click on a session jumps to its terminal.
- The icon shows a badge with the number of waiting sessions.
- The list shows the tokens and the cost of today (≈ API prices) at the bottom.
- **Pause** (⌥⌘P, global hotkey from any app): nudged ignores all events, and Claude Code asks in the terminal. Press ⌥⌘P again to resume.
- **Settings** (⌘,): mode, timeout, show, sound, and hide time per event, the hook buttons, and the "Start at login" switch.
- **Quit** (⌘Q).

## Start at login

Use the "Start at login" switch in Settings (key `start_at_login`).
When it is on, nudged writes the LaunchAgent `~/Library/LaunchAgents/de.donepm.nudged.plist`. macOS starts the app at the next login.
When it is off, nudged removes the plist.
Each time the tray app starts, nudged makes the plist match `start_at_login`.

## Settings

The Settings window saves to `~/.config/nudged/config.toml`.
nudged creates the file with the defaults on first use and keeps your hand edits.
`nudged config` prints the path and the values.

```toml
mode = "answer"        # "answer" (answer in the panel) or "notify" (a click jumps to the terminal)
timeout_minutes = 30   # 1 to 30 minutes until a held request ends and Claude Code asks in the terminal
start_at_login = true  # start nudged at login with a LaunchAgent

[events.permission]    # permission requests
show = true            # show a panel
sound = false          # play a sound
hide_seconds = 0       # seconds until the panel hides; 0 means the panel stays

[events.question]      # questions
show = true            # show a panel
sound = false          # play a sound
hide_seconds = 0       # seconds until the panel hides; 0 means the panel stays

[events.done]          # "Session done"
show = true            # show a panel
sound = false          # play a sound
hide_seconds = 5       # seconds until the panel hides

[events.waits]         # "Session waits"
show = true            # show a panel
sound = false          # play a sound
hide_seconds = 10      # seconds until the panel hides
```

## Subcommands

- `nudged`: start the tray app and the daemon.
- `nudged install-hooks`: add the nudged hooks and the statusline wrapper to `~/.claude/settings.json`.
- `nudged uninstall-hooks`: remove the nudged hooks and restore the statusline.
- `nudged status`: print the sessions (waiting first, then last activity).
- `nudged usage`: print the tokens and the cost of today.
- `nudged config`: print the config path and the values.
- `nudged answer <id> <json>`: answer a held request.
- `nudged focus --tty <tty> --term <TERM_PROGRAM>`: bring the terminal of that TTY to the front.
- `nudged sound <permission|question|notice>`: play a sound.
- `nudged version`: print the version.
- `nudged hook <event>`: Claude Code calls it for each event. You do not run it.
- `nudged statusline [--wrap '<cmd>']`: Claude Code calls it for the statusline. You do not run it.
- `nudged daemon`: run the daemon with no tray (for tests).

## Fallback

When the app does not run or is paused, `nudged hook` exits at once with no output.
Claude Code then works as it does without nudged.

## Build from source

You need Go and Node.js on macOS.

```sh
go mod download
npm --prefix frontend ci
make build
./bin/nudged
```

Run the tests with `make test`.

## License

MIT. See [LICENSE](LICENSE).
