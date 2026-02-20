package precedent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Perttulands/senate/internal/core"
)

// Record is one searchable Senate verdict precedent.
type Record struct {
	CaseID         string        `json:"case_id"`
	Type           string        `json:"type"`
	Summary        string        `json:"summary"`
	Verdict        core.Decision `json:"verdict"`
	Reasoning      string        `json:"reasoning"`
	Implementation string        `json:"implementation"`
	Dissent        string        `json:"dissent,omitempty"`
	Binding        bool          `json:"binding"`
	VerdictAt      string        `json:"verdict_at"`
	Judge          string        `json:"judge"`
	BeadID         string        `json:"bead_id,omitempty"`
	Keywords       []string      `json:"keywords,omitempty"`
}

func FromVerdict(v core.Verdict) Record {
	keywords := extractKeywords(strings.Join([]string{v.Summary, v.Reasoning, v.Implementation, v.Dissent}, " "))
	record := Record{
		CaseID:         v.CaseID,
		Type:           v.Type,
		Summary:        v.Summary,
		Verdict:        v.Verdict,
		Reasoning:      v.Reasoning,
		Implementation: v.Implementation,
		Dissent:        v.Dissent,
		Binding:        v.Binding,
		VerdictAt:      v.VerdictAt,
		Judge:          v.Judge,
		Keywords:       keywords,
	}
	if v.Handoff != nil {
		record.BeadID = v.Handoff.BeadID
	}
	return record
}

func (r Record) Validate() error {
	if strings.TrimSpace(r.CaseID) == "" {
		return fmt.Errorf("record.case_id is required")
	}
	if strings.TrimSpace(r.Type) == "" {
		return fmt.Errorf("record.type is required")
	}
	if strings.TrimSpace(r.Summary) == "" {
		return fmt.Errorf("record.summary is required")
	}
	if err := r.Verdict.Validate(); err != nil {
		return fmt.Errorf("record.verdict: %w", err)
	}
	if strings.TrimSpace(r.VerdictAt) == "" {
		return fmt.Errorf("record.verdict_at is required")
	}
	if _, err := time.Parse(time.RFC3339, r.VerdictAt); err != nil {
		return fmt.Errorf("record.verdict_at must be RFC3339: %w", err)
	}
	return nil
}

// Store appends and searches precedent records in JSONL format.
type Store struct {
	path string
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Add(record Record) error {
	if err := record.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}

func (s *Store) LoadAll() ([]Record, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []Record
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if err := rec.Validate(); err != nil {
			continue
		}
		out = append(out, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type SearchOptions struct {
	Type    string
	Verdict core.Decision
	Limit   int
}

func (s *Store) Search(query string, opts SearchOptions) ([]Record, error) {
	records, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	queryTokens := extractKeywords(query)
	type scored struct {
		record Record
		score  int
		time   time.Time
	}

	results := make([]scored, 0, len(records))
	for _, rec := range records {
		if opts.Type != "" && rec.Type != opts.Type {
			continue
		}
		if opts.Verdict != "" && rec.Verdict != opts.Verdict {
			continue
		}
		score := scoreRecord(rec, queryTokens)
		if len(queryTokens) > 0 && score == 0 {
			continue
		}
		tm, _ := time.Parse(time.RFC3339, rec.VerdictAt)
		results = append(results, scored{record: rec, score: score, time: tm})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score == results[j].score {
			return results[i].time.After(results[j].time)
		}
		return results[i].score > results[j].score
	})

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]Record, 0, len(results))
	for _, r := range results {
		out = append(out, r.record)
	}
	return out, nil
}

func scoreRecord(rec Record, queryTokens []string) int {
	if len(queryTokens) == 0 {
		return 1
	}
	bag := strings.ToLower(strings.Join([]string{
		rec.CaseID,
		rec.Type,
		rec.Summary,
		rec.Reasoning,
		rec.Implementation,
		rec.Dissent,
		strings.Join(rec.Keywords, " "),
	}, " "))
	score := 0
	for _, q := range queryTokens {
		if strings.Contains(bag, q) {
			score++
		}
	}
	return score
}

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
