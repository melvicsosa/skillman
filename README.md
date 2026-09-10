# skillman

Cross-agent AI skill manager with a local UI.

One Go binary that discovers every agent skill on your machine (Claude Code,
Claude plugins, Codex, Cursor, Gemini CLI, Antigravity, OpenCode), lets you
enable or disable them, keeps a vault of downloaded skills, and serves a local
UI on `http://localhost:3010`. See [PLAN.md](PLAN.md) for the design.

## Install

```sh
brew install melvicsosa/tap/skillman
skillman version
```

## Usage

```sh
skillman doctor          # data dir, database state, detected agents
skillman serve           # start the UI on :3010 and open the browser
skillman serve --port 4000 --no-open
skillman --data-dir /path/to/dir doctor
```

Data lives in `~/.skillman` (SQLite database, quarantine, vault).

## Development

Requirements: Go 1.26+, Node 22+, pnpm.

```sh
make web     # install and build the frontend into web/dist
make build   # build ./dist/skillman with version ldflags
make test    # go test ./...
make vet     # go vet ./...
make lint    # golangci-lint if installed, otherwise go vet
make run     # go run ./cmd/skillman serve
```

The Go build embeds `web/dist`. A placeholder `web/dist/index.html` is
committed so `go build` works before the frontend has been built. For
frontend development run `pnpm dev` inside `web/`; API calls are proxied to
`localhost:3010`.

## Release

Releases are built by GoReleaser on `v*` tags and published to GitHub
Releases and the `melvicsosa/homebrew-tap` Homebrew tap.

## License

MIT
