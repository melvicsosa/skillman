# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.7.1] - 2026-09-12

### Changed

- Doctor tab: issues are grouped in an accordion, each row carries a tone stripe by severity, results can be filtered, and the help panel stays pinned while scrolling.

## [0.7.0] - 2026-09-12

### Added

- Doctor tab in the web UI showing spec issues, broken vault links and drift.
- Agents card in Settings listing every supported agent and its skills folder.
- Resizable columns in the skills table.
- Adding a project now creates its skills directory when it does not exist yet.

### Changed

- README restructured with a screenshot, badges and links to the reference docs.

## [0.6.2] - 2026-09-10

### Added

- Native folder picker when adding a project.

### Changed

- Inputs use a single focus ring.

## [0.6.1] - 2026-09-10

### Changed

- Violet brand accent across the UI, green status dot for detected agents, and a divider in the sidebar.

## [0.6.0] - 2026-09-10

### Added

- Custom select controls, sidebar icons, a project manager and a light theme.
- Branded empty states throughout the UI.

## [0.5.1] - 2026-09-10

### Fixed

- Homebrew `opt` paths are persisted in the launchd service and menu bar login plists, so the service keeps working after an upgrade.

## [0.5.0] - 2026-09-10

### Added

- Vault sync fan-out to every enabled, writable agent.
- Drift repair from a chosen source (agent or vault).
- Claude marketplace sources for discovery.
- Export selected vault skills as a Claude plugin.
- URL routing in the web UI.

### Fixed

- The brand icon is used as favicon; the SVG fallback was removed.

## [0.4.0] - 2026-09-10

### Added

- `skillman-tray` menu bar app for macOS and the `skillman tray` command.
- macOS release build for the menu bar app.
- launchd login service (`skillman service`), port and token settings, usage stats, a Settings view and branding.

### Changed

- Release archives may contain a different number of binaries per platform, since the menu bar app ships only on macOS.

## [0.3.0] - 2026-09-10

### Added

- Vault: one copy per downloaded skill, linked or copied into agents.
- Skill conversion between agent formats.
- skills.sh and GitHub registries for search and install.
- Discover and Vault views in the web UI.

## [0.2.0] - 2026-09-09

### Added

- Skill discovery across agent folders and the `scan` command.
- Enable and disable skills per agent (quarantine instead of delete).
- Project support.
- `doctor` command.
- HTTP API and the embedded web UI.

## [0.1.0] - 2026-09-09

### Added

- Initial scaffold: CLI, SQLite storage, embedded UI and the release pipeline.

### Changed

- Release workflow skips Git state validation because the UI build overwrites `web/dist`.

[Unreleased]: https://github.com/melvicsosa/skillman/compare/v0.7.1...HEAD
[0.7.1]: https://github.com/melvicsosa/skillman/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/melvicsosa/skillman/compare/v0.6.2...v0.7.0
[0.6.2]: https://github.com/melvicsosa/skillman/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/melvicsosa/skillman/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/melvicsosa/skillman/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/melvicsosa/skillman/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/melvicsosa/skillman/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/melvicsosa/skillman/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/melvicsosa/skillman/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/melvicsosa/skillman/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/melvicsosa/skillman/releases/tag/v0.1.0
