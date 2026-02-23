package senator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// TmuxSocket is the dedicated socket for Senate sessions
	TmuxSocket = "/tmp/tmux-senate.sock"

	// Response timeout for senator deliberation
	ResponseTimeout = 5 * time.Minute

	// Polling interval for response detection
	PollInterval = 500 * time.Millisecond

	// Response markers
	MarkerInitialStart = "=== INITIAL POSITION START ==="
	MarkerInitialEnd   = "=== INITIAL POSITION END ==="
	MarkerChallengeStart = "=== CHALLENGE RESPONSE START ==="
	MarkerChallengeEnd   = "=== CHALLENGE RESPONSE END ==="
	MarkerFinalStart   = "=== FINAL POSITION START ==="
	MarkerFinalEnd     = "=== FINAL POSITION END ==="
)

// SessionState represents the current state of a senator session
type SessionState string

const (
	StateInitializing SessionState = "initializing"
	StateReady        SessionState = "ready"
	StateDeliberating SessionState = "deliberating"
	StateResponded    SessionState = "responded"
	StateClosed       SessionState = "closed"
)

// SenatorSession manages a tmux session running a Claude senator
type SenatorSession struct {
	Name          string
	CaseID        string
	TmuxSessionID string
	Config        *SenatorConfig
	State         SessionState
	StartTime     time.Time
	TempDir       string // For system prompt files
	WorkingDir    string // Senate project root
}

// SpawnSenator creates a new senator tmux session with Claude
func SpawnSenator(config *SenatorConfig, caseID string, workingDir ...string) (*SenatorSession, error) {
	// Generate unique session name
	sessionID := fmt.Sprintf("senator-%s-%s", config.Name, caseID)

	// Create temp directory for this session
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("senate-%s-*", sessionID))
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	// Resolve working directory: explicit param > SENATE_ROOT env > $HOME/tools/senate
	senateRoot := getSenateRoot()
	if len(workingDir) > 0 && workingDir[0] != "" {
		senateRoot = workingDir[0]
	}

	session := &SenatorSession{
		Name:          config.Name,
		CaseID:        caseID,
		TmuxSessionID: sessionID,
		Config:        config,
		State:         StateInitializing,
		StartTime:     time.Now(),
		TempDir:       tempDir,
		WorkingDir:    senateRoot,
	}

	// Build system prompt from template
	promptFile := filepath.Join(tempDir, "system-prompt.md")
	if err := session.buildSystemPrompt(promptFile); err != nil {
		session.cleanup()
		return nil, fmt.Errorf("build system prompt: %w", err)
	}

	// Construct Claude command
	claudeCmd := fmt.Sprintf(
		"claude -p --model %s --dangerously-skip-permissions --system-prompt-file %s",
		config.Model,
		promptFile,
	)

	// Create tmux session
	cmd := exec.Command("tmux", "-S", TmuxSocket,
		"new-session", "-d", "-s", sessionID,
		"-c", session.WorkingDir,
		claudeCmd,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		session.cleanup()
		return nil, fmt.Errorf("spawn tmux session: %w\noutput: %s", err, output)
	}

	// Give Claude a moment to initialize
	time.Sleep(2 * time.Second)

	session.State = StateReady
	return session, nil
}

// SendPrompt sends a prompt to the senator and waits for response
func (s *SenatorSession) SendPrompt(prompt string) (string, error) {
	if s.State == StateClosed {
		return "", fmt.Errorf("session is closed")
	}

	// Update state
	s.State = StateDeliberating

	// Clear any existing output first
	if err := s.clearBuffer(); err != nil {
		return "", fmt.Errorf("clear buffer: %w", err)
	}

	// Send the prompt via tmux send-keys
	cmd := exec.Command("tmux", "-S", TmuxSocket,
		"send-keys", "-t", s.TmuxSessionID,
		prompt, "Enter",
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("send prompt: %w (output: %s)", err, string(output))
	}

	// Wait for response with timeout
	response, err := s.waitForResponse(ResponseTimeout)
	if err != nil {
		return "", fmt.Errorf("wait for response: %w", err)
	}

	s.State = StateResponded
	return response, nil
}

// waitForResponse polls for completion markers and captures response
func (s *SenatorSession) waitForResponse(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Capture current pane content
		output, err := s.captureOutput()
		if err != nil {
			return "", err
		}

		// Check for any completion marker
		if strings.Contains(output, MarkerInitialEnd) ||
			strings.Contains(output, MarkerChallengeEnd) ||
			strings.Contains(output, MarkerFinalEnd) {

			// Extract the structured response
			return s.extractResponse(output)
		}

		time.Sleep(PollInterval)
	}

	return "", fmt.Errorf("response timeout after %v", timeout)
}

