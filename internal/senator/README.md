# Senator Spawning Implementation

This package implements real LLM senator agents for Senate deliberations, replacing the simulated keyword-based system.

## Architecture

Each senator:
- Runs in a dedicated tmux session
- Uses Claude Opus in pipe mode
- Has distinct identity and decision-making style
- Responds with structured output using markers

## Core Components

### 1. **session.go** - Senator Session Management
```go
// Spawn a senator
session, err := SpawnSenator(config, "CASE-001")

// Send a prompt and get response
response, err := session.SendPrompt("Case details...")

// Clean up
session.Close()
```

### 2. **config.go** - Configuration Loading
```go
// Load Senate configuration
config, err := LoadConfig("config/senate-config.json")

// Get specific senator
senatorConfig, err := config.GetSenatorByName("pragmatist")

// Get default panel
panel := config.GetDefaultPanel()
```

### 3. **integration.go** - Orchestration Helper
```go
// Create orchestrator for a case
orch := NewOrchestrator(config, "CASE-001")

// Spawn the panel
err := orch.SpawnPanel([]string{"pragmatist", "purist", "skeptic"})

// Collect initial positions
positions, err := orch.CollectInitialPositions(case)

// Clean up all sessions
orch.Close()
```

## Usage Example

```go
// 1. Load configuration
config, err := LoadConfig("")
if err != nil {
    log.Fatal(err)
}

// 2. Create orchestrator
orch := NewOrchestrator(config, "SNT-001")
defer orch.Close()

// 3. Spawn senators
err = orch.SpawnPanel(nil) // uses default panel
if err != nil {
    log.Fatal(err)
}

// 4. Present case and collect positions
positions, err := orch.CollectInitialPositions(myCase)
if err != nil {
    log.Fatal(err)
}

// 5. Process positions...
for senator, position := range positions {
    fmt.Printf("%s: %s\n", senator, position.Stance)
}
```

## Response Format

Senators respond with structured markers:

```
=== INITIAL POSITION START ===
Stance: approve
Reasoning: This change improves developer experience...
Concerns: - May reduce error visibility
         - Could hide real issues
=== INITIAL POSITION END ===
```

## Tmux Session Management

Sessions are created on a dedicated socket:
- Socket: `/tmp/tmux-senate.sock`
- Naming: `senator-{name}-{case_id}`
- View: `tmux -S /tmp/tmux-senate.sock attach -t senator-pragmatist-SNT-001`
- List: `tmux -S /tmp/tmux-senate.sock list-sessions`

## Configuration

See `config/senate-config-example.json` for the full schema. Key elements:

```json
{
  "senators": [{
    "name": "pragmatist",
    "model": "claude:opus",
    "identity": {
      "full_name": "Senator Marcus Aurelius",
      "archetype": "The Pragmatic Builder",
      "values": ["shipping velocity", "practical outcomes"]
    },
    "system_prompt_template": "templates/pragmatist.md"
  }]
}
```

## Testing

Run the example test to verify everything works:

```bash
cd $SENATE_ROOT  # defaults to $HOME/tools/senate
go test -v -run Example ./internal/senator
```

## Integration with Existing Engine

To replace the simulated deliberation in `internal/deliberation/engine.go`:

1. Replace `evaluateInitial()` with real senator spawning
2. Use `DeliberationOrchestrator` instead of simulated perspectives
3. Keep existing `core.Case` and `core.Verdict` structures
4. Parse real responses instead of keyword scoring

## Debugging

- Check tmux sessions: `tmux -S /tmp/tmux-senate.sock ls`
- Attach to senator: `tmux -S /tmp/tmux-senate.sock attach -t SESSION`
- View logs: Senator prompts are saved in temp dirs during execution
- Response timeout: Default 5 minutes, configurable

## Notes

- Requires Claude CLI with `--dangerously-skip-permissions` flag
- Each deliberation creates temp files for system prompts
- Sessions are automatically cleaned up on Close()
- Supports parallel position collection for efficiency