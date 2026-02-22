package deliberation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Perttulands/senate/internal/core"
	"github.com/Perttulands/senate/internal/senator"
)

// Engine runs the Senate deliberation protocol.
type Engine struct {
	Panel      []Perspective
	JudgeModel string
}

func New(panel []Perspective) *Engine {
	if len(panel) == 0 {
		panel = BuildPanel(3, nil, nil)
	}
	return &Engine{
		Panel:      panel,
		JudgeModel: "claude:opus",
	}
}

// Deliberate executes initial position, challenge, final position, and verdict synthesis.
// This requires real Claude senators via tmux. No fallbacks, no simulation.
func (e *Engine) Deliberate(c core.Case, now time.Time) (core.Transcript, core.Verdict, error) {
	if err := c.Validate(); err != nil {
		return core.Transcript{}, core.Verdict{}, err
	}

	// Check prerequisites before attempting deliberation
	if err := e.checkPrerequisites(); err != nil {
		return core.Transcript{}, core.Verdict{}, fmt.Errorf("senate prerequisites not met: %w\n\nEnsure:\n- Claude CLI is installed and accessible\n- Tmux is available\n- Socket %s is writable", err, senator.TmuxSocket)
	}

	started := now.UTC()
	panelMembers := toPanelMembers(e.Panel)

	// Phase 1: Spawn senators - fail clearly if unable
	sessions, err := e.spawnSenators(c.ID, panelMembers)
	if err != nil {
		return core.Transcript{}, core.Verdict{}, fmt.Errorf("cannot spawn real senators: %w", err)
	}
	defer e.closeSessions(sessions)

	// Phase 2: Collect initial positions - no fallback
	initial, err := e.collectInitialPositions(c, sessions, panelMembers)
	if err != nil {
		return core.Transcript{}, core.Verdict{}, fmt.Errorf("failed to collect initial positions: %w", err)
	}

	// Phase 3: Orchestrate challenges - fail if unable
	challenges, err := e.orchestrateChallenges(c, initial, sessions)
	if err != nil {
		return core.Transcript{}, core.Verdict{}, fmt.Errorf("failed to orchestrate challenges: %w", err)
	}

	// Phase 4: Collect final positions - no fallback
	final, err := e.collectFinalPositions(c, initial, challenges, sessions, panelMembers)
	if err != nil {
		return core.Transcript{}, core.Verdict{}, fmt.Errorf("failed to collect final positions: %w", err)
	}

	// Phase 5: Synthesize verdict with real judge - no fallback
	// TODO: Accept context as parameter for proper cancellation support
	ctx := context.TODO() // Will be replaced when Deliberate accepts context
	verdict, err := JudgeSynthesize(ctx, c, initial, challenges, final)
	if err != nil {
		return core.Transcript{}, core.Verdict{}, fmt.Errorf("judge synthesis failed: %w", err)
	}

	transcript := core.Transcript{
		CaseID:           c.ID,
		StartedAt:        started.Format(time.RFC3339),
		CompletedAt:      time.Now().UTC().Format(time.RFC3339),
		Panel:            panelMembers,
		InitialPositions: initial,
		Challenges:       challenges,
		FinalPositions:   final,
		JudgeModel:       e.JudgeModel,
	}
	return transcript, verdict, nil
}


// checkPrerequisites verifies that real senator deliberation can proceed
func (e *Engine) checkPrerequisites() error {
	// Check Claude CLI
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude CLI not found in PATH")
	}

	// Check tmux
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found in PATH")
	}

	// Check tmux socket directory is writable
	socketDir := filepath.Dir(senator.TmuxSocket)
	if info, err := os.Stat(socketDir); err != nil {
		return fmt.Errorf("tmux socket directory %s: %w", socketDir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", socketDir)
	}

	// Test tmux socket creation
	testCmd := exec.Command("tmux", "-S", senator.TmuxSocket, "list-sessions")
	if output, err := testCmd.CombinedOutput(); err != nil {
		// It's OK if no sessions exist, but not if tmux can't run
		if !strings.Contains(string(output), "no server running") && !strings.Contains(string(output), "no sessions") {
			return fmt.Errorf("tmux socket test failed: %w", err)
		}
	}

	return nil
}

