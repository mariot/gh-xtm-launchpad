# AGENTS Guide

## Project Snapshot
- GitHub CLI extension in Go (`module github.com/mariot/gh-xtm-launchpad`).
- Runtime entrypoint is `main.go`, which delegates to `cmd.Execute()`.
- CLI is Cobra-based (`cmd/root.go` + subcommands).

## Architecture and Data Flow
- Primary command is `sync` in `cmd/sync.go`; `rootCmd` has no default `Run`.
- `sync` flow:
  1. Build static target list: `OpenCTI-Platform/connectors`, `OpenAEV-Platform/collectors`.
  2. Resolve local paths under `repositories/<name>`.
  3. If local repo exists with `.git`, run `gh repo sync --source <owner/repo>` in that directory.
  4. If missing, run `gh repo clone <owner/repo> <absolute-path>`.
  5. Verify auth/session by calling `GET /user` via `api.DefaultRESTClient()`.
- Command execution is wrapped by `runGHCommand`, which uses `gh.Exec(...)` and mirrors stdout/stderr.

## Developer Workflows
- Run sync step locally:
  ```bash
  go run . sync
  ```
- Build extension binary:
  ```bash
  go build -o gh-xtm-launchpad .
  ```
- Run tests:
  ```bash
  go test ./...
  ```
- Release is tag-driven: pushing `v*` triggers `.github/workflows/release.yml`.

## Project Conventions (Observed)
- Keep command logic in `cmd/`; use small helpers (`syncRepository`, `runGHCommand`) instead of extra package layers.
- Error handling style is print and exit (`os.Exit(1)`) inside Cobra command handlers.
- Local external repos live only in `/repositories/` and are ignored by git (`.gitignore`).
- For small API responses, use inline structs (example: `response := struct{ Login string }{}`).
- Tests stub command execution through `runRepositoryCommand` (see `cmd/sync_test.go`) instead of invoking real `gh`/network.

## Integrations and Boundaries
- Runtime dependency: `github.com/cli/go-gh/v2` for both GitHub API clients and `gh` command execution.
- CLI framework dependency: `github.com/spf13/cobra`.
- Release automation: `cli/gh-extension-precompile@v2` with attestations enabled.

## Files to Read First
- `cmd/sync.go` - core sync behavior and `gh` command adapter.
- `cmd/sync_test.go` - expected sync semantics and test doubles.
- `cmd/root.go` - Cobra root command wiring.
- `main.go` - thin program entrypoint.
- `.github/workflows/release.yml` - release trigger and permissions.
