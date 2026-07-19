package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/langtind/gren/internal/events"
)

func TestRenderHookEvents_ShowsGlyphsAndPhases(t *testing.T) {
	now := time.Now()
	evs := []events.Event{
		{TS: now, Phase: "install", Status: events.StatusStart},
		{TS: now, Phase: "install", Status: events.StatusOK, Detail: "bun install done"},
		{TS: now, Phase: "migrate", Status: events.StatusInterrupted, Detail: "hook exited before phase completed"},
	}
	out := RenderHookEvents(evs)
	for _, want := range []string{"install", "migrate", "✓", "⊘", "bun install done"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %s", want, out)
		}
	}
}

func TestRenderHookEvents_Empty(t *testing.T) {
	if out := RenderHookEvents(nil); out != "" {
		t.Errorf("expected empty string for nil events, got: %q", out)
	}
}

func TestRenderHookEvents_SamePhaseDifferentApps(t *testing.T) {
	now := time.Now()
	evs := []events.Event{
		{TS: now, Phase: "migrate", App: "web", Status: events.StatusOK},
		{TS: now, Phase: "migrate", App: "api", Status: events.StatusError, Detail: "alembic failed"},
	}
	out := RenderHookEvents(evs)
	if !strings.Contains(out, "web / migrate") {
		t.Errorf("expected 'web / migrate', got: %s", out)
	}
	if !strings.Contains(out, "api / migrate") {
		t.Errorf("expected 'api / migrate', got: %s", out)
	}
	if !strings.Contains(out, "alembic failed") {
		t.Errorf("expected detail preserved, got: %s", out)
	}
}

func TestRenderHookEvents_StatusFollowsLastEvent(t *testing.T) {
	now := time.Now()
	evs := []events.Event{
		{TS: now, Phase: "install", Status: events.StatusStart},
		{TS: now, Phase: "install", Status: events.StatusOK},
	}
	out := RenderHookEvents(evs)
	if !strings.Contains(out, "✓") {
		t.Errorf("expected final ok glyph, got: %s", out)
	}
	if strings.Contains(out, "…") {
		t.Errorf("should not render start glyph when phase closed, got: %s", out)
	}
}