// captureOutput captures the current tmux pane content
func (s *SenatorSession) captureOutput() (string, error) {
	cmd := exec.Command("tmux", "-S", TmuxSocket,
		"capture-pane", "-t", s.TmuxSessionID, "-p",
	)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("capture pane: %w", err)
	}

	return string(output), nil
}

// extractResponse extracts the structured response between markers
func (s *SenatorSession) extractResponse(output string) (string, error) {
	// Try each marker pair in order
	markers := []struct {
		start, end string
	}{
		{MarkerInitialStart, MarkerInitialEnd},
		{MarkerChallengeStart, MarkerChallengeEnd},
		{MarkerFinalStart, MarkerFinalEnd},
	}

	for _, m := range markers {
		startIdx := strings.Index(output, m.start)
		endIdx := strings.Index(output, m.end)

		if startIdx >= 0 && endIdx > startIdx {
			// Include the markers in the response
			response := output[startIdx : endIdx+len(m.end)]
			return strings.TrimSpace(response), nil
		}
	}

	return "", fmt.Errorf("no complete response found between markers")
}

// clearBuffer clears the tmux pane buffer
func (s *SenatorSession) clearBuffer() error {
	cmd := exec.Command("tmux", "-S", TmuxSocket,
		"clear-history", "-t", s.TmuxSessionID,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clear buffer: %w (output: %s)", err, string(output))
	}
	return nil
}

// buildSystemPrompt generates the system prompt file from template
func (s *SenatorSession) buildSystemPrompt(outputPath string) error {
	// For MVP, we'll use the template path directly
	// In production, this would load memory, precedents, etc.
	templatePath := filepath.Join(s.WorkingDir, s.Config.SystemPromptTemplate)

	// Check if template exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		// Fall back to a simple default prompt
		defaultPrompt := fmt.Sprintf(`You are %s in the Athena Senate.

Identity: %s
Archetype: %s
Decision Style: %s

When deliberating, structure your responses with clear markers:

=== INITIAL POSITION START ===
Stance: [approve/reject/amend/defer]
Reasoning: [your detailed reasoning]
Concerns: [key concerns as bullet points]
=== INITIAL POSITION END ===

Provide thoughtful analysis from your unique perspective.`,
			s.Config.Identity.FullName,
			s.Config.Identity.FullName,
			s.Config.Identity.Archetype,
			s.Config.Identity.DecisionStyle,
		)

		return os.WriteFile(outputPath, []byte(defaultPrompt), 0644)
	}

	// Read template
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	// Simple template substitution for case ID
	promptContent := strings.ReplaceAll(string(content), "{case_id}", s.CaseID)
	promptContent = strings.ReplaceAll(promptContent, "{{CASE_ID}}", s.CaseID)

	// Write to output file
	return os.WriteFile(outputPath, []byte(promptContent), 0644)
}

// Close terminates the senator session
func (s *SenatorSession) Close() error {
	if s.State == StateClosed {
		return nil
	}

	// Kill tmux session
	cmd := exec.Command("tmux", "-S", TmuxSocket,
		"kill-session", "-t", s.TmuxSessionID,
	)

	// Capture output but ignore error if session doesn't exist
	if output, err := cmd.CombinedOutput(); err != nil {
		// Only log if it's not a "session not found" error
		if !strings.Contains(string(output), "can't find session") {
			fmt.Fprintf(os.Stderr, "warning: failed to kill tmux session %s: %v (output: %s)\n",
				s.TmuxSessionID, err, string(output))
		}
	}

	// Cleanup temp files
	s.cleanup()

	s.State = StateClosed
	return nil
}

// cleanup removes temporary files
func (s *SenatorSession) cleanup() {
	if s.TempDir != "" {
		os.RemoveAll(s.TempDir)
	}
}

// IsAlive checks if the tmux session is still running
func (s *SenatorSession) IsAlive() bool {
	cmd := exec.Command("tmux", "-S", TmuxSocket,
		"has-session", "-t", s.TmuxSessionID,
	)
	return cmd.Run() == nil
}

// GetState returns the current session state
func (s *SenatorSession) GetState() SessionState {
	if !s.IsAlive() && s.State != StateClosed {
		s.State = StateClosed
	}
	return s.State
}