package senator

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// TestSpawnSenatorIntegration demonstrates spawning a senator and getting a response.
// Skips automatically when tmux is not available (CI, containers, etc.).
func TestSpawnSenatorIntegration(t *testing.T) {
	// Skip when tmux is unavailable
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available, skipping integration test")
	}
	cmd := exec.Command("tmux", "-S", TmuxSocket, "list-sessions")
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "no server running") || strings.Contains(outStr, "No such file") {
			t.Skip("tmux server not running, skipping integration test")
		}
	}

	// Load configuration
	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Get the pragmatist senator config
	senatorConfig, err := config.GetSenatorByName("pragmatist")
	if err != nil {
		t.Fatalf("get senator config: %v", err)
	}

	// Spawn the senator
	session, err := SpawnSenator(senatorConfig, "TEST-001")
	if err != nil {
		t.Fatalf("spawn senator: %v", err)
	}
	defer session.Close()

	// Prepare a test case prompt
	casePrompt := `=== CASE PRESENTATION START ===
Case ID: TEST-001
Type: rule_evolution
Summary: Should we relax error handling requirements in test files?

Question: Our static analysis requires explicit error handling for all function calls,
even in test files. This creates verbose test code. Should we relax this requirement
for *_test.go files?

Evidence:
- Current rule flags 247 instances in test files
- Test readability is reduced by error handling boilerplate
- Tests fail anyway if errors occur
- Other teams have test-specific relaxed rules

Please provide your initial position on this case.
=== CASE PRESENTATION END ===`

	// Send the prompt
	response, err := session.SendPrompt(casePrompt)
	if err != nil {
		t.Fatalf("send prompt: %v", err)
	}

	if response == "" {
		t.Fatal("expected non-empty response")
	}

	// Parse the stance from response
	if strings.Contains(response, "Stance:") {
		lines := strings.Split(response, "\n")
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "Stance:") {
				fmt.Printf("Detected stance: %s\n", strings.TrimSpace(line))
				break
			}
		}
	}
}
