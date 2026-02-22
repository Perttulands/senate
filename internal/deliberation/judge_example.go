package deliberation

import (
	"context"
	"fmt"

	"github.com/Perttulands/senate/internal/core"
)

// ExampleJudgeSynthesis demonstrates how the judge synthesis works
func ExampleJudgeSynthesis() {
	// Example case
	testCase := core.Case{
		ID:       "SNT-TEST-001",
		Type:     "rule_evolution",
		Summary:  "Relax Truthsayer rule TS-042 for async handlers",
		Question: "Should we relax rule TS-042 to reduce false positives in async error handlers?",
		Evidence: []string{
			"15 false positives in the last week",
			"All false positives occur in async error handlers",
			"Developer satisfaction surveys show frustration with this rule",
			"Rule catches real issues 70% of the time",
		},
		FiledAt: "2024-01-15T10:00:00Z",
		FiledBy: "developer-experience-team",
	}

	// Example initial positions
	initialPositions := []core.Position{
		{
			AgentID:     "pragmatist",
			Model:       "claude:opus",
			Perspective: "The Pragmatic Builder",
			Round:       "initial",
			Stance:      core.DecisionApprove,
			Reasoning:   "The 30% false positive rate is unacceptable for developer productivity. We should relax the rule and iterate based on real usage.",
			Concerns:    "Might miss some edge cases, but we can adjust later",
		},
		{
			AgentID:     "purist",
			Model:       "claude:opus",
			Perspective: "The Architectural Purist",
			Round:       "initial",
			Stance:      core.DecisionReject,
			Reasoning:   "The rule exists for a reason. 70% accuracy is quite good. We should fix the async handlers instead of weakening our standards.",
			Concerns:    "Relaxing rules sets a bad precedent",
		},
		{
			AgentID:     "skeptic",
			Model:       "claude:opus",
			Perspective: "The Critical Skeptic",
			Round:       "initial",
			Stance:      core.DecisionAmend,
			Reasoning:   "Neither full approval nor rejection is warranted. We need a targeted amendment that addresses async handlers specifically without weakening the rule broadly.",
			Concerns:    "Need to ensure we don't create new blind spots",
		},
	}

	// Example challenges
	challenges := []core.Challenge{
		{
			From:      "pragmatist",
			To:        "purist",
			Challenge: "Your position prioritizes theoretical purity over developer experience. How do you justify maintaining a rule that frustrates developers 30% of the time?",
		},
		{
			From:      "purist",
			To:        "pragmatist",
			Challenge: "Your rush to relax standards could introduce subtle bugs. What's your plan to ensure we don't miss critical errors in async handlers?",
		},
	}

	// Example final positions (after deliberation)
	finalPositions := []core.Position{
		{
			AgentID:   "pragmatist",
			Model:     "claude:opus",
			Round:     "final",
			Stance:    core.DecisionAmend,
			Reasoning: "After considering the purist's concerns, I agree that a targeted amendment for async handlers is better than a blanket relaxation.",
		},
		{
			AgentID:   "purist",
			Model:     "claude:opus",
			Round:     "final",
			Stance:    core.DecisionAmend,
			Reasoning: "The skeptic's approach is sound. A specific carve-out for async patterns preserves the rule's intent while addressing the pain points.",
		},
		{
			AgentID:   "skeptic",
			Model:     "claude:opus",
			Round:     "final",
			Stance:    core.DecisionAmend,
			Reasoning: "The panel has converged on a balanced solution that maintains standards while improving developer experience.",
		},
	}

	// Call the judge synthesis (would normally call real Claude)
	ctx := context.Background()
	verdict, err := JudgeSynthesize(ctx, testCase, initialPositions, challenges, finalPositions)
	if err != nil {
		fmt.Printf("Judge synthesis error: %v\n", err)
		return
	}

	// Display the verdict
	fmt.Printf("=== VERDICT FOR CASE %s ===\n", verdict.CaseID)
	fmt.Printf("Decision: %s\n", verdict.Verdict)
	fmt.Printf("Reasoning: %s\n", verdict.Reasoning)
	fmt.Printf("Implementation: %s\n", verdict.Implementation)
	if verdict.Dissent != "" {
		fmt.Printf("Dissent: %s\n", verdict.Dissent)
	}
	fmt.Printf("Binding: %v\n", verdict.Binding)
	fmt.Printf("Judge: %s\n", verdict.Judge)
}