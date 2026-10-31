# Project context

Every factory agent reads this file first. Only the orchestrator writes it: it
fills it in on `/factory:init`, re-checks it at the start of each job, and
refreshes it after each merge.

## Repo
- Base: `origin/main`, or the local target branch when it already contains the base
  (local merges not pushed yet). Every job's worktree starts there.
- Merge target: `main` (pushed to GitHub after each merge; CI runs there)
- Language / stack: Go 1.27 (module `github.com/rokde/nudged`; deps: BurntSushi/toml v1.6.0); wailsapp/wails/v3 v3.0.0 (tray app in `internal/app`, `nudged` with no args); Vue 3/TS/Vite frontend in `frontend/` (Go package `frontend` embeds `dist/`; Vite output in git-ignored `frontend/dist/app`, a committed placeholder `frontend/dist/index.html` keeps Go tests working without npm). Hash router over `frontend/src/views/*.vue`; Go↔JS only via Wails events (`@wailsio/runtime`). TypeScript pinned ^6 (vue-tsc not ready for 7)
- Install: `go mod download`; for frontend work also `npm --prefix frontend ci`. Run once in each new worktree.
- Frontend check: `npm --prefix frontend run build`
- Test command: `go vet ./... && make test`
- Lint / format: `gofmt -l . && go vet ./...`

## Sources
- GitHub issues: rokde/nudged
- Jira: none

## Conventions
- Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`).
- One binary; each subcommand lives in `cmd_<name>.go` and calls `register` in its own `init()`. Shared wire types in `internal/proto`; libraries in `internal/<pkg>` (now: app, config, daemon, focus, hookclient, installer, sound, statusline, usage). `main.go` has a `usage()` func, so import `internal/usage` as `usagelog`.
- Unix socket paths on macOS max 104 bytes: tests use `os.MkdirTemp("", "nd")`, not `t.TempDir()`.
- CI: `.github/workflows/ci.yml` on `macos-latest` runs `go vet`, `go test` and the frontend build (Node 26).
- Tray app: `internal/app/app.go` (menu, pause path `applyToggle` shared by menu and ⌥⌘P hotkey in `hotkey_darwin.go`, asset handler `assets()`).
- Product requirements live in the
"nudged – PRD" doc: https://claude.ai/artifact/4FSUSazTkkEPLHN2uLc4HZ

## Factory rules
- Product code never goes into `factory/` or `.claude/`.
- One job = one branch `factory/<job-id>` in its own worktree
  `.claude/worktrees/factory-<job-id>`, created from the base ref.
- Smallest complete change. A known limit gets a `shortcut: <limit>, <when to upgrade>` comment.
