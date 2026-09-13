# skillman

One place to see, switch and share the skills of every AI coding agent on your machine.

Every AI coding agent keeps its own skills folder. None of them can switch a single skill off, and copies of the same skill drift apart across agents. skillman is one Go binary that finds every skill on your machine, lets you enable or disable each one per agent, keeps one copy in a vault, and serves a local UI.

<p><img src="https://raw.githubusercontent.com/melvicsosa/skillman/main/assets/demo.gif" alt="skillman CLI demo" width="100%"></p>

## Install

```sh
brew install melvicsosa/tap/skillman   # macOS and Linux
skillman scan                          # discover the skills on disk
skillman serve                         # open the UI on http://localhost:3010
```

Linux without Homebrew: download the tarball for your architecture from the [releases page](https://github.com/melvicsosa/skillman/releases) and put `skillman` on your `PATH`. Windows builds from source with `go install github.com/melvicsosa/skillman/cmd/skillman@latest`.

## Reference

- [Supported agents](agents.md): every global and project path skillman scans, per agent.
- [Commands](commands.md): the complete CLI reference, every flag and the `<ref>` formats.
- [Changelog](https://github.com/melvicsosa/skillman/blob/main/CHANGELOG.md)
- [Contributing](https://github.com/melvicsosa/skillman/blob/main/CONTRIBUTING.md)

## How the vault works

The vault is a directory inside `~/.skillman` that holds one copy of each skill you downloaded or adopted. Agents get a symlink into the vault by default, so every agent reads the same files and an update lands everywhere at once.

1. **Add** a skill from skills.sh, GitHub, a Claude marketplace or a local file. It lands in the vault and is linked into the agents you choose.
   ```sh
   skillman add owner/repo/path --to claude-code --to codex
   skillman add skillssh:owner/repo/skill --all
   ```
2. **Adopt** a skill an agent already has, so the vault becomes its source of truth.
   ```sh
   skillman vault adopt my-skill --from claude-code --link
   ```
3. **Sync** fans a vault entry out to every enabled agent. Turn it on per skill and every `skillman scan` keeps the copies aligned.
   ```sh
   skillman vault sync-mode my-skill on
   skillman sync --all
   ```
4. **Doctor** reports copies that drifted apart and repairs them from the copy you trust.
   ```sh
   skillman doctor
   skillman doctor --fix-drift my-skill --from vault
   ```

Sync never overwrites a directory that is not the vault entry; it reports a conflict instead. The Claude plugin cache is read-only and is never written to.

## Enable and disable without deleting

`skillman disable <skill> --agent <id>` moves the skill folder into a quarantine directory inside `~/.skillman`; `enable` moves it back. No skill files are ever deleted. Project-scoped skills use `--project <path>`.

## Source

[github.com/melvicsosa/skillman](https://github.com/melvicsosa/skillman), MIT license.
