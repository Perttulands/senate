package precedent

import (
	"os"
	"path/filepath"
	"strings"
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

// --- SearchRelevantPrecedent ---

func TestSearchRelevantPrecedent_MatchesByType(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "precedents", "index.jsonl"))
	now := time.Now().UTC()
	s.Add(Record{
		CaseID: "senate-rel-1", Type: "rule_evolution", Summary: "Coverage rule update",
		Verdict: core.DecisionAmend, Reasoning: "Coverage thresholds need updating",
		Implementation: "Set to 80%", Binding: true,
		VerdictAt: now.Format(time.RFC3339), Judge: "test",
	})
	s.Add(Record{
		CaseID: "senate-rel-2", Type: "general", Summary: "Coverage discussion",
		Verdict: core.DecisionApprove, Reasoning: "Coverage is fine",
		Implementation: "No changes", Binding: true,
		VerdictAt: now.Format(time.RFC3339), Judge: "test",
	})

	results, err := s.SearchRelevantPrecedent("rule_evolution", []string{"coverage", "thresholds"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	// Should only return rule_evolution type
	for _, r := range results {
		if r.Type != "rule_evolution" {
			t.Errorf("expected type rule_evolution, got %q", r.Type)
		}
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].CaseID != "senate-rel-1" {
		t.Errorf("case_id = %q, want senate-rel-1", results[0].CaseID)
	}
}

func TestSearchRelevantPrecedent_EmptyKeywords(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "precedents", "index.jsonl"))
	now := time.Now().UTC()
	s.Add(Record{
		CaseID: "senate-rel-3", Type: "general", Summary: "Some precedent",
		Verdict: core.DecisionApprove, Reasoning: "R",
		Implementation: "I", Binding: true,
		VerdictAt: now.Format(time.RFC3339), Judge: "test",
	})

	// Empty keywords → empty query → all records score 1 (matched by scoreRecord default)
	results, err := s.SearchRelevantPrecedent("general", []string{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestSearchRelevantPrecedent_LimitTo5(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "precedents", "index.jsonl"))
	now := time.Now().UTC()
	for i := 0; i < 10; i++ {
		s.Add(Record{
			CaseID: "senate-lim-" + string(rune('a'+i)), Type: "general",
			Summary: "Limit test precedent",
			Verdict: core.DecisionApprove, Reasoning: "R",
			Implementation: "I", Binding: true,
			VerdictAt: now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339),
			Judge: "test",
		})
	}

	results, err := s.SearchRelevantPrecedent("general", []string{"limit", "test"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) > 5 {
		t.Errorf("expected at most 5 results, got %d", len(results))
	}
}

// --- Record.Validate extended ---

func TestRecordValidate_MissingCaseID(t *testing.T) {
	r := Record{Type: "general", Summary: "S", Verdict: core.DecisionApprove,
		VerdictAt: time.Now().UTC().Format(time.RFC3339)}
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "case_id") {
		t.Errorf("expected case_id error, got %v", err)
	}
}

func TestRecordValidate_MissingType(t *testing.T) {
	r := Record{CaseID: "s-1", Summary: "S", Verdict: core.DecisionApprove,
		VerdictAt: time.Now().UTC().Format(time.RFC3339)}
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "type") {
		t.Errorf("expected type error, got %v", err)
	}
}

func TestRecordValidate_MissingSummary(t *testing.T) {
	r := Record{CaseID: "s-1", Type: "general", Verdict: core.DecisionApprove,
		VerdictAt: time.Now().UTC().Format(time.RFC3339)}
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "summary") {
		t.Errorf("expected summary error, got %v", err)
	}
}

func TestRecordValidate_InvalidVerdict(t *testing.T) {
	r := Record{CaseID: "s-1", Type: "general", Summary: "S", Verdict: "bogus",
		VerdictAt: time.Now().UTC().Format(time.RFC3339)}
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "verdict") {
		t.Errorf("expected verdict error, got %v", err)
	}
}

func TestRecordValidate_MissingVerdictAt(t *testing.T) {
	r := Record{CaseID: "s-1", Type: "general", Summary: "S", Verdict: core.DecisionApprove}
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "verdict_at") {
		t.Errorf("expected verdict_at error, got %v", err)
	}
}

func TestRecordValidate_InvalidVerdictAt(t *testing.T) {
	r := Record{CaseID: "s-1", Type: "general", Summary: "S", Verdict: core.DecisionApprove,
		VerdictAt: "not-a-date"}
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("expected RFC3339 error, got %v", err)
	}
}

func TestRecordValidate_Success(t *testing.T) {
	r := Record{CaseID: "s-1", Type: "general", Summary: "S", Verdict: core.DecisionApprove,
		VerdictAt: time.Now().UTC().Format(time.RFC3339)}
	if err := r.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- LoadAll edge cases ---

func TestLoadAll_MalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.jsonl")
	// Write a mix of valid, invalid, and empty lines
	content := `{"case_id":"s-1","type":"general","summary":"Valid","verdict":"approved","reasoning":"R","implementation":"I","binding":true,"verdict_at":"` + time.Now().UTC().Format(time.RFC3339) + `","judge":"test"}
not json at all

{"case_id":"","type":"general","summary":"S","verdict":"approved","verdict_at":"` + time.Now().UTC().Format(time.RFC3339) + `"}
`
	os.WriteFile(path, []byte(content), 0644)

	s := New(path)
	records, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	// Only the first valid line should survive
	if len(records) != 1 {
		t.Fatalf("expected 1 valid record, got %d", len(records))
	}
	if records[0].CaseID != "s-1" {
		t.Errorf("case_id = %q", records[0].CaseID)
	}
}

// --- Add validation error ---

func TestAdd_ValidationError(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "index.jsonl"))
	err := s.Add(Record{}) // completely empty, should fail validation
	if err == nil {
		t.Fatal("expected validation error for empty record")
	}
}
