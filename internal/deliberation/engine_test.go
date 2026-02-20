package deliberation

import (
	"testing"
	"time"

	"github.com/Perttulands/senate/internal/core"
)

func TestBuildPanelVariesPerspectives(t *testing.T) {
	panel := BuildPanel(4, nil, nil)
	if len(panel) != 4 {
		t.Fatalf("expected 4 panel members, got %d", len(panel))
	}
	if panel[0].Name == panel[1].Name {
		t.Fatalf("expected varied perspectives, got %q and %q", panel[0].Name, panel[1].Name)
	}
}

func TestDeliberateProducesBindingVerdictAndTranscript(t *testing.T) {
	engine := New(BuildPanel(3, nil, nil))
	c := core.Case{
		ID:       "senate-001",
		Type:     "rule_evolution",
		Summary:  "Amend silent fallback rule",
		Question: "Should we amend rule X to reduce false positives?",
		Evidence: []string{"47 false positives", "recent regressions"},
		FiledAt:  time.Now().UTC().Format(time.RFC3339),
	}
	transcript, verdict, err := engine.Deliberate(c, time.Now().UTC())
	if err != nil {
		t.Fatalf("deliberate: %v", err)
	}
	if transcript.CaseID != c.ID {
		t.Fatalf("expected transcript for %s, got %s", c.ID, transcript.CaseID)
	}
	if len(transcript.InitialPositions) != 3 || len(transcript.FinalPositions) != 3 {
		t.Fatalf("expected 3 positions per round, got initial=%d final=%d", len(transcript.InitialPositions), len(transcript.FinalPositions))
	}
	if verdict.CaseID != c.ID {
		t.Fatalf("expected verdict for %s, got %s", c.ID, verdict.CaseID)
	}
	if verdict.Verdict == "" {
		t.Fatal("expected synthesized verdict decision")
	}
	if err := verdict.Validate(); err != nil {
		t.Fatalf("verdict validation failed: %v", err)
	}
}

func TestMajorityDecisionTieFallsBackToDefer(t *testing.T) {
	counts := map[core.Decision]int{
		core.DecisionApprove: 1,
		core.DecisionReject:  1,
		core.DecisionAmend:   0,
		core.DecisionDefer:   0,
	}
	if got := majorityDecision(counts); got != core.DecisionDefer {
		t.Fatalf("expected deferred tie-break, got %s", got)
	}
}
