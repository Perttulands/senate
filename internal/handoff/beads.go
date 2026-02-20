package handoff

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Perttulands/senate/internal/core"
)

var beadIDPattern = regexp.MustCompile(`([A-Za-z0-9]+-[A-Za-z0-9][A-Za-z0-9-]*)`)

// Runner executes external commands (bd create).
type Runner interface {
	Run(ctx context.Context, name string, args []string, dir string) (string, error)
}

type execRunner struct{}

func (e execRunner) Run(ctx context.Context, name string, args []string, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Result captures bead creation status.
type Result struct {
	BeadID string
	Title  string
	Status string
}

// CreateBeadForVerdict creates a Beads issue for binding implementation work.
func CreateBeadForVerdict(ctx context.Context, runner Runner, workspaceDir string, verdict core.Verdict) (Result, error) {
	if runner == nil {
		runner = execRunner{}
	}
	if !verdict.Binding || verdict.Verdict == core.DecisionDefer {
		return Result{Status: "skipped"}, nil
	}
	if workspaceDir == "" {
		workspaceDir = defaultWorkspaceDir()
	}

	target := inferTargetSystem(verdict.Type)
	title := fmt.Sprintf("[%s] Senate %s: %s", target, verdict.CaseID, trimTo(verdict.Summary, 80))
	description := strings.TrimSpace(fmt.Sprintf("Binding Senate verdict for case %s\n\nVerdict: %s\nReasoning: %s\nImplementation: %s\n", verdict.CaseID, verdict.Verdict, verdict.Reasoning, verdict.Implementation))

	args := []string{"create", "--title", title, "--priority", "2", "--description", description, "--silent"}
	out, err := runner.Run(ctx, "bd", args, workspaceDir)
	if err != nil {
		return Result{}, fmt.Errorf("bd create failed: %w (%s)", err, strings.TrimSpace(out))
	}

	beadID := parseBeadID(out)
	if beadID == "" {
		return Result{}, fmt.Errorf("unable to parse bead id from output %q", strings.TrimSpace(out))
	}

	return Result{BeadID: beadID, Title: title, Status: "created"}, nil
}

func parseBeadID(out string) string {
	clean := strings.TrimSpace(out)
	if clean == "" {
		return ""
	}
	lines := strings.Split(clean, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if beadIDPattern.MatchString(last) {
		return beadIDPattern.FindString(last)
	}
	if beadIDPattern.MatchString(clean) {
		return beadIDPattern.FindString(clean)
	}
	return ""
}

func inferTargetSystem(caseType string) string {
	switch strings.TrimSpace(caseType) {
	case "rule_evolution":
		return "truthsayer"
	case "gate_criteria":
		return "centurion"
	case "priority_triage":
		return "athena"
	case "dispute_resolution":
		return "athena"
	default:
		return "athena"
	}
}

func trimTo(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	if max <= 1 {
		return ""
	}
	return string(r[:max-1]) + "..."
}

func defaultWorkspaceDir() string {
	if ws := strings.TrimSpace(os.Getenv("ATHENA_WORKSPACE")); ws != "" {
		return ws
	}
	return filepath.FromSlash("/home/chrote/athena/workspace")
}