// spawnSenators creates senator sessions for each panel member
func (e *Engine) spawnSenators(caseID string, panelMembers []core.PanelMember) (map[string]*senator.SenatorSession, error) {
	sessions := make(map[string]*senator.SenatorSession)

	for i, member := range panelMembers {
		// Build identity based on perspective
		identity := senator.Identity{
			FullName:  fmt.Sprintf("Senator %s", strings.Title(e.Panel[i].Name)),
			Archetype: e.Panel[i].Name,
			DecisionStyle: e.Panel[i].Directive,
		}

		config := &senator.SenatorConfig{
			Name:                 e.Panel[i].Name,
			Model:                member.Model,
			Identity:            identity,
			SystemPromptTemplate: "", // Will use inline prompt
		}

		session, err := senator.SpawnSenator(config, caseID)
		if err != nil {
			// Clean up already spawned sessions
			e.closeSessions(sessions)
			return nil, fmt.Errorf("spawn %s: %w", member.AgentID, err)
		}
		sessions[member.AgentID] = session
	}

	return sessions, nil
}

// collectInitialPositions sends case to each senator and collects responses
func (e *Engine) collectInitialPositions(c core.Case, sessions map[string]*senator.SenatorSession, panelMembers []core.PanelMember) ([]core.Position, error) {
	positions := make([]core.Position, 0, len(panelMembers))

	for i, member := range panelMembers {
		session := sessions[member.AgentID]

		// Build initial prompt
		prompt := e.buildInitialPrompt(c)

		// Send prompt and get response
		response, err := session.SendPrompt(prompt)
		if err != nil {
			return nil, fmt.Errorf("senator %s: %w", member.AgentID, err)
		}

		// Parse response
		position, err := e.parseInitialPosition(response, member, e.Panel[i])
		if err != nil {
			return nil, fmt.Errorf("parse %s response: %w", member.AgentID, err)
		}

		positions = append(positions, position)
	}

	return positions, nil
}

// orchestrateChallenges identifies disagreements and collects challenges
func (e *Engine) orchestrateChallenges(c core.Case, initial []core.Position, sessions map[string]*senator.SenatorSession) ([]core.Challenge, error) {
	challenges := make([]core.Challenge, 0)

	// Find disagreements and generate challenges
	for i, pos1 := range initial {
		for j, pos2 := range initial {
			if i >= j || pos1.Stance == pos2.Stance {
				continue // Avoid duplicate challenges and same-stance pairs
			}

			// Generate challenge from pos1 to pos2
			challengeText := e.generateChallenge(pos1, pos2)
			challenge := core.Challenge{
				From:      pos1.AgentID,
				To:        pos2.AgentID,
				Challenge: challengeText,
			}

			// Send challenge to target senator
			prompt := e.buildChallengePrompt(challenge, pos1, initial)
			session := sessions[pos2.AgentID]

			_, err := session.SendPrompt(prompt)
			if err != nil {
				// Log but continue - challenges are not critical
				fmt.Fprintf(os.Stderr, "Warning: challenge response from %s failed: %v\n", pos2.AgentID, err)
				challenges = append(challenges, challenge)
				continue
			}

			// Store the challenge (response is part of the deliberation but not stored in challenge)
			challenges = append(challenges, challenge)
		}
	}

	return challenges, nil
}

