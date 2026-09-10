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
skillman scan                      # rebuild the skill cache from disk
skillman list [--agent codex] [--project <path>] [--json]
skillman enable|disable <skill> --agent <id> [--project <path>]
skillman agents | agent enable|disable <id>
skillman project add|list|remove <path>
skillman doctor [--json]           # spec issues, drift, broken vault links, rejected conversions

# Registry and vault (Phase 2)
skillman search <query> [--source skillssh|github] [--json]
skillman add <ref> [--to <agent>]... [--all] [--copy] [--project <path>] [--command]
#   <ref>: owner/repo[/path][@ref], https://github.com/o/r/tree/<ref>/<path>,
#          skillssh:<owner>/<repo>/<skill>, a local dir/.mdc/.md/.zip/.tar.gz,
#          or a .zip/.tar.gz/SKILL.md URL
skillman vault list [--json]
skillman vault install <name> --to <agent>... [--project <path>] [--copy] [--command]
skillman vault uninstall <name> --agent <id> [--project <path>]
skillman vault update [<name>]     # re-fetch from the recorded source
skillman vault remove <name> [--force]
skillman import-lock               # register ~/.agents/.skill-lock.json skills in the vault

skillman serve                     # start the UI on :3010 and open the browser
skillman serve --port 4000 --no-open
skillman --data-dir /path/to/dir doctor
```

Data lives in `~/.skillman` (SQLite database, quarantine, vault). Downloaded
skills are stored once in `~/.skillman/vault/<name>` with a
`<name>.vault.json` provenance sidecar and symlinked into agents (or copied
with `--copy`). Sources that are not spec skills (Cursor `.mdc` rules, Claude
and OpenCode commands, Claude plugins) are normalized on the way in.

Tokens are optional: `GITHUB_TOKEN` (or the `github_token` setting) raises
GitHub rate limits; `SKILLSSH_TOKEN` (or `skillssh_token`, a Vercel OIDC
token) unlocks the skills.sh v1 API (trending, curated, file snapshots).
Without it, search uses the public legacy endpoint and skill files are
fetched from the source GitHub repository.

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
