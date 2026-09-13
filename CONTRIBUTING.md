# Contributing to skillman

Thanks for taking the time to contribute. Bug reports, agent support requests and pull requests are all welcome.

## Build and test

Requirements: Go 1.26+, Node 22+, pnpm.

```sh
make web         # pnpm install and build the frontend into web/dist
make build       # ./dist/skillman (and ./dist/skillman-tray on macOS)
go test ./...    # or: make test
make lint        # golangci-lint if installed, otherwise go vet
make run         # go run ./cmd/skillman serve
```

The Go binary embeds `web/dist`; a placeholder `web/dist/index.html` is committed so `go build` works before the frontend is built. For frontend work run `pnpm dev` inside `web/`; API calls are proxied to `localhost:3010`.

CI runs `make web`, `go vet ./...`, `go test ./...` and `go build ./...` on every pull request. Run the same locally before opening one.

## Where the code lives

The Go side follows a hexagonal layout:

| Path | What goes there |
| --- | --- |
| `internal/domain` | Entities and ports (`Agent`, `Skill`, `Project`, registry and storage interfaces). No I/O. |
| `internal/app` | Use cases: scan, enable/disable, vault, sync, doctor, drift, export, settings. |
| `internal/adapters` | Implementations of the ports: `agents` (supported agents), `fs`, `storage` (SQLite), `http` (API and embedded UI), `registry` (skills.sh, GitHub, marketplaces), `convert`, `picker`, `service`. |
| `internal/cli` | Cobra commands; thin wrappers over `internal/app`. |
| `internal/tray` | macOS menu bar app. |
| `cmd/skillman`, `cmd/skillman-tray` | Entry points. |
| `web/` | React + Vite frontend, built into `web/dist`. |

Keep dependencies pointing inward: `cli`, `http` and `tray` call `app`; `app` depends on `domain` ports; `adapters` implement them.

## Adding support for a new agent

Supported agents are declared in one place: the `Registry` slice in
[`internal/adapters/agents/registry.go`](internal/adapters/agents/registry.go).
Each entry is a `Spec` with an `ID`, a display `Name`, the `GlobalDirs` it reads skills from (first entry is the primary location) and the `ProjectDirs` relative to a project root.

To add one:

1. Append a `Spec` to `Registry` with the agent's global and project skills directories.
2. Cover it in `internal/adapters/agents/registry_test.go` and `discover_test.go`.
3. Add the agent to the table in `docs/agents.md` and to "Which agents it works with" in `README.md`.

If you use an agent skillman does not know about and do not want to send a pull request, open an issue with the **Agent support request** template. The agent name, where it stores skills and a link to its docs are enough.

## Pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages: `feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `ci:`. Scopes are optional (`feat(doctor): ...`).
- One change per pull request. Keep unrelated refactors and formatting out of it.
- Add or update tests next to the code they cover, and update `docs/` when behaviour or commands change.
- Add a line under `[Unreleased]` in `CHANGELOG.md` for user-visible changes.

## Releases

Releases are cut by pushing a `v*` tag. GoReleaser builds the binaries, publishes them to GitHub Releases and updates the `melvicsosa/homebrew-tap` formula.
