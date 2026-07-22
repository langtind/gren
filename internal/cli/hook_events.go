package cli

import (
	"fmt"
	"io"

	"github.com/langtind/gren/internal/core"
	"github.com/langtind/gren/internal/events"
)

// eventGlyph returns the single-rune status indicator for an event.
// Kept identical between live streaming and batch summary rendering so the
// user sees the same glyph in both places.
func eventGlyph(s events.Status) string {
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

// RenderEventLine formats a single event into the indented display line used
// by both the batch summary (printHookEvents) and the live streaming
// observer. Shape: "  <glyph> [<app> / ]<phase>[  — <detail>]".
func RenderEventLine(e events.Event) string {
	name := e.Phase
	if e.App != "" {
		name = e.App + " / " + e.Phase
	}
	line := fmt.Sprintf("  %s %s", eventGlyph(e.Status), name)
	if e.Detail != "" {
		line += "  — " + e.Detail
	}
	return line
}

// streamEventsTo returns an observer callback that writes each event to w as
// a newline-terminated line. Flushes on every write so piped consumers see
// phases land as they happen.
func streamEventsTo(w io.Writer) func(events.Event) {
	return func(e events.Event) {
		fmt.Fprintln(w, RenderEventLine(e))
	}
}

// printHookEvents writes a phase summary whenever any hook in results produced
// events. Always runs — on success so the user sees what the hook actually did,
// and on failure so interrupted phases are visible.
//
// Writes to humanOut, not stdout: in JSON mode the same phases are carried
// inside the payload, and printing them again on stdout would corrupt it.
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
		fmt.Fprintln(humanOut())
		fmt.Fprintln(humanOut(), "Hook phases:")
		for _, e := range all {
			fmt.Fprintln(humanOut(), RenderEventLine(e))
		}
	}
	if failed && len(files) > 0 {
		fmt.Fprintln(humanOut())
		fmt.Fprintln(humanOut(), "Hook failed. Event log for post-mortem:")
		for _, f := range files {
			fmt.Fprintln(humanOut(), "  "+f)
		}
	}
}
