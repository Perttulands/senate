package senator

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Perttulands/senate/internal/core"
)

// DeliberationOrchestrator manages multiple senator sessions for a deliberation
type DeliberationOrchestrator struct {
	Config    *Config
	CaseID    string
	Sessions  map[string]*SenatorSession
	mu        sync.Mutex
}

// NewOrchestrator creates a new deliberation orchestrator
func NewOrchestrator(config *Config, caseID string) *DeliberationOrchestrator {
	return &DeliberationOrchestrator{
		Config:   config,
		CaseID:   caseID,
		Sessions: make(map[string]*SenatorSession),
	}
}

// SpawnPanel spawns the senator panel for deliberation
func (o *DeliberationOrchestrator) SpawnPanel(senatorNames []string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// If no specific senators requested, use default panel
	if len(senatorNames) == 0 {
		panel := o.Config.GetDefaultPanel()
		senatorNames = make([]string, len(panel))
		for i, s := range panel {
			senatorNames[i] = s.Name
		}
	}

	// Spawn each senator
	for _, name := range senatorNames {
		senatorConfig, err := o.Config.GetSenatorByName(name)
		if err != nil {
			return fmt.Errorf("get senator %s config: %w", name, err)
		}

		session, err := SpawnSenator(senatorConfig, o.CaseID)
		if err != nil {
			// Clean up any already spawned sessions
			o.cleanupSessions()
			return fmt.Errorf("spawn senator %s: %w", name, err)
		}

		o.Sessions[name] = session
	}

	return nil
}

// CollectInitialPositions sends the case and collects initial positions
func (o *DeliberationOrchestrator) CollectInitialPositions(c core.Case) (map[string]core.Position, error) {
	// Format case presentation
	casePresentation := o.formatCasePresentation(c)

	positions := make(map[string]core.Position)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make(chan error, len(o.Sessions))

	// Collect positions in parallel
	for name, session := range o.Sessions {
		wg.Add(1)
		go func(senatorName string, sess *SenatorSession) {
			defer wg.Done()

			response, err := sess.SendPrompt(casePresentation)
			if err != nil {
				errors <- fmt.Errorf("senator %s: %w", senatorName, err)
				return
			}

			position, err := o.parsePosition(response, senatorName, "initial")
			if err != nil {
				errors <- fmt.Errorf("parse %s position: %w", senatorName, err)
				return
			}

			mu.Lock()
			positions[senatorName] = position
			mu.Unlock()
		}(name, session)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errs []string
	for err := range errors {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to collect positions: %s", strings.Join(errs, "; "))
	}

	return positions, nil
}

// SendChallenge sends a challenge from one senator to another
func (o *DeliberationOrchestrator) SendChallenge(targetSenator, fromSenator, challenge string) (string, error) {
	session, ok := o.Sessions[targetSenator]
	if !ok {
		return "", fmt.Errorf("senator %s not found", targetSenator)
	}

	prompt := fmt.Sprintf(`CHALLENGE from %s:
"%s"

Please respond to this challenge:`, fromSenator, challenge)

	return session.SendPrompt(prompt)
}

// Close terminates all senator sessions
func (o *DeliberationOrchestrator) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.cleanupSessions()
}

// cleanupSessions closes all active sessions
func (o *DeliberationOrchestrator) cleanupSessions() {
	for _, session := range o.Sessions {
		session.Close()
	}
	o.Sessions = make(map[string]*SenatorSession)
}

// formatCasePresentation formats a case for senator consumption
func (o *DeliberationOrchestrator) formatCasePresentation(c core.Case) string {
	var evidence strings.Builder
	for _, e := range c.Evidence {
		evidence.WriteString(fmt.Sprintf("- %s\n", e))
	}

	return fmt.Sprintf(`=== CASE PRESENTATION START ===
Case ID: %s
Type: %s
Filed: %s

Summary: %s

Question: %s

Evidence:
%s
Requested Decision: %s
=== CASE PRESENTATION END ===

Please provide your initial position on this case.`,
		c.ID, c.Type, c.FiledAt,
		c.Summary, c.Question,
		evidence.String(),
		c.RequestedDecision,
	)
}

// parsePosition extracts a Position from senator response
func (o *DeliberationOrchestrator) parsePosition(response, senatorName, round string) (core.Position, error) {
	// Extract content between markers
	if !strings.Contains(response, "Stance:") {
		return core.Position{}, fmt.Errorf("no stance found in response")
	}

	// Simple parsing - in production would be more robust
	lines := strings.Split(response, "\n")
	var stance, reasoning, concerns string
	var inReasoning, inConcerns bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "Stance:") {
			stanceStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "Stance:"))
			stance = normalizeStance(stanceStr)
		} else if strings.HasPrefix(trimmed, "Reasoning:") {
			inReasoning = true
			inConcerns = false
			reasoning = strings.TrimSpace(strings.TrimPrefix(trimmed, "Reasoning:"))
		} else if strings.HasPrefix(trimmed, "Concerns:") {
			inReasoning = false
			inConcerns = true
			concerns = strings.TrimSpace(strings.TrimPrefix(trimmed, "Concerns:"))
		} else if inReasoning && trimmed != "" && !strings.Contains(trimmed, "===") {
			reasoning += " " + trimmed
		} else if inConcerns && trimmed != "" && !strings.Contains(trimmed, "===") {
			concerns += " " + trimmed
		}
	}

	// Get senator config for metadata
	senatorConfig, _ := o.Config.GetSenatorByName(senatorName)

	return core.Position{
		AgentID:     fmt.Sprintf("senator-%s", senatorName),
		Model:       senatorConfig.Model,
		Perspective: senatorName,
		Round:       round,
		Stance:      core.Decision(stance),
		Reasoning:   strings.TrimSpace(reasoning),
		Concerns:    strings.TrimSpace(concerns),
	}, nil
}

// normalizeStance converts various stance formats to core.Decision
func normalizeStance(stance string) string {
	s := strings.ToLower(strings.TrimSpace(stance))

	// Map common variations
	switch {
	case strings.Contains(s, "approve"):
		return string(core.DecisionApprove)
	case strings.Contains(s, "reject"):
		return string(core.DecisionReject)
	case strings.Contains(s, "amend"):
		return string(core.DecisionAmend)
	case strings.Contains(s, "defer"):
		return string(core.DecisionDefer)
	default:
		return s
	}
}