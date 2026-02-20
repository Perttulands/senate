package deliberation

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Perttulands/senate/internal/core"
)

// Engine runs the Senate deliberation protocol.
type Engine struct {
	Panel      []Perspective
	JudgeModel string
}

func New(panel []Perspective) *Engine {
	if len(panel) == 0 {
		panel = BuildPanel(3, nil, nil)
	}
	return &Engine{
		Panel:      panel,
		JudgeModel: "claude:opus",
	}
}

// Deliberate executes initial position, challenge, final position, and verdict synthesis.
func (e *Engine) Deliberate(c core.Case, now time.Time) (core.Transcript, core.Verdict, error) {
	if err := c.Validate(); err != nil {
		return core.Transcript{}, core.Verdict{}, err
	}
	started := now.UTC()
	panelMembers := toPanelMembers(e.Panel)

	initial := make([]core.Position, 0, len(panelMembers))
	for i, seat := range panelMembers {
		p := e.Panel[i]
		stance, reason, concerns := evaluateInitial(c, p)
		initial = append(initial, core.Position{
			AgentID:     seat.AgentID,
			Model:       seat.Model,
			Perspective: seat.Perspective,
			Round:       "initial",
			Stance:      stance,
			Reasoning:   reason,
			Concerns:    concerns,
		})
	}

	challenges := buildChallenges(c, initial)
	final := finalizePositions(c, initial, challenges)
	verdict := synthesizeVerdict(c, final, e.JudgeModel, started.Add(2*time.Minute))

	transcript := core.Transcript{
		CaseID:           c.ID,
		StartedAt:        started.Format(time.RFC3339),
		CompletedAt:      started.Add(2 * time.Minute).Format(time.RFC3339),
		Panel:            panelMembers,
		InitialPositions: initial,
		Challenges:       challenges,
		FinalPositions:   final,
		JudgeModel:       e.JudgeModel,
	}
	return transcript, verdict, nil
}

func evaluateInitial(c core.Case, p Perspective) (core.Decision, string, string) {
	risk := tokenScore(c.Question+" "+c.Summary, []string{"security", "unsafe", "drop", "delete", "disable", "bypass", "without tests", "rollback"})
	urgency := tokenScore(c.Question+" "+c.Summary, []string{"urgent", "blocker", "ship", "today", "immediately", "unblock"})
	evidenceWeight := len(c.Evidence)

	switch p.Name {
	case "pragmatist":
		if risk >= 2 {
			return core.DecisionReject, "The change introduces high risk compared to delivery value.", "Risk reduction plan is missing."
		}
		if urgency >= 1 || evidenceWeight >= 2 {
			return core.DecisionApprove, "The path is actionable now and clears immediate delivery constraints.", "Document rollback and ownership."
		}
		return core.DecisionAmend, "Direction is viable but needs tighter scope before execution.", "Define measurable acceptance criteria."
	case "purist":
		if risk >= 1 {
			return core.DecisionReject, "Correctness and safety guarantees are not strong enough for approval.", "Failure modes are under-specified."
		}
		if evidenceWeight == 0 {
			return core.DecisionDefer, "There is not enough evidence to make a durable decision.", "Need concrete examples or data."
		}
		return core.DecisionAmend, "The proposal is directionally sound but requires stronger invariants.", "Specify exact rule boundaries."
	case "skeptic":
		if evidenceWeight == 0 {
			return core.DecisionDefer, "The case lacks objective evidence and should not be bound yet.", "Gather incidents, diffs, or metrics first."
		}
		if risk >= 1 {
			return core.DecisionReject, "Edge-case risk remains unresolved under realistic failure scenarios.", "Mitigations are implied but not explicit."
		}
		return core.DecisionAmend, "Adopt with guardrails to contain unknowns.", "Time-box follow-up validation."
	default:
		if risk >= 2 {
			return core.DecisionReject, "Risk exceeds confidence in current plan.", "Need safer rollout shape."
		}
		if evidenceWeight >= 1 {
			return core.DecisionAmend, "Proceed with modifications grounded in the provided evidence.", "Capture precedent terms explicitly."
		}
		return core.DecisionDefer, "Insufficient evidence for a binding conclusion.", "Collect at least one concrete artifact."
	}
}

func buildChallenges(c core.Case, initial []core.Position) []core.Challenge {
	challenges := make([]core.Challenge, 0, len(initial))
	for _, pos := range initial {
		target := strongestCounter(pos.Stance, initial)
		if target.AgentID == "" {
			continue
		}
		text := fmt.Sprintf("Your %s stance underweights %s tradeoffs for case %s.", strings.ToLower(string(target.Stance)), strings.ToLower(string(pos.Stance)), c.ID)
		challenges = append(challenges, core.Challenge{
			From:      pos.AgentID,
			To:        target.AgentID,
			Challenge: text,
		})
	}
	return challenges
}

