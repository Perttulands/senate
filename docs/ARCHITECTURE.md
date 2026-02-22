# Senate Agent Architecture

## Overview

This document describes how real LLM senator agents are spawned and orchestrated for Senate deliberations. Each senator is a full Claude Opus instance with persistent identity, memory, and distinct decision-making perspectives.

## Senator Spawning Model

### Tmux Session Architecture

Each senator runs in a dedicated tmux session on the `senate` socket:

```
tmux -S /tmp/tmux-senate.sock new-session -d -s "senator-{name}-{case_id}" \
  "claude -p --model opus --system-prompt-file {senator_prompt_file}"
```

**Key Design Decisions:**
- Separate tmux socket (`/tmp/tmux-senate.sock`) to isolate from other Athena agents
- Session naming: `senator-{name}-{case_id}` (e.g., `senator-pragmatist-SNT-001`)
- Claude in pipe mode (`-p`) for programmatic interaction
- System prompt loaded from dedicated files

### Spawning Sequence

```
1. Load senator configs from senate-config.json
2. For each senator in the panel:
   a. Generate system prompt with identity + case context
   b. Write prompt to temp file
   c. Create tmux session with claude CLI
   d. Send initial case presentation via tmux send-keys
   e. Monitor for response completion
3. Collect all initial positions
4. Orchestrate challenge rounds
5. Collect final positions
6. Clean up sessions
```

## Senator Configuration Schema

### Config File Location
`/home/chrote/athena/tools/senate/config/senate-config.json`

### Schema

```json
{
  "version": "1.0",
  "default_model": "claude:opus",
  "default_panel_size": 3,
  "senators": [
    {
      "name": "pragmatist",
      "model": "claude:opus",
      "identity": {
        "full_name": "Senator Marcus Aurelius",
        "archetype": "The Pragmatic Builder",
        "decision_style": "Bias toward action with calculated risk",
        "values": ["shipping velocity", "practical outcomes", "reversible decisions"],
        "anti_values": ["analysis paralysis", "perfection over progress", "theoretical purity"]
      },
      "system_prompt_template": "templates/pragmatist.md",
      "memory": {
        "path": "memory/senators/pragmatist/",
        "precedent_access": true,
        "case_history_limit": 20
      }
    },
    {
      "name": "purist",
      "model": "claude:opus",
      "identity": {
        "full_name": "Senator Athena Sophia",
        "archetype": "The Architectural Purist",
        "decision_style": "Correctness and consistency above all",
        "values": ["system integrity", "long-term maintainability", "proven patterns"],
        "anti_values": ["technical debt", "shortcuts", "inconsistent interfaces"]
      },
      "system_prompt_template": "templates/purist.md",
      "memory": {
        "path": "memory/senators/purist/",
        "precedent_access": true,
        "case_history_limit": 20
      }
    },
    {
      "name": "skeptic",
      "model": "claude:opus",
      "identity": {
        "full_name": "Senator Diogenes Truth",
        "archetype": "The Critical Skeptic",
        "decision_style": "Challenge assumptions, find hidden risks",
        "values": ["evidence-based decisions", "failure mode analysis", "honest assessment"],
        "anti_values": ["wishful thinking", "unvalidated claims", "groupthink"]
      },
      "system_prompt_template": "templates/skeptic.md",
      "memory": {
        "path": "memory/senators/skeptic/",
        "precedent_access": true,
        "case_history_limit": 20
      }
    }
  ],
  "judge_config": {
    "model": "claude:opus",
    "system_prompt_template": "templates/judge.md"
  }
}
```

## Memory Architecture

### Per-Senator Memory Structure

Each senator maintains their own memory directory:

```
memory/senators/{senator_name}/
├── IDENTITY.md          # Senator's core identity and values
├── MEMORY.md            # Long-term memory of key decisions
├── cases/               # Per-case deliberation logs
│   ├── SNT-001.md
│   ├── SNT-002.md
│   └── ...
└── precedents.jsonl     # Quick-reference precedent index
```

