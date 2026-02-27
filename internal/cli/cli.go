package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Perttulands/senate/internal/core"
	"github.com/Perttulands/senate/internal/handoff"
	"github.com/Perttulands/senate/internal/precedent"
	"github.com/Perttulands/senate/internal/store"
)

const Version = "0.2.0"

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
	case "ask":
		return cmdAsk(cmdArgs)
	case "start":
		return cmdStart(cmdArgs)
	case "file-case":
		return cmdFileCase(cmdArgs)
	case "precedent":
		return cmdPrecedent(cmdArgs)
	case "handoff":
		return cmdHandoff(cmdArgs)
	default:
		errorf("unknown command: %s", cmd)
		usage()
		return 1
	}
}

// --- Commands ---

func cmdHealth(args []string) int {
	verbose := flagBool(args, "--verbose")
	hasError := false

	fmt.Print("Claude CLI: ")
	if path, err := exec.LookPath("claude"); err != nil {
		fmt.Println("NOT FOUND")
		hasError = true
		if verbose {
			fmt.Printf("  Error: %v\n", err)
		}
	} else {
		fmt.Println("ok")
		if verbose {
			fmt.Printf("  Path: %s\n", path)
		}
	}

	fmt.Println()
	if hasError {
		fmt.Println("Senate is NOT ready — fix the issues above")
		return 1
	}
	fmt.Println("Senate is ready")
	return 0
}

// AskResult is the agent-friendly JSON output from senate ask.
type AskResult struct {
	CaseID         string        `json:"case_id"`
	Verdict        string        `json:"verdict"`
	Reasoning      string        `json:"reasoning"`
	Implementation string        `json:"implementation,omitempty"`
	Dissent        string        `json:"dissent,omitempty"`
	Positions      []AskPosition `json:"positions"`
}

// AskPosition is a compact position summary.
type AskPosition struct {
	Senator     string `json:"senator"`
	Stance      string `json:"stance"`
	KeyArgument string `json:"key_argument"`
}

// cmdAsk runs a deliberation in pipe mode and returns JSON.
func cmdAsk(args []string) int {
	flags := parseFlags(args)
	agents := parseInt(flags["agents"], 3)
	caseType := strings.TrimSpace(flags["type"])
	if caseType == "" {
		caseType = "general"
	}
	filedBy := strings.TrimSpace(flags["filed-by"])
	if filedBy == "" {
		filedBy = "agent"
	}
	model := strings.TrimSpace(flags["model"])
	if model == "" {
		model = "sonnet"
	}

	// Extract question: positional arg or stdin pipe
	question := extractPositionalArg(args)
	if question == "" {
		stat, err := os.Stdin.Stat()
		if err != nil {
			errorf("stat stdin: %v", err)
			return 1
		}
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := os.ReadFile("/dev/stdin")
			if err == nil {
				question = strings.TrimSpace(string(data))
			}
		}
	}
	if question == "" {
		errorf("usage: senate ask \"question\" [--agents <n>]")
		return 1
	}

	// Build case
	stateDir := resolveStateDir(flags["state-dir"])
	d, err := store.New(stateDir)
	if err != nil {
		errorf("init store: %v", err)
		return 1
	}

	now := time.Now().UTC()
	c := core.Case{
		Type:     caseType,
		Summary:  question,
		Question: question,
		FiledBy:  filedBy,
	}
	c.Normalize(now)
	if err := d.SaveCase(c); err != nil {
		errorf("save case: %v", err)
		return 1
	}

	// Build protocol — need temp dir first for verdict path
	tempDir, err := os.MkdirTemp("", "senate-*")
	if err != nil {
		errorf("create temp dir: %v", err)
		return 1
	}
	defer os.RemoveAll(tempDir)

	verdictFile := VerdictPath(tempDir)
	prompt := BuildSystemPrompt("ask", verdictFile, c.ID)
	promptFile := filepath.Join(tempDir, "protocol.md")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		errorf("write protocol: %v", err)
		return 1
	}

	agentsJSON := BuildAgentsJSON(agents)

	fmt.Fprintf(os.Stderr, "senate: deliberating with %d agents (%s)...\n", agents, senatorLabel(agents))
	fmt.Fprintf(os.Stderr, "senate: case %s\n", c.ID)

	// Run Claude in pipe mode
	cmd := exec.Command("claude", "-p",
		"--dangerously-skip-permissions",
		"--model", model,
		"--system-prompt-file", promptFile,
		"--agents", agentsJSON,
	)
	cmd.Stdin = strings.NewReader(question)
	cmd.Stderr = os.Stderr // Show claude's progress on stderr

	if err := cmd.Run(); err != nil {
		errorf("claude: %v", err)
		return 1
	}

	// Read verdict from file
	data, err := os.ReadFile(verdictFile)
	if err != nil {
		errorf("verdict not written — claude may not have completed the protocol: %v", err)
		return 1
	}

	var result AskResult
	if err := json.Unmarshal(data, &result); err != nil {
		errorf("parse verdict: %v", err)
		// Output raw content as fallback
		fmt.Fprintln(os.Stderr, "senate: raw verdict output:")
		fmt.Fprintln(os.Stderr, string(data))
		return 1
	}

	// Store verdict
	verdict := core.Verdict{
		CaseID:    c.ID,
		FiledAt:   c.FiledAt,
		VerdictAt: time.Now().UTC().Format(time.RFC3339),
		Type:      c.Type,
		Summary:   c.Summary,
		Verdict:   core.Decision(result.Verdict),
		Reasoning: result.Reasoning,
		Implementation: result.Implementation,
		Dissent:   result.Dissent,
		Binding:   result.Verdict != "deferred",
		Judge:     "claude-" + model,
	}
	if err := d.SaveVerdict(verdict); err != nil {
		fmt.Fprintf(os.Stderr, "senate: warning: save verdict failed: %v\n", err)
	}

	// Save precedent
	prec := precedent.New(d.PrecedentIndexPath())
	if pErr := prec.Add(precedent.FromVerdict(verdict)); pErr != nil {
		fmt.Fprintf(os.Stderr, "senate: warning: precedent save failed: %v\n", pErr)
	}

	// JSON to stdout
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		errorf("json encode: %v", err)
		return 1
	}

	return 0
}

