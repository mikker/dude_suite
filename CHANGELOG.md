# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project adheres to Semantic Versioning.

## [Unreleased]

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
