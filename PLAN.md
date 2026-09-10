# skillman — cross-agent skill manager

One Go binary, installed via Homebrew, that:

- discovers every agent skill on the machine, global and per project,
  grouped by agent;
- enables/disables skills and agents;
- keeps a **vault** of downloaded skills and installs them into any agent,
  converting when the source is not in the standard format;
- **searches** public skill registries and downloads from them;
- serves a local UI on `http://localhost:3010`. Every action is available
  from the CLI and from the UI, through the same use cases.

Target agents: claude-code, claude (plugins), codex, cursor, gemini-cli,
antigravity, opencode. Copilot is cheap to add later (same conventions).

---

## 1. Research facts that shape the design (verified 2026-09-09)

### 1.1 The standard already exists: Agent Skills spec

https://agentskills.io/specification. A skill is a directory `<name>/` with
`SKILL.md` (YAML frontmatter + markdown body) and optional `scripts/`,
`references/`, `assets/`.

| Field | Required | Rule |
|---|---|---|
| `name` | yes | 1–64 chars, `[a-z0-9-]`, must equal the directory name |
| `description` | yes | 1–1024 chars, carries the trigger text |
| `license`, `compatibility`, `metadata` (string map), `allowed-tools` | no | |

Claude Code adds extra keys (`user-invocable`, `disable-model-invocation`,
`context`, `model`, `paths`, `hooks`, ...) that other agents ignore.

On this machine the same skill's `SKILL.md` is **byte-identical** across all
seven agent dirs (194 files checked). The only differences found were stale
copies, not format differences. So "conversion" is never about rewriting the
skill body. It is about (a) validating/normalizing to spec and (b) emitting a
per-agent wrapper when one is needed (see §4).

### 1.2 Where agents look for skills (official docs)

| Agent | Global | Project |
|---|---|---|
| Claude Code | `~/.claude/skills` | `.claude/skills` |
| Codex | `~/.agents/skills` (`~/.codex/skills` legacy) | `.agents/skills` (`.codex/skills` legacy) |
| Cursor | `~/.cursor/skills`, `~/.agents/skills` | `.cursor/skills`, `.agents/skills` (+ legacy `.claude/skills`, `.codex/skills`) |
| Gemini CLI | `~/.gemini/skills`, `~/.agents/skills` | `.gemini/skills`, `.agents/skills` |
| Antigravity | `~/.gemini/config/skills` (`~/.gemini/antigravity/skills` symlinks to it) | `.agents/skills` |
| OpenCode | `~/.config/opencode/skills` | `.opencode/skills`, `.claude/skills`, `.agents/skills` |

**`.agents/skills` is the cross-agent convention** (Codex, Cursor, Gemini,
Antigravity, OpenCode, Copilot read it). Claude Code is the notable exception:
it only reads `.claude/skills`, which is why projects on this machine symlink
`.claude/skills/<name>` to `../../.agents/skills/<name>`.

Other facts:
- Every agent loads by **directory presence**. No agent has a per-skill
  disable switch. Claude only has `enabledPlugins` for whole plugins.
- Global agent dirs are **independent copies** (six inodes per skill) and
  already drift: the Codex copy is four months older than the others.
- `_shared/` exists in every skills dir and is not a skill. Filter it.
- `~/.agents/.skill-lock.json` is the vercel `skills` CLI lockfile: per skill
  `source` (`owner/repo`), `sourceType` (`github`), `skillPath`,
  `skillFolderHash`, `installedAt`. Good import source for provenance.
- Claude usage telemetry lives in `~/.claude.json` under `skillUsage`
  (`usageCount`, `lastUsedAt`).
- Claude plugins: `~/.claude/plugins/cache/<marketplace>/<plugin>/<version>/`
  with `skills/<name>/SKILL.md` plus per-agent wrappers
  (`.claude-plugin/plugin.json`, `.cursor-plugin/`, `.codex-plugin/`,
  `agents/openai.yaml`). Registry in `installed_plugins.json`.

