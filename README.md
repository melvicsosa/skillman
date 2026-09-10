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
skillman search <query> [--source skillssh|github|marketplace] [--json]
skillman add <ref> [--to <agent>]... [--all] [--copy] [--project <path>] [--command]
#   <ref>: owner/repo[/path][@ref], https://github.com/o/r/tree/<ref>/<path>,
#          skillssh:<owner>/<repo>/<skill>, marketplace:<owner>/<repo>/<plugin>,
#          a local dir/.mdc/.md/.zip/.tar.gz, or a .zip/.tar.gz/SKILL.md URL
skillman vault list [--json]
skillman vault install <name> --to <agent>... [--project <path>] [--copy] [--command]
skillman vault uninstall <name> --agent <id> [--project <path>]
skillman vault update [<name>]     # re-fetch from the recorded source
skillman vault remove <name> [--force]
skillman import-lock               # register ~/.agents/.skill-lock.json skills in the vault

# Sync, drift repair and export (Phase 4)
skillman sync [<name>...] [--all] [--project <path>] [--dry-run]
skillman vault sync-mode <name> on|off    # fan out on every scan
skillman vault adopt <name> --from <agent> [--link]
skillman doctor --fix-drift <name> --from <agent|vault> [--to <agent>]... [--project <path>]
skillman export plugin <out-dir> --name <plugin> [--version 0.1.0] [--description ...] <skill>...

# Settings and background service (Phase 3)
skillman config get|set port|github_token|skillssh_token|marketplaces [<value>]
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
- **Vault** to install, update, sync and remove downloaded skills across
  agents, with per-entry auto-sync, row selection and "Export as Claude
  plugin".
- **Discover** to search skills.sh, GitHub and Claude marketplaces, browse
  trending and curated lists, and add by reference.
- **Settings** for the port, write-only registry tokens, the marketplace
  list and the background service (install, restart, uninstall). Changing
  the port while the service runs offers a one-click restart.
- Deep links: `/`, `/agent/<id>`, `/vault`, `/discover` and `/settings`
  select the matching view (the menu bar app opens `/settings`).

## Sync

`skillman sync` makes sure vault entries are installed in every enabled,
writable agent (the read-only Claude plugin cache is skipped). Entries are
symlinked into the agent dirs, or copied when they were installed with
`--copy`. A skill directory of the same name that is not the vault entry is
reported as a **conflict** and never overwritten:

```sh
skillman sync --all --dry-run        # show linked / already / conflict per agent
skillman sync my-skill other-skill   # only these entries
skillman sync --all --project ~/code/app
skillman vault sync-mode my-skill on # every `skillman scan` fans this entry out
```

`POST /api/sync` and `POST /api/vault/{name}/sync` do the same from the UI
("Sync all" and the per-row "Sync" button); the auto-sync flag is
`PATCH /api/vault/{name} {"autoSync": true}` and is stored in the entry's
`.vault.json` sidecar, so it survives a database rebuild.

## Drift repair

`skillman doctor` reports **drift** when copies of the same skill differ.
Each drift group lists its copies with hashes and whether they can be
repaired (symlinks into the vault and read-only plugin copies cannot).
Pick the copy that wins and overwrite the others in place:

```sh
skillman doctor --fix-drift my-skill --from vault          # the vault entry wins
skillman doctor --fix-drift my-skill --from codex --to cursor
skillman vault adopt my-skill --from codex [--link]        # not in the vault yet? adopt it first
```

`vault adopt` copies an agent's skill into the vault as a `local` entry
(ref = original path); with `--link` the agent directory becomes a symlink
into the vault. The UI Doctor panel offers "Repair from …" and "Adopt into
vault" per drift group (`POST /api/doctor/drift/{name}/repair`,
`POST /api/vault/adopt`).

## Marketplaces

Claude plugin marketplaces are GitHub repositories with a
`.claude-plugin/marketplace.json` listing plugins (`name`, `description`
and a `source` that is either a path inside the repository or a
`{source, url|repo, path, ref, sha}` object). skillman resolves a plugin to
its GitHub location, downloads it and stores every `skills/<name>` in the
vault:

```sh
skillman search --source marketplace mcp
skillman add marketplace:anthropics/claude-plugins-official/plugin-dev --all
skillman config set marketplaces anthropics/claude-plugins-official,my-org/marketplace
```

Adding a marketplace repository itself (`skillman add owner/repo`) is
rejected with the list of plugins it contains.

## Claude plugin export

Turn vault skills into a Claude Code plugin directory:

```sh
skillman export plugin ./out --name my-plugin --version 0.1.0 --description "Team skills" skill-a skill-b
```

writes `out/my-plugin/.claude-plugin/plugin.json` (`name`, `version`,
`description`), byte-for-byte copies under `out/my-plugin/skills/<name>/`
and a `README.md` listing them. In the Vault view select rows and use
"Export as Claude plugin" (`POST /api/vault/export`).

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

## Menu bar app

`skillman-tray` is a macOS status item (ships in the macOS release archive
and with `brew install melvicsosa/tap/skillman`). Start it with
`skillman tray`; it shows the server state (`Running on :3010 · v0.3.0`,
`Stopped`, `Service not installed`) and offers **Open skillman**,
**Start / Stop / Restart** (through launchd when the service is installed,
otherwise a detached `skillman serve --no-open` tracked by
`~/.skillman/serve.pid`), **Install / Uninstall background service**, a
**Port** submenu (3010, 3011, 3012, 4010, 8010 or Custom… in Settings; a
change restarts the server) and a **Launch at login** checkbox that writes
`~/Library/LaunchAgents/com.melvicsosa.skillman-tray.plist` (`RunAtLoad`,
no `KeepAlive`). `skillman tray --login` / `--no-login` toggle that item from
the shell (`skillman-tray --install-login` / `--uninstall-login`). The tray
honours `--data-dir` and `SKILLMAN_DATA_DIR`, and finds `skillman` next to
its own binary or in `PATH`.

## Development

Requirements: Go 1.26+, Node 22+, pnpm.

```sh
make web     # install and build the frontend into web/dist
make build   # build ./dist/skillman (and ./dist/skillman-tray on macOS)
make build-tray # build the macOS menu bar app only (needs CGO)
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

Releases are built by GoReleaser on `v*` tags (on a macOS runner, because
`skillman-tray` needs CGO; the CGO-free `skillman` Linux binaries are
cross-compiled from there) and published to GitHub Releases and the
`melvicsosa/homebrew-tap` Homebrew tap. The darwin archives contain both
binaries; the linux archives only `skillman`.

## License

MIT
