package events

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestTailer_ReadsCompleteLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.ndjson")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan Event, 8)
	invalid := make(chan string, 8)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		Tail(ctx, path, ch, invalid)
	}()

	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	defer f.Close()

	// Write one complete event.
	f.WriteString(`{"ts":"2026-04-20T22:51:52Z","phase":"p1","status":"start"}` + "\n")
	select {
	case got := <-ch:
		if got.Phase != "p1" {
			t.Errorf("unexpected phase: %s", got.Phase)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first event")
	}

	// Write partial then completion — tailer must buffer.
	f.WriteString(`{"ts":"2026-04-20T22:51:53Z","phase":"p2",`)
	time.Sleep(150 * time.Millisecond) // give tailer time to attempt a read
	select {
	case ev := <-ch:
		t.Errorf("unexpected event from partial line: %+v", ev)
	default:
	}
	f.WriteString(`"status":"ok"}` + "\n")
	select {
	case got := <-ch:
		if got.Phase != "p2" || got.Status != StatusOK {
			t.Errorf("unexpected event after completion: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for second event")
	}

	cancel()
	wg.Wait()
}

func TestTailer_ReportsInvalidLinesAndContinues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.ndjson")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan Event, 8)
	invalid := make(chan string, 8)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		Tail(ctx, path, ch, invalid)
	}()

	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	defer f.Close()
	f.WriteString("not-json\n")
	f.WriteString(`{"ts":"2026-04-20T22:51:53Z","phase":"ok","status":"ok"}` + "\n")

	// Must get the valid one even though garbage also appeared.
	select {
	case ev := <-ch:
		if ev.Phase != "ok" {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for valid event")
	}
	// Garbage should be reported.
	select {
	case line := <-invalid:
		if line == "" {
			t.Errorf("expected non-empty invalid line")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for invalid report")
	}

	cancel()
	wg.Wait()
}

func TestTailer_FinalDrainOnCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.ndjson")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan Event, 8)
	invalid := make(chan string, 8)
	done := make(chan struct{})
	go func() {
		Tail(ctx, path, ch, invalid)
		close(done)
	}()

	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	// Write just before cancel — tailer should still drain this.
	f.WriteString(`{"ts":"2026-04-20T22:51:54Z","phase":"final","status":"ok"}` + "\n")
	f.Close()
	time.Sleep(100 * time.Millisecond)
	cancel()

	<-done

	// Final event must be in the channel.
	select {
	case ev := <-ch:
		if ev.Phase != "final" {
			t.Errorf("unexpected phase on cancel: %s", ev.Phase)
		}
	default:
		t.Error("expected final event before cancel drain")
	}
}
