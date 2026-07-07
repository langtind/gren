package cli

import (
	"strings"
	"testing"
)

func TestTailLinesReturnsLastN(t *testing.T) {
	got := tailLines("a\nb\nc\nd\n", 2)
	if len(got) != 2 || got[0] != "c" || got[1] != "d" {
		t.Errorf("tailLines = %v, want [c d]", got)
	}
}

func TestLastErrorBlockFindsLastWithContinuation(t *testing.T) {
	logText := strings.Join([]string{
		"[2026-07-07 10:00:00.000] [INFO] started",
		"[2026-07-07 10:00:01.000] [ERROR] first fail",
		"[2026-07-07 10:00:02.000] [INFO] ok",
		"[2026-07-07 10:00:03.000] [ERROR] post-create hook failed: exit status 1",
		"stdout: boom",
		"more boom",
	}, "\n")
	got := lastErrorBlock(logText)
	if !strings.Contains(got, "exit status 1") || !strings.Contains(got, "more boom") {
		t.Errorf("lastErrorBlock missing last block/continuation: %q", got)
	}
	if strings.Contains(got, "first fail") {
		t.Errorf("lastErrorBlock returned an earlier error too: %q", got)
	}
}

func TestLastErrorBlockNoErrors(t *testing.T) {
	if got := lastErrorBlock("[2026-07-07 10:00:00.000] [INFO] fine"); !strings.Contains(got, "no [ERROR]") {
		t.Errorf("lastErrorBlock = %q, want no-error message", got)
	}
}
