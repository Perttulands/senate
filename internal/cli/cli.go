package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Perttulands/senate/internal/core"
	"github.com/Perttulands/senate/internal/deliberation"
	"github.com/Perttulands/senate/internal/handoff"
	"github.com/Perttulands/senate/internal/precedent"
	"github.com/Perttulands/senate/internal/store"
)

const Version = "0.1.0"

// Run executes the Senate CLI.
func Run(args []string) int {
	if len(args) < 2 {
		usage()
		return 1
	}

	cmd := args[1]
	cmdArgs := args[2:]

	switch cmd {
	case "help", "-h", "--help":
		usage()
		return 0
	case "version":
		fmt.Println("senate", Version)
		return 0
	case "health":
		return cmdHealth(cmdArgs)
	case "deliberate":
		return cmdDeliberate(cmdArgs)
	case "precedent":
		return cmdPrecedent(cmdArgs)
	case "handoff":
		return cmdHandoff(cmdArgs)
	case "file-case":
		return cmdFileCase(cmdArgs)
	default:
		errorf("unknown command: %s", cmd)
		usage()
		return 1
	}
}

func cmdHealth(args []string) int {
	verbose := flagBool(args, "--verbose")
	hasError := false

	// Check Claude CLI
	fmt.Print("Claude CLI: ")
	if path, err := exec.LookPath("claude"); err != nil {
		fmt.Println("❌ NOT FOUND")
		hasError = true
		if verbose {
			fmt.Printf("  Error: %v\n", err)
			fmt.Println("  Install: https://docs.anthropic.com/claude/docs/claude-cli")
		}
	} else {
		fmt.Println("✓")
		if verbose {
			fmt.Printf("  Path: %s\n", path)
		}
	}

	// Check tmux
	fmt.Print("Tmux: ")
	if path, err := exec.LookPath("tmux"); err != nil {
		fmt.Println("❌ NOT FOUND")
		hasError = true
		if verbose {
			fmt.Printf("  Error: %v\n", err)
			fmt.Println("  Install: sudo apt-get install tmux")
		}
	} else {
		fmt.Println("✓")
		if verbose {
			fmt.Printf("  Path: %s\n", path)
		}
	}

	// Check tmux socket
	const tmuxSocket = "/tmp/tmux-senate.sock"
	fmt.Print("Tmux socket: ")
	socketDir := filepath.Dir(tmuxSocket)
	if info, err := os.Stat(socketDir); err != nil {
		fmt.Printf("❌ DIR NOT ACCESSIBLE (%s)\n", socketDir)
		hasError = true
		if verbose {
			fmt.Printf("  Error: %v\n", err)
		}
	} else if !info.IsDir() {
		fmt.Printf("❌ NOT A DIRECTORY (%s)\n", socketDir)
		hasError = true
	} else {
		// Test tmux socket
		cmd := exec.Command("tmux", "-S", tmuxSocket, "list-sessions")
		if output, err := cmd.CombinedOutput(); err != nil {
			if strings.Contains(string(output), "no server running") || strings.Contains(string(output), "no sessions") {
				fmt.Println("✓")
				if verbose {
					fmt.Printf("  Socket: %s (no active sessions)\n", tmuxSocket)
				}
			} else {
				fmt.Println("❌ SOCKET TEST FAILED")
				hasError = true
				if verbose {
					fmt.Printf("  Error: %v\n", err)
					fmt.Printf("  Output: %s\n", output)
				}
			}
		} else {
			fmt.Println("✓")
			if verbose {
				fmt.Printf("  Socket: %s\n", tmuxSocket)
				fmt.Printf("  Active sessions:\n%s", output)
			}
		}
	}

	// Check bd (beads CLI)
	fmt.Print("Beads CLI (bd): ")
	if path, err := exec.LookPath("bd"); err != nil {
		fmt.Println("⚠️  NOT FOUND (optional for handoff)")
		if verbose {
			fmt.Printf("  Error: %v\n", err)
			fmt.Println("  Note: Required only for automatic verdict handoff")
		}
	} else {
		fmt.Println("✓")
		if verbose {
			fmt.Printf("  Path: %s\n", path)
		}
	}

	// Summary
	fmt.Println()
	if hasError {
		fmt.Println("❌ Senate is NOT ready for real deliberation")
		fmt.Println("Fix the issues above before running 'senate deliberate'")
		return 1
	}

	fmt.Println("✅ Senate is ready for real deliberation")
	return 0
}

