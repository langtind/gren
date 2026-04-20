package events

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"
)

// Tail reads NDJSON events from path as they are appended. Valid events are
// sent to events; lines that fail ParseLine are sent to invalid (non-blocking
// drop if the channel is full). Exits when ctx is cancelled. Designed to be
// run in a goroutine.
//
// Partial lines (no trailing newline) are buffered until the newline arrives,
// both during the live run and across reads. Emit only on newline.
func Tail(ctx context.Context, path string, events chan<- Event, invalid chan<- string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

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
				drainLines(&buf, events, invalid)
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
			// Final drain — catch anything written since last read.
			for {
				n, _ := f.Read(chunk)
				if n == 0 {
					break
				}
				buf.Write(chunk[:n])
			}
			drainLines(&buf, events, invalid)
			return
		case <-ticker.C:
		}
	}
}

// drainLines splits buf on \n, parses complete lines, leaves any trailing
// partial line in buf for later.
func drainLines(buf *bytes.Buffer, events chan<- Event, invalid chan<- string) {
	for {
		data := buf.Bytes()
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			return
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
				// Caller's invalid buffer full — drop.
			}
			continue
		}
		select {
		case events <- ev:
		default:
			// Caller's event buffer full — drop to keep hook execution unblocked.
		}
	}
}