func strongestCounter(stance core.Decision, positions []core.Position) core.Position {
	for _, p := range positions {
		if p.Stance != stance {
			return p
		}
	}
	return core.Position{}
}

func finalizePositions(c core.Case, initial []core.Position, challenges []core.Challenge) []core.Position {
	counts := countDecisions(initial)
	majority := majorityDecision(counts)
	final := make([]core.Position, 0, len(initial))

	for _, p := range initial {
		out := p
		out.Round = "final"
		if majority != "" && p.Stance != majority {
			if p.Stance == core.DecisionApprove && (majority == core.DecisionReject || majority == core.DecisionDefer) {
				out.Stance = core.DecisionAmend
				out.Reasoning = "After challenge review, approval is too broad; amendment better matches observed risk."
			}
			if p.Stance == core.DecisionReject && majority == core.DecisionApprove {
				out.Stance = core.DecisionAmend
				out.Reasoning = "After challenge review, bounded amendment is safer than outright rejection."
			}
		}
		final = append(final, out)
	}

	if len(challenges) == 0 && len(c.Evidence) == 0 {
		for i := range final {
			if final[i].Stance == core.DecisionApprove {
				final[i].Stance = core.DecisionAmend
				final[i].Reasoning = "Without challenges or evidence, amendment is the safer consensus posture."
			}
		}
	}
	return final
}

func synthesizeVerdict(c core.Case, final []core.Position, judge string, verdictAt time.Time) core.Verdict {
	counts := countDecisions(final)
	decision := majorityDecision(counts)
	if decision == "" {
		decision = core.DecisionDefer
	}

	majorityReasons := make([]string, 0, len(final))
	minorityReasons := make([]string, 0, len(final))
	for _, p := range final {
		if p.Stance == decision {
			majorityReasons = append(majorityReasons, p.Reasoning)
		} else {
			minorityReasons = append(minorityReasons, fmt.Sprintf("%s: %s", p.AgentID, p.Reasoning))
		}
	}

	reasoning := strings.TrimSpace(strings.Join(uniqueFirstN(majorityReasons, 2), " "))
	if reasoning == "" {
		reasoning = "Panel did not converge strongly; defaulting to defer for safety."
	}
	implementation := buildImplementationText(c, decision)
	binding := decision != core.DecisionDefer

	return core.Verdict{
		CaseID:         c.ID,
		FiledAt:        c.FiledAt,
		VerdictAt:      verdictAt.UTC().Format(time.RFC3339),
		Type:           c.Type,
		Summary:        c.Summary,
		Verdict:        decision,
		Reasoning:      reasoning,
		Implementation: implementation,
		Dissent:        strings.Join(uniqueFirstN(minorityReasons, 2), " | "),
		Binding:        binding,
		Judge:          judge,
		FinalPositions: final,
	}
}

func buildImplementationText(c core.Case, decision core.Decision) string {
	if strings.TrimSpace(c.RequestedDecision) != "" {
		return c.RequestedDecision
	}
	switch decision {
	case core.DecisionApprove:
		return "Proceed with implementation as proposed and document this verdict as precedent."
	case core.DecisionReject:
		return "Do not implement the requested change; file a follow-up with safer alternatives."
	case core.DecisionAmend:
		return "Implement a narrowed version with explicit guardrails and measurable acceptance criteria."
	default:
		return "Collect additional evidence and re-file the case for renewed deliberation."
	}
}

func countDecisions(positions []core.Position) map[core.Decision]int {
	counts := map[core.Decision]int{
		core.DecisionApprove: 0,
		core.DecisionReject:  0,
		core.DecisionAmend:   0,
		core.DecisionDefer:   0,
	}
	for _, p := range positions {
		counts[p.Stance]++
	}
	return counts
}

func majorityDecision(counts map[core.Decision]int) core.Decision {
	type pair struct {
		decision core.Decision
		count    int
	}
	ordered := []pair{
		{decision: core.DecisionApprove, count: counts[core.DecisionApprove]},
		{decision: core.DecisionReject, count: counts[core.DecisionReject]},
		{decision: core.DecisionAmend, count: counts[core.DecisionAmend]},
		{decision: core.DecisionDefer, count: counts[core.DecisionDefer]},
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].count > ordered[j].count
	})
	if ordered[0].count == 0 {
		return ""
	}
	if len(ordered) > 1 && ordered[0].count == ordered[1].count {
		return core.DecisionDefer
	}
	return ordered[0].decision
}

func tokenScore(text string, tokens []string) int {
	text = strings.ToLower(text)
	score := 0
	for _, tok := range tokens {
		if strings.Contains(text, tok) {
			score++
		}
	}
	return score
}

func uniqueFirstN(items []string, n int) []string {
	if n <= 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, n)
	for _, item := range items {
		clean := strings.TrimSpace(item)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
		if len(out) == n {
			break
		}
	}
	return out
}
