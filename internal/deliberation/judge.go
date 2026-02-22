package deliberation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Perttulands/senate/internal/core"
)

// JudgeClient interface for LLM interaction
type JudgeClient interface {
	SendPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// ClaudeJudge implements JudgeClient using Claude CLI
type ClaudeJudge struct {
	Model string
}

// SendPrompt sends a prompt to Claude and returns the response
func (c *ClaudeJudge) SendPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Create temporary files for system prompt and user prompt
	tmpDir := os.TempDir()
	systemFile := filepath.Join(tmpDir, fmt.Sprintf("senate-judge-system-%d.txt", time.Now().Unix()))
	userFile := filepath.Join(tmpDir, fmt.Sprintf("senate-judge-user-%d.txt", time.Now().Unix()))

	// Write prompts to files
	if err := os.WriteFile(systemFile, []byte(systemPrompt), 0644); err != nil {
		return "", fmt.Errorf("failed to write system prompt: %w", err)
	}
	defer os.Remove(systemFile)

	if err := os.WriteFile(userFile, []byte(userPrompt), 0644); err != nil {
		return "", fmt.Errorf("failed to write user prompt: %w", err)
	}
	defer os.Remove(userFile)

	// Execute claude command
	args := []string{
		"--model", c.Model,
		"--system-prompt-file", systemFile,
		"--message-file", userFile,
		"--no-cache",
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude command failed: %w\nOutput: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

// JudgeSynthesize creates a binding verdict by having Claude Opus synthesize the deliberation
func JudgeSynthesize(ctx context.Context, c core.Case, initialPositions []core.Position, challenges []core.Challenge, finalPositions []core.Position) (core.Verdict, error) {
	// Initialize the judge client
	judge := &ClaudeJudge{Model: "opus"}

	// Build comprehensive synthesis prompt
	systemPrompt := buildJudgeSystemPrompt()
	userPrompt := buildJudgeUserPrompt(c, initialPositions, challenges, finalPositions)

	// Send to Claude Opus for synthesis
	response, err := judge.SendPrompt(ctx, systemPrompt, userPrompt)
	if err != nil {
		return core.Verdict{}, fmt.Errorf("failed to get judge response: %w", err)
	}

	// Parse the response into a Verdict
	verdict, err := parseJudgeResponse(response, c, finalPositions)
	if err != nil {
		return core.Verdict{}, fmt.Errorf("failed to parse judge response: %w", err)
	}

	return verdict, nil
}

func buildJudgeSystemPrompt() string {
	return `You are the Honorable Judge of the Athena Senate, responsible for synthesizing binding verdicts from senator deliberations.

## Your Role
- Review all senator positions, challenges, and final stances
- Synthesize a clear, actionable verdict
- Provide implementation guidance when approving
- Summarize dissenting views fairly
- Focus on practical outcomes while maintaining system integrity

## Decision Framework
1. Analyze the progression from initial to final positions
2. Identify key points of convergence and remaining conflicts
3. Weigh evidence and reasoning quality, not just vote counts
4. Consider long-term implications and precedent setting
5. Default to safety (defer) when uncertainty is high

## Output Format
You must provide a structured verdict with these exact sections:

VERDICT: [approved/rejected/amended/deferred]
REASONING: [2-3 paragraphs explaining the decision based on the deliberation]
IMPLEMENTATION: [Specific steps for implementation if approved/amended, or next steps if rejected/deferred]
DISSENT: [Fair summary of minority positions and concerns that were not addressed by the verdict]

Be decisive but fair. Your verdict is binding and will guide implementation.`
}

func buildJudgeUserPrompt(c core.Case, initialPositions []core.Position, challenges []core.Challenge, finalPositions []core.Position) string {
	var prompt strings.Builder

	// Case presentation
	fmt.Fprintf(&prompt, "## CASE: %s\n", c.ID)
	fmt.Fprintf(&prompt, "Type: %s\n", c.Type)
	fmt.Fprintf(&prompt, "Summary: %s\n", c.Summary)
	fmt.Fprintf(&prompt, "Question: %s\n\n", c.Question)

	// Evidence
	if len(c.Evidence) > 0 {
		fmt.Fprintf(&prompt, "## EVIDENCE\n")
		for i, evidence := range c.Evidence {
			fmt.Fprintf(&prompt, "%d. %s\n", i+1, evidence)
		}
		fmt.Fprintf(&prompt, "\n")
	}

	// Initial positions
	fmt.Fprintf(&prompt, "## INITIAL POSITIONS\n\n")
	for _, pos := range initialPositions {
		fmt.Fprintf(&prompt, "### %s (%s)\n", pos.AgentID, pos.Perspective)
		fmt.Fprintf(&prompt, "Stance: %s\n", pos.Stance)
		fmt.Fprintf(&prompt, "Reasoning: %s\n", pos.Reasoning)
		if pos.Concerns != "" {
			fmt.Fprintf(&prompt, "Concerns: %s\n", pos.Concerns)
		}
		fmt.Fprintf(&prompt, "\n")
	}

	// Challenges
	if len(challenges) > 0 {
		fmt.Fprintf(&prompt, "## CHALLENGES\n\n")
		for _, ch := range challenges {
			fmt.Fprintf(&prompt, "%s → %s:\n", ch.From, ch.To)
			fmt.Fprintf(&prompt, "%s\n\n", ch.Challenge)
		}
	}

	// Final positions
	fmt.Fprintf(&prompt, "## FINAL POSITIONS\n\n")
	for _, pos := range finalPositions {
		fmt.Fprintf(&prompt, "### %s\n", pos.AgentID)
		fmt.Fprintf(&prompt, "Final Stance: %s\n", pos.Stance)
		fmt.Fprintf(&prompt, "Final Reasoning: %s\n\n", pos.Reasoning)
	}

	// Request
	fmt.Fprintf(&prompt, "## YOUR TASK\n")
	fmt.Fprintf(&prompt, "Synthesize a binding verdict based on this deliberation. ")
	fmt.Fprintf(&prompt, "Consider the quality of arguments, not just vote counts. ")
	fmt.Fprintf(&prompt, "Provide clear implementation guidance.\n\n")
	fmt.Fprintf(&prompt, "Please provide your verdict now.")

	return prompt.String()
}

func parseJudgeResponse(response string, c core.Case, finalPositions []core.Position) (core.Verdict, error) {
	// Extract structured sections from the response
	verdict := extractSection(response, "VERDICT:", "REASONING:")
	reasoning := extractSection(response, "REASONING:", "IMPLEMENTATION:")
	implementation := extractSection(response, "IMPLEMENTATION:", "DISSENT:")
	dissent := extractSection(response, "DISSENT:", "")

	// Map verdict text to Decision enum
	var decision core.Decision
	verdictLower := strings.ToLower(strings.TrimSpace(verdict))
	switch verdictLower {
	case "approved", "approve":
		decision = core.DecisionApprove
	case "rejected", "reject":
		decision = core.DecisionReject
	case "amended", "amend":
		decision = core.DecisionAmend
	case "deferred", "defer":
		decision = core.DecisionDefer
	default:
		// Try to extract from response if not in expected format
		if strings.Contains(strings.ToLower(response), "approve") {
			decision = core.DecisionApprove
		} else if strings.Contains(strings.ToLower(response), "reject") {
			decision = core.DecisionReject
		} else if strings.Contains(strings.ToLower(response), "amend") {
			decision = core.DecisionAmend
		} else {
			decision = core.DecisionDefer
		}
	}

	// Clean up the extracted text
	reasoning = strings.TrimSpace(reasoning)
	implementation = strings.TrimSpace(implementation)
	dissent = strings.TrimSpace(dissent)

	// Set defaults if parsing failed
	if reasoning == "" {
		reasoning = "The judge has reviewed all positions and challenges to reach this verdict."
	}
	if implementation == "" && decision != core.DecisionDefer {
		implementation = fmt.Sprintf("Implement as per the %s decision and document as precedent.", decision)
	}
	if implementation == "" && decision == core.DecisionDefer {
		implementation = "Gather additional information and re-file the case with more evidence."
	}

	// Build the verdict
	return core.Verdict{
		CaseID:         c.ID,
		FiledAt:        c.FiledAt,
		VerdictAt:      time.Now().UTC().Format(time.RFC3339),
		Type:           c.Type,
		Summary:        c.Summary,
		Verdict:        decision,
		Reasoning:      reasoning,
		Implementation: implementation,
		Dissent:        dissent,
		Binding:        decision != core.DecisionDefer,
		Judge:          "claude-opus",
		FinalPositions: finalPositions,
	}, nil
}

// extractSection extracts text between start marker and end marker (or end of string)
func extractSection(text, startMarker, endMarker string) string {
	startIdx := strings.Index(text, startMarker)
	if startIdx == -1 {
		return ""
	}

	// Move past the marker
	startIdx += len(startMarker)

	// Find the end
	endIdx := len(text)
	if endMarker != "" {
		if idx := strings.Index(text[startIdx:], endMarker); idx != -1 {
			endIdx = startIdx + idx
		}
	}

	return strings.TrimSpace(text[startIdx:endIdx])
}

// VerdictResponse represents the expected JSON structure from judge
type VerdictResponse struct {
	Verdict        string `json:"verdict"`
	Reasoning      string `json:"reasoning"`
	Implementation string `json:"implementation"`
	Dissent        string `json:"dissent"`
}

// parseJudgeResponseJSON attempts to parse JSON-formatted response (alternative implementation)
func parseJudgeResponseJSON(response string, c core.Case, finalPositions []core.Position) (core.Verdict, error) {
	var verdictResp VerdictResponse
	if err := json.Unmarshal([]byte(response), &verdictResp); err != nil {
		// Fall back to text parsing
		return parseJudgeResponse(response, c, finalPositions)
	}

	// Map verdict string to Decision
	var decision core.Decision
	switch strings.ToLower(verdictResp.Verdict) {
	case "approved", "approve":
		decision = core.DecisionApprove
	case "rejected", "reject":
		decision = core.DecisionReject
	case "amended", "amend":
		decision = core.DecisionAmend
	case "deferred", "defer":
		decision = core.DecisionDefer
	default:
		return core.Verdict{}, fmt.Errorf("invalid verdict: %s", verdictResp.Verdict)
	}

	return core.Verdict{
		CaseID:         c.ID,
		FiledAt:        c.FiledAt,
		VerdictAt:      time.Now().UTC().Format(time.RFC3339),
		Type:           c.Type,
		Summary:        c.Summary,
		Verdict:        decision,
		Reasoning:      verdictResp.Reasoning,
		Implementation: verdictResp.Implementation,
		Dissent:        verdictResp.Dissent,
		Binding:        decision != core.DecisionDefer,
		Judge:          "claude-opus",
		FinalPositions: finalPositions,
	}, nil
}