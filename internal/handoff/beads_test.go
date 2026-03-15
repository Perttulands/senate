package handoff

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Perttulands/senate/internal/core"
)

type fakeRunner struct {
	out  string
	err  error
	name string
	args []string
	dir  string
}

func (f *fakeRunner) Run(_ context.Context, name string, args []string, dir string) (string, error) {
	f.name = name
	f.args = append([]string{}, args...)
	f.dir = dir
	return f.out, f.err
}

func TestCreateBeadForVerdictCreated(t *testing.T) {
	r := &fakeRunner{out: "athena-xyz\n"}
	v := core.Verdict{
		CaseID:         "senate-9",
		FiledAt:        time.Now().UTC().Format(time.RFC3339),
		VerdictAt:      time.Now().UTC().Format(time.RFC3339),
		Type:           "rule_evolution",
		Summary:        "Amend silent fallback",
		Verdict:        core.DecisionAmend,
		Reasoning:      "Reasoning",
		Implementation: "Implementation",
		Binding:        true,
		Judge:          "claude:opus",
	}
	res, err := CreateBeadForVerdict(context.Background(), r, "/tmp/workspace", v)
	if err != nil {
		t.Fatalf("create bead: %v", err)
	}
	if res.BeadID != "athena-xyz" {
		t.Fatalf("expected bead id athena-xyz, got %q", res.BeadID)
	}
	if r.name != "br" {
		t.Fatalf("expected br command, got %q", r.name)
	}
}

func TestCreateBeadForVerdictSkipDefer(t *testing.T) {
	res, err := CreateBeadForVerdict(context.Background(), &fakeRunner{}, "", core.Verdict{Verdict: core.DecisionDefer})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.Status != "skipped" {
		t.Fatalf("expected skipped result, got %q", res.Status)
	}
}

func TestCreateBeadForVerdictError(t *testing.T) {
	r := &fakeRunner{err: errors.New("failed")}
	v := core.Verdict{Binding: true, Verdict: core.DecisionApprove, Type: "general", CaseID: "senate-1", Summary: "S", Reasoning: "R", Implementation: "I"}
	_, err := CreateBeadForVerdict(context.Background(), r, "/tmp/workspace", v)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseBeadID(t *testing.T) {
	if got := parseBeadID("Created issue: athena-123"); got != "athena-123" {
		t.Fatalf("unexpected bead id: %q", got)
	}
}

func TestCreateBeadForVerdictSkipNonBinding(t *testing.T) {
	res, err := CreateBeadForVerdict(context.Background(), &fakeRunner{}, "", core.Verdict{
		Binding: false,
		Verdict: core.DecisionApprove,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.Status != "skipped" {
		t.Fatalf("expected skipped, got %q", res.Status)
	}
}

func TestCreateBeadForVerdictUnparsableID(t *testing.T) {
	r := &fakeRunner{out: "!!!###$$$"}
	v := core.Verdict{Binding: true, Verdict: core.DecisionApprove, Type: "general", CaseID: "senate-1", Summary: "S", Reasoning: "R", Implementation: "I"}
	_, err := CreateBeadForVerdict(context.Background(), r, "/tmp/workspace", v)
	if err == nil {
		t.Fatal("expected error for unparsable bead ID")
	}
}

func TestCreateBeadForVerdictRequiresWorkspace(t *testing.T) {
	r := &fakeRunner{out: "pol-abc\n"}
	v := core.Verdict{
		Binding:        true,
		Verdict:        core.DecisionApprove,
		Type:           "general",
		CaseID:         "senate-ws",
		Summary:        "Test workspace default",
		Reasoning:      "R",
		Implementation: "I",
		Judge:          "claude:opus",
		FiledAt:        time.Now().UTC().Format(time.RFC3339),
		VerdictAt:      time.Now().UTC().Format(time.RFC3339),
	}
	_, err := CreateBeadForVerdict(context.Background(), r, "", v)
	if err == nil {
		t.Fatal("expected explicit workspace error")
	}
	if !strings.Contains(err.Error(), "workspace dir is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseBeadIDEmpty(t *testing.T) {
	if got := parseBeadID(""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestParseBeadIDMultiline(t *testing.T) {
	out := "Creating bead...\nDone\npol-xyz-123\n"
	if got := parseBeadID(out); got != "pol-xyz-123" {
		t.Fatalf("expected pol-xyz-123 from multiline, got %q", got)
	}
}

func TestInferTargetSystem(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"rule_evolution", "truthsayer"},
		{"gate_criteria", "centurion"},
		{"priority_triage", "athena"},
		{"dispute_resolution", "athena"},
		{"general", "athena"},
		{"", "athena"},
	}
	for _, tt := range tests {
		if got := inferTargetSystem(tt.input); got != tt.want {
			t.Errorf("inferTargetSystem(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTrimTo(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"hello world", 5, "hell..."},
		{"", 5, ""},
		{"ab", 1, ""},
		{"exactly10!", 10, "exactly10!"},
	}
	for _, tt := range tests {
		if got := trimTo(tt.input, tt.max); got != tt.want {
			t.Errorf("trimTo(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

// --- CreateBeadFromVerdict ---

func TestCreateBeadFromVerdict_BrNotFound(t *testing.T) {
	v := core.Verdict{
		CaseID:         "senate-br-missing",
		FiledAt:        time.Now().UTC().Format(time.RFC3339),
		VerdictAt:      time.Now().UTC().Format(time.RFC3339),
		Type:           "general",
		Summary:        "Test br not found",
		Verdict:        core.DecisionApprove,
		Reasoning:      "R",
		Implementation: "I",
		Binding:        true,
		Judge:          "test",
	}
	_, err := CreateBeadFromVerdict(context.Background(), v)
	if err == nil {
		t.Fatal("expected explicit workspace error")
	}
	if !strings.Contains(err.Error(), "workspace dir is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- parseBeadID edge cases ---

func TestParseBeadID_WhitespaceOnly(t *testing.T) {
	if got := parseBeadID("   \n  \n  "); got != "" {
		t.Fatalf("expected empty for whitespace, got %q", got)
	}
}

func TestParseBeadID_SingleLineMatch(t *testing.T) {
	if got := parseBeadID("athena-456"); got != "athena-456" {
		t.Fatalf("expected athena-456, got %q", got)
	}
}

func TestParseBeadID_DottedChild(t *testing.T) {
	if got := parseBeadID("pol-10j3.6"); got != "pol-10j3.6" {
		t.Fatalf("expected pol-10j3.6, got %q", got)
	}
}

func TestParseBeadID_DottedMultiSegment(t *testing.T) {
	if got := parseBeadID("Created: pol-abc.1.2\n"); got != "pol-abc.1.2" {
		t.Fatalf("expected pol-abc.1.2, got %q", got)
	}
}

func TestCreateBeadForVerdictTitleFormat(t *testing.T) {
	r := &fakeRunner{out: "athena-title\n"}
	v := core.Verdict{
		CaseID:         "senate-fmt",
		FiledAt:        time.Now().UTC().Format(time.RFC3339),
		VerdictAt:      time.Now().UTC().Format(time.RFC3339),
		Type:           "gate_criteria",
		Summary:        "Add coverage threshold",
		Verdict:        core.DecisionApprove,
		Reasoning:      "Coverage matters",
		Implementation: "Set min 80%",
		Binding:        true,
		Judge:          "claude:sonnet",
	}
	res, err := CreateBeadForVerdict(context.Background(), r, "/tmp/w", v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Title == "" {
		t.Fatal("expected non-empty title")
	}
	if res.Status != "created" {
		t.Fatalf("expected created, got %q", res.Status)
	}
}
