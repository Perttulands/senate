package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- BuildAgentsJSON ---

func TestBuildAgentsJSON_ValidN(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5} {
		result := BuildAgentsJSON(n)
		if result == "" {
			t.Fatalf("n=%d: expected non-empty JSON", n)
		}

		var agents map[string]agentDef
		if err := json.Unmarshal([]byte(result), &agents); err != nil {
			t.Fatalf("n=%d: invalid JSON: %v", n, err)
		}
		if len(agents) != n {
			t.Fatalf("n=%d: expected %d agents, got %d", n, n, len(agents))
		}

		// Verify each agent has required fields
		for name, agent := range agents {
			if !strings.HasPrefix(name, "senator-") {
				t.Errorf("n=%d: agent name %q missing senator- prefix", n, name)
			}
			if agent.Description == "" {
				t.Errorf("n=%d: agent %s has empty description", n, name)
			}
			if agent.Prompt == "" {
				t.Errorf("n=%d: agent %s has empty prompt", n, name)
			}
			if agent.Model != "sonnet" {
				t.Errorf("n=%d: agent %s model=%q, want sonnet", n, name, agent.Model)
			}
		}
	}
}

func TestBuildAgentsJSON_ZeroDefaultsTo3(t *testing.T) {
	result := BuildAgentsJSON(0)
	var agents map[string]agentDef
	if err := json.Unmarshal([]byte(result), &agents); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents for n=0, got %d", len(agents))
	}
}

func TestBuildAgentsJSON_NegativeDefaultsTo3(t *testing.T) {
	result := BuildAgentsJSON(-5)
	var agents map[string]agentDef
	if err := json.Unmarshal([]byte(result), &agents); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents for n=-5, got %d", len(agents))
	}
}

func TestBuildAgentsJSON_ExceedsCatalogClamped(t *testing.T) {
	result := BuildAgentsJSON(100)
	var agents map[string]agentDef
	if err := json.Unmarshal([]byte(result), &agents); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(agents) != len(senatorCatalog) {
		t.Fatalf("expected %d agents (catalog size), got %d", len(senatorCatalog), len(agents))
	}
}

func TestBuildAgentsJSON_AgentNamesMatchCatalog(t *testing.T) {
	result := BuildAgentsJSON(len(senatorCatalog))
	var agents map[string]agentDef
	if err := json.Unmarshal([]byte(result), &agents); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, s := range senatorCatalog {
		key := "senator-" + s.Name
		if _, ok := agents[key]; !ok {
			t.Errorf("expected agent %q in output", key)
		}
	}
}

func TestBuildAgentsJSON_PromptContainsSenatorIdentity(t *testing.T) {
	result := BuildAgentsJSON(1)
	var agents map[string]agentDef
	if err := json.Unmarshal([]byte(result), &agents); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	agent := agents["senator-pragmatist"]
	if !strings.Contains(agent.Prompt, senatorCatalog[0].FullName) {
		t.Error("prompt should contain senator full name")
	}
	if !strings.Contains(agent.Prompt, senatorCatalog[0].Archetype) {
		t.Error("prompt should contain senator archetype")
	}
	if !strings.Contains(agent.Prompt, senatorCatalog[0].Philosophy) {
		t.Error("prompt should contain senator philosophy")
	}
}

// --- BuildSystemPrompt ---

func TestBuildSystemPrompt_AskMode(t *testing.T) {
	prompt := BuildSystemPrompt("ask", "/tmp/verdict.json", "senate-001")
	if !strings.Contains(prompt, "Athena Senate") {
		t.Error("prompt should contain 'Athena Senate'")
	}
	if !strings.Contains(prompt, "/tmp/verdict.json") {
		t.Error("ask mode prompt should contain verdict path")
	}
	if !strings.Contains(prompt, "senate-001") {
		t.Error("prompt should contain case ID")
	}
	if !strings.Contains(prompt, "VERDICT:") {
		t.Error("ask mode should instruct to print VERDICT: summary line")
	}
}

func TestBuildSystemPrompt_StartMode(t *testing.T) {
	prompt := BuildSystemPrompt("start", "/tmp/verdict.json", "senate-002")
	if !strings.Contains(prompt, "Athena Senate") {
		t.Error("prompt should contain 'Athena Senate'")
	}
	if !strings.Contains(prompt, "/tmp/verdict.json") {
		t.Error("start mode prompt should contain verdict path")
	}
	if !strings.Contains(prompt, "senate-002") {
		t.Error("prompt should contain case ID")
	}
	// start mode should NOT have the one-line VERDICT: instruction
	if strings.Contains(prompt, "VERDICT:") {
		t.Error("start mode should not have VERDICT: summary instruction")
	}
	if !strings.Contains(prompt, "present the verdict clearly to the user") {
		t.Error("start mode should instruct interactive presentation")
	}
}

