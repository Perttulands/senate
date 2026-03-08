# Senate

![Senate Banner](banner.png)

*The Ecclesia convenes. Multiple voices. One verdict. Binding.*

---

Senate is a multi-agent deliberation tool. When a decision is too important for one agent's judgment — architecture choices, rule changes, policy disputes — Senate convenes a panel of AI agents with genuinely different perspectives, lets them argue, and synthesizes a binding verdict. The verdict becomes searchable precedent. The decision can be handed off as real work.

It exists because single-agent systems make decisions the way a person talks to themselves in the shower: confidently, without pushback, and sometimes dangerously wrong. Senate forces the argument to happen before the decision ships.

---

In Athens, the Ecclesia was where citizens gathered to debate matters that affected everyone. Not a rubber stamp. Not a suggestion box. A deliberation — messy, argumentative, and binding. When the Ecclesia voted, the city moved. You didn't get to relitigate because you slept in.

The Senate works the same way, except the citizens are AI agents and the matters at hand are architectural decisions, rule changes, and system evolution. When a question is too important for one agent's judgment — should Centurion require 70% coverage? Should Truthsayer amend a rule? — it goes to the Ecclesia. Multiple perspectives are spawned. They argue. A judge synthesizes the debate into a verdict. The verdict is recorded as precedent. The precedent is searchable. And the decision gets handed off as real work via beads.

No backroom deals. No revisionism. If you want to overturn a verdict, you file a new case and argue it on the merits. The Ecclesia has a long memory and a short tolerance for re-litigation.

## Build

```bash
go build ./cmd/senate
```

## Quick Start

```bash
# Check prerequisites
senate health

# Run a deliberation (non-interactive, emits JSON)
senate ask "Should Centurion require 70% coverage for new code?"

# File a structured case
senate file-case \
  --type rule_evolution \
  --summary "Amend silent-fallback rule to exclude trap handlers" \
  --question "Should we amend rule X to exclude trap handlers?"

# Search past verdicts
senate precedent search --query "coverage threshold"
```

## Commands

### `senate ask "<question>" [flags]`

Run a moderated deliberation in pipe mode. Emits structured JSON to stdout.

```bash
senate ask "Should we require integration tests for all new APIs?" \
  --agents 3 \
  --type architecture \
  --filed-by athena \
  --model sonnet
```

Flags:
- `--agents <n>` — number of senators (default: 3, max: 5)
- `--type <type>` — case type (default: `general`)
- `--filed-by <name>` — filer identity (default: `agent`)
- `--model <model>` — moderator model passed to `claude` (default: `sonnet`)
- `--state-dir <path>` — storage root override

Output JSON fields: `case_id`, `verdict`, `reasoning`, `implementation` (optional), `dissent` (optional), `positions[]` (senator, stance, key_argument).

### `senate start [flags]`

Interactive deliberation with live terminal passthrough. Verdicts are persisted after the session ends.

Flags:
- `--agents <n>` — panel size (default: 3, max: 5)
- `--model <model>` — moderator model (default: `sonnet`)
- `--state-dir <path>` — storage root override

### `senate file-case [flags]`

Validate and persist a new case record. Prints the case ID on success.

Required flags:
- `--type <type>` — one of: `rule_evolution`, `gate_criteria`, `dispute`, `priority`, `architecture`, `general`
- `--summary <text>`
- `--question <text>`

Optional flags:
- `--requested-decision <text>`
- `--filed-by <name>`
- `--evidence <text>` (repeatable)
- `--state-dir <path>`

### `senate precedent search [flags]`

Search the precedent index by keyword scoring and optional filters.

```bash
senate precedent search \
  --query "coverage threshold" \
  --type rule_evolution \
  --verdict approved \
  --limit 10 \
  --json
```

Flags:
- `--query <text>` — search terms
- `--type <type>` — exact case-type filter
- `--verdict <decision>` — filter by verdict (`approved`, `rejected`, `amended`, `deferred`; synonyms accepted)
- `--limit <n>` — max results (default: 20)
- `--state-dir <path>`
- `--json` — output JSON array

