package core

import (
	"strings"
	"testing"
	"time"
)

// ── Decision validation ────────────────────────────────────

func TestDecisionValidate_RejectsInvalid(t *testing.T) {
	invalid := []Decision{"", "APPROVED", "maybe", "pending", "unknown"}
	for _, d := range invalid {
		if err := d.Validate(); err == nil {
			t.Errorf("expected Decision(%q) to be rejected", d)
		}
	}
}

func TestDecisionValidate_AcceptsValid(t *testing.T) {
	valid := []Decision{DecisionApprove, DecisionReject, DecisionAmend, DecisionDefer}
	for _, d := range valid {
		if err := d.Validate(); err != nil {
			t.Errorf("expected Decision(%q) to pass, got %v", d, err)
		}
	}
}

// ── Case validation ────────────────────────────────────────

func validCase() Case {
	return Case{
		ID:       "senate-1",
		Type:     "rule_evolution",
		Summary:  "summary text",
		Question: "question text",
		FiledAt:  time.Now().UTC().Format(time.RFC3339),
	}
}

func TestCaseValidate_MissingID(t *testing.T) {
	c := validCase()
	c.ID = ""
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
	if !strings.Contains(err.Error(), "case.id") {
		t.Fatalf("expected case.id error, got: %v", err)
	}
}

func TestCaseValidate_MissingType(t *testing.T) {
	c := validCase()
	c.Type = ""
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for missing type")
	}
	if !strings.Contains(err.Error(), "case.type") {
		t.Fatalf("expected case.type error, got: %v", err)
	}
}

func TestCaseValidate_MissingSummary(t *testing.T) {
	c := validCase()
	c.Summary = "   "
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for blank summary")
	}
	if !strings.Contains(err.Error(), "case.summary") {
		t.Fatalf("expected case.summary error, got: %v", err)
	}
}

func TestCaseValidate_MissingQuestion(t *testing.T) {
	c := validCase()
	c.Question = ""
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for missing question")
	}
	if !strings.Contains(err.Error(), "case.question") {
		t.Fatalf("expected case.question error, got: %v", err)
	}
}

func TestCaseValidate_BadFiledAtFormat(t *testing.T) {
	c := validCase()
	c.FiledAt = "2026-02-27"
	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for non-RFC3339 filed_at")
	}
	if !strings.Contains(err.Error(), "case.filed_at must be RFC3339") {
		t.Fatalf("expected RFC3339 error, got: %v", err)
	}
}

func TestCaseValidate_ValidPasses(t *testing.T) {
	c := validCase()
	c.Evidence = []string{"fact one", "fact two"}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid case to pass, got: %v", err)
	}
}

// ── Verdict validation ─────────────────────────────────────

func validVerdict() Verdict {
	now := time.Now().UTC().Format(time.RFC3339)
	return Verdict{
		CaseID:         "senate-1",
		FiledAt:        now,
		VerdictAt:      now,
		Type:           "rule_evolution",
		Summary:        "summary text",
		Verdict:        DecisionAmend,
		Reasoning:      "reasoning text",
		Implementation: "implementation",
		Binding:        true,
		Judge:          "claude:opus",
	}
}

func TestVerdictValidate_MissingCaseID(t *testing.T) {
	v := validVerdict()
	v.CaseID = ""
	err := v.Validate()
	if err == nil {
		t.Fatal("expected error for missing case_id")
	}
	if !strings.Contains(err.Error(), "verdict.case_id") {
		t.Fatalf("expected verdict.case_id error, got: %v", err)
	}
}

func TestVerdictValidate_MissingReasoning(t *testing.T) {
	v := validVerdict()
	v.Reasoning = ""
	err := v.Validate()
	if err == nil {
		t.Fatal("expected error for missing reasoning")
	}
	if !strings.Contains(err.Error(), "verdict.reasoning") {
		t.Fatalf("expected verdict.reasoning error, got: %v", err)
	}
}

func TestVerdictValidate_MissingJudge(t *testing.T) {
	v := validVerdict()
	v.Judge = ""
	err := v.Validate()
	if err == nil {
		t.Fatal("expected error for missing judge")
	}
	if !strings.Contains(err.Error(), "verdict.judge") {
		t.Fatalf("expected verdict.judge error, got: %v", err)
	}
}

func TestVerdictValidate_InvalidDecision(t *testing.T) {
	v := validVerdict()
	v.Verdict = "garbage"
	err := v.Validate()
	if err == nil {
		t.Fatal("expected error for invalid decision")
	}
	if !strings.Contains(err.Error(), "verdict.verdict") {
		t.Fatalf("expected verdict.verdict error, got: %v", err)
	}
}

func TestVerdictValidate_BadVerdictAtFormat(t *testing.T) {
	v := validVerdict()
	v.VerdictAt = "not-a-date"
	err := v.Validate()
	if err == nil {
		t.Fatal("expected error for bad verdict_at format")
	}
	if !strings.Contains(err.Error(), "verdict.verdict_at must be RFC3339") {
		t.Fatalf("expected RFC3339 error, got: %v", err)
	}
}

func TestVerdictValidate_ValidPasses(t *testing.T) {
	v := validVerdict()
	if err := v.Validate(); err != nil {
		t.Fatalf("expected valid verdict to pass, got: %v", err)
	}
}
