# Changelog

All notable changes to Senate.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)

## [Unreleased]

### Changed
- README: restored mythology intro (The Ecclesia), character description, "Part of the Agora" section

## [2026-02-20]

### Added
- Bootstrapped new `senate` Go repository with standalone CLI, docs, tests, and Makefile (`SEN-001`).
- Implemented deliberation spawner (`SEN-003`) with configurable N-agent panel and varied perspectives.
- Implemented verdict synthesis (`SEN-004`) with auditable transcript and binding judge output.
- Implemented precedent storage/search (`SEN-005`) via `state/precedents/index.jsonl`.
- Implemented implementation handoff (`SEN-006`) that creates Beads tasks from binding verdicts.
- Added `file-case` Relay handoff stub interface while keeping `SEN-002` integration out of scope.

### Changed
- Added the initial Keep a Changelog document for Senate.

### Fixed
- Added missing `cmd/senate/main.go` CLI entrypoint and corrected `.gitignore` to track `cmd/senate`.
