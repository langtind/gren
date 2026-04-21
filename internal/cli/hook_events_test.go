package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/langtind/gren/internal/events"
)

func TestRenderEventLine_IncludesGlyphPhaseAndDetail(t *testing.T) {
	got := RenderEventLine(events.Event{Phase: "install", Status: events.StatusOK, Detail: "deps up to date"})
	if !strings.Contains(got, "✓") || !strings.Contains(got, "install") || !strings.Contains(got, "deps up to date") {
		t.Errorf("missing expected fragments in line: %q", got)
	}
	if !strings.HasPrefix(got, "  ") {
		t.Errorf("expected 2-space indent prefix, got: %q", got)
	}
}

func TestRenderEventLine_PrefixesAppWhenSet(t *testing.T) {
	got := RenderEventLine(events.Event{Phase: "migrate", App: "jobb", Status: events.StatusStart})
	if !strings.Contains(got, "jobb / migrate") {
		t.Errorf("expected 'jobb / migrate' in: %q", got)
	}
}

func TestStreamEventsTo_WritesOneLinePerEvent(t *testing.T) {
	var buf bytes.Buffer
	obs := streamEventsTo(&buf)
	obs(events.Event{Phase: "a", Status: events.StatusStart})
	obs(events.Event{Phase: "a", Status: events.StatusOK})
	obs(events.Event{Phase: "b", Status: events.StatusError, Detail: "boom"})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "… a") || !strings.Contains(lines[1], "✓ a") || !strings.Contains(lines[2], "✗ b") || !strings.Contains(lines[2], "boom") {
		t.Errorf("unexpected line content: %#v", lines)
	}
}
