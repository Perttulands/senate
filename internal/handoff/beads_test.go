package handoff

import (
	"context"
	"errors"
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
	if r.name != "bd" {
		t.Fatalf("expected bd command, got %q", r.name)
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