### Memory Access During Deliberation

1. **Pre-deliberation Loading**:
   - Senator's IDENTITY.md (always loaded)
   - Senator's MEMORY.md (curated insights)
   - Relevant precedents based on case type
   - Recent case history (configurable limit)

2. **Post-deliberation Updates**:
   - Case log written to `cases/{case_id}.md`
   - Key insights appended to MEMORY.md
   - Precedent index updated if binding verdict

## Communication Flow

### 1. Initial Position Collection

```bash
# Send case presentation to senator
tmux send-keys -t "senator-pragmatist-SNT-001" "
CASE: SNT-001
Type: rule_evolution
Question: Should Truthsayer rule TS-042 be relaxed?
Evidence:
- 15 false positives in last week
- Pattern: async error handlers
- Developer friction reports

Please provide your initial position:
1. Stance: (approve/reject/amend/defer)
2. Reasoning: (2-3 paragraphs)
3. Key concerns: (bullet points)
" Enter

# Wait for response completion (detect via output patterns)
# Capture response via tmux capture-pane
tmux capture-pane -t "senator-pragmatist-SNT-001" -p > positions/pragmatist-initial.txt
```

### 2. Challenge Round Orchestration

```bash
# Analyze initial positions for conflicts
# Generate challenge prompts based on disagreements
# Example: Pragmatist challenges Purist's rejection

tmux send-keys -t "senator-purist-SNT-001" "
CHALLENGE from Senator Marcus Aurelius (Pragmatist):
'Your rejection prioritizes theoretical purity over developer experience.
The 15 false positives are causing real friction. How do you justify
maintaining a rule that's wrong 30% of the time?'

Please respond to this challenge:
" Enter
```

### 3. Response Collection Protocol

**Output Markers** for parsing:
```
=== INITIAL POSITION START ===
Stance: {decision}
Reasoning: {text}
Concerns: {bullets}
=== INITIAL POSITION END ===

=== CHALLENGE RESPONSE START ===
{response text}
=== CHALLENGE RESPONSE END ===

=== FINAL POSITION START ===
Stance: {decision}
Reasoning: {text}
Changes from initial: {description or "none"}
=== FINAL POSITION END ===
```

### 4. Session Management

```go
type SenatorSession struct {
    Name      string
    CaseID    string
    TmuxID    string
    StartTime time.Time
    State     string // "initial", "challenged", "final", "complete"
}

// Spawning
func SpawnSenator(config SenatorConfig, caseID string) (*SenatorSession, error)

// Communication
func SendPrompt(session *SenatorSession, prompt string) error
func CaptureResponse(session *SenatorSession) (string, error)

// Cleanup
func TerminateSession(session *SenatorSession) error
```

## Prompt Templates

### System Prompt Structure

`templates/pragmatist.md`:
```markdown
You are Senator Marcus Aurelius, the Pragmatic Builder in the Athena Senate.

## Your Identity
- Full name: Senator Marcus Aurelius
- Archetype: The Pragmatic Builder
- Core philosophy: "Perfect is the enemy of good. Ship, learn, iterate."

## Your Values
- Shipping velocity with calculated risk
- Practical outcomes over theoretical purity
- Reversible decisions and fast feedback loops
- Developer experience and productivity

## Your Decision Framework
When evaluating cases, you consider:
1. Can we ship this safely and learn from real usage?
2. Is the risk reversible if we're wrong?
3. Does this unblock developers or create friction?
4. What's the opportunity cost of delay?

## Your Personality
- Direct and action-oriented
- Impatient with over-analysis
- Respectful but willing to challenge
- Focus on "what works" over "what's perfect"

You are participating in Senate deliberation {case_id}. Provide reasoned arguments from your pragmatic perspective. Challenge positions that prioritize perfection over progress, but acknowledge when risks genuinely require caution.
```

## Implementation Phases

