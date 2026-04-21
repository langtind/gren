package events

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"
)

// Tail reads NDJSON events from path as they are appended. Valid events are
// sent to events (blocking, ctx-aware — never drops). Lines that fail
// ParseLine are sent to invalid as a best-effort non-blocking send. Exits
// when ctx is cancelled. Designed to be run in a goroutine.
//
// Partial lines (no trailing newline) are buffered until the newline arrives,
// both during the live run and across reads. Emit only on newline.
func Tail(ctx context.Context, path string, events chan<- Event, invalid chan<- string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	var buf bytes.Buffer
	chunk := make([]byte, 4096)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Read everything available without blocking on empty.
		for {
			n, err := f.Read(chunk)
			if n > 0 {
				buf.Write(chunk[:n])
				if !drainLines(ctx, &buf, events, invalid) {
					return
				}
			}
			if err == io.EOF || n == 0 {
				break
			}
			if err != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			// Final drain — catch anything written since last read. We've
			// been cancelled, so drain with a fresh background ctx (we still
			// want to deliver remaining events to the consumer) but keep
			// sends best-effort: the consumer is expected to be winding up too.
			for {
				n, _ := f.Read(chunk)
				if n == 0 {
					break
				}
				buf.Write(chunk[:n])
			}
			drainLines(context.Background(), &buf, events, invalid)
			return
		case <-ticker.C:
		}
	}
}

// drainLines splits buf on \n, parses complete lines, and leaves any trailing
// partial line in buf for later. Returns false if ctx was cancelled mid-drain
// (caller should exit). Valid events use a blocking ctx-aware send to avoid
// silently dropping phase data; invalid lines are best-effort (they're
// diagnostic, not load-bearing).
func drainLines(ctx context.Context, buf *bytes.Buffer, events chan<- Event, invalid chan<- string) bool {
	for {
		data := buf.Bytes()
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			return true
		}
		line := string(data[:i])
		buf.Next(i + 1)
		if line == "" {
			continue
		}
		ev, err := ParseLine(line)
		if err != nil {
			select {
			case invalid <- line:
			default:
				// invalid channel full — drop; this is diagnostic.
			}
			continue
		}
		select {
		case events <- ev:
		case <-ctx.Done():
			return false
		}
	}
}