func cmdDeliberate(args []string) int {
	flags := parseFlags(args)
	stateDir := resolveStateDir(flags["state-dir"])
	d, err := store.New(stateDir)
	if err != nil {
		errorf("init store: %v", err)
		return 1
	}

	c, err := loadCase(flags["case"], flags["quick"], flags["filed-by"])
	if err != nil {
		errorf("load case: %v", err)
		return 1
	}

	now := time.Now().UTC()
	c.Normalize(now)
	if err := c.Validate(); err != nil {
		errorf("case validation: %v", err)
		return 1
	}
	if err := d.SaveCase(c); err != nil {
		errorf("save case: %v", err)
		return 1
	}

	// Query relevant precedents before deliberation
	prec := precedent.New(d.PrecedentIndexPath())
	keywords := extractKeywords(c.Summary + " " + c.Question)
	relevantPrecedents, err := prec.SearchRelevantPrecedent(c.Type, keywords)
	if err != nil {
		// Log but don't fail if precedent search fails
		errorf("warning: precedent search failed: %v", err)
	} else {
		// Convert precedents to summaries for case context
		for _, rec := range relevantPrecedents {
			c.Precedents = append(c.Precedents, core.PrecedentSummary{
				CaseID:    rec.CaseID,
				Verdict:   rec.Verdict,
				Summary:   rec.Summary,
				Reasoning: rec.Reasoning,
			})
		}
	}

	agents := parseInt(flags["agents"], 3)
	panel := deliberation.BuildPanel(agents, splitCSV(flags["perspectives"]), splitCSV(flags["models"]))
	engine := deliberation.New(panel)
	transcript, verdict, err := engine.Deliberate(c, now)
	if err != nil {
		errorf("deliberation: %v", err)
		return 1
	}

	if err := d.SaveTranscript(transcript); err != nil {
		errorf("save transcript: %v", err)
		return 1
	}

	if !flagBool(args, "--no-handoff") {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		workspace := flags["workspace"]
		res, hErr := handoff.CreateBeadForVerdict(ctx, nil, workspace, verdict)
		if hErr != nil {
			errorf("handoff: %v", hErr)
			return 1
		}
		if res.Status == "created" {
			verdict.Handoff = &core.Handoff{
				System:    inferTargetSystem(verdict.Type),
				BeadID:    res.BeadID,
				Status:    res.Status,
				CreatedAt: now.Format(time.RFC3339),
			}
		}
	}

	if err := d.SaveVerdict(verdict); err != nil {
		errorf("save verdict: %v", err)
		return 1
	}

	prec = precedent.New(d.PrecedentIndexPath())
	if err := prec.Add(precedent.FromVerdict(verdict)); err != nil {
		errorf("save precedent: %v", err)
		return 1
	}

	if flagBool(args, "--json") {
		outputJSON(verdict)
		return 0
	}

	fmt.Printf("case_id: %s\n", verdict.CaseID)
	fmt.Printf("verdict: %s\n", verdict.Verdict)
	fmt.Printf("binding: %t\n", verdict.Binding)
	fmt.Printf("verdict_file: %s\n", d.VerdictPath(verdict.CaseID))
	fmt.Printf("transcript_file: %s\n", d.TranscriptPath(verdict.CaseID))
	if verdict.Handoff != nil && verdict.Handoff.BeadID != "" {
		fmt.Printf("handoff_bead: %s\n", verdict.Handoff.BeadID)
	}
	return 0
}

func cmdPrecedent(args []string) int {
	if len(args) == 0 {
		errorf("usage: senate precedent search --query <text> [flags]")
		return 1
	}
	sub := args[0]
	args = args[1:]

	switch sub {
	case "search":
		flags := parseFlags(args)
		stateDir := resolveStateDir(flags["state-dir"])
		d, err := store.New(stateDir)
		if err != nil {
			errorf("init store: %v", err)
			return 1
		}
		query := strings.TrimSpace(flags["query"])
		results, err := precedent.New(d.PrecedentIndexPath()).Search(query, precedent.SearchOptions{
			Type:    strings.TrimSpace(flags["type"]),
			Verdict: parseDecision(flags["verdict"]),
			Limit:   parseInt(flags["limit"], 20),
		})
		if err != nil {
			errorf("precedent search: %v", err)
			return 1
		}
		if flagBool(args, "--json") {
			outputJSON(results)
			return 0
		}
		if len(results) == 0 {
			fmt.Println("no precedent matches")
			return 0
		}
		for _, rec := range results {
			fmt.Printf("%s %s %s\n", rec.CaseID, rec.Verdict, rec.Summary)
		}
		return 0
	default:
		errorf("unknown precedent subcommand: %s", sub)
		return 1
	}
}

