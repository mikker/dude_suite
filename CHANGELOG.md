# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project adheres to Semantic Versioning.

## [Unreleased]

### Changed
- Automated release packaging and Homebrew tap updates.

## [0.3.1] - 2026-01-13

### Changed
- Refactored output selection copying internals.

## [0.3.0] - 2026-01-13

### Added
- Output pane mouse selection copies to clipboard on release.

## [0.2.0] - 2026-01-13

### Added
- `persistent: true` tasks use a play icon while running.
- `autostart: true` tasks run when suite starts.

### Changed
- Exiting suite now cancels running tasks.

## [0.1.3] - 2026-01-13

### Added
- `ctrl+z` now suspends the TUI to allow backgrounding with job control.

### Changed
- Pressing a task hotkey while it is running now focuses that task.

## [0.1.2] - 2026-01-12

### Added
- `ctrl+r` restart hotkey for tasks.

### Fixed
- Killing a task now terminates descendant processes to stop run-loop style tasks.
