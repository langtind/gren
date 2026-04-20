package events

import (
	"testing"
	"time"
)

func TestParseLine_Valid(t *testing.T) {
	line := `{"ts":"2026-04-20T22:51:52Z","phase":"migrate","status":"start","app":"referat","detail":"alembic upgrade head"}`
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Phase != "migrate" || ev.Status != StatusStart || ev.App != "referat" {
		t.Errorf("unexpected event: %+v", ev)
	}
	if ev.Detail != "alembic upgrade head" {
		t.Errorf("unexpected detail: %q", ev.Detail)
	}
	wantTS, _ := time.Parse(time.RFC3339, "2026-04-20T22:51:52Z")
	if !ev.TS.Equal(wantTS) {
		t.Errorf("ts mismatch: got %v want %v", ev.TS, wantTS)
	}
}

func TestParseLine_MinimalFields(t *testing.T) {
	line := `{"ts":"2026-04-20T22:51:52Z","phase":"install","status":"ok"}`
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.App != "" || ev.Detail != "" {
		t.Errorf("expected empty optional fields, got %+v", ev)
	}
}

func TestParseLine_MissingRequired(t *testing.T) {
	cases := []string{
		`{"phase":"x","status":"ok"}`,                 // missing ts
		`{"ts":"2026-04-20T22:51:52Z","status":"ok"}`, // missing phase
		`{"ts":"2026-04-20T22:51:52Z","phase":"x"}`,   // missing status
	}
	for _, line := range cases {
		if _, err := ParseLine(line); err == nil {
			t.Errorf("expected error for %q", line)
		}
	}
}

func TestParseLine_UnknownStatus(t *testing.T) {
	line := `{"ts":"2026-04-20T22:51:52Z","phase":"x","status":"warn"}`
	if _, err := ParseLine(line); err == nil {
		t.Errorf("expected error for unknown status")
	}
}

func TestParseLine_GarbageJSON(t *testing.T) {
	cases := []string{"not-json", "", "   ", "{", `{"ts":"bad-timestamp","phase":"x","status":"ok"}`}
	for _, line := range cases {
		if _, err := ParseLine(line); err == nil {
			t.Errorf("expected error for %q", line)
		}
	}
}

func TestParseLine_HugeDetail(t *testing.T) {
	big := make([]byte, 128*1024)
	for i := range big {
		big[i] = 'x'
	}
	line := `{"ts":"2026-04-20T22:51:52Z","phase":"x","status":"ok","detail":"` + string(big) + `"}`
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ev.Detail) != len(big) {
		t.Errorf("detail truncated")
	}
}