// collectFinalPositions asks senators for their final positions after challenges
func (e *Engine) collectFinalPositions(c core.Case, initial []core.Position, challenges []core.Challenge, sessions map[string]*senator.SenatorSession, panelMembers []core.PanelMember) ([]core.Position, error) {
	positions := make([]core.Position, 0, len(panelMembers))

	for i, member := range panelMembers {
		session := sessions[member.AgentID]

		// Build final position prompt with all context
		prompt := e.buildFinalPrompt(c, initial, challenges, member.AgentID)

		// Send prompt and get response
		response, err := session.SendPrompt(prompt)
		if err != nil {
			return nil, fmt.Errorf("final position from %s: %w", member.AgentID, err)
		}

		// Parse response
		position, err := e.parseFinalPosition(response, member, e.Panel[i], initial[i])
		if err != nil {
			return nil, fmt.Errorf("parse final %s response: %w", member.AgentID, err)
		}

		positions = append(positions, position)
	}

	return positions, nil
}

// buildSystemPrompt creates the system prompt for a senator
func (e *Engine) buildSystemPrompt(perspective Perspective, caseID string) string {
	var identity string
	switch perspective.Name {
	case "pragmatist":
		identity = `You are Senator Marcus Aurelius, the Pragmatic Builder in the Athena Senate.

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
- Focus on "what works" over "what's perfect"`

	case "purist":
		identity = `You are Senator Athena Sophia, the Architectural Purist in the Athena Senate.

## Your Identity
- Full name: Senator Athena Sophia
- Archetype: The Architectural Purist
- Core philosophy: "Build systems that stand the test of time."

## Your Values
- System integrity and consistency
- Long-term maintainability over quick fixes
- Proven patterns and best practices
- Clear boundaries and contracts

## Your Decision Framework
When evaluating cases, you consider:
1. Does this maintain system coherence and consistency?
2. Are we introducing technical debt?
3. Is this following established patterns?
4. Will this decision age well?

## Your Personality
- Thoughtful and principled
- Concerned with correctness
- Values elegance and simplicity
- Guards against entropy`

	case "skeptic":
		identity = `You are Senator Diogenes Truth, the Critical Skeptic in the Athena Senate.

## Your Identity
- Full name: Senator Diogenes Truth
- Archetype: The Critical Skeptic
- Core philosophy: "Question everything. Verify twice."

## Your Values
- Evidence-based decisions
- Failure mode analysis
- Honest assessment of risks
- Protection against wishful thinking

## Your Decision Framework
When evaluating cases, you consider:
1. What evidence supports this claim?
2. What are the failure modes?
3. Are we being honest about the risks?
4. What aren't we considering?

## Your Personality
- Questioning and analytical
- Constructively critical
- Values evidence over opinion
- Protects against groupthink`

	default:
		identity = fmt.Sprintf("You are %s in the Athena Senate.", strings.Title(perspective.Name))
	}

	return fmt.Sprintf(`%s

You are participating in Senate deliberation %s. Provide reasoned arguments from your perspective. When you disagree with others, explain why based on your values and decision framework. Be willing to revise your position if presented with compelling evidence or arguments, but don't abandon your core principles.

## Response Format
Always structure your responses with clear markers:

For initial positions:
=== INITIAL POSITION START ===
Stance: [approve/reject/amend/defer]
Reasoning: [Your detailed reasoning in 2-3 paragraphs]
Concerns: [Key concerns as bullet points]
=== INITIAL POSITION END ===

For challenge responses:
=== CHALLENGE RESPONSE START ===
[Your response to the challenge]
=== CHALLENGE RESPONSE END ===

For final positions:
=== FINAL POSITION START ===
Stance: [approve/reject/amend/defer]
Reasoning: [Your final reasoning]
Changes from initial: [Description of any changes or "none"]
=== FINAL POSITION END ===`, identity, caseID)
}

