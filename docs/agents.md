# Supported agents

skillman scans the folders below for every agent. The first path in each cell is the primary location; installs, syncs and `vault install` go there. The other paths are scanned as well so skills placed there by other tools still show up.

| Agent id | Global skills paths | Project paths |
| --- | --- | --- |
| `claude-code` | `~/.claude/skills` | `.claude/skills` |
| `claude` (plugins, read-only) | `~/.claude/plugins/cache` | none |
| `codex` | `~/.agents/skills`, `~/.codex/skills` | `.agents/skills`, `.codex/skills` |
| `cursor` | `~/.cursor/skills`, `~/.agents/skills` | `.cursor/skills`, `.agents/skills`, `.claude/skills`, `.codex/skills` |
| `gemini-cli` | `~/.gemini/skills`, `~/.agents/skills` | `.gemini/skills`, `.agents/skills` |
| `antigravity` | `~/.gemini/config/skills`, `~/.gemini/antigravity/skills` | `.agents/skills` |
| `opencode` | `~/.config/opencode/skills` | `.opencode/skills`, `.claude/skills`, `.agents/skills` |

Project paths are relative to a project registered with `skillman project add <path>` or passed with `--project <path>`.

## Claude plugin cache

The `claude` agent reads the Claude plugin cache at `~/.claude/plugins/cache`. It is **read-only**: skillman lists the skills inside installed plugins, but never installs into, syncs to, enables, disables or moves anything there. `skillman sync` skips it.

## Skill usage counts

Claude Code skill usage counts are read from `~/.claude.json` (`skillUsage`) and shown as the `USED` column in `skillman list` and in the UI.

## Hiding an agent

`skillman agent disable <id>` hides an agent from scans and the UI without touching any files; `skillman agent enable <id>` shows it again.

## Overriding the home directory

`SKILLMAN_HOME` overrides what `~` expands to for every agent directory and for `~/.claude.json`. It is meant for isolated runs and tests.
