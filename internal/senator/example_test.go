package senator

import (
	"fmt"
	"log"
	"strings"
)

// ExampleSpawnSenator demonstrates spawning a senator and getting a response
// Run this with: go test -run Example
func ExampleSpawnSenator() {
	// Load configuration
	config, err := LoadConfig("")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Get the pragmatist senator config
	senatorConfig, err := config.GetSenatorByName("pragmatist")
	if err != nil {
		log.Fatalf("get senator config: %v", err)
	}

	// Spawn the senator
	fmt.Println("Spawning senator...")
	session, err := SpawnSenator(senatorConfig, "TEST-001")
	if err != nil {
		log.Fatalf("spawn senator: %v", err)
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
	fmt.Println("Sending case presentation...")
	response, err := session.SendPrompt(casePrompt)
	if err != nil {
		log.Fatalf("send prompt: %v", err)
	}

	// Display the response
	fmt.Println("\nSenator response received!")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println(response)
	fmt.Println(strings.Repeat("-", 60))

	// Parse the stance from response
	if strings.Contains(response, "Stance:") {
		lines := strings.Split(response, "\n")
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "Stance:") {
				fmt.Printf("\nDetected stance: %s\n", strings.TrimSpace(line))
				break
			}
		}
	}

	fmt.Println("\nSession closed successfully!")

	// Output:
	// Spawning senator...
	// Sending case presentation...
	//
	// Senator response received!
	// ------------------------------------------------------------
	// === INITIAL POSITION START ===
	// Stance: approve
	// Reasoning: ...
	// Concerns: ...
	// === INITIAL POSITION END ===
	// ------------------------------------------------------------
	//
	// Detected stance: Stance: approve
	//
	// Session closed successfully!
}