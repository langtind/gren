package ui

import (
	"fmt"
	"strings"

	"github.com/langtind/gren/internal/events"
)

// RenderHookEvents collapses a flat event list into per-phase status lines.
// Returns empty string if there are no events.
func RenderHookEvents(evs []events.Event) string {
	if len(evs) == 0 {
		return ""
	}
	type row struct {
		phase, app, detail string
		status             events.Status
	}
	order := []string{}
	rows := map[string]*row{}
	for _, e := range evs {
		k := e.Phase + "\x00" + e.App
		r, ok := rows[k]
		if !ok {
			r = &row{phase: e.Phase, app: e.App}
			rows[k] = r
			order = append(order, k)
		}
		r.status = e.Status
		if e.Detail != "" {
			r.detail = e.Detail
		}
	}
	var b strings.Builder
	for _, k := range order {
		r := rows[k]
		glyph := glyphFor(r.status)
		name := r.phase
		if r.app != "" {
			name = r.app + " / " + r.phase
		}
		line := fmt.Sprintf("  %s %s", glyph, name)
		if r.detail != "" {
			line += "  — " + r.detail
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func glyphFor(s events.Status) string {
	switch s {
	case events.StatusStart:
		return "…"
	case events.StatusOK:
		return "✓"
	case events.StatusError:
		return "✗"
	case events.StatusInterrupted:
		return "⊘"
	default:
		return "?"
	}
}
