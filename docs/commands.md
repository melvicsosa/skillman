# Commands

Complete reference for the `skillman` CLI. Every command accepts `--data-dir <path>`; most list commands accept `--json`.

## Skills and agents

```sh
skillman scan [--project <path>] [--json]
skillman list [--agent <id>] [--project <path>] [--global] [--disabled] [--json]
skillman enable  <skill> --agent <id> [--project <path>]
skillman disable <skill> --agent <id> [--project <path>]
skillman agents [--json]
skillman agent enable|disable <id>        # hide an agent from scans; no files touched
skillman project add <path> [--create-skills-dir]
skillman project list|remove <path|id>
```

`project add` requires a folder with a `.git` entry or a per-project skills dir; `--create-skills-dir` creates `.agents/skills` in a plain folder and registers it anyway.

`disable` moves the skill folder to the quarantine directory inside the data directory; `enable` moves it back. No skill files are ever deleted.

## Discover and vault

```sh
skillman search <query> [--source skillssh|github|marketplace] [--json]
skillman add <ref> [--to <agent>]... [--all] [--copy] [--project <path>] [--command]
skillman vault list [--json]
skillman vault install <name> --to <agent>... [--all] [--project <path>] [--copy] [--command]
skillman vault uninstall <name> --agent <id> [--project <path>]
skillman vault update [<name>]            # re-fetch from the recorded source
skillman vault remove <name> [--force]
skillman import-lock [--file <path>]      # register ~/.agents/.skill-lock.json skills
```

### `<ref>` formats

`<ref>` accepts:

- `owner/repo[/path][@ref]`
- a GitHub tree URL
- `skillssh:<owner>/<repo>/<skill>`
- `marketplace:<owner>/<repo>/<plugin>`
- a local directory, `.mdc`, `.md`, `.zip` or `.tar.gz` file
- a `.zip`, `.tar.gz` or `SKILL.md` URL

Cursor rules, Claude and OpenCode commands and Claude plugins are normalized into spec skills on the way in. `--command` also writes a `.claude/commands/<name>.md` stub for Claude Code.

```sh
skillman add marketplace:anthropics/claude-plugins-official/plugin-dev --all
skillman config set marketplaces anthropics/claude-plugins-official,my-org/marketplace
```

## Sync, drift and export

```sh
skillman sync [<name>...] [--all] [--project <path>] [--dry-run] [--json]
skillman vault sync-mode <name> on|off    # fan out on every scan
skillman vault adopt <name> --from <agent> [--link]
skillman doctor [--json]
skillman doctor --fix-drift <name> --from <agent|vault> [--to <agent>]... [--project <path>]
skillman export plugin <out-dir> --name <plugin> [--version 0.1.0] [--description ...] <skill>...
```

### Sync and adopt notes

- Sync never overwrites a same-named directory that is not the vault entry; it is reported as a conflict.
- The read-only Claude plugin cache is skipped by sync.
- `vault adopt` copies an agent's skill into the vault as a `local` entry; `--link` replaces the agent directory with a symlink into the vault.

## Server, settings and service

```sh
skillman serve [--port <n>] [--no-open]
skillman config get|set port|github_token|skillssh_token|marketplaces [<value>]
skillman service install|uninstall|start|stop|restart|status [--json]   # macOS only
skillman tray [--login|--no-login]                                      # macOS only
skillman version
```

`skillman service install` writes a launchd LaunchAgent that runs `skillman serve --no-open` at login and restarts it on failure. On Linux and Windows, `service` reports unsupported.

The UI has four views: Skills, Vault, Discover and Settings. Deep links `/`, `/agent/<id>`, `/vault`, `/discover` and `/settings` open the matching view.

### Tokens

Tokens are optional. `github_token` (or `GITHUB_TOKEN`) raises GitHub rate limits; `skillssh_token` (or `SKILLSSH_TOKEN`) unlocks the skills.sh v1 API (trending, curated). Without it, search uses the public endpoint and skill files come from the source GitHub repository.

## Data and files

| Item | Location |
| --- | --- |
| Data directory | `~/.skillman` (override with `--data-dir`; the tray also reads `SKILLMAN_DATA_DIR`) |
| SQLite cache | inside the data directory; rebuilt by `skillman scan` |
| Vault | `~/.skillman/vault/<name>` plus a `<name>.vault.json` provenance sidecar |
| Quarantine | inside the data directory; holds disabled skills |
| Service logs | `~/.skillman/logs/` |
| Server pid | `~/.skillman/serve.pid` |
| `SKILLMAN_HOME` | Overrides what `~` expands to for agent directories and `~/.claude.json` (used for isolated runs and tests) |
