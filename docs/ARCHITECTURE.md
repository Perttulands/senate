# Senate Architecture

## Overview

Senate is a multi-agent deliberation system. A single `claude` process acts as moderator, with senator sub-agents defined via Claude's `--agents` JSON flag and spawned by the Task tool during deliberation.

No tmux. No external config files. Senators are compiled into the binary.

## Core Components

### Senator Catalog (`internal/cli/protocol.go`)

Five senator archetypes are hardcoded in `senatorCatalog`:

| Name        | Archetype              | Perspective                        |
|-------------|------------------------|------------------------------------|
| pragmatist  | The Pragmatic Builder  | Ship, learn, iterate               |
| purist      | The Architectural Purist | System integrity, long-term design |
| skeptic     | The Critical Skeptic   | Evidence, failure modes, risks     |
| steward     | The Operational Steward | Stability, blast radius, rollback  |
| advocate    | The User Advocate      | User value, accessibility, trust   |

Each senator has a `FullName`, `Philosophy`, `Values`, `Framework`, and `Personality` — all embedded as Go string literals.

### Agent Definition (`BuildAgentsJSON`)

`BuildAgentsJSON(n)` selects the first `n` senators from the catalog (default 3, max 5) and returns a JSON string for the `--agents` flag. Each agent entry contains:

- **Key**: `senator-{name}` (e.g., `senator-pragmatist`)
- **Description**: Used by Claude's Task tool to select the right sub-agent
- **Prompt**: The senator's full identity, values, decision framework, and output format instructions
- **Model**: Hardcoded to `"sonnet"` for all senators

Senators respond with a JSON object: `{"stance": "...", "reasoning": "...", "concerns": "..."}`.

### Moderator Prompt (`BuildSystemPrompt`)

`BuildSystemPrompt(mode, verdictPath, caseID)` generates the system prompt for the moderator (the main `claude` process). The prompt defines a four-phase deliberation protocol:

1. **Initial Positions** — Spawn each senator sub-agent via the Task tool (in parallel). Present the case and collect independent stances.
2. **Challenges** — Identify disagreements. Resume dissenting senators with opposing arguments.
3. **Final Positions** — Resume each senator with full deliberation context. They may revise or hold.
4. **Verdict** — Moderator synthesizes a binding verdict based on argument quality, evidence, and long-term implications.

The `mode` parameter controls output behavior:
- `"ask"` (pipe mode): Write verdict JSON to `verdictPath`, print one-line summary to stdout.
- `"start"` (interactive): Present verdict to user, then write to `verdictPath`.

### Verdict Output

The moderator writes a JSON verdict file to a temp directory using Claude's Write tool. The verdict contains:

```json
{
  "case_id": "SNT-...",
  "verdict": "approved|rejected|amended|deferred",
  "reasoning": "...",
  "implementation": "...",
  "dissent": "...",
  "positions": [{"senator": "pragmatist", "stance": "approved", "key_argument": "..."}]
}
```

After Claude exits, Go code reads and parses this file, stores the verdict, and updates the precedent index.

## Execution Model

### `senate ask` (pipe mode)

1. Parse question from positional arg or stdin
2. Create a `core.Case` and persist it via `store`
3. Create temp dir; write system prompt to `protocol.md`
4. Build agents JSON
5. Run: `claude -p --dangerously-skip-permissions --model <model> --system-prompt-file <file> --agents <json>` with question on stdin
6. Read `verdict.json` from temp dir
7. Parse, store verdict and precedent
8. Print verdict JSON to stdout

Default moderator model: `sonnet`. Overridable with `--model`.

### `senate start` (interactive mode)

Same setup, but runs `claude` in foreground with stdin/stdout/stderr attached. User types the question interactively. Verdict is read and stored after the session ends.

## Persistence

- **Cases and verdicts**: Stored via `internal/store` in `state/` (overridable with `--state-dir` or `SENATE_STATE_DIR`)
- **Precedents**: JSONL index at `state/precedents.jsonl`, searchable via `senate precedent search`
- **Handoffs**: `senate handoff --case-id <id>` creates an implementation bead from a verdict

## Other Commands

- `senate health` — Checks that `claude` CLI is on PATH
- `senate file-case` — File a case without deliberating (for deferred or batched use)
- `senate precedent search` — Full-text search over past verdicts
- `senate handoff` — Create a bead for implementing a verdict
