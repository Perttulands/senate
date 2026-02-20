package deliberation

import (
	"fmt"
	"strings"

	"github.com/Perttulands/senate/internal/core"
)

// Perspective configures one panel seat.
type Perspective struct {
	Name      string
	Model     string
	Directive string
}

var defaultCatalog = []Perspective{
	{
		Name:      "pragmatist",
		Model:     "claude:sonnet",
		Directive: "Prioritize velocity and practical outcomes while keeping risk acceptable.",
	},
	{
		Name:      "purist",
		Model:     "claude:sonnet",
		Directive: "Prioritize correctness, consistency, and long-term maintainability.",
	},
	{
		Name:      "skeptic",
		Model:     "claude:sonnet",
		Directive: "Challenge assumptions and focus on hidden risks and missing evidence.",
	},
	{
		Name:      "steward",
		Model:     "claude:haiku",
		Directive: "Prioritize operational stability and low blast-radius implementation.",
	},
	{
		Name:      "advocate",
		Model:     "claude:haiku",
		Directive: "Prioritize user value and impact while preserving trust.",
	},
}

// BuildPanel constructs N panel members with varied perspectives.
func BuildPanel(n int, names []string, models []string) []Perspective {
	if n <= 0 {
		n = 3
	}
	catalog := defaultCatalog
	if len(names) > 0 {
		catalog = make([]Perspective, 0, len(names))
		for _, name := range names {
			clean := strings.TrimSpace(name)
			if clean == "" {
				continue
			}
			catalog = append(catalog, Perspective{
				Name:      clean,
				Model:     "claude:sonnet",
				Directive: fmt.Sprintf("Represent the %s perspective with rigorous argumentation.", clean),
			})
		}
		if len(catalog) == 0 {
			catalog = defaultCatalog
		}
	}

	panel := make([]Perspective, 0, n)
	for i := 0; i < n; i++ {
		seat := catalog[i%len(catalog)]
		if len(models) > 0 {
			if m := strings.TrimSpace(models[i%len(models)]); m != "" {
				seat.Model = m
			}
		}
		panel = append(panel, Perspective{
			Name:      seat.Name,
			Model:     seat.Model,
			Directive: seat.Directive,
		})
	}
	return panel
}

func toPanelMembers(panel []Perspective) []core.PanelMember {
	out := make([]core.PanelMember, 0, len(panel))
	for i, p := range panel {
		out = append(out, core.PanelMember{
			AgentID:     fmt.Sprintf("agent-%d", i+1),
			Model:       p.Model,
			Perspective: p.Name,
		})
	}
	return out
}