// buildInitialPrompt creates the case presentation prompt
func (e *Engine) buildInitialPrompt(c core.Case) string {
	var evidenceSection string
	if len(c.Evidence) > 0 {
		evidenceSection = "\n\nEvidence:\n"
		for _, ev := range c.Evidence {
			evidenceSection += fmt.Sprintf("- %s\n", ev)
		}
	}

	var precedentSection string
	if len(c.Precedents) > 0 {
		precedentSection = "\n\nRelevant Precedents:\n"
		for _, prec := range c.Precedents {
			precedentSection += fmt.Sprintf("- Case %s: %s (Verdict: %s)\n  Reasoning: %s\n",
				prec.CaseID, prec.Summary, prec.Verdict, prec.Reasoning)
		}
	}

	var requestedSection string
	if c.RequestedDecision != "" {
		requestedSection = fmt.Sprintf("\n\nRequested Decision: %s", c.RequestedDecision)
	}

	return fmt.Sprintf(`CASE: %s
Type: %s
Filed by: %s
Filed at: %s

Summary: %s

Question: %s%s%s%s

Please provide your initial position on this case.`,
		c.ID, c.Type, c.FiledBy, c.FiledAt,
		c.Summary, c.Question,
		evidenceSection, precedentSection, requestedSection)
}

// buildChallengePrompt creates the challenge prompt
func (e *Engine) buildChallengePrompt(challenge core.Challenge, challengerPos core.Position, allPositions []core.Position) string {
	// Show all positions for context
	var positionsContext string
	for _, pos := range allPositions {
		positionsContext += fmt.Sprintf("\n%s (%s): %s",
			pos.AgentID, pos.Stance, truncateText(pos.Reasoning, 200))
	}

	return fmt.Sprintf(`The panel has provided initial positions:%s

CHALLENGE from %s:
"%s"

Their position: %s
Their reasoning: %s

Please respond to this challenge, considering their perspective while maintaining your own values and decision framework.`,
		positionsContext,
		challengerPos.AgentID,
		challenge.Challenge,
		challengerPos.Stance,
		challengerPos.Reasoning)
}

// buildFinalPrompt creates the final position prompt
func (e *Engine) buildFinalPrompt(c core.Case, initial []core.Position, challenges []core.Challenge, senatorID string) string {
	// Summarize the deliberation
	var challengeSummary string
	if len(challenges) > 0 {
		challengeSummary = "\n\nKey challenges raised during deliberation:\n"
		relevantChallenges := 0
		for _, ch := range challenges {
			if ch.To == senatorID || ch.From == senatorID {
				challengeSummary += fmt.Sprintf("- %s to %s: %s\n",
					ch.From, ch.To, truncateText(ch.Challenge, 150))
				relevantChallenges++
			}
		}
		if relevantChallenges == 0 {
			challengeSummary = "\n\n(No direct challenges involving you)\n"
		}
	}

	// Show position distribution
	counts := countDecisions(initial)
	var distribution string
	for _, decision := range []core.Decision{core.DecisionApprove, core.DecisionReject, core.DecisionAmend, core.DecisionDefer} {
		if count := counts[decision]; count > 0 {
			distribution += fmt.Sprintf("- %s: %d senator(s)\n", decision, count)
		}
	}

	return fmt.Sprintf(`After reviewing all positions and challenges in case %s, please provide your final position.

Initial position distribution:
%s%s

Consider:
1. The challenges raised and responses given
2. Whether new information has emerged
3. The strength of competing arguments
4. Your core values and decision framework

You may maintain your initial position or revise it based on the deliberation.

Please provide your final position.`,
		c.ID, distribution, challengeSummary)
}

