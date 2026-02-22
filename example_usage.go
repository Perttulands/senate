package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Perttulands/senate/internal/core"
	"github.com/Perttulands/senate/internal/handoff"
)

// Example showing how to use CreateBeadFromVerdict with JSON input
func main() {
	// Example verdict JSON as specified in requirements
	verdictJSON := `{
		"case_id": "senate-001",
		"verdict": "approved",
		"reasoning": "This change improves system reliability without introducing risks. The proposed monitoring adjustments will reduce alert fatigue while maintaining visibility into critical issues.",
		"implementation": "1. Update config/alerts.yaml with new thresholds\n2. Deploy to staging for 24h validation\n3. Roll out to production with phased approach",
		"dissent": "One panelist suggested more conservative thresholds initially",
		"summary": "Adjust monitoring alert thresholds for reduced noise",
		"type": "rule_evolution",
		"binding": true,
		"judge": "claude-3-opus",
		"filed_at": "2024-01-15T10:00:00Z",
		"verdict_at": "2024-01-15T10:30:00Z"
	}`

	// Parse the JSON into a Verdict struct
	var verdict core.Verdict
	if err := json.Unmarshal([]byte(verdictJSON), &verdict); err != nil {
		log.Fatalf("Failed to parse verdict JSON: %v", err)
	}

	// Create context
	ctx := context.Background()

	// Call CreateBeadFromVerdict
	beadID, err := handoff.CreateBeadFromVerdict(ctx, verdict)
	if err != nil {
		log.Fatalf("Failed to create bead: %v", err)
	}

	// Success!
	fmt.Printf("Successfully created bead with ID: %s\n", beadID)

	// The created bead will have:
	// - Title: "[truthsayer] Senate senate-001: Adjust monitoring alert thresholds for reduced noise"
	// - Description containing:
	//   - Case ID for traceability
	//   - Verdict decision
	//   - Full reasoning
	//   - Implementation steps
	// - Priority: 2 (medium)
}

/* Expected bd command that will be executed:

bd create \
  --title "[truthsayer] Senate senate-001: Adjust monitoring alert thresholds for reduced noise" \
  --priority "2" \
  --description "Binding Senate verdict for case senate-001

Verdict: approved
Reasoning: This change improves system reliability without introducing risks. The proposed monitoring adjustments will reduce alert fatigue while maintaining visibility into critical issues.
Implementation: 1. Update config/alerts.yaml with new thresholds
2. Deploy to staging for 24h validation
3. Roll out to production with phased approach
" \
  --silent
*/