func cmdHandoff(args []string) int {
	flags := parseFlags(args)
	caseID := strings.TrimSpace(flags["case-id"])
	if caseID == "" {
		errorf("usage: senate handoff --case-id <id> [--workspace <path>] [--state-dir <path>]")
		return 1
	}

	d, err := store.New(resolveStateDir(flags["state-dir"]))
	if err != nil {
		errorf("init store: %v", err)
		return 1
	}

	v, err := d.LoadVerdict(caseID)
	if err != nil {
		errorf("load verdict: %v", err)
		return 1
	}
	if v.Handoff != nil && strings.TrimSpace(v.Handoff.BeadID) != "" {
		fmt.Printf("handoff already exists: %s\n", v.Handoff.BeadID)
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	res, err := handoff.CreateBeadForVerdict(ctx, nil, flags["workspace"], v)
	if err != nil {
		errorf("handoff: %v", err)
		return 1
	}
	if res.Status == "created" {
		v.Handoff = &core.Handoff{
			System:    inferTargetSystem(v.Type),
			BeadID:    res.BeadID,
			Status:    "created",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := d.SaveVerdict(v); err != nil {
			errorf("save verdict: %v", err)
			return 1
		}
		if err := precedent.New(d.PrecedentIndexPath()).Add(precedent.FromVerdict(v)); err != nil {
			errorf("update precedent index: %v", err)
			return 1
		}
	}
	if flagBool(args, "--json") {
		outputJSON(res)
		return 0
	}
	fmt.Printf("handoff status: %s\n", res.Status)
	if res.BeadID != "" {
		fmt.Printf("handoff bead: %s\n", res.BeadID)
	}
	return 0
}

// cmdFileCase handles filing a new case from command-line flags.
func cmdFileCase(args []string) int {
	flags := parseFlags(args)
	stateDir := resolveStateDir(flags["state-dir"])
	d, err := store.New(stateDir)
	if err != nil {
		errorf("init store: %v", err)
		return 1
	}

	// Build case from flags
	now := time.Now().UTC()
	c := core.Case{
		ID:                core.NewCaseID(now),
		Type:              strings.TrimSpace(flags["type"]),
		Summary:           strings.TrimSpace(flags["summary"]),
		Question:          strings.TrimSpace(flags["question"]),
		RequestedDecision: strings.TrimSpace(flags["requested-decision"]),
		FiledAt:           now.Format(time.RFC3339),
		FiledBy:           strings.TrimSpace(flags["filed-by"]),
		Evidence:          collectEvidence(args),
	}

	// Validate required fields
	if c.Type == "" {
		errorf("--type is required")
		return 1
	}
	if c.Summary == "" {
		errorf("--summary is required")
		return 1
	}
	if c.Question == "" {
		errorf("--question is required")
		return 1
	}

	// Validate case type
	validTypes := []string{"rule_evolution", "gate_criteria", "dispute", "priority", "architecture", "general"}
	if !contains(validTypes, c.Type) {
		errorf("invalid case type: %s (must be one of: %s)", c.Type, strings.Join(validTypes, ", "))
		return 1
	}

	// Normalize fills in defaults
	c.Normalize(now)

	// Final validation
	if err := c.Validate(); err != nil {
		errorf("case validation: %v", err)
		return 1
	}

	// Save the case
	if err := d.SaveCase(c); err != nil {
		errorf("save case: %v", err)
		return 1
	}

	// Print case ID on success
	fmt.Println(c.ID)
	return 0
}

func loadCase(caseFile, quickQuestion, filedBy string) (core.Case, error) {
	if strings.TrimSpace(quickQuestion) != "" {
		return core.Case{
			Type:     "general",
			Summary:  quickQuestion,
			Question: quickQuestion,
			FiledBy:  strings.TrimSpace(filedBy),
		}, nil
	}
	if strings.TrimSpace(caseFile) == "" {
		return core.Case{}, errors.New("must provide --case <file> or --quick <question>")
	}
	data, err := os.ReadFile(filepath.Clean(caseFile))
	if err != nil {
		return core.Case{}, err
	}
	var c core.Case
	if err := json.Unmarshal(data, &c); err != nil {
		return core.Case{}, err
	}
	if strings.TrimSpace(c.FiledBy) == "" {
		c.FiledBy = strings.TrimSpace(filedBy)
	}
	return c, nil
}

func appendJSONL(path string, value any) error {
	line, err := json.Marshal(value)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}

func parseFlags(args []string) map[string]string {
	flags := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			flags[strings.TrimPrefix(a, "--")] = args[i+1]
			i++
			continue
		}
		flags[strings.TrimPrefix(a, "--")] = "true"
	}
	return flags
}

