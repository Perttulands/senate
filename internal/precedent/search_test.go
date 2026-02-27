package precedent

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Perttulands/senate/internal/core"
)

// helpers ────────────────────────────────────────────────────

func newStore(t *testing.T) *Store {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "precedents", "index.jsonl"))
}

func seedRecords(t *testing.T, s *Store, recs []Record) {
	t.Helper()
	for _, r := range recs {
		if err := s.Add(r); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func baseRecord(caseID, typ, summary string, verdict core.Decision, at time.Time) Record {
	return Record{
		CaseID:         caseID,
		Type:           typ,
		Summary:        summary,
		Verdict:        verdict,
		Reasoning:      "reasoning for " + caseID,
		Implementation: "implementation for " + caseID,
		Binding:        true,
		VerdictAt:      at.Format(time.RFC3339),
		Judge:          "claude:opus",
	}
}

// tests ─────────────────────────────────────────────────────

func TestSearch_TypeFilter(t *testing.T) {
	s := newStore(t)
	now := time.Now().UTC()
	seedRecords(t, s, []Record{
		baseRecord("s-1", "rule_evolution", "amend cleanup rule", core.DecisionAmend, now),
		baseRecord("s-2", "gate_criteria", "raise coverage bar", core.DecisionApprove, now.Add(-time.Hour)),
		baseRecord("s-3", "rule_evolution", "amend logging rule", core.DecisionAmend, now.Add(-2*time.Hour)),
	})

	// Filter for rule_evolution only — should exclude s-2
	results, err := s.Search("amend rule", SearchOptions{Type: "rule_evolution"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Type != "rule_evolution" {
			t.Fatalf("expected type rule_evolution, got %q (case %s)", r.Type, r.CaseID)
		}
	}
}

func TestSearch_VerdictFilter(t *testing.T) {
	s := newStore(t)
	now := time.Now().UTC()
	seedRecords(t, s, []Record{
		baseRecord("s-1", "dispute", "reject unsafe merge", core.DecisionReject, now),
		baseRecord("s-2", "dispute", "approve safe merge", core.DecisionApprove, now.Add(-time.Hour)),
		baseRecord("s-3", "dispute", "reject risky deploy", core.DecisionReject, now.Add(-2*time.Hour)),
	})

	results, err := s.Search("merge deploy", SearchOptions{Verdict: core.DecisionReject})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Verdict != core.DecisionReject {
			t.Fatalf("expected verdict rejected, got %q (case %s)", r.Verdict, r.CaseID)
		}
	}
}

func TestSearch_EmptyResult(t *testing.T) {
	s := newStore(t)
	now := time.Now().UTC()
	seedRecords(t, s, []Record{
		baseRecord("s-1", "rule_evolution", "amend cleanup rule", core.DecisionAmend, now),
	})

	// Query with terms that don't appear anywhere
	results, err := s.Search("xylophone quantum", SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestSearch_EmptyStore(t *testing.T) {
	s := newStore(t)

	results, err := s.Search("anything", SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results from empty store, got %d", len(results))
	}
}

func TestSearch_MultipleMatches_OrderedByScore(t *testing.T) {
	s := newStore(t)
	now := time.Now().UTC()

	// s-1 matches "coverage" only; s-2 matches both "coverage" and "enforcement"
	r1 := baseRecord("s-1", "gate_criteria", "raise coverage threshold", core.DecisionApprove, now)
	r2 := baseRecord("s-2", "gate_criteria", "coverage enforcement policy", core.DecisionAmend, now.Add(-time.Hour))
	r2.Reasoning = "enforcement mechanisms are key"
	seedRecords(t, s, []Record{r1, r2})

	// Two query tokens: "coverage" matches both, "enforcement" matches only s-2
	results, err := s.Search("coverage enforcement", SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// s-2 scores 2 (both tokens), s-1 scores 1 — s-2 first
	if results[0].CaseID != "s-2" {
		t.Fatalf("expected s-2 first (higher score), got %s", results[0].CaseID)
	}
}

func TestSearch_LimitRespectsOption(t *testing.T) {
	s := newStore(t)
	now := time.Now().UTC()
	var recs []Record
	for i := 0; i < 5; i++ {
		recs = append(recs, baseRecord(
			"s-"+string(rune('a'+i)),
			"general",
			"testing limit functionality",
			core.DecisionApprove,
			now.Add(-time.Duration(i)*time.Hour),
		))
	}
	seedRecords(t, s, recs)

	results, err := s.Search("testing limit", SearchOptions{Limit: 2})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (limit), got %d", len(results))
	}
}

func TestSearch_CombinedTypeAndVerdictFilter(t *testing.T) {
	s := newStore(t)
	now := time.Now().UTC()
	seedRecords(t, s, []Record{
		baseRecord("s-1", "rule_evolution", "amend rule", core.DecisionAmend, now),
		baseRecord("s-2", "rule_evolution", "approve rule", core.DecisionApprove, now.Add(-time.Hour)),
		baseRecord("s-3", "gate_criteria", "amend gate", core.DecisionAmend, now.Add(-2*time.Hour)),
	})

	results, err := s.Search("rule gate amend approve", SearchOptions{
		Type:    "rule_evolution",
		Verdict: core.DecisionAmend,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].CaseID != "s-1" {
		t.Fatalf("expected s-1, got %s", results[0].CaseID)
	}
}
