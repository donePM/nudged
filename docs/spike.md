# Spike report (Phase 0)

This report answers the 9 open technical questions of the PRD (table "Offene Technikfragen").
Each answer has evidence and a decision for the later slices.
The tests ran on 2026-10-31 against Claude Code 2.1.316 on macOS (arm64).

## Method

- A temp settings file adds one command hook for each event and a statusline command.
  We pass it with `claude --settings <temp file> --setting-sources project`.
  We did not change `~/.claude/settings.json`.
- The hook script copies stdin to a file and writes a log line with the time and the PID.
  A control file can make it sleep, and a second control file can give the reply on stdout.
  It writes a log line when it gets SIGTERM, SIGINT or SIGHUP.
- Headless runs use `claude -p`. Interactive runs use `expect` with a pseudo terminal (120 x 40).
- The model is Haiku. All prompts are test prompts in an empty test folder.
- The fixtures in `internal/proto/testdata/hooks/` are the captured samples.
  We replaced the home path, the folder, the session id, the prompt id and the tool use id with neutral values.

Short reference for the times below: the log line format is `HH:MM:SS start|end|SIGNAL <event> pid=<pid>`.

## Q1 Can a hook answer `AskUserQuestion`?

Answer: yes. Two hook events can answer it: `PermissionRequest` (best) and `PreToolUse`.
Evidence: Docs https://code.claude.com/docs/en/hooks#tools-that-require-user-interaction say: return `permissionDecision: "allow"` with `updatedInput` that holds the original `questions` and an `answers` object. Interactive run 1: a `PreToolUse` hook returned `answers: {"Which color do you like?": "Blue"}`, and the model replied "Blue" (`PostToolUse.tool_response.answers` = `{"Which color do you like?":"Blue"}`). Interactive run 4: a `PermissionRequest` hook returned `decision.behavior: "allow"` with the same `updatedInput`. The terminal showed "User answered Claude's questions: · Which color do you like? → Blue ⎿ Allowed by PermissionRequest hook", and the model replied "Blue". Headless runs do not offer the tool: `claude -p` gave `ToolSearch` "select:AskUserQuestion" with `matches: []`.
Decision: S7 holds a question on `PermissionRequest` with `tool_name: "AskUserQuestion"`, not on `PreToolUse`. Reason: a blocked `PreToolUse` hook hides the question in the terminal (see Q2), and a blocked `PermissionRequest` hook does not. The `nudged hook PreToolUse` call must return at once with no output. The hook client sets `Request.Kind` to `question` for a `PermissionRequest` on `AskUserQuestion`, so `internal/proto` needs no type change. S16 sends the reply in the format of `question/PermissionRequest.reply.json`. Multi-select answers join the labels with ", " (docs: "Multi-select answers join labels with commas"). S16 must test free text: we did not test a value that is not an option label. The PRD fallback (show only, click jumps to the terminal) is not necessary.

## Q2 Does Claude Code show the permission dialog in the terminal while the hook blocks?

Answer: yes for `PermissionRequest`, no for `PreToolUse`.
Evidence: Interactive run 2 (`--permission-mode default`, `PermissionRequest` hook sleeps 40 s, timeout 1800): `18:17:21 start PermissionRequest pid=37617`. The terminal showed "Bash command … touch d.txt … Do you want to proceed? ❯ 1. Yes 2. Yes, and always allow access to … 3. Yes, and switch to auto mode 4. No" at 18:17:21. We pressed Enter at 18:17:24. The command ran: `18:17:25 start PostToolUse`, and `d.txt` exists. Interactive run 4: the question "☐ Color Which color do you like? ❯ 1. Red 2. Blue" was on screen while the `PermissionRequest` hook slept (18:20:14 to 18:20:34). Interactive run 3 (`PreToolUse` hook sleeps 25 s): the screen showed "running PreToolUse hook" and no question from 18:18:13 to 18:18:38. The question came only after the hook ended.
Decision: The terminal stays usable while a panel waits. S7 holds requests only in `PermissionRequest`. No hook of nudged may block in `PreToolUse`. The PRD fallback (terminal blocked, the timeout decides) is not necessary.

## Q3 How does nudged see an answer in the terminal, so that it can close the panel?

