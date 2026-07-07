//go:build windows

package core

import (
	"io"
	"os"
	"os/exec"
)

// runInteractiveCaptured on Windows has no pty: run with direct stdio but tee
// stdout/stderr into out on a best-effort basis. herdr is macOS/Linux; this
// only keeps the Windows build green.
func runInteractiveCaptured(cmd *exec.Cmd, stdin io.Reader, out io.Writer) error {
	cmd.Stdin = stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, out)
	cmd.Stderr = io.MultiWriter(os.Stderr, out)
	return cmd.Run()
}