### 1.3 Registries with a usable API

| Source | Surface | Auth |
|---|---|---|
| **skills.sh** (vercel-labs) | `https://skills.sh/api/v1/skills/search?q=`, `/skills?view=trending`, `/skills/curated`, `/skills/{source}/{skill}` returns `files[]` with contents + hash | none documented |
| **GitHub repo** | `GET /repos/{o}/{r}/tarball/{ref}` then extract subpath. Accept `owner/repo`, `owner/repo/path`, full URL, `.../tree/<ref>/<path>` | 60 req/h anon, 5000/h with token |
| **anthropics/skills** | plain GitHub repo, `skills/<name>/SKILL.md` | via GitHub source |
| **Claude marketplaces** | `.claude-plugin/marketplace.json` in a repo (`anthropics/claude-plugins-official`, others) lists plugins with `source` | via GitHub source |
| Curated lists (obra/superpowers, ComposioHQ/awesome-claude-skills, VoltAgent/awesome-agent-skills) | GitHub repos, no API | via GitHub source |

GitHub code search (`filename:SKILL.md`) needs auth and is capped at 10
req/min. Not worth it for MVP: skills.sh search covers discovery, GitHub
covers download.

### 1.4 Reference project: gentle-ai

GoReleaser binary formula in `gentleman-programming/tap`, `CGO_ENABLED=0`,
state in `~/.gentle-ai/*.json` with a lock file, no daemon, hand-rolled CLI +
Bubble Tea TUI, `internal/agents/<name>/adapter` per agent. We copy the
release pipeline and the per-agent adapter shape, not the JSON state.

---

## 2. Design decisions

1. **Filesystem is the source of truth; SQLite is cache plus intent.**
   `modernc.org/sqlite` (pure Go, no CGO) at `~/.skillman/skillman.db`. A scan
   rebuilds everything from disk. The DB holds only what disk cannot express:
   disabled state, agent enabled flag, registered projects, vault provenance,
   scan history, settings. Delete it, re-scan, nothing lost.
2. **Disable = quarantine move.** `disable` moves
   `<agentdir>/<name>` to `~/.skillman/disabled/<agent>/<name>` and records it;
   `enable` moves it back. Works with every loader. If the DB is wiped, a scan
   of the quarantine dir restores the state. Symlinks are moved, targets are
   never touched.
3. **Agent disable = hide and skip.** No files touched. Bulk-disabling an
   agent's skills is an explicit separate command.
4. **The vault is the canonical local store.** `~/.skillman/vault/<name>/`
   holds one copy of each downloaded skill plus `vault.json` provenance
   (source, ref, hash, installed_at). Agents receive **symlinks** into the
   vault by default (one copy, no drift), or copies when the user asks
   (`--copy`) or when an agent cannot follow symlinks (none known today).
5. **Conversion is normalize + wrap, never rewrite.** See §4.
6. **Registries are adapters behind one interface.** `Search(query)`,
   `Fetch(ref) -> skill files`. MVP ships skills.sh and GitHub. Marketplaces
   and curated lists are GitHub-backed sources added later.
7. **Server is optional, UI is embedded.** `skillman serve` on `:3010`
   (`--port`, persisted setting), opens the browser, serves a Vite + React +
   TypeScript build from `embed.FS`. No Node at runtime.
8. **Menu bar icon is Phase 3**, as a separate `skillman-tray` binary
   (`fyne.io/systray`, needs CGO on macOS) in the same release, so the core
   CLI stays CGO-free and cross-compiles like gentle-ai.

---

## 3. Domain model

```
Agent      id, name, enabled, globalDirs[], projectDirs[], supportsSymlink
Project    id, root, name, registeredAt
Skill      identity (agent, scope, path); name, description, version,
           contentHash, isSymlink, linkTarget, state(enabled|disabled),
           quarantinePath, vaultRef?, lastSeenAt
VaultEntry name, path, contentHash, source{type, ref, url, subpath},
           sourceHash, installedAt, updatedAt, spec{valid, issues[]}
Registry   id, kind(skillssh|github), Search(), Fetch()
```

