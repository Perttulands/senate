package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Perttulands/senate/internal/core"
)

func TestSaveAndLoadVerdict(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	v := core.Verdict{
		CaseID:         "senate-1",
		FiledAt:        time.Now().UTC().Format(time.RFC3339),
		VerdictAt:      time.Now().UTC().Format(time.RFC3339),
		Type:           "rule_evolution",
		Summary:        "Summary",
		Verdict:        core.DecisionApprove,
		Reasoning:      "Reasoning",
		Implementation: "Do thing",
		Binding:        true,
		Judge:          "claude:opus",
	}
	if err := d.SaveVerdict(v); err != nil {
		t.Fatalf("save verdict: %v", err)
	}
	got, err := d.LoadVerdict("senate-1")
	if err != nil {
		t.Fatalf("load verdict: %v", err)
	}
	if got.CaseID != v.CaseID || got.Verdict != v.Verdict {
		t.Fatalf("unexpected verdict loaded: %+v", got)
	}
}

func TestPaths(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if got := d.PrecedentIndexPath(); filepath.Base(got) != "index.jsonl" {
		t.Fatalf("expected precedent index filename, got %q", got)
	}
	if got := d.RelayOutboxPath(); filepath.Base(got) != "case-filed.jsonl" {
		t.Fatalf("expected relay outbox filename, got %q", got)
	}
}