### `senate handoff --case-id <id> [flags]`

Create a Beads implementation task from a stored verdict. Skipped for non-binding or deferred verdicts.

Required flag:
- `--case-id <id>`

Optional flags:
- `--state-dir <path>`
- `--workspace <path>`
- `--json`

### `senate health [--verbose]`

Check that `claude` CLI is available in PATH.

### `senate version`

Print `senate 0.2.0`.

## Case Schema

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

## Senator Catalog

Five senators are available; the first N are selected for each panel:

| Senator | Character | Perspective |
|---------|-----------|-------------|
| `pragmatist` | Senator Marcus Aurelius | Practical tradeoffs — ship, learn, iterate |
| `purist` | Senator Athena Sophia | Principled correctness and long-term integrity |
| `skeptic` | Senator Diogenes Truth | Challenge assumptions, find failure modes |
| `steward` | Senator Gaius Ops | Maintenance burden and operational health |
| `advocate` | Senator Vox Populi | User and developer experience |

Each senator has a distinct philosophy, decision framework, and personality that shapes how they argue. The moderator spawns them as parallel sub-agents, runs challenges between disagreeing senators, collects final positions, and synthesizes a verdict.

## State Layout

By default Senate writes under `./state`. Set `SENATE_STATE_DIR` or `--state-dir` to override.

```
state/
├── cases/<case_id>.json
├── verdicts/<case_id>.json
├── transcripts/<case_id>.json
├── precedents/index.jsonl
└── outbox/case-filed.jsonl
```

## Dependencies

**Required:** `claude` CLI — Senate spawns sub-agents via Claude's native `--agents` flag. Used by `ask` and `start`.

**Optional:** `br` — required when creating Beads handoff tasks via `senate handoff`.

## Current Status

✅ Multi-agent deliberation via `senate ask` (pipe mode, JSON output)
✅ Interactive deliberation via `senate start`
✅ Case filing with validation (`senate file-case`)
✅ Precedent storage and keyword search (`senate precedent search`)
✅ Beads handoff from verdicts (`senate handoff`)
✅ Senator catalog with 5 distinct perspectives and structured prompts
✅ Health check for prerequisites
⚠️ Relay integration for case filing is stubbed — `file-case` writes locally but does not publish to Relay
⚠️ Precedent search is keyword-based, no semantic/embedding search yet
⚠️ No web UI or dashboard for browsing verdicts

## Part of Polis

Senate is one piece of a larger system where AI agents build and maintain software together.

| Tool | What it does |
|------|-------------|
| [ergon-work-orchestration](https://github.com/Perttulands/ergon-work-orchestration) | Work queue and task orchestration |
| [hermes-relay](https://github.com/Perttulands/hermes-relay) | Message relay between agents |
| [cerberus-gate](https://github.com/Perttulands/cerberus-gate) | Quality gate enforcement |
| [chiron-trainer](https://github.com/Perttulands/chiron-trainer) | Agent training framework |
| [learning-loop](https://github.com/Perttulands/learning-loop) | Feedback and improvement cycles |
| **[senate](https://github.com/Perttulands/senate)** | Multi-agent deliberation (you are here) |
| [beads-polis](https://github.com/Perttulands/beads-polis) | Work units and task tracking |
| [truthsayer](https://github.com/Perttulands/truthsayer) | Code rule enforcement |
| [ultimate_bug_scanner](https://github.com/Perttulands/ultimate_bug_scanner) | Bug detection |
| [horkos-oathkeeper](https://github.com/Perttulands/horkos-oathkeeper) | Promise and contract tracking |
| [argus-watcher](https://github.com/Perttulands/argus-watcher) | Server and system monitoring |
| [polis-utils](https://github.com/Perttulands/polis-utils) | Shared utilities |

The [mythology](https://github.com/Perttulands/athena-workspace/blob/main/mythology.md) has the full story.

## License

MIT