func TestBuildSystemPrompt_ContainsProtocolPhases(t *testing.T) {
	prompt := BuildSystemPrompt("ask", "/tmp/v.json", "senate-x")
	phases := []string{"Phase 1", "Phase 2", "Phase 3", "Phase 4"}
	for _, phase := range phases {
		if !strings.Contains(prompt, phase) {
			t.Errorf("prompt missing %s", phase)
		}
	}
}

func TestBuildSystemPrompt_ContainsVerdictJSONSchema(t *testing.T) {
	prompt := BuildSystemPrompt("ask", "/tmp/v.json", "senate-x")
	fields := []string{"case_id", "verdict", "reasoning", "implementation", "dissent", "positions"}
	for _, f := range fields {
		if !strings.Contains(prompt, f) {
			t.Errorf("prompt missing schema field %q", f)
		}
	}
}

// --- SenatorNames ---

func TestSenatorNames_ValidN(t *testing.T) {
	names := SenatorNames(3)
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	expected := []string{"pragmatist", "purist", "skeptic"}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("names[%d]=%q, want %q", i, names[i], want)
		}
	}
}

func TestSenatorNames_ExceedsCatalog(t *testing.T) {
	names := SenatorNames(100)
	if len(names) != len(senatorCatalog) {
		t.Fatalf("expected %d names, got %d", len(senatorCatalog), len(names))
	}
}

func TestSenatorNames_Zero(t *testing.T) {
	names := SenatorNames(0)
	if len(names) != 0 {
		t.Fatalf("expected 0 names, got %d", len(names))
	}
}

// --- WriteTempFiles ---

func TestWriteTempFiles_Success(t *testing.T) {
	promptFile, tempDir, err := WriteTempFiles("test prompt content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if tempDir == "" {
		t.Fatal("tempDir should not be empty")
	}
	if promptFile == "" {
		t.Fatal("promptFile should not be empty")
	}
	if filepath.Base(promptFile) != "protocol.md" {
		t.Errorf("expected protocol.md, got %s", filepath.Base(promptFile))
	}

	data, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("read prompt file: %v", err)
	}
	if string(data) != "test prompt content" {
		t.Errorf("prompt file content = %q, want %q", string(data), "test prompt content")
	}
}

func TestWriteTempFiles_MkdirError(t *testing.T) {
	// Set TMPDIR to a path that doesn't exist and can't be created
	t.Setenv("TMPDIR", "/dev/null/nonexistent")

	_, _, err := WriteTempFiles("test prompt")
	if err == nil {
		t.Fatal("expected error when TMPDIR is invalid")
	}
	if !strings.Contains(err.Error(), "create temp dir") {
		t.Errorf("error should mention 'create temp dir', got: %v", err)
	}
}

func TestWriteTempFiles_EmptyPrompt(t *testing.T) {
	promptFile, tempDir, err := WriteTempFiles("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	data, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("read prompt file: %v", err)
	}
	if string(data) != "" {
		t.Errorf("expected empty content, got %q", string(data))
	}
}

// --- VerdictPath ---

func TestVerdictPath(t *testing.T) {
	got := VerdictPath("/tmp/senate-abc")
	want := filepath.Join("/tmp/senate-abc", "verdict.json")
	if got != want {
		t.Errorf("VerdictPath = %q, want %q", got, want)
	}
}

// --- senatorLabel ---

func TestSenatorLabel(t *testing.T) {
	label := senatorLabel(3)
	if !strings.Contains(label, "pragmatist") {
		t.Error("label should contain pragmatist")
	}
	if !strings.Contains(label, "purist") {
		t.Error("label should contain purist")
	}
	if !strings.Contains(label, "skeptic") {
		t.Error("label should contain skeptic")
	}
	// Should be comma-separated
	parts := strings.Split(label, ", ")
	if len(parts) != 3 {
		t.Errorf("expected 3 comma-separated parts, got %d: %q", len(parts), label)
	}
}

func TestSenatorLabel_Single(t *testing.T) {
	label := senatorLabel(1)
	if label != "pragmatist" {
		t.Errorf("expected 'pragmatist', got %q", label)
	}
}

// --- senatorCatalog integrity ---

func TestSenatorCatalogIntegrity(t *testing.T) {
	if len(senatorCatalog) < 3 {
		t.Fatalf("catalog should have at least 3 senators, got %d", len(senatorCatalog))
	}
	seen := map[string]bool{}
	for i, s := range senatorCatalog {
		if s.Name == "" {
			t.Errorf("senator[%d] has empty Name", i)
		}
		if s.FullName == "" {
			t.Errorf("senator[%d] %s has empty FullName", i, s.Name)
		}
		if s.Archetype == "" {
			t.Errorf("senator[%d] %s has empty Archetype", i, s.Name)
		}
		if s.Philosophy == "" {
			t.Errorf("senator[%d] %s has empty Philosophy", i, s.Name)
		}
		if seen[s.Name] {
			t.Errorf("duplicate senator name: %s", s.Name)
		}
		seen[s.Name] = true
	}
}
