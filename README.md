# Senate

Senate (Ecclesia) is the Agora's multi-agent deliberation tool for high-impact decisions.

It implements:
- Multi-agent perspective spawning (`SEN-003`)
- Judge synthesis into binding verdicts (`SEN-004`)
- Searchable precedent storage (`SEN-005`)
- Implementation handoff via Beads (`SEN-006`)

`SEN-002` (Relay case filing integration) is intentionally stubbed here with `senate file-case`.

## Build

```bash
go build ./cmd/senate
```

## Quick Start

```bash
# Deliberate from a case file
senate deliberate --case ./case.json

# Ad-hoc deliberation
senate deliberate --quick "Should Centurion require 70% coverage for new code?"

# Search precedent
senate precedent search --query "coverage threshold"
```

## Case Schema (input)

```json
{
  "id": "senate-001",
  "type": "rule_evolution",
  "summary": "Amend silent-fallback rule to exclude trap handlers",
  "question": "Should we amend rule X to exclude trap handlers?",
  "evidence": [
    "state/reports/fp-47.md",
    "bead:athena-123"
  ],
  "requested_decision": "Add exception for || true in trap/cleanup context",
  "filed_at": "2026-02-19T23:00:00Z",
  "filed_by": "athena"
}
```

## State Layout

By default Senate writes under `./state`:

- `state/cases/<case_id>.json`
- `state/transcripts/<case_id>.json`
- `state/verdicts/<case_id>.json`
- `state/precedents/index.jsonl`
- `state/outbox/case-filed.jsonl` (Relay stub queue)

Set `SENATE_STATE_DIR` or `--state-dir` to override.

## Commands

```bash
senate deliberate --case <file> [--agents N] [--no-handoff] [--json]
senate file-case --case <file> [--json]            # SEN-002 stub
senate precedent search --query <text> [--limit N] [--type TYPE] [--verdict DECISION]
senate handoff --case-id <id> [--workspace <path>]
senate version
```
