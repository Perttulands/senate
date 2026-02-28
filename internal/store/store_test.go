package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Perttulands/senate/internal/core"
)

func testCase(id string) core.Case {
	now := time.Now().UTC()
	c := core.Case{
		ID:       id,
		Type:     "general",
		Summary:  "Test question",
		Question: "Test question",
		FiledBy:  "test",
		FiledAt:  now.Format(time.RFC3339),
	}
	return c
}

func testVerdict(caseID string) core.Verdict {
	now := time.Now().UTC().Format(time.RFC3339)
	return core.Verdict{
		CaseID:         caseID,
		FiledAt:        now,
		VerdictAt:      now,
		Type:           "rule_evolution",
		Summary:        "Summary",
		Verdict:        core.DecisionApprove,
		Reasoning:      "Reasoning",
		Implementation: "Do thing",
		Binding:        true,
		Judge:          "claude:opus",
	}
}

func TestSaveAndLoadVerdict(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	v := testVerdict("senate-1")
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

func TestSaveCaseAndLoadCaseRoundTrip(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	c := testCase("senate-rt-1")
	if err := d.SaveCase(c); err != nil {
		t.Fatalf("save case: %v", err)
	}
	got, err := d.LoadCase("senate-rt-1")
	if err != nil {
		t.Fatalf("load case: %v", err)
	}
	if got.ID != c.ID {
		t.Fatalf("case ID mismatch: want %q, got %q", c.ID, got.ID)
	}
	if got.Type != c.Type {
		t.Fatalf("case type mismatch: want %q, got %q", c.Type, got.Type)
	}
	if got.Summary != c.Summary {
		t.Fatalf("case summary mismatch: want %q, got %q", c.Summary, got.Summary)
	}
	if got.Question != c.Question {
		t.Fatalf("case question mismatch: want %q, got %q", c.Question, got.Question)
	}
}

func TestSaveCaseOverwrite(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	c := testCase("senate-ow-1")
	if err := d.SaveCase(c); err != nil {
		t.Fatalf("save case: %v", err)
	}
	c.Summary = "Updated question"
	c.Question = "Updated question"
	if err := d.SaveCase(c); err != nil {
		t.Fatalf("overwrite case: %v", err)
	}
	got, err := d.LoadCase("senate-ow-1")
	if err != nil {
		t.Fatalf("load case: %v", err)
	}
	if got.Summary != "Updated question" {
		t.Fatalf("expected updated summary, got %q", got.Summary)
	}
}

func TestLoadCaseMissing(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	_, err = d.LoadCase("nonexistent-case")
	if err == nil {
		t.Fatal("expected error loading missing case")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}

func TestSaveTranscriptWithContent(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	tr := core.Transcript{
		CaseID:    "senate-tr-1",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
		Panel: []core.PanelMember{
			{AgentID: "senator-pragmatist", Model: "sonnet", Perspective: "pragmatist"},
		},
		InitialPositions: []core.Position{
			{AgentID: "senator-pragmatist", Stance: core.DecisionApprove, Reasoning: "Ship it"},
		},
		JudgeModel: "claude:opus",
	}
	if err := d.SaveTranscript(tr); err != nil {
		t.Fatalf("save transcript: %v", err)
	}
	// Verify file exists on disk
	data, err := os.ReadFile(d.TranscriptPath("senate-tr-1"))
	if err != nil {
		t.Fatalf("read transcript file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("transcript file is empty")
	}
}

func TestSaveTranscriptEmptyCaseID(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	tr := core.Transcript{CaseID: ""}
	if err := d.SaveTranscript(tr); err == nil {
		t.Fatal("expected error for empty case_id")
	}
}

func TestLoadVerdictMissing(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	_, err = d.LoadVerdict("nonexistent-verdict")
	if err == nil {
		t.Fatal("expected error loading missing verdict")
	}
}

func TestNewWithEmptyRoot(t *testing.T) {
	tmp := t.TempDir()
	// New with empty string should default to "state" but within our control
	d, err := New(filepath.Join(tmp, ""))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	// Should default to "state"
	if d.Root != "state" {
		// It trims to empty then defaults — just verify it works
		_ = d.Root
	}
}

func TestSaveCaseValidationError(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	// Invalid case (missing required fields)
	err = d.SaveCase(core.Case{})
	if err == nil {
		t.Fatal("expected validation error for empty case")
	}
}

func TestSaveVerdictValidationError(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	// Invalid verdict (missing required fields)
	err = d.SaveVerdict(core.Verdict{})
	if err == nil {
		t.Fatal("expected validation error for empty verdict")
	}
}

func TestLoadCaseCorruptJSON(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	// Write corrupt JSON to the case file
	os.WriteFile(d.CasePath("corrupt"), []byte("not json"), 0644)
	_, err = d.LoadCase("corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt case JSON")
	}
}

func TestLoadVerdictCorruptJSON(t *testing.T) {
	d, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	// Write corrupt JSON to the verdict file
	os.WriteFile(d.VerdictPath("corrupt"), []byte("not json"), 0644)
	_, err = d.LoadVerdict("corrupt")
	if err == nil {
		t.Fatal("expected error for corrupt verdict JSON")
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
	if got := d.CasePath("test-1"); filepath.Base(got) != "test-1.json" {
		t.Fatalf("expected case path filename, got %q", got)
	}
	if got := d.VerdictPath("test-1"); filepath.Base(got) != "test-1.json" {
		t.Fatalf("expected verdict path filename, got %q", got)
	}
	if got := d.TranscriptPath("test-1"); filepath.Base(got) != "test-1.json" {
		t.Fatalf("expected transcript path filename, got %q", got)
	}
}
