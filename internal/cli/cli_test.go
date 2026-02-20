package cli

import "testing"

func TestParseDecision(t *testing.T) {
	if got := parseDecision("approve"); got == "" {
		t.Fatal("expected parsed decision")
	}
	if got := parseDecision("nonsense"); got != "" {
		t.Fatalf("expected empty decision for nonsense, got %s", got)
	}
}

func TestSplitCSV(t *testing.T) {
	parts := splitCSV("a, b,,c")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
}
