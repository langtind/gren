//go:build !windows

package core

import (
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// eot is Ctrl-D, the canonical-mode end-of-transmission character.
const eot = 0x04

// runInteractiveCaptured runs cmd attached to a pseudo-terminal, copying stdin
// into the pty and the pty output to out (typically a MultiWriter of the real
// terminal and a disk sink). The child sees a real TTY, so interactive tools,
// prompts, and colors behave exactly as with direct stdio. Returns cmd's exit
// error. stdout and stderr are merged (a single pty), as a terminal shows them.
func runInteractiveCaptured(cmd *exec.Cmd, stdin io.Reader, out io.Writer) error {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = ptmx.Close() }()

	// Keep the pty sized to the controlling terminal.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	ch <- syscall.SIGWINCH // initial resize
	defer signal.Stop(ch)

	// Put the outer terminal in raw mode so keystrokes pass through untouched;
	// the pty's line discipline handles echo/cooking. Skip when stdin isn't a
	// terminal (tests / non-tty callers).
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if oldState, err := term.MakeRaw(int(f.Fd())); err == nil {
			defer func() { _ = term.Restore(int(f.Fd()), oldState) }()
		}
	}

	// When the copy ends, no more input will ever arrive. A pty has no
	// half-close, so send EOT: the line discipline turns it into a read() of 0,
	// which is EOF to the child. Without it a hook that prompts blocks forever
	// on a pty nobody is attached to. With a real terminal the copy only ends
	// when stdin closes, where EOF is equally correct.
	go func() {
		_, _ = io.Copy(ptmx, stdin)
		_, _ = ptmx.Write([]byte{eot})
	}()
	_, _ = io.Copy(out, ptmx) // drains until the child closes the pty (EIO on Linux is expected)
	return cmd.Wait()
}