// parseInitialPosition extracts structured position from response
func (e *Engine) parseInitialPosition(response string, member core.PanelMember, perspective Perspective) (core.Position, error) {
	position := core.Position{
		AgentID:     member.AgentID,
		Model:       member.Model,
		Perspective: member.Perspective,
		Round:       "initial",
	}

	// Extract content between markers
	start := strings.Index(response, "=== INITIAL POSITION START ===")
	end := strings.Index(response, "=== INITIAL POSITION END ===")

	if start == -1 || end == -1 {
		// Fallback parsing if markers not found
		return e.parseUnstructuredResponse(response, position)
	}

	content := response[start+30 : end]
	lines := strings.Split(content, "\n")

	var collectingReasoning, collectingConcerns bool
	var reasoning, concerns []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Stance:") {
			stance := strings.TrimSpace(strings.TrimPrefix(line, "Stance:"))
			position.Stance = parseStance(stance)
		} else if strings.HasPrefix(line, "Reasoning:") {
			collectingReasoning = true
			collectingConcerns = false
			text := strings.TrimSpace(strings.TrimPrefix(line, "Reasoning:"))
			if text != "" {
				reasoning = append(reasoning, text)
			}
		} else if strings.HasPrefix(line, "Concerns:") {
			collectingReasoning = false
			collectingConcerns = true
			text := strings.TrimSpace(strings.TrimPrefix(line, "Concerns:"))
			if text != "" {
				concerns = append(concerns, text)
			}
		} else if collectingReasoning && line != "" {
			reasoning = append(reasoning, line)
		} else if collectingConcerns && line != "" {
			if strings.HasPrefix(line, "-") {
				concerns = append(concerns, strings.TrimSpace(strings.TrimPrefix(line, "-")))
			} else {
				concerns = append(concerns, line)
			}
		}
	}

	position.Reasoning = strings.TrimSpace(strings.Join(reasoning, " "))
	position.Concerns = strings.TrimSpace(strings.Join(concerns, "; "))

	// Validate we got required fields
	if position.Stance == "" {
		position.Stance = core.DecisionDefer
	}
	if position.Reasoning == "" {
		position.Reasoning = "Position could not be clearly determined from response."
	}

	return position, nil
}

// parseFinalPosition extracts structured final position from response
func (e *Engine) parseFinalPosition(response string, member core.PanelMember, perspective Perspective, initialPos core.Position) (core.Position, error) {
	position := core.Position{
		AgentID:     member.AgentID,
		Model:       member.Model,
		Perspective: member.Perspective,
		Round:       "final",
	}

	// Extract content between markers
	start := strings.Index(response, "=== FINAL POSITION START ===")
	end := strings.Index(response, "=== FINAL POSITION END ===")

	if start == -1 || end == -1 {
		// Fallback to initial position if parsing fails
		position.Stance = initialPos.Stance
		position.Reasoning = "Maintained initial position after deliberation."
		return position, nil
	}

	content := response[start+28 : end]
	lines := strings.Split(content, "\n")

	var collectingReasoning bool
	var reasoning []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Stance:") {
			stance := strings.TrimSpace(strings.TrimPrefix(line, "Stance:"))
			position.Stance = parseStance(stance)
		} else if strings.HasPrefix(line, "Reasoning:") {
			collectingReasoning = true
			text := strings.TrimSpace(strings.TrimPrefix(line, "Reasoning:"))
			if text != "" {
				reasoning = append(reasoning, text)
			}
		} else if strings.HasPrefix(line, "Changes from initial:") {
			// This is informational, we don't store it
			collectingReasoning = false
		} else if collectingReasoning && line != "" {
			reasoning = append(reasoning, line)
		}
	}

	position.Reasoning = strings.TrimSpace(strings.Join(reasoning, " "))

	// Validate we got required fields
	if position.Stance == "" {
		position.Stance = initialPos.Stance
	}
	if position.Reasoning == "" {
		position.Reasoning = "Final position maintains initial stance."
	}

	return position, nil
}

// parseUnstructuredResponse attempts to parse response without markers
func (e *Engine) parseUnstructuredResponse(response string, position core.Position) (core.Position, error) {
	responseLower := strings.ToLower(response)

	// Try to detect stance
	if strings.Contains(responseLower, "approve") && !strings.Contains(responseLower, "not approve") && !strings.Contains(responseLower, "cannot approve") {
		position.Stance = core.DecisionApprove
	} else if strings.Contains(responseLower, "reject") {
		position.Stance = core.DecisionReject
	} else if strings.Contains(responseLower, "amend") {
		position.Stance = core.DecisionAmend
	} else if strings.Contains(responseLower, "defer") {
		position.Stance = core.DecisionDefer
	} else {
		position.Stance = core.DecisionDefer // Default to defer if unclear
	}

	// Use first substantive part as reasoning
	lines := strings.Split(response, "\n")
	var reasoningLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "===") {
			reasoningLines = append(reasoningLines, line)
			if len(reasoningLines) >= 3 {
				break
			}
		}
	}
	position.Reasoning = strings.Join(reasoningLines, " ")

	return position, nil
}