// cmdStart runs an interactive deliberation.
func cmdStart(args []string) int {
	flags := parseFlags(args)
	agents := parseInt(flags["agents"], 3)
	model := strings.TrimSpace(flags["model"])
	if model == "" {
		model = "sonnet"
	}

	// Create temp dir first so we know the verdict path
	tempDir, err := os.MkdirTemp("", "senate-*")
	if err != nil {
		errorf("create temp dir: %v", err)
		return 1
	}
	defer os.RemoveAll(tempDir)

	// We don't have a case ID yet — use a placeholder, Claude will fill it
	now := time.Now().UTC()
	caseID := core.NewCaseID(now)
	verdictFile := VerdictPath(tempDir)
	prompt := BuildSystemPrompt("start", verdictFile, caseID)

	promptFile := filepath.Join(tempDir, "protocol.md")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		errorf("write protocol: %v", err)
		return 1
	}

	agentsJSON := BuildAgentsJSON(agents)

	fmt.Printf("Senate convened — %d senators (%s)\n", agents, senatorLabel(agents))
	fmt.Printf("Ask your question and the senate will deliberate.\n\n")

	// Run Claude interactively (foreground)
	cmd := exec.Command("claude",
		"--dangerously-skip-permissions",
		"--model", model,
		"--system-prompt-file", promptFile,
		"--agents", agentsJSON,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		errorf("claude: %v", err)
		return 1
	}

	// Try to read and store the verdict
	stateDir := resolveStateDir(flags["state-dir"])
	if data, err := os.ReadFile(verdictFile); err == nil {
		var result AskResult
		if err := json.Unmarshal(data, &result); err == nil {
			d, sErr := store.New(stateDir)
			if sErr == nil {
				verdict := core.Verdict{
					CaseID:    result.CaseID,
					FiledAt:   now.Format(time.RFC3339),
					VerdictAt: time.Now().UTC().Format(time.RFC3339),
					Type:      "general",
					Summary:   "Interactive deliberation",
					Verdict:   core.Decision(result.Verdict),
					Reasoning: result.Reasoning,
					Implementation: result.Implementation,
					Dissent:   result.Dissent,
					Binding:   result.Verdict != "deferred",
					Judge:     "claude-" + model,
				}
				if err := d.SaveVerdict(verdict); err != nil {
					fmt.Fprintf(os.Stderr, "senate: warning: save verdict: %v\n", err)
				}
				prec := precedent.New(d.PrecedentIndexPath())
				_ = prec.Add(precedent.FromVerdict(verdict))
			}
		}
	}

	return 0
}

func cmdFileCase(args []string) int {
	flags := parseFlags(args)
	stateDir := resolveStateDir(flags["state-dir"])
	d, err := store.New(stateDir)
	if err != nil {
		errorf("init store: %v", err)
		return 1
	}

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

	validTypes := []string{"rule_evolution", "gate_criteria", "dispute", "priority", "architecture", "general"}
	if !contains(validTypes, c.Type) {
		errorf("invalid case type: %s (must be one of: %s)", c.Type, strings.Join(validTypes, ", "))
		return 1
	}

	c.Normalize(now)
	if err := c.Validate(); err != nil {
		errorf("case validation: %v", err)
		return 1
	}
	if err := d.SaveCase(c); err != nil {
		errorf("save case: %v", err)
		return 1
	}

	fmt.Println(c.ID)
	return 0
}

func cmdPrecedent(args []string) int {
	if len(args) == 0 {
		errorf("usage: senate precedent search --query <text>")
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
		errorf("usage: senate handoff --case-id <id>")
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
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
		prec := precedent.New(d.PrecedentIndexPath())
		_ = prec.Add(precedent.FromVerdict(v))
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

// --- Helpers ---

func extractPositionalArg(args []string) string {
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				i++
			}
			continue
		}
		return strings.TrimSpace(args[i])
	}
	return ""
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
	default:
		return "athena"
	}
}

func usage() {
	fmt.Print(`senate - multi-agent deliberation system

COMMANDS:
  senate ask "question" [flags]           Ask the senate (agent-friendly, JSON output)
  senate start [--agents <n>]             Convene the senate interactively
  senate health                           Check prerequisites
  senate file-case [flags]                File a new case
  senate precedent search --query <text>  Search verdict precedents
  senate handoff --case-id <id>           Create implementation bead from verdict
  senate version                          Print version

ASK FLAGS:
  --agents <n>           Number of senators (default 3, max 5)
  --type <type>          Case type (default: general)
  --filed-by <name>      Who is asking (default: "agent")
  --model <model>        Moderator model (default: sonnet)
  --state-dir <path>     Override state root

START FLAGS:
  --agents <n>           Number of senators (default 3, max 5)
  --model <model>        Model (default: sonnet)
  --state-dir <path>     Override state root

EXAMPLES:
  senate ask "Should we use Redis or Postgres for caching?"
  senate ask "Approve this architecture?" --agents 2 --type architecture
  echo "long question..." | senate ask --agents 3
  senate start --agents 2
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

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