Answer: partly. Claude Code sends no signal and no event for "answered in the terminal".
Evidence: Interactive run 2: the user answered in the terminal at 18:17:24. The hook process was not stopped: it got SIGTERM only at the session exit (`18:17:40 SIGNAL PermissionRequest pid=37617`), 16 s later. Between these times, events of the same session came: `18:17:25 PostToolUse` (same `tool_name` and `tool_input`), `18:17:26 Stop`. The `PermissionRequest` input has no `tool_use_id` (docs https://code.claude.com/docs/en/hooks#permissionrequest-input), so nudged cannot match the request to the `PostToolUse` by id. A statusline call also came at 18:17:24, but statusline calls also come while a request waits (run 3: 18:18:13), so they are not a signal.
Decision: Use the PRD fallback. S7 ends a pending request with no output on the next hook event of the same `session_id` (and the same `agent_id`, because subagents run tools in parallel). S7 also ends it when the hook client disconnects: Claude Code sends SIGTERM to the hook at session end, and the socket closes. The hook output after a terminal answer has no effect, but S7 must not send it.

## Q4 Does Claude Code accept a hook timeout of 1800 s?

Answer: yes. Claude Code accepts 1800 and waits longer than the default of 600 s.
Evidence: Headless run (`claude -p --permission-mode default`, `PermissionRequest` hook with `"timeout": 1800` sleeps 620 s, then replies allow): `18:17:12 start PermissionRequest pid=37324`, `18:27:32 end PermissionRequest pid=37324` (620 s, no SIGNAL line), `18:27:33 start PostToolUse`, and the model replied "DONE". `q4.txt` exists. Claude Code showed no error for `timeout: 1800` on any event. Docs https://code.claude.com/docs/en/hooks#common-fields name a default of 600 s for `command` hooks and no maximum. We did not wait the full 1800 s.
Decision: S8 installs `"timeout": 1800` on the `PermissionRequest` hook, and the config default stays 30 min. The daemon ends the request itself at the configured timeout (S7), before Claude Code does. Other events return at once, so S8 gives them a short timeout (`SessionEnd` has a budget of at most 60 s). The PRD fallback (a shorter maximum) is not necessary.

## Q5 Does the statusline JSON give effort and context fill?

Answer: yes.
Evidence: The captured `statusline.json` (Claude Code 2.1.316) has `"effort": {"level": "medium"}`, `"context_window": {"used_percentage": 19, "remaining_percentage": 81, "context_window_size": 200000, …}`, `"model": {"id": "claude-haiku-5-5", "display_name": "Haiku 5.5"}`, `"session_id"` and `"version": "2.1.316"`. Docs https://code.claude.com/docs/en/statusline#available-data: `effort` is absent when the model does not support effort, and `used_percentage` can be `null` early in the session.
Decision: S5 sends `session_id`, `model.display_name`, `effort.level` and `context_window.used_percentage`. The `session_id` is the same as in the hook input, so S6 joins the two. A missing or `null` field hides that field in the UI (the PRD fallback for one field). Hooks give `effort.level` too (`PreToolUse`, `PostToolUse`, `Stop`), but the statusline is the main source.

## Q6 Is Wails v3 stable enough for a tray and frameless panels?

Answer: partly. The release and the API are ready. We did not run a GUI in the spike.
Evidence: `gh api repos/wailsapp/wails/releases` gives `v3.0.0 2026-10-26T11:15:13Z prerelease=false` (after `v3.0.0-beta.34`). `go list -m -versions github.com/wailsapp/wails/v3` lists `v3.0.0`. The v3.0.0 module has `SystemTray.SetTemplateIcon`, `SystemTray.AttachWindow`, `SystemTray.SetLabel`, `WebviewWindowOptions.Frameless`, `WebviewWindowOptions.AlwaysOnTop` and `ActivationPolicyAccessory` (no Dock icon) in `pkg/application`, and the examples `systray-basic`, `systray-menu`, `frameless` and `global-shortcuts`. `CGO_ENABLED=1 go build ./examples/systray-basic` with go1.27.3 darwin/arm64: "Go build: Success" in 12.7 s, binary 15.8 MB.
Decision: S12 uses Wails v3.0.0 and is the real runtime check (tray, no Dock icon, frameless panel always on top). The release is new (5 days), so S12 pins `v3.0.0` exactly. If S12 fails, the fallback is Tauri v2 with the Go hook client (PRD).

## Q7 Can nudged jump to a Ghostty window by TTY?

Answer: yes, with Ghostty 1.4.0.
Evidence: `/Applications/Ghostty.app/Contents/Info.plist` has `CFBundleShortVersionString` 1.4.0 and `NSAppleScriptEnabled` 1. `Contents/Resources/Ghostty.sdef` has the class `terminal` with the property `tty` ("TTY device path for this terminal (e.g. /dev/ttys016)") and the command `focus` ("Focus a terminal, bringing its window to the front."). `osascript -e 'tell application "Ghostty" to get {id, tty} of every terminal'` printed two ids and `/dev/ttys000, /dev/ttys001`. We did not run `focus`, because it moves the window of the user.
Decision: S10 runs `tell application "Ghostty" to focus (first terminal whose tty is "/dev/ttysNNN")` and then `activate`. If the script fails (older Ghostty with no dictionary), S10 only activates the app (PRD fallback). `ps -o tty=` gives `ttys003` with no `/dev/` prefix, so S4 or S10 adds the prefix. The first call asks the user for the macOS Automation permission. In `claude -p` runs the hook has no TTY (`??`), so S10 has no target there.

## Q8 Does a cask `postflight` remove the quarantine flag in our own tap?

Answer: partly. The docs say yes for a third-party tap. We did not test an install, because the tap does not exist yet.
Evidence: Homebrew Cask Cookbook (https://github.com/Homebrew/brew/blob/main/docs/Cask-Cookbook.md, Homebrew 7.0.11 installed): "Casks in official Homebrew taps must use these structured stanzas; legacy Ruby flight blocks are rejected. The legacy forms remain available temporarily for third-party tap compatibility." The structured `postflight_steps` block has a `run` step ("run one executable with literal arguments"). The steps run in a sandbox that permits writes to the app directory. GoReleaser docs (https://goreleaser.com/customization/publish/homebrew_casks/, "Signing and Notarizing") show `hooks.post.install_steps` with `run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", …]` for unsigned apps.
Decision: S20 uses the GoReleaser `homebrew_casks` hook with a `run "/usr/bin/xattr"` step on the installed `nudged.app`, not a legacy Ruby `postflight` block. The README also shows `xattr -d com.apple.quarantine` (PRD fallback), because the sandbox can block the step. S20 tests a real install after the human creates `donePM/homebrew-tap`.

## Q9 Is the name `nudged` free on GitHub and Homebrew?

Answer: yes.
Evidence: `brew info nudged`: "No available formula with the name "nudged"". `brew info --cask nudged`: "Cask 'nudged' is unavailable: No Cask with this name exists." `brew search nudged`: only `nuget`, `nuage`, `nudge` (the cask `nudge` is "Nudge", "Application for enforcing OS updates", macadmins). `gh search repos nudged`: `axelpale/nudged` and `axelpale/nudged-py` (a 2D point transform library), no macOS app and no Claude Code tool. `gh api repos/rokde/nudged`: the repo exists and is ours. `gh api repos/donePM/homebrew-tap`: 404.
Decision: Keep the name `nudged` and the cask token `nudged` in `donePM/homebrew-tap`. The README names the difference to the admin tool "Nudge". The PRD fallback (other name) is not necessary.

## Other findings

- `Notification` with `notification_type: "permission_prompt"` comes about 6 s after a permission dialog (run 3: `PermissionRequest` 18:18:38, `Notification` 18:18:44). `PermissionRequest` covers this case, so S8 installs the `Notification` hook with the matcher `idle_prompt` only.
- `idle_prompt` came 60 s after the last answer (run 1: `Stop` 18:14:52, `Notification` 18:15:52).
- `PermissionRequest` also comes for `AskUserQuestion` in auto mode (run 3 and run 4, `permission_mode: "auto"`).
- `Stop` can come while a subagent still runs. Then `background_tasks` is not empty (headless run with a background subagent: `"background_tasks":[{"type":"subagent","status":"running",…}]`). S17 shows "Session done" only when `background_tasks` is empty.
- `SessionEnd` hooks have a short time budget (1.5 s default, 60 s max, docs https://code.claude.com/docs/en/hooks#sessionend). The hook client must return at once.
- A hook with no output and exit code 0 changes nothing. This is the nudged fallback when the daemon is not running.
- The `PermissionRequest` deny reply works: headless run with `{"decision":{"behavior":"deny","message":"Denied in nudged panel"}}`, and the model quoted "Denied in nudged panel".

## Hook JSON

The stdin samples are in `internal/proto/testdata/hooks/<Event>.json`.
The tested replies are in `internal/proto/testdata/hooks/<Event>.reply.json`.
The question flow (Q1) is in `internal/proto/testdata/hooks/question/`.
"No output" means: exit code 0 and an empty stdout.

### PermissionRequest

stdin (`PermissionRequest.json`):

```json
{
  "session_id": "00000000-0000-4000-8000-000000000001",
  "transcript_path": "/Users/dev/.claude/projects/-Users-dev-project/00000000-0000-4000-8000-000000000001.jsonl",
  "cwd": "/Users/dev/project",
  "scratchpad_dir": "/private/tmp/claude-501/-Users-dev-project/00000000-0000-4000-8000-000000000001/scratchpad",
  "prompt_id": "00000000-0000-4000-8000-000000000002",
  "permission_mode": "default",
  "effort": {
    "level": "medium"
  },
  "hook_event_name": "PermissionRequest",
  "tool_name": "Bash",
  "tool_input": {
    "command": "touch d.txt",
    "description": "Create empty file d.txt"
  },
  "permission_suggestions": [
    {
      "type": "addDirectories",
      "directories": [
        "/Users/dev/project"
      ],
      "destination": "session"
    },
    {
      "type": "setMode",
      "mode": "acceptEdits",
      "destination": "session"
    }
  ]
}
```

stdout: Allow (tested, `PermissionRequest.reply.json`):

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PermissionRequest",
    "decision": {
      "behavior": "allow"
    }
  }
}
```

stdout: Deny (tested in a headless run):

```json
{"hookSpecificOutput": {"hookEventName": "PermissionRequest", "decision": {"behavior": "deny", "message": "Denied in nudged panel"}}}
```

stdout: "Always allow …" (from the docs, not tested): `behavior: "allow"` and `updatedPermissions` with one entry of `permission_suggestions`.

```json
{"hookSpecificOutput": {"hookEventName": "PermissionRequest", "decision": {"behavior": "allow", "updatedPermissions": [{"type": "setMode", "mode": "acceptEdits", "destination": "session"}]}}}
```

stdout on timeout, pause or terminal answer: no output. The terminal dialog stays.

### PermissionRequest for `AskUserQuestion` (question flow)

stdin (`question/PermissionRequest.json`):

```json
{
  "session_id": "00000000-0000-4000-8000-000000000001",
  "transcript_path": "/Users/dev/.claude/projects/-Users-dev-project/00000000-0000-4000-8000-000000000001.jsonl",
  "cwd": "/Users/dev/project",
  "scratchpad_dir": "/private/tmp/claude-501/-Users-dev-project/00000000-0000-4000-8000-000000000001/scratchpad",
  "prompt_id": "00000000-0000-4000-8000-000000000002",
  "permission_mode": "auto",
  "effort": {
    "level": "medium"
  },
  "hook_event_name": "PermissionRequest",
  "tool_name": "AskUserQuestion",
  "tool_input": {
    "questions": [
      {
        "question": "Which color do you like?",
        "header": "Color",
        "options": [
          {
            "label": "Red",
            "description": "Choose red"
          },
          {
            "label": "Blue",
            "description": "Choose blue"
          }
        ],
        "multiSelect": false
      }
    ]
  }
}
```

stdout (tested, `question/PermissionRequest.reply.json`):

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PermissionRequest",
    "decision": {
      "behavior": "allow",
      "updatedInput": {
        "questions": [
          {
            "question": "Which color do you like?",
            "header": "Color",
            "options": [
              {
                "label": "Red",
                "description": "Choose red"
              },
              {
                "label": "Blue",
                "description": "Choose blue"
              }
            ],
            "multiSelect": false
          }
        ],
        "answers": {
          "Which color do you like?": "Blue"
        }
      }
    }
  }
}
```

### PreToolUse (tool `AskUserQuestion`)

stdin (`PreToolUse.json`):

```json
{
  "session_id": "00000000-0000-4000-8000-000000000001",
  "transcript_path": "/Users/dev/.claude/projects/-Users-dev-project/00000000-0000-4000-8000-000000000001.jsonl",
  "cwd": "/Users/dev/project",
  "scratchpad_dir": "/private/tmp/claude-501/-Users-dev-project/00000000-0000-4000-8000-000000000001/scratchpad",
  "prompt_id": "00000000-0000-4000-8000-000000000002",
  "permission_mode": "auto",
  "effort": {
    "level": "medium"
  },
  "hook_event_name": "PreToolUse",
  "tool_name": "AskUserQuestion",
  "tool_input": {
    "questions": [
      {
        "question": "Which color do you like?",
        "header": "Color",
        "options": [
          {
            "label": "Red",
            "description": "Choose red"
          },
          {
            "label": "Blue",
            "description": "Choose blue"
          }
        ],
        "multiSelect": false
      }
    ]
  },
  "tool_use_id": "toolu_01AAAAAAAAAAAAAAAAAAAAAAAA"
}
```

stdout (tested in run 1, `PreToolUse.reply.json`). nudged does not use it, because it blocks the terminal (Q2). nudged sends no output on `PreToolUse`.

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "updatedInput": {
      "questions": [
        {
          "question": "Which color do you like?",
          "header": "Color",
          "options": [
            {
              "label": "Red",
              "description": "Choose red"
            },
            {
              "label": "Blue",
              "description": "Choose blue"
            }
          ],
          "multiSelect": false
        }
      ],
      "answers": {
        "Which color do you like?": "Blue"
      }
    }
  }
}
```

### PostToolUse

stdin (`PostToolUse.json`):

```json
{
  "session_id": "00000000-0000-4000-8000-000000000001",
  "transcript_path": "/Users/dev/.claude/projects/-Users-dev-project/00000000-0000-4000-8000-000000000001.jsonl",
  "cwd": "/Users/dev/project",
  "scratchpad_dir": "/private/tmp/claude-501/-Users-dev-project/00000000-0000-4000-8000-000000000001/scratchpad",
  "prompt_id": "00000000-0000-4000-8000-000000000002",
  "permission_mode": "default",
  "effort": {
    "level": "medium"
  },
  "hook_event_name": "PostToolUse",
  "tool_name": "Bash",
  "tool_input": {
    "command": "touch d.txt",
    "description": "Create empty file d.txt"
  },
  "tool_response": {
    "stdout": "",
    "stderr": "",
    "interrupted": false,
    "isImage": false,
    "noOutputExpected": true
  },
  "tool_use_id": "toolu_01AAAAAAAAAAAAAAAAAAAAAAAA",
  "duration_ms": 982
}
```

stdout: no output.

### Stop

stdin (`Stop.json`):

```json
{
  "session_id": "00000000-0000-4000-8000-000000000001",
  "transcript_path": "/Users/dev/.claude/projects/-Users-dev-project/00000000-0000-4000-8000-000000000001.jsonl",
  "cwd": "/Users/dev/project",
  "scratchpad_dir": "/private/tmp/claude-501/-Users-dev-project/00000000-0000-4000-8000-000000000001/scratchpad",
  "prompt_id": "00000000-0000-4000-8000-000000000002",
  "permission_mode": "default",
  "effort": {
    "level": "medium"
  },
  "hook_event_name": "Stop",
  "stop_hook_active": false,
  "last_assistant_message": "DONE",
  "background_tasks": [],
  "session_crons": []
}
```

stdout: no output.

### Notification (idle)

stdin (`Notification.json`):

```json
{
  "session_id": "00000000-0000-4000-8000-000000000001",
  "transcript_path": "/Users/dev/.claude/projects/-Users-dev-project/00000000-0000-4000-8000-000000000001.jsonl",
  "cwd": "/Users/dev/project",
  "scratchpad_dir": "/private/tmp/claude-501/-Users-dev-project/00000000-0000-4000-8000-000000000001/scratchpad",
  "prompt_id": "00000000-0000-4000-8000-000000000002",
  "hook_event_name": "Notification",
  "message": "Claude is waiting for your input",
  "notification_type": "idle_prompt"
}
```

stdout: no output.

### SessionStart

stdin (`SessionStart.json`):

```json
{
  "session_id": "00000000-0000-4000-8000-000000000001",
  "transcript_path": "/Users/dev/.claude/projects/-Users-dev-project/00000000-0000-4000-8000-000000000001.jsonl",
  "cwd": "/Users/dev/project",
  "scratchpad_dir": "/private/tmp/claude-501/-Users-dev-project/00000000-0000-4000-8000-000000000001/scratchpad",
  "hook_event_name": "SessionStart",
  "source": "startup",
  "model": "claude-haiku-5-5"
}
```

stdout: no output.

### SessionEnd

stdin (`SessionEnd.json`):

```json
{
  "session_id": "00000000-0000-4000-8000-000000000001",
  "transcript_path": "/Users/dev/.claude/projects/-Users-dev-project/00000000-0000-4000-8000-000000000001.jsonl",
  "cwd": "/Users/dev/project",
  "scratchpad_dir": "/private/tmp/claude-501/-Users-dev-project/00000000-0000-4000-8000-000000000001/scratchpad",
  "prompt_id": "00000000-0000-4000-8000-000000000002",
  "hook_event_name": "SessionEnd",
  "reason": "prompt_input_exit"
}
```

stdout: no output.

### UserPromptSubmit

stdin (`UserPromptSubmit.json`):

```json
{
  "session_id": "00000000-0000-4000-8000-000000000001",
  "transcript_path": "/Users/dev/.claude/projects/-Users-dev-project/00000000-0000-4000-8000-000000000001.jsonl",
  "cwd": "/Users/dev/project",
  "scratchpad_dir": "/private/tmp/claude-501/-Users-dev-project/00000000-0000-4000-8000-000000000001/scratchpad",
  "prompt_id": "00000000-0000-4000-8000-000000000002",
  "permission_mode": "default",
  "hook_event_name": "UserPromptSubmit",
  "prompt": "Use the Bash tool to run exactly: touch d.txt . Do not use subagents. Then reply DONE."
}
```

stdout: no output.

### Statusline

stdin (`statusline.json`):

```json
{
  "session_id": "00000000-0000-4000-8000-000000000001",
  "transcript_path": "/Users/dev/.claude/projects/-Users-dev-project/00000000-0000-4000-8000-000000000001.jsonl",
  "cwd": "/Users/dev/project",
  "scratchpad_dir": "/private/tmp/claude-501/-Users-dev-project/00000000-0000-4000-8000-000000000001/scratchpad",
  "prompt_id": "00000000-0000-4000-8000-000000000002",
  "effort": {
    "level": "medium"
  },
  "session_name": "Color preference question",
  "model": {
    "id": "claude-haiku-5-5",
    "display_name": "Haiku 5.5"
  },
  "workspace": {
    "current_dir": "/Users/dev/project",
    "project_dir": "/Users/dev/project",
    "added_dirs": []
  },
  "version": "2.1.316",
  "output_style": {
    "name": "default"
  },
  "cost": {
    "total_cost_usd": 0.00291897,
    "total_duration_ms": 13461,
    "total_api_duration_ms": 4656,
    "total_lines_added": 0,
    "total_lines_removed": 0
  },
  "context_window": {
    "total_input_tokens": 37364,
    "total_output_tokens": 5,
    "context_window_size": 200000,
    "current_usage": {
      "input_tokens": 2,
      "output_tokens": 5,
      "cache_creation_input_tokens": 232,
      "cache_read_input_tokens": 37130
    },
    "used_percentage": 19,
    "remaining_percentage": 81
  },
  "exceeds_200k_tokens": false,
  "prompt_cache": {
    "warm": true,
    "caching_observed": true,
    "ttl": "1h",
    "expires_at": 1793470491,
    "requests": 2,
    "misses": 0,
    "expected_rebuilds": 0,
    "hit_ratio": 0.8614824957044673,
    "cache_write_tokens": 10315,
    "miss_recache_tokens": 0,
    "last_miss_at": null,
    "last_miss_cause": null,
    "miss_causes": {},
    "recache_tokens_if_cold": 37364
  },
  "fast_mode": false,
  "thinking": {
    "enabled": true
  },
  "rate_limits": {
    "five_hour": {
      "used_percentage": 9,
      "resets_at": 1793481000
    },
    "seven_day": {
      "used_percentage": 2,
      "resets_at": 1793991600
    }
  }
}
```

stdout: the text of the status line. `nudged statusline` prints the output of the wrapped command.