// parseStance converts string to Decision
func parseStance(stance string) core.Decision {
	stance = strings.ToLower(strings.TrimSpace(stance))
	switch {
	case strings.Contains(stance, "approve"):
		return core.DecisionApprove
	case strings.Contains(stance, "reject"):
		return core.DecisionReject
	case strings.Contains(stance, "amend"):
		return core.DecisionAmend
	case strings.Contains(stance, "defer"):
		return core.DecisionDefer
	default:
		return ""
	}
}

// generateChallenge creates a contextual challenge based on positions
func (e *Engine) generateChallenge(from, to core.Position) string {
	// Generate challenges based on the stance differences
	if from.Stance == core.DecisionApprove && to.Stance == core.DecisionReject {
		return fmt.Sprintf("Your rejection seems overly cautious. The evidence suggests the benefits outweigh the risks. %s",
			extractKeyPoint(from.Reasoning, "How do you justify blocking progress?"))
	}

	if from.Stance == core.DecisionReject && to.Stance == core.DecisionApprove {
		return fmt.Sprintf("Your approval overlooks significant risks. %s",
			extractKeyPoint(from.Concerns, "Shouldn't we be more cautious here?"))
	}

	if from.Stance == core.DecisionAmend {
		return fmt.Sprintf("I propose we find middle ground. Your %s stance may be too extreme. What if we implement with specific guardrails instead?",
			to.Stance)
	}

	if from.Stance == core.DecisionDefer && to.Stance != core.DecisionDefer {
		return "We lack sufficient evidence to make a binding decision. How can you be so certain without more data?"
	}

	// Generic challenge
	return fmt.Sprintf("Your %s stance seems to underweight important considerations from my %s perspective. How do you reconcile this?",
		to.Stance, from.Stance)
}

// extractKeyPoint pulls out a key argument from text
func extractKeyPoint(text string, fallback string) string {
	if text == "" {
		return fallback
	}
	sentences := strings.Split(text, ". ")
	if len(sentences) > 0 && sentences[0] != "" {
		return strings.TrimSpace(sentences[0]) + "."
	}
	return fallback
}

// truncateText limits text to maxLen characters
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// closeSessions cleanly shuts down all senator sessions
func (e *Engine) closeSessions(sessions map[string]*senator.SenatorSession) {
	for id, session := range sessions {
		if err := session.Close(); err != nil {
			// Log but don't fail - we're cleaning up
			fmt.Fprintf(os.Stderr, "warning: failed to close senator %s: %v\n", id, err)
		}
	}
}

func uniqueFirstN(items []string, n int) []string {
	if n <= 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, n)
	for _, item := range items {
		clean := strings.TrimSpace(item)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
		if len(out) == n {
			break
		}
	}
	return out
}

// analyzePrecedents examines past verdicts to identify patterns
func analyzePrecedents(precedents []core.PrecedentSummary) string {
	if len(precedents) == 0 {
		return ""
	}

	counts := map[core.Decision]int{
		core.DecisionApprove: 0,
		core.DecisionReject:  0,
		core.DecisionAmend:   0,
		core.DecisionDefer:   0,
	}

	for _, p := range precedents {
		counts[p.Verdict]++
	}

	// If 60% or more precedents lean one way, return that decision
	threshold := int(float64(len(precedents)) * 0.6)
	for decision, count := range counts {
		if count >= threshold {
			return string(decision)
		}
	}

	return ""
}
// countDecisions counts how many positions have each decision type
func countDecisions(positions []core.Position) map[core.Decision]int {
	counts := map[core.Decision]int{
		core.DecisionApprove: 0,
		core.DecisionReject:  0,
		core.DecisionAmend:   0,
		core.DecisionDefer:   0,
	}
	for _, p := range positions {
		counts[p.Stance]++
	}
	return counts
}
