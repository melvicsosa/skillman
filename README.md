<p align="center"><img src="assets/brand/banner.png" width="720" alt="skillman"></p>

# skillman

Agent Skill Management Tool

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

# Settings and background service (Phase 3)
skillman config get|set port|github_token|skillssh_token [<value>]
skillman service install           # launchd LaunchAgent, runs `serve` at login
skillman service status [--json]   # installed? running? pid, port, answering?
skillman service start|stop|restart|uninstall

skillman serve                     # start the UI (port setting, default 3010) and open the browser
skillman serve --port 4000 --no-open
skillman --data-dir /path/to/dir doctor
```

## UI

`skillman serve` opens a local web app:

- **Skills** per agent, global or per project, with enable/disable toggles,
  symlink/vault/drift/invalid flags and a Doctor panel.
- **Used N×** column and "most used" sort from Claude Code's own telemetry
  (`~/.claude.json`, `skillUsage`), also exposed as `GET /api/usage` and the
  `USED` column of `skillman list`.
- **Vault** to install, update and remove downloaded skills across agents.
- **Discover** to search skills.sh and GitHub, browse trending and curated
  lists, and add by reference.
- **Settings** for the port, write-only registry tokens and the background
  service (install, restart, uninstall). Changing the port while the service
  runs offers a one-click restart.

## Background service

`skillman service install` writes
`~/Library/LaunchAgents/com.melvicsosa.skillman.plist` pointing at the
current binary (`skillman --data-dir <dir> serve --no-open`, `RunAtLoad`,
restarted on failure) and loads it with `launchctl bootstrap`. Logs go to
`~/.skillman/logs/`. `service restart` (and the UI button) run
`launchctl kickstart -k`, so a port change takes effect immediately; the
server also writes `~/.skillman/serve.pid` and explains who holds the port
when it is already taken. Linux and Windows report the service as
unsupported; `serve` works everywhere.

Data lives in `~/.skillman` (SQLite database, quarantine, vault). Downloaded
skills are stored once in `~/.skillman/vault/<name>` with a
`<name>.vault.json` provenance sidecar and symlinked into agents (or copied
with `--copy`). Sources that are not spec skills (Cursor `.mdc` rules, Claude
and OpenCode commands, Claude plugins) are normalized on the way in.

Tokens are optional and can be stored with `skillman config set` or in the
Settings view: `GITHUB_TOKEN` (or the `github_token` setting) raises
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