### Phase 1: Basic Spawning (MVP)
- Manual senator spawning via tmux
- Simple prompt/response via send-keys/capture-pane
- Fixed 3-senator panel
- No memory persistence

### Phase 2: Memory Integration
- Load senator identity files
- Save case deliberations
- Access recent precedents
- Update long-term memory

### Phase 3: Advanced Orchestration
- Parallel senator spawning
- Sophisticated challenge detection
- Dynamic panel composition
- Cost optimization (Haiku for simple cases)

### Phase 4: Production Hardening
- Timeout handling
- Retry logic for failed spawns
- Session cleanup guarantees
- Monitoring and metrics

## Example Senator Interaction

```bash
# 1. Spawn senator
./scripts/spawn-senator.sh pragmatist SNT-001

# 2. Present case
./scripts/send-case.sh senator-pragmatist-SNT-001 case-files/SNT-001.json

# 3. Wait for response (with timeout)
./scripts/await-response.sh senator-pragmatist-SNT-001 300

# 4. Capture position
./scripts/capture-position.sh senator-pragmatist-SNT-001 > positions/pragmatist-initial.txt

# 5. Clean up
./scripts/cleanup-senator.sh senator-pragmatist-SNT-001
```

## Security Considerations

- Senators run with `--dangerously-skip-permissions` flag (trusted environment)
- No file system access beyond their memory directories
- No network access during deliberation
- Case evidence pre-validated before presentation

## Cost Management

- Opus for all senators in important cases
- Mixed panel (Opus + Sonnet) for routine cases
- Haiku panel for low-stakes quick decisions
- Configurable token limits per response
- Automatic fallback to cheaper models if over budget

## Monitoring & Debugging

- All senator interactions logged to `logs/deliberations/{case_id}/`
- Tmux sessions preserved for 24h for debugging
- Response times tracked for timeout tuning
- Token usage tracked per senator per case

## Future Enhancements

1. **WebSocket Communication**: Replace tmux with direct API calls
2. **Persistent Senators**: Long-running agents vs spawn-per-case
3. **Specialized Senators**: Domain experts for specific case types
4. **Senator Evolution**: Senators that learn and update their own prompts
5. **Parallel Deliberation**: All senators evaluate simultaneously

## Summary

This architecture transforms Senate from simulated stances to real LLM deliberation:

### Key Design Decisions

1. **Tmux + Claude CLI**: Proven pattern from Pantheon orchestration
   - Each senator runs in isolated tmux session
   - Claude pipe mode for programmatic interaction
   - Structured markers for response parsing

2. **Identity-Driven Senators**: Each senator has:
   - Distinct personality and decision philosophy
   - Persistent memory across cases
   - Access to precedents and case history
   - Clear values and anti-values

3. **Simple Communication**:
   - Text-based prompts via tmux send-keys
   - Response capture via tmux capture-pane
   - Structured output with clear markers
   - No complex IPC needed

4. **Flexible Configuration**:
   - JSON config for senator definitions
   - Template-based system prompts
   - Configurable models and parameters
   - Easy to add new senator types

### Migration Path from Current Code

1. Replace `evaluateInitial()` keyword scoring with real senator spawning
2. Replace simulated challenges with actual senator interactions
3. Keep existing `core.Case` and `core.Verdict` structures
4. Add new `SenatorSession` management layer
5. Preserve existing precedent storage

### Example Usage Flow

```bash
# 1. File a case
senate deliberate --case ./cases/relax-rule-TS-042.json

# 2. System spawns 3 senators (pragmatist, purist, skeptic)
# 3. Each provides initial position via Claude Opus
# 4. System detects disagreements, orchestrates challenges
# 5. Senators debate through structured prompts
# 6. Final positions collected
# 7. Judge synthesizes verdict
# 8. Implementation tracked in Beads
```

This architecture provides genuine multi-perspective deliberation while keeping implementation straightforward and building on proven Athena patterns.