package cli

import (
	"fmt"

	"github.com/langtind/gren/internal/core"
	"github.com/langtind/gren/internal/events"
)

// printHookEvents writes a phase summary to stdout whenever any hook in
// results produced events. Always runs — on success so the user sees what
// the hook actually did, and on failure so interrupted phases are visible.
func printHookEvents(results []core.HookResult) {
	var all []events.Event
	var files []string
	for _, r := range results {
		all = append(all, r.Events...)
		if r.EventsFile != "" {
			files = append(files, r.EventsFile)
		}
	}
	failed := core.HooksFailed(results)
	if len(all) == 0 && !failed {
		return
	}
	if len(all) > 0 {
		fmt.Println()
		fmt.Println("Hook phases:")
		for _, e := range all {
			glyph := "?"
			switch e.Status {
			case events.StatusStart:
				glyph = "…"
			case events.StatusOK:
				glyph = "✓"
			case events.StatusError:
				glyph = "✗"
			case events.StatusInterrupted:
				glyph = "⊘"
			}
			name := e.Phase
			if e.App != "" {
				name = e.App + " / " + e.Phase
			}
			line := fmt.Sprintf("  %s %s", glyph, name)
			if e.Detail != "" {
				line += "  — " + e.Detail
			}
			fmt.Println(line)
		}
	}
	if failed && len(files) > 0 {
		fmt.Println()
		fmt.Println("Hook failed. Event log for post-mortem:")
		for _, f := range files {
			fmt.Println("  " + f)
		}
	}
}
