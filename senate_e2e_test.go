package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Perttulands/senate/internal/core"
	"github.com/Perttulands/senate/internal/precedent"
	"github.com/Perttulands/senate/internal/store"
)

var senateBin string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "senate-e2e-bin-*")
	if err != nil {
		panic(err)
	}
	senateBin = filepath.Join(tmp, "senate")
	cmd := exec.Command("go", "build", "-o", senateBin, "./cmd/senate")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("build senate binary: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

func runSenate(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(senateBin, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("run senate: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// --- E2E Tests ---

func TestE2E_Version(t *testing.T) {
	stdout, _, code := runSenate(t, "version")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(stdout, "senate") {
		t.Errorf("expected 'senate' in version output, got %q", stdout)
	}
}

func TestE2E_Help(t *testing.T) {
	stdout, _, code := runSenate(t, "help")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	for _, word := range []string{"senate", "ask", "start", "health", "file-case", "precedent", "handoff"} {
		if !strings.Contains(stdout, word) {
			t.Errorf("help output missing %q", word)
		}
	}
}

func TestE2E_HelpFlags(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		stdout, _, code := runSenate(t, flag)
		if code != 0 {
			t.Errorf("flag %q: exit code %d", flag, code)
		}
		if !strings.Contains(stdout, "senate") {
			t.Errorf("flag %q: missing 'senate' in output", flag)
		}
	}
}

func TestE2E_UnknownCommand(t *testing.T) {
	_, _, code := runSenate(t, "bogus-command")
	if code != 1 {
		t.Fatalf("expected exit 1 for unknown command, got %d", code)
	}
}

func TestE2E_NoArgs(t *testing.T) {
	_, _, code := runSenate(t)
	if code != 1 {
		t.Fatalf("expected exit 1 for no args, got %d", code)
	}
}

func TestE2E_FileCase(t *testing.T) {
	stateDir := t.TempDir()
	stdout, _, code := runSenate(t, "file-case",
		"--state-dir", stateDir,
		"--type", "general",
		"--summary", "E2E test case",
		"--question", "Does E2E work?",
		"--filed-by", "e2e-test",
	)
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	caseID := strings.TrimSpace(stdout)
	if !strings.HasPrefix(caseID, "senate-") {
		t.Fatalf("expected senate- prefix, got %q", caseID)
	}

	// Verify case was persisted
	d, err := store.New(stateDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	c, err := d.LoadCase(caseID)
	if err != nil {
		t.Fatalf("load case: %v", err)
	}
	if c.Summary != "E2E test case" {
		t.Errorf("summary = %q", c.Summary)
	}
	if c.FiledBy != "e2e-test" {
		t.Errorf("filed_by = %q", c.FiledBy)
	}
}

func TestE2E_FileCaseInvalidType(t *testing.T) {
	stateDir := t.TempDir()
	_, _, code := runSenate(t, "file-case",
		"--state-dir", stateDir,
		"--type", "invalid_type",
		"--summary", "Bad type",
		"--question", "Will this fail?",
	)
	if code != 1 {
		t.Fatalf("expected exit 1 for invalid type, got %d", code)
	}
}

func TestE2E_FileCaseMissingRequired(t *testing.T) {
	stateDir := t.TempDir()
	// Missing --type
	_, _, code := runSenate(t, "file-case",
		"--state-dir", stateDir,
		"--summary", "No type",
		"--question", "Fail?",
	)
	if code != 1 {
		t.Fatalf("expected exit 1 for missing --type, got %d", code)
	}
}

func TestE2E_PrecedentSearch(t *testing.T) {
	stateDir := t.TempDir()

	// Seed a precedent
	d, _ := store.New(stateDir)
	prec := precedent.New(d.PrecedentIndexPath())
	now := time.Now().UTC()
	prec.Add(precedent.Record{
		CaseID:         "senate-e2e-prec",
		Type:           "general",
		Summary:        "E2E precedent for search",
		Verdict:        core.DecisionApprove,
		Reasoning:      "Test reasoning",
		Implementation: "Test implementation",
		Binding:        true,
		VerdictAt:      now.Format(time.RFC3339),
		Judge:          "e2e-test",
	})

	stdout, _, code := runSenate(t, "precedent", "search",
		"--query", "E2E precedent",
		"--state-dir", stateDir,
	)
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(stdout, "senate-e2e-prec") {
		t.Errorf("expected senate-e2e-prec in output, got %q", stdout)
	}
}

func TestE2E_PrecedentSearchJSON(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	prec := precedent.New(d.PrecedentIndexPath())
	now := time.Now().UTC()
	prec.Add(precedent.Record{
		CaseID:         "senate-e2e-json",
		Type:           "architecture",
		Summary:        "JSON output test case",
		Verdict:        core.DecisionReject,
		Reasoning:      "Test reasoning",
		Implementation: "Test impl",
		Binding:        true,
		VerdictAt:      now.Format(time.RFC3339),
		Judge:          "e2e",
	})

	stdout, _, code := runSenate(t, "precedent", "search",
		"--query", "JSON output",
		"--state-dir", stateDir,
		"--json",
	)
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	var results []precedent.Record
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, stdout)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].CaseID != "senate-e2e-json" {
		t.Errorf("case_id = %q, want senate-e2e-json", results[0].CaseID)
	}
}

func TestE2E_PrecedentSearchEmpty(t *testing.T) {
	stateDir := t.TempDir()
	store.New(stateDir)

	stdout, _, code := runSenate(t, "precedent", "search",
		"--query", "nonexistent",
		"--state-dir", stateDir,
	)
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(stdout, "no precedent matches") {
		t.Errorf("expected 'no precedent matches', got %q", stdout)
	}
}

func TestE2E_PrecedentNoSubcommand(t *testing.T) {
	_, _, code := runSenate(t, "precedent")
	if code != 1 {
		t.Fatalf("expected exit 1 for no subcommand, got %d", code)
	}
}

func TestE2E_HandoffMissingCaseID(t *testing.T) {
	_, _, code := runSenate(t, "handoff")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestE2E_HandoffMissingVerdict(t *testing.T) {
	stateDir := t.TempDir()
	store.New(stateDir)
	_, _, code := runSenate(t, "handoff",
		"--case-id", "nonexistent",
		"--state-dir", stateDir,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestE2E_Health(t *testing.T) {
	stdout, _, code := runSenate(t, "health")
	// Exit code depends on whether claude CLI is installed
	if code != 0 && code != 1 {
		t.Fatalf("expected exit 0 or 1, got %d", code)
	}
	if !strings.Contains(stdout, "Claude CLI") {
		t.Errorf("expected 'Claude CLI' in health output, got %q", stdout)
	}
}

func TestE2E_FileCaseAndPrecedentRoundtrip(t *testing.T) {
	stateDir := t.TempDir()

	// File a case via the binary
	stdout, _, code := runSenate(t, "file-case",
		"--state-dir", stateDir,
		"--type", "architecture",
		"--summary", "Roundtrip architecture decision",
		"--question", "Should we use microservices?",
		"--filed-by", "e2e",
		"--evidence", "design-doc.md",
	)
	if code != 0 {
		t.Fatalf("file-case exit code %d", code)
	}
	caseID := strings.TrimSpace(stdout)

	// Seed a matching precedent
	d, _ := store.New(stateDir)
	prec := precedent.New(d.PrecedentIndexPath())
	now := time.Now().UTC()
	prec.Add(precedent.Record{
		CaseID:         caseID,
		Type:           "architecture",
		Summary:        "Roundtrip architecture decision",
		Verdict:        core.DecisionApprove,
		Reasoning:      "Microservices work well",
		Implementation: "Use Docker",
		Binding:        true,
		VerdictAt:      now.Format(time.RFC3339),
		Judge:          "e2e",
	})

	// Search for the precedent
	stdout, _, code = runSenate(t, "precedent", "search",
		"--query", "architecture microservices",
		"--state-dir", stateDir,
	)
	if code != 0 {
		t.Fatalf("precedent search exit code %d", code)
	}
	if !strings.Contains(stdout, caseID) {
		t.Errorf("expected %s in search results, got %q", caseID, stdout)
	}
}
