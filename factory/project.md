# Project context

Every factory agent reads this file first. Only the orchestrator writes it: it
fills it in on `/factory:init`, re-checks it at the start of each job, and
refreshes it after each merge.

## Repo
- Base: `origin/main`, or the local target branch when it already contains the base
  (local merges not pushed yet). Every job's worktree starts there.
- Merge target: `main` (local, not pushed)
- Language / stack: Go 1.27 (module `github.com/rokde/nudged`, stdlib so far); later Wails v3 + Vue 3/TS/Vite frontend (S12)
- Install: `go mod download`. Run it once in each new worktree.
- Test command: `go vet ./... && make test`
- Lint / format: `gofmt -l . && go vet ./...`

## Sources
- GitHub issues: rokde/nudged
- Jira: none

## Conventions
- Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`).
- One binary; each subcommand lives in `cmd_<name>.go` and calls `register` in its own `init()`. Shared wire types in `internal/proto`; libraries in `internal/<pkg>`.
- CI: `.github/workflows/ci.yml` on `macos-latest` runs `go vet` and `go test`.
- Product requirements live in the
"nudged – PRD" doc: https://claude.ai/artifact/4FSUSazTkkEPLHN2uLc4HZ

## Factory rules
- Product code never goes into `factory/` or `.claude/`.
- One job = one branch `factory/<job-id>` in its own worktree
  `.claude/worktrees/factory-<job-id>`, created from the base ref.
- Smallest complete change. A known limit gets a `shortcut: <limit>, <when to upgrade>` comment.
