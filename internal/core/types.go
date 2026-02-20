package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Decision is the judge output for a case.
type Decision string

const (
	DecisionApprove Decision = "approved"
	DecisionReject  Decision = "rejected"
	DecisionAmend   Decision = "amended"
	DecisionDefer   Decision = "deferred"
)

func (d Decision) Validate() error {
	switch d {
	case DecisionApprove, DecisionReject, DecisionAmend, DecisionDefer:
		return nil
	default:
		return fmt.Errorf("invalid decision %q", d)
	}
}

// Case is a normalized Senate case file.
type Case struct {
	ID                string   `json:"id"`
	Type              string   `json:"type"`
	Summary           string   `json:"summary"`
	Question          string   `json:"question"`
	Evidence          []string `json:"evidence,omitempty"`
	RequestedDecision string   `json:"requested_decision,omitempty"`
	FiledAt           string   `json:"filed_at"`
	FiledBy           string   `json:"filed_by,omitempty"`
}

func (c *Case) Normalize(now time.Time) {
	c.ID = strings.TrimSpace(c.ID)
	if c.ID == "" {
		c.ID = NewCaseID(now)
	}
	c.Type = strings.TrimSpace(c.Type)
	if c.Type == "" {
		c.Type = "general"
	}
	c.Summary = strings.TrimSpace(c.Summary)
	c.Question = strings.TrimSpace(c.Question)
	if c.Summary == "" {
		c.Summary = c.Question
	}
	if c.Question == "" {
		c.Question = c.Summary
	}
	if strings.TrimSpace(c.FiledAt) == "" {
		c.FiledAt = now.UTC().Format(time.RFC3339)
	}
}

func (c Case) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return errors.New("case.id is required")
	}
	if strings.TrimSpace(c.Type) == "" {
		return errors.New("case.type is required")
	}
	if strings.TrimSpace(c.Summary) == "" {
		return errors.New("case.summary is required")
	}
	if strings.TrimSpace(c.Question) == "" {
		return errors.New("case.question is required")
	}
	if strings.TrimSpace(c.FiledAt) == "" {
		return errors.New("case.filed_at is required")
	}
	if _, err := time.Parse(time.RFC3339, c.FiledAt); err != nil {
		return fmt.Errorf("case.filed_at must be RFC3339: %w", err)
	}
	for i, e := range c.Evidence {
		if strings.TrimSpace(e) == "" {
			return fmt.Errorf("case.evidence[%d] must not be empty", i)
		}
	}
	return nil
}

// PanelMember captures one deliberation participant.
type PanelMember struct {
	AgentID     string `json:"agent_id"`
	Model       string `json:"model"`
	Perspective string `json:"perspective"`
}

// Position captures an agent position at a specific round.
type Position struct {
	AgentID     string   `json:"agent_id"`
	Model       string   `json:"model"`
	Perspective string   `json:"perspective"`
	Round       string   `json:"round"`
	Stance      Decision `json:"stance"`
	Reasoning   string   `json:"reasoning"`
	Concerns    string   `json:"concerns,omitempty"`
}

// Challenge captures one direct challenge between agents.
type Challenge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Challenge string `json:"challenge"`
}

// Transcript is the auditable deliberation output.
type Transcript struct {
	CaseID           string        `json:"case_id"`
	StartedAt        string        `json:"started_at"`
	CompletedAt      string        `json:"completed_at"`
	Panel            []PanelMember `json:"panel"`
	InitialPositions []Position    `json:"initial_positions"`
	Challenges       []Challenge   `json:"challenges"`
	FinalPositions   []Position    `json:"final_positions"`
	JudgeModel       string        `json:"judge_model"`
}

// Handoff stores implementation tracking metadata.
type Handoff struct {
	System    string `json:"system"`
	BeadID    string `json:"bead_id,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at,omitempty"`
}

// Verdict is the binding Senate result.
type Verdict struct {
	CaseID         string     `json:"case_id"`
	FiledAt        string     `json:"filed_at"`
	VerdictAt      string     `json:"verdict_at"`
	Type           string     `json:"type"`
	Summary        string     `json:"summary"`
	Verdict        Decision   `json:"verdict"`
	Reasoning      string     `json:"reasoning"`
	Implementation string     `json:"implementation"`
	Dissent        string     `json:"dissent,omitempty"`
	Binding        bool       `json:"binding"`
	Judge          string     `json:"judge"`
	FinalPositions []Position `json:"final_positions"`
	Handoff        *Handoff   `json:"handoff,omitempty"`
}

func (v Verdict) Validate() error {
	if strings.TrimSpace(v.CaseID) == "" {
		return errors.New("verdict.case_id is required")
	}
	if strings.TrimSpace(v.FiledAt) == "" {
		return errors.New("verdict.filed_at is required")
	}
	if _, err := time.Parse(time.RFC3339, v.FiledAt); err != nil {
		return fmt.Errorf("verdict.filed_at must be RFC3339: %w", err)
	}
	if strings.TrimSpace(v.VerdictAt) == "" {
		return errors.New("verdict.verdict_at is required")
	}
	if _, err := time.Parse(time.RFC3339, v.VerdictAt); err != nil {
		return fmt.Errorf("verdict.verdict_at must be RFC3339: %w", err)
	}
	if strings.TrimSpace(v.Type) == "" {
		return errors.New("verdict.type is required")
	}
	if strings.TrimSpace(v.Summary) == "" {
		return errors.New("verdict.summary is required")
	}
	if err := v.Verdict.Validate(); err != nil {
		return fmt.Errorf("verdict.verdict: %w", err)
	}
	if strings.TrimSpace(v.Reasoning) == "" {
		return errors.New("verdict.reasoning is required")
	}
	if strings.TrimSpace(v.Judge) == "" {
		return errors.New("verdict.judge is required")
	}
	return nil
}
