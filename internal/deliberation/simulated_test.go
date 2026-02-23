package deliberation

import (
	"fmt"
	"strings"
	"time"

	"github.com/Perttulands/senate/internal/core"
)

// simulatedDeliberate provides test-only simulated deliberation
// This is ONLY for unit tests, NOT for production use
func simulatedDeliberate(e *Engine, c core.Case, started time.Time, panelMembers []core.PanelMember) (core.Transcript, core.Verdict, error) {
	initial := make([]core.Position, 0, len(panelMembers))
	for i, seat := range panelMembers {
		p := e.Panel[i]
		stance, reason, concerns := evaluateInitialSimulated(c, p)
		initial = append(initial, core.Position{
			AgentID:     seat.AgentID,
			Model:       seat.Model,
			Perspective: seat.Perspective,
			Round:       "initial",
			Stance:      stance,
			Reasoning:   reason,
			Concerns:    concerns,
		})
	}

	challenges := buildChallengesSimulated(c, initial)
	final := finalizePositionsSimulated(c, initial, challenges)
	verdict := synthesizeVerdictSimulated(c, final, e.JudgeModel, time.Now().UTC())

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

// evaluateInitialSimulated generates test positions based on keywords
func evaluateInitialSimulated(c core.Case, p Perspective) (core.Decision, string, string) {
	risk := tokenScore(c.Question+" "+c.Summary, []string{"security", "unsafe", "drop", "delete", "disable", "bypass", "without tests", "rollback"})
	urgency := tokenScore(c.Question+" "+c.Summary, []string{"urgent", "blocker", "ship", "today", "immediately", "unblock"})
	evidenceWeight := len(c.Evidence)

	switch p.Name {
	case "pragmatist":
		if risk >= 2 {
			return core.DecisionReject, "TEST: The change introduces high risk compared to delivery value.", "TEST: Risk reduction plan is missing."
		}
		if urgency >= 1 || evidenceWeight >= 2 {
			return core.DecisionApprove, "TEST: The path is actionable now and clears immediate delivery constraints.", "TEST: Document rollback and ownership."
		}
		return core.DecisionAmend, "TEST: Direction is viable but needs tighter scope before execution.", "TEST: Define measurable acceptance criteria."
	case "purist":
		if risk >= 1 {
			return core.DecisionReject, "TEST: Correctness and safety guarantees are not strong enough for approval.", "TEST: Failure modes are under-specified."
		}
		if evidenceWeight == 0 {
			return core.DecisionDefer, "TEST: There is not enough evidence to make a durable decision.", "TEST: Need concrete examples or data."
		}
		return core.DecisionAmend, "TEST: The proposal is directionally sound but requires stronger invariants.", "TEST: Specify exact rule boundaries."
	case "skeptic":
		if evidenceWeight == 0 {
			return core.DecisionDefer, "TEST: The case lacks objective evidence and should not be bound yet.", "TEST: Gather incidents, diffs, or metrics first."
		}
		if risk >= 1 {
			return core.DecisionReject, "TEST: Edge-case risk remains unresolved under realistic failure scenarios.", "TEST: Mitigations are implied but not explicit."
		}
		return core.DecisionAmend, "TEST: Adopt with guardrails to contain unknowns.", "TEST: Time-box follow-up validation."
	default:
		if risk >= 2 {
			return core.DecisionReject, "TEST: Risk exceeds confidence in current plan.", "TEST: Need safer rollout shape."
		}
		return core.DecisionApprove, "TEST: Net positive change with acceptable constraints.", "TEST: Monitor post-implementation signals."
	}
}

// buildChallengesSimulated creates test challenges based on position conflicts
func buildChallengesSimulated(c core.Case, positions []core.Position) []core.Challenge {
	// Simulated challenge generation for testing
	var challenges []core.Challenge
	for i, pos1 := range positions {
		for j, pos2 := range positions {
			if i >= j || pos1.Stance == pos2.Stance {
				continue
			}
			challenges = append(challenges, core.Challenge{
				From:      pos1.AgentID,
				To:        pos2.AgentID,
				Challenge: fmt.Sprintf("TEST: Your %s stance seems to ignore %s", pos2.Stance, pos1.Concerns),
			})
		}
	}
	return challenges
}

// finalizePositionsSimulated generates test final positions
func finalizePositionsSimulated(c core.Case, initial []core.Position, challenges []core.Challenge) []core.Position {
	// For testing: some senators change positions based on challenges
	final := make([]core.Position, len(initial))
	for i, pos := range initial {
		final[i] = core.Position{
			AgentID:     pos.AgentID,
			Model:       pos.Model,
			Perspective: pos.Perspective,
			Round:       "final",
			Stance:      pos.Stance,
			Reasoning:   "TEST: " + pos.Reasoning + " (maintained after challenges)",
			Concerns:    pos.Concerns,
		}
		// Simulate some position changes for testing
		if i == 0 && len(challenges) > 2 {
			final[i].Stance = core.DecisionAmend
			final[i].Reasoning = "TEST: Changed position after considering challenges"
		}
	}
	return final
}

// synthesizeVerdictSimulated creates a test verdict
func synthesizeVerdictSimulated(c core.Case, positions []core.Position, judgeModel string, now time.Time) core.Verdict {
	// Count stances for testing
	counts := make(map[core.Decision]int)
	for _, p := range positions {
		counts[p.Stance]++
	}

	// Simple test logic: majority wins, ties go to amend
	var verdict core.Decision
	maxCount := 0
	for d, count := range counts {
		if count > maxCount {
			verdict = d
			maxCount = count
		}
	}
	if maxCount == 0 || (len(counts) > 1 && maxCount == 1) {
		verdict = core.DecisionAmend
	}

	implementation := "TEST: This is a simulated verdict for testing only."
	if verdict == core.DecisionApprove {
		implementation = "TEST: Proceed with implementation as proposed."
	} else if verdict == core.DecisionReject {
		implementation = "TEST: Do not proceed with this change."
	}

	return core.Verdict{
		CaseID:         c.ID,
		Verdict:        verdict,
		Type:           c.Type,
		Summary:        c.Summary,
		Reasoning:      "TEST: Simulated verdict based on position counts",
		Implementation: implementation,
		Dissent:        "",
		Binding:        false, // Test verdicts are never binding
		Judge:     judgeModel,
		VerdictAt: now.Format(time.RFC3339),
	}
}

// tokenScore counts keyword matches for test simulation
func tokenScore(text string, keywords []string) int {
	lower := strings.ToLower(text)
	score := 0
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			score++
		}
	}
	return score
}