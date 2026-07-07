//go:build !windows

package core

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestRunInteractiveCapturedTeesOutput(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo hello-stdout; echo hello-stderr >&2")
	var out bytes.Buffer
	if err := runInteractiveCaptured(cmd, strings.NewReader(""), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "hello-stdout") {
		t.Errorf("captured output missing stdout: %q", got)
	}
	if !strings.Contains(got, "hello-stderr") {
		t.Errorf("captured output missing stderr (pty merges streams): %q", got)
	}
}

func TestRunInteractiveCapturedPropagatesExit(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 3")
	var out bytes.Buffer
	if err := runInteractiveCaptured(cmd, strings.NewReader(""), &out); err == nil {
		t.Fatal("expected non-nil error for exit 3")
	}
}
