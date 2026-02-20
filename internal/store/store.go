package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Perttulands/senate/internal/core"
)

const (
	casesDir       = "cases"
	verdictsDir    = "verdicts"
	transcriptsDir = "transcripts"
	precedentsDir  = "precedents"
	outboxDir      = "outbox"
)

// Dir provides filesystem storage for Senate state.
type Dir struct {
	Root string
}

// New initializes the Senate state directory tree.
func New(root string) (*Dir, error) {
	if strings.TrimSpace(root) == "" {
		root = "state"
	}
	paths := []string{
		filepath.Join(root, casesDir),
		filepath.Join(root, verdictsDir),
		filepath.Join(root, transcriptsDir),
		filepath.Join(root, precedentsDir),
		filepath.Join(root, outboxDir),
	}
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", p, err)
		}
	}
	return &Dir{Root: root}, nil
}

func (d *Dir) CasePath(caseID string) string {
	return filepath.Join(d.Root, casesDir, caseID+".json")
}

func (d *Dir) VerdictPath(caseID string) string {
	return filepath.Join(d.Root, verdictsDir, caseID+".json")
}

func (d *Dir) TranscriptPath(caseID string) string {
	return filepath.Join(d.Root, transcriptsDir, caseID+".json")
}

func (d *Dir) PrecedentIndexPath() string {
	return filepath.Join(d.Root, precedentsDir, "index.jsonl")
}

func (d *Dir) RelayOutboxPath() string {
	return filepath.Join(d.Root, outboxDir, "case-filed.jsonl")
}

func (d *Dir) SaveCase(c core.Case) error {
	if err := c.Validate(); err != nil {
		return err
	}
	return atomicWriteJSON(d.CasePath(c.ID), c)
}

func (d *Dir) LoadCase(caseID string) (core.Case, error) {
	var c core.Case
	data, err := os.ReadFile(d.CasePath(caseID))
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("decode case %s: %w", caseID, err)
	}
	return c, nil
}

func (d *Dir) SaveTranscript(t core.Transcript) error {
	if strings.TrimSpace(t.CaseID) == "" {
		return fmt.Errorf("transcript.case_id is required")
	}
	return atomicWriteJSON(d.TranscriptPath(t.CaseID), t)
}

func (d *Dir) SaveVerdict(v core.Verdict) error {
	if err := v.Validate(); err != nil {
		return err
	}
	return atomicWriteJSON(d.VerdictPath(v.CaseID), v)
}

func (d *Dir) LoadVerdict(caseID string) (core.Verdict, error) {
	var v core.Verdict
	data, err := os.ReadFile(d.VerdictPath(caseID))
	if err != nil {
		return v, err
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("decode verdict %s: %w", caseID, err)
	}
	return v, nil
}

func atomicWriteJSON(path string, v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return atomicWrite(path, out)
}

func atomicWrite(path string, body []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
