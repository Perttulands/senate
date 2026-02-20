# 🏛️ Senate

![Senate](images/acropolis.png)


*The Ecclesia convenes. Multiple voices. One verdict. Binding.*

---

In Athens, the Ecclesia was where citizens gathered to debate matters that affected everyone. Not a rubber stamp. Not a suggestion box. A deliberation — messy, argumentative, and binding. When the Ecclesia voted, the city moved. You didn't get to relitigate because you slept in.

The Senate works the same way, except the citizens are AI agents and the matters at hand are architectural decisions, rule changes, and system evolution. When a question is too important for one agent's judgment — should Centurion require 70% coverage? Should Truthsayer amend a rule? — it goes to the Ecclesia. Multiple perspectives are spawned. They argue. A judge synthesizes the debate into a verdict. The verdict is recorded as precedent. The precedent is searchable. And the decision gets handed off as real work via beads.

No backroom deals. No revisionism. If you want to overturn a verdict, you file a new case and argue it on the merits. The Ecclesia has a long memory and a short tolerance for re-litigation.

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

## Part of the Agora

Senate was forged in **[Athena's Agora](https://github.com/Perttulands/athena-workspace)** — an autonomous coding system where AI agents build software and the hard decisions go through deliberation, not diktat.

[Argus](https://github.com/Perttulands/argus) watches the server. [Truthsayer](https://github.com/Perttulands/truthsayer) watches the code. [Oathkeeper](https://github.com/Perttulands/oathkeeper) watches the promises. [Relay](https://github.com/Perttulands/relay) carries the messages. Senate decides what the rules should be.

The [mythology](https://github.com/Perttulands/athena-workspace/blob/main/mythology.md) has the full story.

## License

MIT
