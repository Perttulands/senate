# Judge Synthesis Implementation

## Overview

The judge synthesis feature replaces the simulated verdict generation with real Claude Opus-based synthesis. The judge reviews all deliberation data and produces binding verdicts.

## Architecture

### Components

1. **JudgeSynthesize Function** (`judge.go`)
   - Main entry point for verdict synthesis
   - Takes case, positions, challenges, and produces verdict
   - Handles Claude CLI interaction

2. **ClaudeJudge Client**
   - Interfaces with Claude CLI
   - Manages temporary prompt files
   - Executes claude command with proper flags

3. **Prompt Generation**
   - System prompt defines judge role and framework
   - User prompt includes full deliberation context
   - Structured format for consistent parsing

4. **Response Parsing**
   - Extracts verdict sections from Claude's response
   - Maps text decisions to core.Decision enum
   - Provides fallback defaults for robustness

## Integration Points

### Engine Integration
```go
// In engine.go Deliberate() method
if _, cmdErr := exec.LookPath("claude"); cmdErr == nil {
    // Use real judge synthesis
    verdict, err = JudgeSynthesize(context.Background(), c, initial, challenges, final)
} else {
    // Fall back to simulated
    verdict = synthesizeVerdict(c, final, e.JudgeModel, started.Add(2*time.Minute))
}
```

### Fallback Behavior
- Checks for Claude CLI availability
- Falls back to simulated verdict if:
  - Claude CLI not found
  - Judge synthesis fails
- Logs errors to stderr for debugging

## Prompt Design

### System Prompt Elements
- Judge identity and authority
- Decision framework for each verdict type
- Output format requirements
- Guiding principles

### User Prompt Structure
1. Case presentation (ID, type, summary, question)
2. Evidence list
3. Initial positions from all senators
4. Challenges between senators
5. Final positions after deliberation
6. Clear task directive

## Verdict Schema

```go
type Verdict struct {
    CaseID         string
    Verdict        Decision      // approved/rejected/amended/deferred
    Reasoning      string        // 2-3 paragraphs
    Implementation string        // Actionable steps
    Dissent        string        // Minority concerns
    Binding        bool          // false only for deferred
    Judge          string        // "claude-opus"
    // ... other fields
}
```

## Usage Example

```go
// Given a deliberation transcript
verdict, err := JudgeSynthesize(
    ctx,
    caseData,
    initialPositions,
    challenges,
    finalPositions,
)
if err != nil {
    // Handle error or fall back
}

// Use verdict for implementation handoff
if verdict.Binding {
    beadID, err := handoff.CreateBeadFromVerdict(ctx, verdict)
}
```

## Claude CLI Requirements

### Installation
The system expects `claude` command to be available in PATH.

### Required Flags
- `--model opus`: Uses Claude Opus for best judgment
- `--system-prompt-file`: Judge role and framework
- `--message-file`: Deliberation context
- `--no-cache`: Ensures fresh synthesis

### Error Handling
- Command not found: Falls back to simulated
- Execution errors: Logged and falls back
- Parsing errors: Uses section extraction with defaults

## Testing

### Manual Testing
```bash
# Ensure Claude CLI is available
which claude

# Run a test deliberation
cd /home/chrote/athena/tools/senate
go run ./cmd/senate deliberate --case ./test-cases/example.json
```

### Automated Testing
- Mock the ClaudeJudge interface for unit tests
- Test parsing with various response formats
- Verify fallback behavior works correctly

## Future Enhancements

1. **Async Processing**
   - Non-blocking judge synthesis
   - Progress indicators during synthesis

2. **Response Caching**
   - Cache verdicts for identical deliberations
   - Useful for testing and demos

3. **Alternative Models**
   - Support for other judge models
   - A/B testing different judges

4. **Structured Output**
   - Request JSON format from Claude
   - More reliable parsing

5. **Verdict Templates**
   - Pre-structured verdict formats
   - Consistency across case types

## Monitoring

Track these metrics:
- Judge synthesis success rate
- Time to verdict
- Fallback frequency
- Token usage per verdict
- Verdict distribution (approve/reject/amend/defer)

## Security Considerations

- Temporary files are cleaned up after use
- No sensitive data persisted to disk
- Claude runs without network access
- Input sanitization for prompt injection