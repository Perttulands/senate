package senator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// getSenateRoot returns the senate project root directory.
// It checks SENATE_ROOT env var first, then falls back to $HOME/tools/senate.
func getSenateRoot() string {
	if root := os.Getenv("SENATE_ROOT"); root != "" {
		return root
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "tools", "senate")
}

// Config represents the complete Senate configuration
type Config struct {
	Version           string          `json:"version"`
	DefaultModel      string          `json:"default_model"`
	DefaultPanelSize  int             `json:"default_panel_size"`
	SessionTimeoutSec int             `json:"session_timeout_seconds"`
	Senators          []SenatorConfig `json:"senators"`
	AlternateSenators []SenatorConfig `json:"alternate_senators,omitempty"`
	JudgeConfig       JudgeConfig     `json:"judge_config"`
	Communication     CommConfig      `json:"communication"`
	Spawning          SpawnConfig     `json:"spawning"`
}

// SenatorConfig defines a single senator's configuration
type SenatorConfig struct {
	Name                 string         `json:"name"`
	Model                string         `json:"model"`
	Identity             Identity       `json:"identity"`
	SystemPromptTemplate string         `json:"system_prompt_template"`
	Memory               MemoryConfig   `json:"memory"`
	ResponseConfig       ResponseConfig `json:"response_config,omitempty"`
}

// Identity defines a senator's persona
type Identity struct {
	FullName      string   `json:"full_name"`
	Archetype     string   `json:"archetype"`
	DecisionStyle string   `json:"decision_style"`
	Values        []string `json:"values"`
	AntiValues    []string `json:"anti_values"`
}

// MemoryConfig defines memory access settings
type MemoryConfig struct {
	Path               string `json:"path"`
	PrecedentAccess    bool   `json:"precedent_access"`
	CaseHistoryLimit   int    `json:"case_history_limit"`
	UpdateLongTermMem  bool   `json:"update_long_term_memory"`
}

// ResponseConfig defines response generation parameters
type ResponseConfig struct {
	MaxTokens               int     `json:"max_tokens"`
	Temperature             float64 `json:"temperature"`
	RequireStructuredOutput bool    `json:"require_structured_output"`
}

// JudgeConfig defines the judge configuration
type JudgeConfig struct {
	Model                string  `json:"model"`
	SystemPromptTemplate string  `json:"system_prompt_template"`
	SynthesisApproach    string  `json:"synthesis_approach"`
	RequireUnanimous     bool    `json:"require_unanimous"`
	DissentThreshold     float64 `json:"dissent_threshold"`
}

// CommConfig defines communication settings
type CommConfig struct {
	ResponseMarkers  ResponseMarkers `json:"response_markers"`
	CaptureMethod    string          `json:"capture_method"`
	ResponseDetection string         `json:"response_detection"`
	PollingIntervalMs int            `json:"polling_interval_ms"`
	MaxWaitSeconds    int            `json:"max_wait_seconds"`
}

// ResponseMarkers defines the markers used to parse responses
type ResponseMarkers struct {
	InitialStart   string `json:"initial_start"`
	InitialEnd     string `json:"initial_end"`
	ChallengeStart string `json:"challenge_start"`
	ChallengeEnd   string `json:"challenge_end"`
	FinalStart     string `json:"final_start"`
	FinalEnd       string `json:"final_end"`
}

// SpawnConfig defines tmux spawning configuration
type SpawnConfig struct {
	TmuxSocket       string   `json:"tmux_socket"`
	SessionPrefix    string   `json:"session_prefix"`
	WorkingDirectory string   `json:"working_directory"`
	ClaudeFlags      []string `json:"claude_flags"`
}

// LoadConfig loads Senate configuration from JSON file
func LoadConfig(configPath string) (*Config, error) {
	// Default to example config if not specified
	if configPath == "" {
		configPath = filepath.Join(getSenateRoot(), "config", "senate-config-example.json")
	}

	// Make path absolute
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	// Read config file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config JSON: %w", err)
	}

	// Apply defaults
	config.applyDefaults()

	// Validate
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &config, nil
}

