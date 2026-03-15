package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Perttulands/senate/internal/core"
	"github.com/Perttulands/senate/internal/precedent"
	"github.com/Perttulands/senate/internal/store"
)

// --- helpers for capturing stdout/stderr ---

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// --- Run dispatcher ---

func TestRun_NoArgs(t *testing.T) {
	code := Run([]string{"senate"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestRun_EmptyArgs(t *testing.T) {
	code := Run([]string{})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestRun_Help(t *testing.T) {
	for _, flag := range []string{"help", "-h", "--help"} {
		out := captureStdout(t, func() {
			code := Run([]string{"senate", flag})
			if code != 0 {
				t.Errorf("help flag %q: expected exit 0, got %d", flag, code)
			}
		})
		if !strings.Contains(out, "senate") {
			t.Errorf("help flag %q: expected usage output", flag)
		}
	}
}

func TestRun_Version(t *testing.T) {
	out := captureStdout(t, func() {
		code := Run([]string{"senate", "version"})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})
	if !strings.Contains(out, Version) {
		t.Errorf("version output should contain %q, got %q", Version, out)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	code := Run([]string{"senate", "bogus"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

// --- cmdFileCase ---

func TestCmdFileCase_MissingType(t *testing.T) {
	stateDir := t.TempDir()
	code := cmdFileCase([]string{
		"--state-dir", stateDir,
		"--summary", "Test summary",
		"--question", "Test question?",
	})
	if code != 1 {
		t.Fatalf("expected exit 1 for missing --type, got %d", code)
	}
}

func TestCmdFileCase_MissingSummary(t *testing.T) {
	stateDir := t.TempDir()
	code := cmdFileCase([]string{
		"--state-dir", stateDir,
		"--type", "general",
		"--question", "Test question?",
	})
	if code != 1 {
		t.Fatalf("expected exit 1 for missing --summary, got %d", code)
	}
}

func TestCmdFileCase_MissingQuestion(t *testing.T) {
	stateDir := t.TempDir()
	code := cmdFileCase([]string{
		"--state-dir", stateDir,
		"--type", "general",
		"--summary", "Test summary",
	})
	if code != 1 {
		t.Fatalf("expected exit 1 for missing --question, got %d", code)
	}
}

func TestCmdFileCase_InvalidType(t *testing.T) {
	stateDir := t.TempDir()
	code := cmdFileCase([]string{
		"--state-dir", stateDir,
		"--type", "invalid_type",
		"--summary", "Test summary",
		"--question", "Test question?",
	})
	if code != 1 {
		t.Fatalf("expected exit 1 for invalid type, got %d", code)
	}
}

func TestCmdFileCase_Success(t *testing.T) {
	stateDir := t.TempDir()
	var code int
	out := captureStdout(t, func() {
		code = cmdFileCase([]string{
			"--state-dir", stateDir,
			"--type", "general",
			"--summary", "Test summary",
			"--question", "Should we do this?",
			"--filed-by", "tester",
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	caseID := strings.TrimSpace(out)
	if !strings.HasPrefix(caseID, "senate-") {
		t.Fatalf("expected case ID starting with senate-, got %q", caseID)
	}

	// Verify case file was written
	d, err := store.New(stateDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	c, err := d.LoadCase(caseID)
	if err != nil {
		t.Fatalf("load case: %v", err)
	}
	if c.Summary != "Test summary" {
		t.Errorf("summary = %q, want %q", c.Summary, "Test summary")
	}
	if c.FiledBy != "tester" {
		t.Errorf("filed_by = %q, want %q", c.FiledBy, "tester")
	}
}

func TestCmdFileCase_AllValidTypes(t *testing.T) {
	validTypes := []string{"rule_evolution", "gate_criteria", "dispute", "priority", "architecture", "general"}
	for _, typ := range validTypes {
		stateDir := t.TempDir()
		var code int
		captureStdout(t, func() {
			code = cmdFileCase([]string{
				"--state-dir", stateDir,
				"--type", typ,
				"--summary", "Test",
				"--question", "Question?",
			})
		})
		if code != 0 {
			t.Errorf("type %q: expected exit 0, got %d", typ, code)
		}
	}
}

func TestCmdFileCase_WithEvidence(t *testing.T) {
	stateDir := t.TempDir()
	var code int
	out := captureStdout(t, func() {
		code = cmdFileCase([]string{
			"--state-dir", stateDir,
			"--type", "general",
			"--summary", "Test",
			"--question", "Question?",
			"--evidence", "file1.md",
			"--evidence", "file2.md",
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	caseID := strings.TrimSpace(out)
	d, _ := store.New(stateDir)
	c, err := d.LoadCase(caseID)
	if err != nil {
		t.Fatalf("load case: %v", err)
	}
	if len(c.Evidence) != 2 {
		t.Errorf("expected 2 evidence items, got %d", len(c.Evidence))
	}
}

// --- cmdPrecedent ---

func TestCmdPrecedent_NoSubcommand(t *testing.T) {
	code := cmdPrecedent([]string{})
	if code != 1 {
		t.Fatalf("expected exit 1 for no subcommand, got %d", code)
	}
}

func TestCmdPrecedent_UnknownSubcommand(t *testing.T) {
	code := cmdPrecedent([]string{"bogus"})
	if code != 1 {
		t.Fatalf("expected exit 1 for unknown subcommand, got %d", code)
	}
}

func TestCmdPrecedent_SearchEmpty(t *testing.T) {
	stateDir := t.TempDir()
	// Initialize store so precedent dir exists
	store.New(stateDir)

	var code int
	out := captureStdout(t, func() {
		code = cmdPrecedent([]string{
			"search", "--query", "anything", "--state-dir", stateDir,
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "no precedent matches") {
		t.Errorf("expected 'no precedent matches', got %q", out)
	}
}

func TestCmdPrecedent_SearchWithResults(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	prec := precedent.New(d.PrecedentIndexPath())
	now := time.Now().UTC()
	prec.Add(precedent.Record{
		CaseID:         "senate-test-1",
		Type:           "general",
		Summary:        "Coverage threshold debate",
		Verdict:        core.DecisionApprove,
		Reasoning:      "Good coverage improves quality",
		Implementation: "Set threshold to 70%",
		Binding:        true,
		VerdictAt:      now.Format(time.RFC3339),
		Judge:          "claude:test",
	})

	var code int
	out := captureStdout(t, func() {
		code = cmdPrecedent([]string{
			"search", "--query", "coverage threshold", "--state-dir", stateDir,
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "senate-test-1") {
		t.Errorf("expected to find senate-test-1 in output, got %q", out)
	}
}

func TestCmdPrecedent_SearchJSON(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	prec := precedent.New(d.PrecedentIndexPath())
	now := time.Now().UTC()
	prec.Add(precedent.Record{
		CaseID:         "senate-json-1",
		Type:           "general",
		Summary:        "JSON output test",
		Verdict:        core.DecisionReject,
		Reasoning:      "Reasoning",
		Implementation: "Implementation",
		Binding:        true,
		VerdictAt:      now.Format(time.RFC3339),
		Judge:          "claude:test",
	})

	var code int
	out := captureStdout(t, func() {
		code = cmdPrecedent([]string{
			"search", "--query", "JSON output", "--state-dir", stateDir, "--json",
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// Verify it's valid JSON array
	var results []precedent.Record
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].CaseID != "senate-json-1" {
		t.Errorf("result case_id = %q, want senate-json-1", results[0].CaseID)
	}
}

func TestCmdPrecedent_SearchWithTypeFilter(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	prec := precedent.New(d.PrecedentIndexPath())
	now := time.Now().UTC()
	prec.Add(precedent.Record{
		CaseID: "senate-t1", Type: "rule_evolution", Summary: "Rule test",
		Verdict: core.DecisionAmend, Reasoning: "R", Implementation: "I",
		Binding: true, VerdictAt: now.Format(time.RFC3339), Judge: "test",
	})
	prec.Add(precedent.Record{
		CaseID: "senate-t2", Type: "general", Summary: "General test",
		Verdict: core.DecisionApprove, Reasoning: "R", Implementation: "I",
		Binding: true, VerdictAt: now.Format(time.RFC3339), Judge: "test",
	})

	var code int
	out := captureStdout(t, func() {
		code = cmdPrecedent([]string{
			"search", "--query", "test", "--type", "rule_evolution", "--state-dir", stateDir,
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "senate-t1") {
		t.Error("expected senate-t1 in filtered results")
	}
	if strings.Contains(out, "senate-t2") {
		t.Error("senate-t2 should be filtered out by type")
	}
}

// --- cmdHandoff ---

func TestCmdHandoff_MissingCaseID(t *testing.T) {
	code := cmdHandoff([]string{})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestCmdHandoff_VerdictNotFound(t *testing.T) {
	stateDir := t.TempDir()
	store.New(stateDir)
	code := cmdHandoff([]string{"--case-id", "nonexistent", "--state-dir", stateDir})
	if code != 1 {
		t.Fatalf("expected exit 1 for missing verdict, got %d", code)
	}
}

func TestCmdHandoff_AlreadyHandedOff(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	now := time.Now().UTC()
	v := core.Verdict{
		CaseID:    "senate-ho-1",
		FiledAt:   now.Format(time.RFC3339),
		VerdictAt: now.Format(time.RFC3339),
		Type:      "general",
		Summary:   "Already handed off",
		Verdict:   core.DecisionApprove,
		Reasoning: "R",
		Judge:     "test",
		Binding:   true,
		Handoff: &core.Handoff{
			System:    "athena",
			BeadID:    "existing-bead-123",
			Status:    "created",
			CreatedAt: now.Format(time.RFC3339),
		},
	}
	d.SaveVerdict(v)

	var code int
	out := captureStdout(t, func() {
		code = cmdHandoff([]string{"--case-id", "senate-ho-1", "--state-dir", stateDir})
	})
	if code != 0 {
		t.Fatalf("expected exit 0 for already handed off, got %d", code)
	}
	if !strings.Contains(out, "existing-bead-123") {
		t.Errorf("expected existing bead ID in output, got %q", out)
	}
}

// --- loadCase ---

func TestLoadCase_QuickQuestion(t *testing.T) {
	c, err := loadCase("", "Should we do this?", "tester")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Question != "Should we do this?" {
		t.Errorf("question = %q", c.Question)
	}
	if c.Type != "general" {
		t.Errorf("type = %q, want general", c.Type)
	}
	if c.FiledBy != "tester" {
		t.Errorf("filed_by = %q, want tester", c.FiledBy)
	}
}

func TestLoadCase_NoCaseNoQuestion(t *testing.T) {
	_, err := loadCase("", "", "")
	if err == nil {
		t.Fatal("expected error for no case and no question")
	}
}

func TestLoadCase_FromFile(t *testing.T) {
	caseFile := filepath.Join(t.TempDir(), "case.json")
	c := core.Case{
		ID:       "senate-file-1",
		Type:     "architecture",
		Summary:  "File-based case",
		Question: "From file?",
		FiledAt:  time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(c)
	os.WriteFile(caseFile, data, 0644)

	loaded, err := loadCase(caseFile, "", "override-filer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.ID != "senate-file-1" {
		t.Errorf("id = %q", loaded.ID)
	}
	if loaded.FiledBy != "override-filer" {
		t.Errorf("filed_by should be set when empty in file, got %q", loaded.FiledBy)
	}
}

func TestLoadCase_FromFileWithFiledBy(t *testing.T) {
	caseFile := filepath.Join(t.TempDir(), "case.json")
	c := core.Case{
		ID:       "senate-file-2",
		Type:     "general",
		Summary:  "Has filer",
		Question: "Q?",
		FiledAt:  time.Now().UTC().Format(time.RFC3339),
		FiledBy:  "original-filer",
	}
	data, _ := json.Marshal(c)
	os.WriteFile(caseFile, data, 0644)

	loaded, err := loadCase(caseFile, "", "override")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should keep original filer since it's not empty
	if loaded.FiledBy != "original-filer" {
		t.Errorf("filed_by = %q, want original-filer", loaded.FiledBy)
	}
}

func TestLoadCase_InvalidJSON(t *testing.T) {
	caseFile := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(caseFile, []byte("not json"), 0644)
	_, err := loadCase(caseFile, "", "")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadCase_MissingFile(t *testing.T) {
	_, err := loadCase("/nonexistent/case.json", "", "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// --- Helper functions ---

func TestParseFlags(t *testing.T) {
	flags := parseFlags([]string{"--type", "general", "--agents", "3", "--verbose"})
	if flags["type"] != "general" {
		t.Errorf("type = %q", flags["type"])
	}
	if flags["agents"] != "3" {
		t.Errorf("agents = %q", flags["agents"])
	}
	if flags["verbose"] != "true" {
		t.Errorf("verbose = %q", flags["verbose"])
	}
}

func TestParseFlags_Empty(t *testing.T) {
	flags := parseFlags([]string{})
	if len(flags) != 0 {
		t.Errorf("expected empty flags, got %v", flags)
	}
}

func TestParseFlags_PositionalIgnored(t *testing.T) {
	flags := parseFlags([]string{"positional", "--key", "value"})
	if flags["key"] != "value" {
		t.Errorf("key = %q", flags["key"])
	}
	if _, ok := flags["positional"]; ok {
		t.Error("positional arg should not be in flags")
	}
}

func TestFlagBool(t *testing.T) {
	args := []string{"--json", "--verbose", "positional"}
	if !flagBool(args, "--json") {
		t.Error("expected --json to be true")
	}
	if !flagBool(args, "--verbose") {
		t.Error("expected --verbose to be true")
	}
	if flagBool(args, "--missing") {
		t.Error("expected --missing to be false")
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		raw      string
		fallback int
		want     int
	}{
		{"3", 5, 3},
		{"", 5, 5},
		{"0", 5, 5},
		{"-1", 5, 5},
		{"abc", 5, 5},
		{" 7 ", 5, 7},
	}
	for _, tt := range tests {
		got := parseInt(tt.raw, tt.fallback)
		if got != tt.want {
			t.Errorf("parseInt(%q, %d) = %d, want %d", tt.raw, tt.fallback, got, tt.want)
		}
	}
}

func TestParseDecision(t *testing.T) {
	tests := []struct {
		input string
		want  core.Decision
	}{
		{"approve", core.DecisionApprove},
		{"approved", core.DecisionApprove},
		{"reject", core.DecisionReject},
		{"rejected", core.DecisionReject},
		{"amend", core.DecisionAmend},
		{"amended", core.DecisionAmend},
		{"defer", core.DecisionDefer},
		{"deferred", core.DecisionDefer},
		{"APPROVE", core.DecisionApprove},
		{"Rejected", core.DecisionReject},
		{"nonsense", ""},
		{"", ""},
		{"  approve  ", core.DecisionApprove},
	}
	for _, tt := range tests {
		got := parseDecision(tt.input)
		if got != tt.want {
			t.Errorf("parseDecision(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"a, b,,c", 3},
		{"", 0},
		{"   ", 0},
		{"single", 1},
		{" a , b , c , d ", 4},
	}
	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitCSV(%q) = %d parts, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestExtractPositionalArg(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"question here"}, "question here"},
		{[]string{"--flag", "value", "positional"}, "positional"},
		{[]string{"--flag", "value"}, ""},
		{[]string{}, ""},
		{[]string{"--solo-flag", "arg"}, ""},  // "arg" consumed as flag value
	}
	for _, tt := range tests {
		got := extractPositionalArg(tt.args)
		if got != tt.want {
			t.Errorf("extractPositionalArg(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestCollectEvidence(t *testing.T) {
	args := []string{"--evidence", "file1.md", "--type", "general", "--evidence", "file2.md"}
	ev := collectEvidence(args)
	if len(ev) != 2 {
		t.Fatalf("expected 2 evidence items, got %d", len(ev))
	}
	if ev[0] != "file1.md" || ev[1] != "file2.md" {
		t.Errorf("evidence = %v", ev)
	}
}

func TestCollectEvidence_Empty(t *testing.T) {
	ev := collectEvidence([]string{"--type", "general"})
	if len(ev) != 0 {
		t.Errorf("expected no evidence, got %v", ev)
	}
}

func TestCollectEvidence_EmptyValue(t *testing.T) {
	ev := collectEvidence([]string{"--evidence", "  "})
	if len(ev) != 0 {
		t.Errorf("expected no evidence for whitespace value, got %v", ev)
	}
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}
	if !contains(slice, "b") {
		t.Error("expected true for 'b'")
	}
	if contains(slice, "d") {
		t.Error("expected false for 'd'")
	}
	if contains(nil, "a") {
		t.Error("expected false for nil slice")
	}
}

func TestResolveStateDir(t *testing.T) {
	// Explicit flag
	if got := resolveStateDir("/custom/path"); got != "/custom/path" {
		t.Errorf("expected /custom/path, got %q", got)
	}

	// Empty falls back to env or "state"
	old := os.Getenv("SENATE_STATE_DIR")
	os.Setenv("SENATE_STATE_DIR", "/env/path")
	defer os.Setenv("SENATE_STATE_DIR", old)
	if got := resolveStateDir(""); got != "/env/path" {
		t.Errorf("expected /env/path from env, got %q", got)
	}

	os.Setenv("SENATE_STATE_DIR", "")
	if got := resolveStateDir(""); got != "state" {
		t.Errorf("expected 'state' default, got %q", got)
	}
}

func TestInferTargetSystem(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"rule_evolution", "truthsayer"},
		{"gate_criteria", "centurion"},
		{"general", "athena"},
		{"", "athena"},
		{"unknown", "athena"},
	}
	for _, tt := range tests {
		got := inferTargetSystem(tt.input)
		if got != tt.want {
			t.Errorf("inferTargetSystem(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestUsage(t *testing.T) {
	out := captureStdout(t, func() {
		usage()
	})
	required := []string{"senate", "ask", "start", "health", "file-case", "precedent", "handoff", "version"}
	for _, word := range required {
		if !strings.Contains(out, word) {
			t.Errorf("usage output missing %q", word)
		}
	}
}

func TestOutputJSON(t *testing.T) {
	out := captureStdout(t, func() {
		outputJSON(map[string]string{"key": "value"})
	})
	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("outputJSON produced invalid JSON: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("key = %q, want 'value'", m["key"])
	}
}

func TestErrorf(t *testing.T) {
	out := captureStderr(t, func() {
		errorf("test %s %d", "error", 42)
	})
	if !strings.Contains(out, "senate: test error 42") {
		t.Errorf("errorf output = %q", out)
	}
}

// --- cmdAsk validation path (no claude needed) ---

func TestCmdAsk_NoQuestion(t *testing.T) {
	code := cmdAsk([]string{})
	if code != 1 {
		t.Fatalf("expected exit 1 for no question, got %d", code)
	}
}

// --- Run integration: file-case through Run ---

func TestRun_FileCase_EndToEnd(t *testing.T) {
	stateDir := t.TempDir()
	var code int
	out := captureStdout(t, func() {
		code = Run([]string{"senate", "file-case",
			"--state-dir", stateDir,
			"--type", "architecture",
			"--summary", "End to end test",
			"--question", "Does Run dispatch correctly?",
			"--filed-by", "test",
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	caseID := strings.TrimSpace(out)
	if !strings.HasPrefix(caseID, "senate-") {
		t.Fatalf("expected senate- prefix, got %q", caseID)
	}
}

// --- Run integration: precedent through Run ---

func TestRun_Precedent_EndToEnd(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	prec := precedent.New(d.PrecedentIndexPath())
	now := time.Now().UTC()
	prec.Add(precedent.Record{
		CaseID: "senate-e2e", Type: "general", Summary: "E2E precedent",
		Verdict: core.DecisionApprove, Reasoning: "R", Implementation: "I",
		Binding: true, VerdictAt: now.Format(time.RFC3339), Judge: "test",
	})

	var code int
	out := captureStdout(t, func() {
		code = Run([]string{"senate", "precedent", "search",
			"--query", "E2E", "--state-dir", stateDir,
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "senate-e2e") {
		t.Errorf("expected senate-e2e in output: %q", out)
	}
}

// --- Run integration: handoff through Run ---

func TestRun_Handoff_MissingCaseID(t *testing.T) {
	code := Run([]string{"senate", "handoff"})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

// --- Version constant ---

func TestVersionNotEmpty(t *testing.T) {
	if Version == "" {
		t.Fatal("Version should not be empty")
	}
	parts := strings.Split(Version, ".")
	if len(parts) != 3 {
		t.Errorf("Version %q should be semver (x.y.z)", Version)
	}
}

// Ensure countDecisions is not needed — just verify the exported types compile
func TestAskResultStructure(t *testing.T) {
	r := AskResult{
		CaseID:    "test",
		Verdict:   "approved",
		Reasoning: "r",
		Positions: []AskPosition{{Senator: "s", Stance: "approved", KeyArgument: "a"}},
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal AskResult: %v", err)
	}
	var decoded AskResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal AskResult: %v", err)
	}
	if decoded.CaseID != "test" {
		t.Errorf("decoded case_id = %q", decoded.CaseID)
	}
	if len(decoded.Positions) != 1 {
		t.Errorf("expected 1 position, got %d", len(decoded.Positions))
	}
}

// --- cmdHealth ---

func TestCmdHealth_Basic(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = cmdHealth([]string{})
	})
	// Exit code depends on whether claude CLI is installed
	if code != 0 && code != 1 {
		t.Fatalf("expected exit 0 or 1, got %d", code)
	}
	if !strings.Contains(out, "Claude CLI") {
		t.Errorf("expected 'Claude CLI' in output, got %q", out)
	}
}

func TestCmdHealth_Verbose(t *testing.T) {
	var code int
	out := captureStdout(t, func() {
		code = cmdHealth([]string{"--verbose"})
	})
	if code != 0 && code != 1 {
		t.Fatalf("expected exit 0 or 1, got %d", code)
	}
	// Verbose mode should show either "Path:" or "Error:" depending on whether claude is installed
	if !strings.Contains(out, "Path:") && !strings.Contains(out, "Error:") {
		t.Errorf("verbose output should contain 'Path:' or 'Error:', got %q", out)
	}
}

// --- cmdHandoff extended ---

func TestCmdHandoff_WorkspaceFallbackToGetwd(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	now := time.Now().UTC()
	v := core.Verdict{
		CaseID:    "senate-ws-fallback",
		FiledAt:   now.Format(time.RFC3339),
		VerdictAt: now.Format(time.RFC3339),
		Type:      "general",
		Summary:   "Test workspace fallback",
		Verdict:   core.DecisionDefer,
		Reasoning: "R",
		Judge:     "test",
		Binding:   true,
	}
	d.SaveVerdict(v)

	// No --workspace flag: cmdHandoff should use os.Getwd() fallback, not error
	var code int
	out := captureStdout(t, func() {
		code = cmdHandoff([]string{"--case-id", "senate-ws-fallback", "--state-dir", stateDir})
	})
	// Deferred verdict → skipped (no bead), but the point is it shouldn't
	// error due to empty workspace since os.Getwd() is used as fallback
	if code != 0 {
		t.Fatalf("expected exit 0 (deferred=skipped), got %d", code)
	}
	if !strings.Contains(out, "skipped") {
		t.Errorf("expected 'skipped', got %q", out)
	}
}

func TestCmdHandoff_BindingVerdictUsesGetwdWhenNoWorkspaceFlag(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	now := time.Now().UTC()
	v := core.Verdict{
		CaseID:    "senate-ws-bind",
		FiledAt:   now.Format(time.RFC3339),
		VerdictAt: now.Format(time.RFC3339),
		Type:      "general",
		Summary:   "Binding verdict workspace test",
		Verdict:   core.DecisionApprove,
		Reasoning: "R",
		Implementation: "I",
		Judge:     "test",
		Binding:   true,
	}
	d.SaveVerdict(v)

	// No --workspace flag: should NOT error with "workspace dir is required".
	// It may error because br is not installed in test, but that's a different error.
	stderr := captureStderr(t, func() {
		cmdHandoff([]string{"--case-id", "senate-ws-bind", "--state-dir", stateDir})
	})
	if strings.Contains(stderr, "workspace dir is required") {
		t.Fatal("cmdHandoff should fallback to os.Getwd() when --workspace is not provided, not fail with 'workspace dir is required'")
	}
}

func TestCmdHandoff_DeferredVerdict(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	now := time.Now().UTC()
	v := core.Verdict{
		CaseID:    "senate-deferred",
		FiledAt:   now.Format(time.RFC3339),
		VerdictAt: now.Format(time.RFC3339),
		Type:      "general",
		Summary:   "Deferred verdict for handoff",
		Verdict:   core.DecisionDefer,
		Reasoning: "Need more information",
		Judge:     "test",
		Binding:   true,
	}
	d.SaveVerdict(v)

	var code int
	out := captureStdout(t, func() {
		code = cmdHandoff([]string{"--case-id", "senate-deferred", "--state-dir", stateDir})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "skipped") {
		t.Errorf("expected 'skipped' in output, got %q", out)
	}
}

func TestCmdHandoff_NonBindingVerdict(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	now := time.Now().UTC()
	v := core.Verdict{
		CaseID:    "senate-nonbind",
		FiledAt:   now.Format(time.RFC3339),
		VerdictAt: now.Format(time.RFC3339),
		Type:      "general",
		Summary:   "Non-binding verdict",
		Verdict:   core.DecisionApprove,
		Reasoning: "Approved but non-binding",
		Judge:     "test",
		Binding:   false,
	}
	d.SaveVerdict(v)

	var code int
	out := captureStdout(t, func() {
		code = cmdHandoff([]string{"--case-id", "senate-nonbind", "--state-dir", stateDir})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "skipped") {
		t.Errorf("expected 'skipped' in output, got %q", out)
	}
}

func TestCmdHandoff_DeferredJSON(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	now := time.Now().UTC()
	v := core.Verdict{
		CaseID:    "senate-def-json",
		FiledAt:   now.Format(time.RFC3339),
		VerdictAt: now.Format(time.RFC3339),
		Type:      "general",
		Summary:   "Deferred for JSON",
		Verdict:   core.DecisionDefer,
		Reasoning: "R",
		Judge:     "test",
		Binding:   true,
	}
	d.SaveVerdict(v)

	var code int
	out := captureStdout(t, func() {
		code = cmdHandoff([]string{"--case-id", "senate-def-json", "--state-dir", stateDir, "--json"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if result["Status"] != "skipped" {
		t.Errorf("status = %v, want skipped", result["Status"])
	}
}

func TestCmdHandoff_RunThroughDispatcher(t *testing.T) {
	stateDir := t.TempDir()
	d, _ := store.New(stateDir)
	now := time.Now().UTC()
	v := core.Verdict{
		CaseID:    "senate-dispatch",
		FiledAt:   now.Format(time.RFC3339),
		VerdictAt: now.Format(time.RFC3339),
		Type:      "general",
		Summary:   "Dispatch test",
		Verdict:   core.DecisionDefer,
		Reasoning: "R",
		Judge:     "test",
		Binding:   true,
	}
	d.SaveVerdict(v)

	var code int
	out := captureStdout(t, func() {
		code = Run([]string{"senate", "handoff", "--case-id", "senate-dispatch", "--state-dir", stateDir})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "skipped") {
		t.Errorf("expected 'skipped' in output, got %q", out)
	}
}

// --- cmdFileCase extended ---

func TestCmdFileCase_BadStoreDir(t *testing.T) {
	// Use /dev/null as state dir — can't create subdirectories
	code := cmdFileCase([]string{
		"--state-dir", "/dev/null/impossible",
		"--type", "general",
		"--summary", "Test",
		"--question", "Q?",
	})
	if code != 1 {
		t.Fatalf("expected exit 1 for bad store dir, got %d", code)
	}
}

// --- cmdAsk extended ---

func TestCmdAsk_WithQuestionButBadStoreDir(t *testing.T) {
	code := cmdAsk([]string{"my question", "--state-dir", "/dev/null/impossible"})
	if code != 1 {
		t.Fatalf("expected exit 1 for bad store dir, got %d", code)
	}
}

// --- fakeExecutor: test double for Claude CLI ---

// fakeExecutor is a test double for the Claude CLI executor.
type fakeExecutor struct {
	verdictJSON string // if non-empty, written to verdictFile on Run
	err         error  // if non-nil, returned from Run (simulates non-zero exit)
}

func (f *fakeExecutor) Run(_ context.Context, _, _, _, _, verdictFile string) error {
	if f.err != nil {
		return f.err
	}
	if f.verdictJSON != "" {
		return os.WriteFile(verdictFile, []byte(f.verdictJSON), 0644)
	}
	return nil
}

// withFakeExecutor sets the fake executor for the duration of a test.
func withFakeExecutor(t *testing.T, fe *fakeExecutor) {
	t.Helper()
	old := claudeExecutor
	claudeExecutor = fe
	t.Cleanup(func() { claudeExecutor = old })
}

// --- cmdAsk with test double ---

func TestCmdAsk_HappyPath(t *testing.T) {
	stateDir := t.TempDir()

	withFakeExecutor(t, &fakeExecutor{
		verdictJSON: `{
			"case_id": "senate-happy",
			"verdict": "approved",
			"reasoning": "Sound approach backed by evidence",
			"implementation": "Proceed with Redis caching",
			"dissent": "Minor concern about complexity",
			"positions": [
				{"senator": "pragmatist", "stance": "approved", "key_argument": "Ship it"},
				{"senator": "skeptic", "stance": "amended", "key_argument": "Need monitoring"}
			]
		}`,
	})

	var code int
	out := captureStdout(t, func() {
		code = cmdAsk([]string{"Should we use Redis?", "--state-dir", stateDir})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	// Verify JSON output to stdout
	var result AskResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("stdout not valid JSON: %v\nOutput: %s", err, out)
	}
	if result.Verdict != "approved" {
		t.Errorf("verdict = %q, want approved", result.Verdict)
	}
	if len(result.Positions) != 2 {
		t.Errorf("expected 2 positions, got %d", len(result.Positions))
	}

	// Verify verdict was stored
	d, err := store.New(stateDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(stateDir, "verdicts"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 verdict file, got %d", len(entries))
	}
	caseID := strings.TrimSuffix(entries[0].Name(), ".json")
	loaded, err := d.LoadVerdict(caseID)
	if err != nil {
		t.Fatalf("load verdict: %v", err)
	}
	if loaded.Verdict != core.DecisionApprove {
		t.Errorf("stored verdict = %q, want approved", loaded.Verdict)
	}
	if loaded.Reasoning != "Sound approach backed by evidence" {
		t.Errorf("stored reasoning = %q", loaded.Reasoning)
	}
	if !loaded.Binding {
		t.Error("approved verdict should be binding")
	}

	// Verify precedent was indexed
	prec := precedent.New(d.PrecedentIndexPath())
	results, err := prec.Search("Redis", precedent.SearchOptions{})
	if err != nil {
		t.Fatalf("precedent search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("verdict should be searchable as precedent after indexing")
	}
}

func TestCmdAsk_VerdictFileMissing(t *testing.T) {
	stateDir := t.TempDir()

	// Executor completes successfully but writes no verdict file
	withFakeExecutor(t, &fakeExecutor{})

	var code int
	stderr := captureStderr(t, func() {
		code = cmdAsk([]string{"Where is the verdict?", "--state-dir", stateDir})
	})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "verdict not written") {
		t.Errorf("error should mention missing verdict, got: %q", stderr)
	}
}

func TestCmdAsk_VerdictJSONMalformed(t *testing.T) {
	stateDir := t.TempDir()

	withFakeExecutor(t, &fakeExecutor{
		verdictJSON: `{"verdict": "approved", INVALID JSON`,
	})

	var code int
	stderr := captureStderr(t, func() {
		code = cmdAsk([]string{"Bad JSON coming", "--state-dir", stateDir})
	})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "parse verdict") {
		t.Errorf("error should mention parse failure, got: %q", stderr)
	}
}

func TestCmdAsk_ClaudeExitsNonZero(t *testing.T) {
	stateDir := t.TempDir()

	withFakeExecutor(t, &fakeExecutor{
		err: errors.New("exit status 1"),
	})

	var code int
	stderr := captureStderr(t, func() {
		code = cmdAsk([]string{"Claude will fail", "--state-dir", stateDir})
	})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "claude:") {
		t.Errorf("error should mention claude, got: %q", stderr)
	}
}

func TestCmdAsk_HandoffTriggeredForBindingVerdict(t *testing.T) {
	stateDir := t.TempDir()

	withFakeExecutor(t, &fakeExecutor{
		verdictJSON: `{
			"case_id": "senate-binding",
			"verdict": "approved",
			"reasoning": "Clear approval with strong consensus",
			"implementation": "Implement the changes",
			"positions": [{"senator": "pragmatist", "stance": "approved", "key_argument": "Go for it"}]
		}`,
	})

	captureStdout(t, func() {
		code := cmdAsk([]string{"Binding question", "--state-dir", stateDir, "--type", "rule_evolution"})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})

	// Load the stored verdict and verify it's binding
	// Handoff checks: !verdict.Binding || verdict.Verdict == DecisionDefer → skip
	// So Binding=true + non-deferred → handoff creates a bead
	d, _ := store.New(stateDir)
	entries, _ := os.ReadDir(filepath.Join(stateDir, "verdicts"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 verdict file, got %d", len(entries))
	}
	caseID := strings.TrimSuffix(entries[0].Name(), ".json")
	v, err := d.LoadVerdict(caseID)
	if err != nil {
		t.Fatalf("load verdict: %v", err)
	}
	if !v.Binding {
		t.Fatal("approved verdict must be binding — handoff would not trigger")
	}
	if v.Verdict == core.DecisionDefer {
		t.Fatal("approved verdict should not be deferred")
	}
}

func TestCmdAsk_HandoffSkippedForDeferredVerdict(t *testing.T) {
	stateDir := t.TempDir()

	withFakeExecutor(t, &fakeExecutor{
		verdictJSON: `{
			"case_id": "senate-deferred",
			"verdict": "deferred",
			"reasoning": "Insufficient evidence to decide now",
			"implementation": "",
			"positions": [{"senator": "skeptic", "stance": "deferred", "key_argument": "Need more data"}]
		}`,
	})

	captureStdout(t, func() {
		code := cmdAsk([]string{"Deferred question", "--state-dir", stateDir})
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})

	// Load the stored verdict and verify it's NOT binding
	// Handoff checks: !verdict.Binding → skip (no bead created)
	d, _ := store.New(stateDir)
	entries, _ := os.ReadDir(filepath.Join(stateDir, "verdicts"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 verdict file, got %d", len(entries))
	}
	caseID := strings.TrimSuffix(entries[0].Name(), ".json")
	v, err := d.LoadVerdict(caseID)
	if err != nil {
		t.Fatalf("load verdict: %v", err)
	}
	if v.Binding {
		t.Fatal("deferred verdict must not be binding — handoff would create unwanted beads")
	}
}

// --- Verdict pipeline: the core workflow ---

// TestVerdictPipelineRoundTrip simulates the complete post-deliberation workflow:
// Claude writes a verdict JSON → senate parses it → stores verdict → indexes as precedent → searchable.
// This is the most critical data flow in senate. If this breaks, agents get no verdicts.
func TestVerdictPipelineRoundTrip(t *testing.T) {
	stateDir := t.TempDir()
	d, err := store.New(stateDir)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Simulate what Claude writes as verdict output — this is the exact format
	// that cmdAsk expects to read from the verdict file after Claude exits.
	verdictJSON := `{
		"case_id": "senate-pipeline-test",
		"verdict": "approved",
		"reasoning": "The caching approach using Redis is sound and well-tested in production",
		"implementation": "1. Add Redis dependency\n2. Configure TTL\n3. Add cache invalidation",
		"dissent": "One senator raised concerns about operational complexity",
		"positions": [
			{"senator": "pragmatist", "stance": "approved", "key_argument": "Redis is battle-tested"},
			{"senator": "purist", "stance": "approved", "key_argument": "Clean separation of concerns"},
			{"senator": "skeptic", "stance": "amended", "key_argument": "Need cache invalidation strategy"}
		]
	}`

	// Parse — same logic as cmdAsk lines 209-215
	var result AskResult
	if err := json.Unmarshal([]byte(verdictJSON), &result); err != nil {
		t.Fatalf("parse verdict JSON: %v", err)
	}
	if result.CaseID != "senate-pipeline-test" {
		t.Errorf("case_id = %q", result.CaseID)
	}
	if len(result.Positions) != 3 {
		t.Fatalf("expected 3 positions, got %d", len(result.Positions))
	}

	// Build verdict struct — same as cmdAsk lines 218-232
	now := time.Now().UTC()
	verdict := core.Verdict{
		CaseID:         result.CaseID,
		FiledAt:        now.Format(time.RFC3339),
		VerdictAt:      now.Format(time.RFC3339),
		Type:           "architecture",
		Summary:        "Should we use Redis for caching?",
		Verdict:        core.Decision(result.Verdict),
		Reasoning:      result.Reasoning,
		Implementation: result.Implementation,
		Dissent:        result.Dissent,
		Binding:        result.Verdict != "deferred",
		Judge:          "claude-sonnet",
	}

	// Store verdict — same as cmdAsk line 231
	if err := d.SaveVerdict(verdict); err != nil {
		t.Fatalf("save verdict: %v", err)
	}

	// Load and verify persistence
	loaded, err := d.LoadVerdict(result.CaseID)
	if err != nil {
		t.Fatalf("load verdict: %v", err)
	}
	if loaded.Verdict != core.DecisionApprove {
		t.Errorf("loaded verdict = %q, want approved", loaded.Verdict)
	}
	if !loaded.Binding {
		t.Error("approved verdict should be binding")
	}
	if loaded.Dissent == "" {
		t.Error("dissent should be preserved through store round-trip")
	}
	if loaded.Implementation == "" {
		t.Error("implementation should be preserved through store round-trip")
	}

	// Index as precedent — same as cmdAsk lines 236-239
	prec := precedent.New(d.PrecedentIndexPath())
	if err := prec.Add(precedent.FromVerdict(verdict)); err != nil {
		t.Fatalf("add precedent: %v", err)
	}

	// Search — this is how agents find past decisions
	results, err := prec.Search("Redis caching", precedent.SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("verdict must be searchable as precedent after indexing")
	}
	if results[0].CaseID != "senate-pipeline-test" {
		t.Errorf("precedent search returned wrong case: %q", results[0].CaseID)
	}
	if results[0].Verdict != core.DecisionApprove {
		t.Errorf("precedent verdict = %q", results[0].Verdict)
	}
}

// TestVerdictPipelineDeferredSkipsHandoff verifies the business rule:
// deferred verdicts must not be binding and must not create handoff beads.
// Breaking this would create spurious implementation tasks in Polis.
func TestVerdictPipelineDeferredSkipsHandoff(t *testing.T) {
	stateDir := t.TempDir()
	d, err := store.New(stateDir)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	verdictJSON := `{
		"case_id": "senate-defer-pipeline",
		"verdict": "deferred",
		"reasoning": "Insufficient evidence to decide",
		"implementation": "",
		"dissent": "",
		"positions": [{"senator": "skeptic", "stance": "deferred", "key_argument": "Need more data"}]
	}`

	var result AskResult
	if err := json.Unmarshal([]byte(verdictJSON), &result); err != nil {
		t.Fatalf("parse: %v", err)
	}

	// This is the exact binding check from cmdAsk line 228
	binding := result.Verdict != "deferred"
	if binding {
		t.Fatal("deferred verdict must not be binding — would create unwanted handoff beads")
	}

	// Build and store the verdict
	now := time.Now().UTC()
	verdict := core.Verdict{
		CaseID:    result.CaseID,
		FiledAt:   now.Format(time.RFC3339),
		VerdictAt: now.Format(time.RFC3339),
		Type:      "general",
		Summary:   "Deferred question",
		Verdict:   core.Decision(result.Verdict),
		Reasoning: result.Reasoning,
		Binding:   binding,
		Judge:     "claude-sonnet",
	}
	if err := d.SaveVerdict(verdict); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Verify the stored verdict is not binding
	loaded, _ := d.LoadVerdict(result.CaseID)
	if loaded.Binding {
		t.Fatal("stored deferred verdict should not be binding")
	}
}

// Suppress unused import warnings
var _ = fmt.Sprintf