func flagBool(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		clean := strings.TrimSpace(p)
		if clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

func parseInt(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func parseDecision(raw string) core.Decision {
	raw = strings.TrimSpace(strings.ToLower(raw))
	switch raw {
	case "approve", "approved":
		return core.DecisionApprove
	case "reject", "rejected":
		return core.DecisionReject
	case "amend", "amended":
		return core.DecisionAmend
	case "defer", "deferred":
		return core.DecisionDefer
	default:
		return ""
	}
}

func resolveStateDir(fromFlag string) string {
	if s := strings.TrimSpace(fromFlag); s != "" {
		return s
	}
	if s := strings.TrimSpace(os.Getenv("SENATE_STATE_DIR")); s != "" {
		return s
	}
	return "state"
}

func inferTargetSystem(caseType string) string {
	switch strings.TrimSpace(caseType) {
	case "rule_evolution":
		return "truthsayer"
	case "gate_criteria":
		return "centurion"
	case "priority_triage", "dispute_resolution":
		return "athena"
	default:
		return "athena"
	}
}

func usage() {
	fmt.Print(`senate - multi-agent deliberation system

COMMANDS:
  senate health [--verbose]                     Check if Senate is ready for real deliberation
  senate deliberate --case <file> [flags]      Run deliberation and synthesize a binding verdict
  senate file-case [flags]                      File a new case to Senate
  senate precedent search --query <text>        Search stored verdict precedents
  senate handoff --case-id <id>                 Trigger implementation bead creation from stored verdict
  senate version                                Print version

FLAGS:
  --state-dir <path>          Override Senate state root (default: ./state)
  --json                      Emit JSON output

FILE-CASE FLAGS:
  --type <type>               Case type (required): rule_evolution, gate_criteria, dispute, priority, architecture, general
  --summary <text>            One-line case description (required)
  --question <text>           The specific decision to be made (required)
  --evidence <path>           Evidence path or bead:id (can be used multiple times)
  --requested-decision <text> What the filer is asking for (optional)
  --filed-by <name>           Who is filing the case (optional)

DELIBERATE FLAGS:
  --quick <question>          Build ad-hoc case from a single question
  --agents <n>                Number of panel agents (default 3)
  --perspectives a,b,c        Override perspective labels
  --models m1,m2              Override model labels
  --workspace <path>          Workspace path for bd handoff creation
  --no-handoff                Disable SEN-006 automatic bead creation
`)
}

func outputJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "senate: "+format+"\n", args...)
}

// collectEvidence extracts multiple --evidence flags from args.
func collectEvidence(args []string) []string {
	var evidence []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--evidence" && i+1 < len(args) {
			ev := strings.TrimSpace(args[i+1])
			if ev != "" {
				evidence = append(evidence, ev)
			}
		}
	}
	return evidence
}

// contains checks if a string is in a slice.
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// extractKeywords extracts keywords from text for precedent searching
func extractKeywords(text string) []string {
	text = strings.ToLower(text)
	replacer := strings.NewReplacer(
		",", " ",
		".", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"/", " ",
		"\\", " ",
		"\n", " ",
		"\t", " ",
	)
	text = replacer.Replace(text)
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return nil
	}
	stop := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "that": {}, "this": {}, "from": {},
		"into": {}, "were": {}, "been": {}, "will": {}, "case": {}, "verdict": {},
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) < 3 {
			continue
		}
		if _, ok := stop[p]; ok {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
