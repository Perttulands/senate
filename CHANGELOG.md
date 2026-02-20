# Changelog

All notable changes to Senate.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)

## [Unreleased]

### Added
- 2026-02-20: Bootstrapped new `senate` Go repository with standalone CLI, docs, tests, and Makefile.
- 2026-02-20: Implemented deliberation spawner (`SEN-003`) with configurable N-agent panel and varied perspectives.
- 2026-02-20: Implemented verdict synthesis (`SEN-004`) with auditable transcript and binding judge output.
- 2026-02-20: Implemented precedent storage/search (`SEN-005`) via `state/precedents/index.jsonl`.
- 2026-02-20: Implemented implementation handoff (`SEN-006`) that creates Beads tasks from binding verdicts.
- 2026-02-20: Added `file-case` Relay handoff stub interface while keeping `SEN-002` integration out of scope.

### Fixed
- 2026-02-20: Added missing `cmd/senate/main.go` CLI entrypoint and corrected `.gitignore` to track `cmd/senate`.
