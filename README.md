# gh-xtm-launchpad

A GitHub CLI extension (written in Go with Cobra) that helps you:

1. Sync two upstream repos locally under `repositories/`
2. Build connector/collector Docker images from those repos
3. Run built images with an env file generated from `docker-compose.yml`

Managed upstream repositories:

- `OpenCTI-Platform/connectors` -> `repositories/connectors`
- `OpenAEV-Platform/collectors` -> `repositories/collectors`

## Prerequisites

- GitHub CLI (`gh`) authenticated (`gh auth status`)
- Docker
- Git (used by `build --branch` / `build --pr`)
- Go (only needed for local development with `go run` / `go build`)

## Install As a GitHub CLI Extension

### Install from GitHub (no local source checkout)

Use the repository directly:

```bash
gh extension install mariot/gh-xtm-launchpad
```

Upgrade later:

```bash
gh extension upgrade xtm-launchpad
```

Remove:

```bash
gh extension remove xtm-launchpad
```

### Install from a local checkout (for development)

From a local checkout:

```bash
gh extension install .
```

If already installed from this local path, upgrade after pulling changes:

```bash
gh extension upgrade xtm-launchpad
```

To remove:

```bash
gh extension remove xtm-launchpad
```

## Quick Start

```bash
# 1) Sync repositories into ./repositories
gh xtm-launchpad sync

# 2) Build one target
gh xtm-launchpad build collectors/crowdstrike

# 3) Run it (prompts for missing env values and writes .env)
gh xtm-launchpad run collectors/crowdstrike
```

## Command Reference

### `sync`

Clone missing repos, or fetch updates for existing local clones.

```bash
gh xtm-launchpad sync
```

Behavior:

- If `repositories/<name>/.git` exists: runs `gh repo sync --source <owner/repo>`
- If missing: runs `gh repo clone <owner/repo> <absolute-path>`
- Verifies GitHub session via `GET /user`

### `build <target>`

Build a Docker image from a target path under `repositories/`.

```bash
gh xtm-launchpad build collectors/crowdstrike
gh xtm-launchpad build connectors/external-import/crowdstrike
```

Expected target format:

- `collectors/<path>`
- `connectors/<path>`

Image tags:

- Collector: `gh-xtm-launchpad/collector-<name>:latest`
- Connector: `gh-xtm-launchpad/connector-<name>:latest`

Branch and PR options:

```bash
# Build from origin branch
gh xtm-launchpad build collectors/crowdstrike --branch master

# Build from pull request head
gh xtm-launchpad build collectors/crowdstrike --pr 123
# same as:
gh xtm-launchpad build collectors/crowdstrike --pull-request 123
```

Notes:

- `--branch` and `--pr` are mutually exclusive
- For branch/PR builds, the command fetches and checks out in detached HEAD at repo root (`repositories/collectors` or `repositories/connectors`)

### `run <target>`

Run a previously built image and supply env values using an env file.

```bash
gh xtm-launchpad run collectors/crowdstrike
```

How it works:

- Reads env keys/defaults from `<target>/docker-compose.yml`
- Loads values from env file (default: `<target>/.env`)
- If env file does not exist, creates it
- Prompts only for missing keys
- If you press Enter on a prompted variable with a default, the default is used
- Writes resolved values back to env file
- Launches container with `docker run --rm --env-file <env-file> <image-tag>`

Custom env file path:

```bash
gh xtm-launchpad run collectors/crowdstrike --env-file .env
# or
gh xtm-launchpad run collectors/crowdstrike --env-file collector.local.env
```

## Local Development (without installing extension)

Use these commands while iterating on source code:

```bash
go run . sync
go run . build collectors/crowdstrike
go run . run collectors/crowdstrike
```

## Development

Run tests:

```bash
go test ./...
```

Run linter locally:

```bash
./scripts/lint.sh
```

If needed, install the linter first (macOS):

```bash
brew install golangci-lint
```

Build binary:

```bash
go build -o gh-xtm-launchpad .
```

## CI

On every pull request to `main` and every push to `main`, GitHub Actions runs:

- `golangci-lint`
- `go test ./...`

## Project Layout

- `main.go` - entrypoint (`cmd.Execute()`)
- `cmd/root.go` - root Cobra command
- `cmd/sync.go` - repository sync logic via `gh`
- `cmd/build.go` - target resolution and Docker build logic
- `cmd/run.go` - compose env extraction, prompting, env-file run flow
- `cmd/*_test.go` - behavior-focused unit tests with stubbed command runners
- `repositories/` - local clones (gitignored)


