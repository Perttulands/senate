package core

import (
	"strings"
	"testing"
	"time"
)

func TestCaseNormalizeAndValidate(t *testing.T) {
	now := time.Date(2026, 2, 20, 10, 11, 12, 0, time.UTC)
	c := Case{Question: "Should we amend rule X?"}
	c.Normalize(now)

	if c.ID == "" {
		t.Fatal("expected case ID")
	}
	if c.Type != "general" {
		t.Fatalf("expected default type general, got %q", c.Type)
	}
	if c.Summary == "" {
		t.Fatal("expected summary to default from question")
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid normalized case, got %v", err)
	}
}

func TestCaseValidateRejectsEmptyEvidence(t *testing.T) {
	c := Case{
		ID:       "senate-1",
		Type:     "rule_evolution",
		Summary:  "summary",
		Question: "question",
		FiledAt:  time.Now().UTC().Format(time.RFC3339),
		Evidence: []string{"ok", ""},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "case.evidence[1]") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerdictValidate(t *testing.T) {
	v := Verdict{
		CaseID:         "senate-1",
		FiledAt:        time.Now().UTC().Format(time.RFC3339),
		VerdictAt:      time.Now().UTC().Format(time.RFC3339),
		Type:           "rule_evolution",
		Summary:        "summary",
		Verdict:        DecisionAmend,
		Reasoning:      "reasoning",
		Implementation: "implementation",
		Binding:        true,
		Judge:          "claude:opus",
	}
	if err := v.Validate(); err != nil {
		t.Fatalf("expected valid verdict, got %v", err)
	}
}
