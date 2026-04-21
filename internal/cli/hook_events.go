package cli

import (
	"fmt"
	"io"
	"os"

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
			fmt.Println(RenderEventLine(e))
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

// withStderrEventStream registers a live-streaming observer on wm that
// writes each event to stderr, runs fn, then clears the observer. Returns
// whatever fn returns. stderr is chosen so captured stdout from the hook
// body is not polluted by event lines.
func withStderrEventStream(wm *core.WorktreeManager, fn func()) {
	wm.SetEventObserver(streamEventsTo(os.Stderr))
	defer wm.SetEventObserver(nil)
	fn()
}
