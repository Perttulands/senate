package precedent

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Perttulands/senate/internal/core"
)

func TestAddAndSearch(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "precedents", "index.jsonl"))
	now := time.Now().UTC()
	if err := s.Add(Record{
		CaseID:         "senate-1",
		Type:           "rule_evolution",
		Summary:        "Amend silent fallback",
		Verdict:        core.DecisionAmend,
		Reasoning:      "47 false positives in cleanup handlers",
		Implementation: "exclude trap cleanup context",
		Binding:        true,
		VerdictAt:      now.Format(time.RFC3339),
		Judge:          "claude:opus",
	}); err != nil {
		t.Fatalf("add record: %v", err)
	}

	results, err := s.Search("cleanup trap", SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}
	if results[0].CaseID != "senate-1" {
		t.Fatalf("unexpected case id: %s", results[0].CaseID)
	}
}

func TestFromVerdictIncludesHandoff(t *testing.T) {
	v := core.Verdict{
		CaseID:         "senate-2",
		Type:           "gate_criteria",
		Summary:        "Adjust coverage threshold",
		Verdict:        core.DecisionApprove,
		Reasoning:      "Maintains quality with better velocity",
		Implementation: "set threshold to 70% for new code",
		Binding:        true,
		VerdictAt:      time.Now().UTC().Format(time.RFC3339),
		Judge:          "claude:opus",
		Handoff: &core.Handoff{
			System: "centurion",
			BeadID: "athena-123",
			Status: "created",
		},
	}
	r := FromVerdict(v)
	if r.BeadID != "athena-123" {
		t.Fatalf("expected bead id athena-123, got %q", r.BeadID)
	}
}
