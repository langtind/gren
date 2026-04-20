package events

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusStart Status = "start"
	StatusOK    Status = "ok"
	StatusError Status = "error"
	// StatusInterrupted is added post-hoc by gren when a start has no matching
	// ok/error and the hook exited non-zero. Hooks should not emit this directly,
	// but the parser accepts it so gren can round-trip events it produced itself.
	StatusInterrupted Status = "interrupted"
)

// Event is a single NDJSON event emitted by a hook.
type Event struct {
	TS     time.Time `json:"ts"`
	Phase  string    `json:"phase"`
	Status Status    `json:"status"`
	App    string    `json:"app,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

// ParseLine parses one NDJSON line. Returns error for empty, malformed,
// missing-required, or unknown-status lines. Callers should log and skip.
func ParseLine(line string) (Event, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Event{}, fmt.Errorf("empty line")
	}
	var raw struct {
		TS     string `json:"ts"`
		Phase  string `json:"phase"`
		Status string `json:"status"`
		App    string `json:"app"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return Event{}, fmt.Errorf("invalid json: %w", err)
	}
	if raw.TS == "" {
		return Event{}, fmt.Errorf("missing ts")
	}
	if raw.Phase == "" {
		return Event{}, fmt.Errorf("missing phase")
	}
	if raw.Status == "" {
		return Event{}, fmt.Errorf("missing status")
	}
	ts, err := time.Parse(time.RFC3339, raw.TS)
	if err != nil {
		return Event{}, fmt.Errorf("invalid ts: %w", err)
	}
	switch Status(raw.Status) {
	case StatusStart, StatusOK, StatusError, StatusInterrupted:
		// accepted
	default:
		return Event{}, fmt.Errorf("unknown status: %q", raw.Status)
	}
	return Event{
		TS:     ts,
		Phase:  raw.Phase,
		Status: Status(raw.Status),
		App:    raw.App,
		Detail: raw.Detail,
	}, nil
}