Scope is `global` or `project:<root>`. Two skills with equal `contentHash`
are the same skill; same name with different hash is **drift**.

SQLite tables mirror this: `agents`, `projects`, `skills`, `vault`,
`scans`, `settings`. Migrations embedded as SQL files.

---

## 4. Conversion

Trigger: enabling a vault skill into an agent, or importing something that is
not a spec-shaped skill into the vault. Pipeline:

1. **Detect** the source shape:
   - spec skill (`<dir>/SKILL.md` with `name` + `description`) → no conversion
   - Cursor legacy rule (`.cursor/rules/*.mdc`, frontmatter
     `description`/`globs`/`alwaysApply`) → convertible when
     `alwaysApply: false` and no `globs` (same rule Cursor's own
     `/migrate-to-skills` applies); otherwise reject with the reason
   - Claude command (`.claude/commands/*.md`, single `description` key) →
     convertible
   - OpenCode command (`description`, `agent`, `subtask`) → convertible, the
     `agent`/`subtask` keys are dropped with a warning
   - Claude plugin (`.claude-plugin/plugin.json` + `skills/`) → unpack each
     skill, no conversion needed
   - anything else → rejected, listed in `doctor`
2. **Normalize** to spec: derive `name` from the directory (or filename),
   validate charset/length, require `description` (fall back to the first
   body line and warn), keep unknown keys (agents ignore them), never touch
   the body.
3. **Wrap** for the target when the target needs it:
   - Claude Code: optional `.claude/commands/<name>.md` stub so the skill is
     slash-invocable (`--command` flag)
   - Claude plugin export: `.claude-plugin/plugin.json` + `skills/<name>/`
     (Phase 4)
   - all others: nothing, they read the spec skill as-is
4. **Report** what changed. Conversions are recorded in `vault.json`
   (`convertedFrom`) so `doctor` can show them.

Compatibility is decided per target agent, not per skill: a skill that uses
Claude-only frontmatter keys is still installable everywhere, it just gets a
"Claude-only keys ignored by <agent>" note.

---

## 5. Registry search and install

```
skillman search <query> [--source skillssh|github] [--json]
skillman add <ref> [--to <agent>...|--all] [--copy] [--project <path>]
   <ref> forms: owner/repo, owner/repo/path, https://github.com/.../tree/<ref>/<path>,
   skillssh:<source>/<skill>, a local path, a SKILL.md/.zip/.tar.gz URL
skillman vault list|remove|update [<name>]
skillman import-lock          # read ~/.agents/.skill-lock.json into the vault
```

Flow for `add`: resolve ref → fetch into a temp dir → detect/normalize (§4)
→ write to `vault/<name>` with provenance → link into the requested agents
(global, or `<project>/.agents/skills` plus the `.claude/skills` symlink for
Claude) → rescan those dirs. GitHub token is optional (`settings.github_token`
or `GITHUB_TOKEN`) and only raises rate limits.

UI: a "Discover" tab with a search box, source filter, trending/curated
lists from skills.sh, and an Install button that asks which agents.

---

## 6. Architecture (hexagonal)

```
cmd/skillman/              main, cobra root
internal/domain/           entities + ports (SkillRepo, VaultRepo, AgentFS,
                           Registry, Converter)
internal/app/              use cases: Scan, List, Enable, Disable, ToggleAgent,
                           RegisterProject, Doctor, Search, Add, Convert, Sync
internal/adapters/
  agents/<name>/           path resolution + discovery per agent
  storage/sqlite/          repos + embedded migrations
  fs/                      frontmatter parser, hasher, quarantine mover, linker
  convert/                 detectors + normalizers + wrappers (§4)
  registry/skillssh/       skills.sh API client
  registry/github/         tarball fetch + subpath extract
  http/                    net/http API, embeds web/dist
internal/cli/              cobra commands, thin
web/                       Vite + React + TS (atomic design)
```