// applyDefaults sets default values for missing fields
func (c *Config) applyDefaults() {
	if c.DefaultModel == "" {
		c.DefaultModel = "claude:opus"
	}
	if c.DefaultPanelSize == 0 {
		c.DefaultPanelSize = 3
	}
	if c.SessionTimeoutSec == 0 {
		c.SessionTimeoutSec = 300 // 5 minutes
	}

	// Apply model defaults to senators
	for i := range c.Senators {
		if c.Senators[i].Model == "" {
			c.Senators[i].Model = c.DefaultModel
		}
		if c.Senators[i].ResponseConfig.MaxTokens == 0 {
			c.Senators[i].ResponseConfig.MaxTokens = 1500
		}
		if c.Senators[i].ResponseConfig.Temperature == 0 {
			c.Senators[i].ResponseConfig.Temperature = 0.7
		}
	}

	// Communication defaults
	if c.Communication.CaptureMethod == "" {
		c.Communication.CaptureMethod = "tmux_capture_pane"
	}
	if c.Communication.ResponseDetection == "" {
		c.Communication.ResponseDetection = "marker_based"
	}
	if c.Communication.PollingIntervalMs == 0 {
		c.Communication.PollingIntervalMs = 500
	}
	if c.Communication.MaxWaitSeconds == 0 {
		c.Communication.MaxWaitSeconds = 300
	}

	// Response markers defaults
	rm := &c.Communication.ResponseMarkers
	if rm.InitialStart == "" {
		rm.InitialStart = "=== INITIAL POSITION START ==="
	}
	if rm.InitialEnd == "" {
		rm.InitialEnd = "=== INITIAL POSITION END ==="
	}

	// Spawning defaults
	if c.Spawning.TmuxSocket == "" {
		c.Spawning.TmuxSocket = "/tmp/tmux-senate.sock"
	}
	if c.Spawning.SessionPrefix == "" {
		c.Spawning.SessionPrefix = "senator"
	}
	if c.Spawning.WorkingDirectory == "" {
		c.Spawning.WorkingDirectory = getSenateRoot()
	}
}

// validate checks if the configuration is valid
func (c *Config) validate() error {
	if len(c.Senators) == 0 {
		return fmt.Errorf("no senators defined")
	}

	// Validate each senator
	for _, senator := range c.Senators {
		if senator.Name == "" {
			return fmt.Errorf("senator missing name")
		}
		if senator.Identity.FullName == "" {
			return fmt.Errorf("senator %s missing full name", senator.Name)
		}
		if senator.SystemPromptTemplate == "" && senator.Identity.Archetype == "" {
			return fmt.Errorf("senator %s needs either template or archetype", senator.Name)
		}
	}

	return nil
}

// GetSenatorByName returns a senator config by name
func (c *Config) GetSenatorByName(name string) (*SenatorConfig, error) {
	// Check primary senators
	for i, s := range c.Senators {
		if s.Name == name {
			return &c.Senators[i], nil
		}
	}

	// Check alternate senators
	for i, s := range c.AlternateSenators {
		if s.Name == name {
			return &c.AlternateSenators[i], nil
		}
	}

	return nil, fmt.Errorf("senator %s not found", name)
}

// GetDefaultPanel returns the default panel of senators
func (c *Config) GetDefaultPanel() []SenatorConfig {
	size := c.DefaultPanelSize
	if size > len(c.Senators) {
		size = len(c.Senators)
	}

	panel := make([]SenatorConfig, size)
	copy(panel, c.Senators[:size])
	return panel
}

// LoadMemory loads a senator's memory files
func (s *SenatorConfig) LoadMemory(caseID string) (*SenatorMemory, error) {
	mem := &SenatorMemory{
		Identity: s.Identity,
	}

	if s.Memory.Path == "" {
		return mem, nil
	}

	// Load IDENTITY.md if exists
	identityPath := filepath.Join(s.Memory.Path, "IDENTITY.md")
	if content, err := os.ReadFile(identityPath); err == nil {
		mem.IdentityDoc = string(content)
	}

	// Load MEMORY.md if exists
	memoryPath := filepath.Join(s.Memory.Path, "MEMORY.md")
	if content, err := os.ReadFile(memoryPath); err == nil {
		mem.LongTermMemory = string(content)
	}

	// Load recent case history
	if s.Memory.CaseHistoryLimit > 0 {
		mem.RecentCases = s.loadRecentCases(s.Memory.CaseHistoryLimit)
	}

	return mem, nil
}

// loadRecentCases loads the N most recent case deliberations
func (s *SenatorConfig) loadRecentCases(limit int) []string {
	// This is simplified for MVP
	// In production, would scan cases/ directory and load most recent
	return []string{}
}

// SenatorMemory holds loaded memory content for a senator
type SenatorMemory struct {
	Identity       Identity
	IdentityDoc    string
	LongTermMemory string
	RecentCases    []string
	Precedents     []string
}