Tests: fixture trees per agent adapter, converter table tests (one fixture
per source shape), quarantine round trip, hash drift, registry clients
against recorded HTTP responses.

---

## 7. HTTP API

```
GET   /api/agents                      PATCH /api/agents/{id}     {enabled}
GET   /api/skills?agent=&project=&scope=&q=
POST  /api/skills/{id}/enable          POST  /api/skills/{id}/disable
GET   /api/projects                    POST  /api/projects        {root}
POST  /api/scan                        GET   /api/doctor
GET   /api/vault                       DELETE /api/vault/{name}
POST  /api/vault/{name}/install        {agents[], project?, copy?}
GET   /api/registry/search?q=&source=  GET   /api/registry/trending
POST  /api/registry/add                {ref, agents[], project?}
GET   /api/settings                    PATCH /api/settings        {port,...}
```

---

## 8. UI

Left sidebar: agents with toggles and counts, then "Vault" and "Discover".
Main area for an agent: tabs Global / Projects (project picker), table with
name, description, version, scope, state, drift and "from vault" badges,
row toggle. Vault view: installed-where matrix (skill × agent) with checkboxes
that install/uninstall. Discover view: search, trending, curated, install
dialog choosing agents and scope. Doctor panel: invalid skills, drift,
rejected conversions.

---

## 9. Phases

**Phase 0, scaffolding.** Go module, cobra, sqlite + migrations, GoReleaser,
`homebrew-tap` repo with `Formula/skillman.rb`, tag-triggered release.
Done when `brew install <you>/tap/skillman` installs a hello binary.

**Phase 1, MVP core.** Agent adapters (7), scan, list (global and per
project), enable/disable skill, agent toggle, doctor, `serve` + embedded UI
with the agent/skill views.

**Phase 2, vault + registry.** Vault store and provenance, converter (§4),
skills.sh + GitHub sources, `search`/`add`/`vault`, `import-lock`, Discover
and Vault views in the UI.
Status (2026-09-10): backend and CLI done (vault + sidecars, converter,
skills.sh/GitHub/local/URL sources, `search`/`add`/`vault`/`import-lock`,
`/api/vault` and `/api/registry` routes). Note: skills.sh `/api/v1/*` now
requires a Vercel OIDC token; without one, search uses the public legacy
`/api/search` and skill files come from the GitHub tarball of the source
repository. Discover and Vault UI views done (sidebar entries, skill × agent
install matrix, add-by-ref/import-lock, search/trending/curated with install
dialog); the agent table derives its "vault" badge from `/api/vault` links
because `SkillView` does not expose `vaultRef` yet.

**Phase 3, always on.** launchd LaunchAgent (`service install`), port setting
in UI, `skillman-tray` menu bar binary (open UI, start/stop, port, quit),
usage stats from `~/.claude.json`.
Status (2026-09-10): done. launchd adapter + `service` commands, port/token
settings, `/api/usage`, and `skillman-tray` (`cmd/skillman-tray`,
`internal/tray`, fyne.io/systray behind a `darwin` build tag so `skillman`
stays CGO-free): status line polled every 5 s, open UI, start/stop/restart
(launchd when installed, detached `serve` + pid file otherwise),
install/uninstall service, port submenu, launch-at-login LaunchAgent,
`skillman tray [--login|--no-login]`. Release runs on macOS and ships the
tray in the darwin archives and the brew formula.

**Phase 4, sync.** Fan-out from vault to all enabled agents, drift repair
("update codex copy from vault"), marketplace sources, Claude plugin export.

---

## 10. Assumptions

- Binary `skillman`, data dir `~/.skillman`, default port 3010.
- macOS first; Linux works for free except the tray.
- Symlink installs by default. If an agent turns out not to follow symlinks
  the adapter flips `supportsSymlink` and installs copies.
