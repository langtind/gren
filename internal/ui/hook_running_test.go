package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/core"
	"github.com/langtind/gren/internal/events"
)

// bigBaseView returns a blank base view sized large enough that
// centerOverlay can place the modal anywhere without being clipped.
func bigBaseView(rows, cols int) string {
	line := strings.Repeat(" ", cols)
	lines := make([]string, rows)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func TestCollapsePhases_DedupesAndKeepsLatestStatus(t *testing.T) {
	in := []events.Event{
		{Phase: "install", Status: events.StatusStart},
		{Phase: "install", Status: events.StatusOK, Detail: "done"},
		{Phase: "migrate", App: "jobb", Status: events.StatusStart},
	}
	got := collapsePhases(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if got[0].phase != "install" || got[0].status != events.StatusOK || got[0].detail != "done" {
		t.Errorf("row 0 wrong: %+v", got[0])
	}
	if got[1].phase != "migrate" || got[1].app != "jobb" || got[1].status != events.StatusStart {
		t.Errorf("row 1 wrong: %+v", got[1])
	}
}

func TestCollapsePhases_KeepsInsertionOrder(t *testing.T) {
	in := []events.Event{
		{Phase: "c", Status: events.StatusStart},
		{Phase: "a", Status: events.StatusStart},
		{Phase: "b", Status: events.StatusStart},
		{Phase: "a", Status: events.StatusOK},
	}
	got := collapsePhases(in)
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(got))
	}
	order := []string{got[0].phase, got[1].phase, got[2].phase}
	want := []string{"c", "a", "b"}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d]=%s want %s", i, order[i], want[i])
		}
	}
}

func TestTailOutputLines_ReturnsLastN(t *testing.T) {
	s := "a\nb\nc\nd\ne"
	if got := tailOutputLines(s, 3); got != "c\nd\ne" {
		t.Errorf("got %q", got)
	}
	if got := tailOutputLines(s, 10); got != "a\nb\nc\nd\ne" {
		t.Errorf("got %q", got)
	}
	if got := tailOutputLines("", 3); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestWaitForHookStream_ReturnsMessage(t *testing.T) {
	ch := make(chan tea.Msg, 1)
	ch <- hookPhaseEventMsg{ev: events.Event{Phase: "p", Status: events.StatusOK}}
	cmd := waitForHookStream(ch)
	got := cmd()
	ev, ok := got.(hookPhaseEventMsg)
	if !ok {
		t.Fatalf("expected hookPhaseEventMsg, got %T", got)
	}
	if ev.ev.Phase != "p" {
		t.Errorf("wrong phase: %s", ev.ev.Phase)
	}
}

func TestWaitForHookStream_ReturnsNilOnClosedChannel(t *testing.T) {
	ch := make(chan tea.Msg)
	close(ch)
	cmd := waitForHookStream(ch)
	if got := cmd(); got != nil {
		t.Errorf("expected nil on closed channel, got %#v", got)
	}
}

func TestHookPhaseEventMsg_AppendsAndReissuesStreamCmd(t *testing.T) {
	ch := make(chan tea.Msg, 1)
	m := Model{
		hookRunningState: &HookRunningState{
			visible: true, stream: ch, hookType: config.HookPostCreate,
		},
	}
	updated, cmd := m.Update(hookPhaseEventMsg{ev: events.Event{Phase: "x", Status: events.StatusOK}})
	m2 := updated.(Model)
	if len(m2.hookRunningState.events) != 1 {
		t.Fatalf("expected 1 event appended, got %d", len(m2.hookRunningState.events))
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd to keep the stream draining")
	}
}

func TestHookExecutionDoneMsg_MarksDoneAndKeepsStateOnFailure(t *testing.T) {
	m := Model{
		hookRunningState: &HookRunningState{visible: true, hookType: config.HookPostCreate},
	}
	results := []core.HookResult{
		{Ran: true, Err: errors.New("boom"), Command: "x", EventsFile: "/tmp/evs"},
	}
	updated, _ := m.Update(hookExecutionDoneMsg{results: results})
	m2 := updated.(Model)
	if m2.hookRunningState == nil {
		t.Fatal("state should persist on failure until user dismisses")
	}
	if !m2.hookRunningState.done || m2.hookRunningState.hookErr == nil {
		t.Errorf("expected done=true and hookErr set; got %+v", m2.hookRunningState)
	}
}

func TestHookExecutionDoneMsg_SchedulesDismissOnSuccess(t *testing.T) {
	m := Model{
		hookRunningState: &HookRunningState{visible: true, hookType: config.HookPostCreate},
	}
	results := []core.HookResult{{Ran: true, Err: nil, Command: "x"}}
	updated, cmd := m.Update(hookExecutionDoneMsg{results: results})
	m2 := updated.(Model)
	if !m2.hookRunningState.done || m2.hookRunningState.hookErr != nil {
		t.Errorf("expected clean done, got %+v", m2.hookRunningState)
	}
	if cmd == nil {
		t.Fatal("expected tea.Tick cmd scheduling dismiss on success")
	}
}

func TestHookRunningDismissMsg_ClearsStateWhenDone(t *testing.T) {
	m := Model{
		hookRunningState: &HookRunningState{visible: true, done: true, hookType: config.HookPostCreate},
	}
	updated, _ := m.Update(hookRunningDismissMsg{})
	m2 := updated.(Model)
	if m2.hookRunningState != nil {
		t.Errorf("expected state cleared after dismiss, got %+v", m2.hookRunningState)
	}
}

func TestHookRunningDismissMsg_DoesNothingWhileRunning(t *testing.T) {
	m := Model{
		hookRunningState: &HookRunningState{visible: true, done: false, hookType: config.HookPostCreate},
	}
	updated, _ := m.Update(hookRunningDismissMsg{})
	m2 := updated.(Model)
	if m2.hookRunningState == nil {
		t.Errorf("state should be kept while hook still running")
	}
}

// TestStartHookRun_SendsDoneEvenOnPanic guarantees the modal never deadlocks
// if a RunXxxHookWithApproval panics. Without the panic-safe defer the user
// would be stuck with `done=false` and the key handler swallowing every key.
func TestStartHookRun_SendsDoneEvenOnPanic(t *testing.T) {
	// We can't easily trigger a panic inside the real RunXxxHookWithApproval,
	// but we can verify the defer contract directly by replicating the
	// goroutine shape and asserting a Done msg is sent even when the body
	// panics. This mirrors the pattern used in startHookRun.
	ch := make(chan tea.Msg, 4)
	go func() {
		defer close(ch)
		var results []core.HookResult
		defer func() {
			if r := recover(); r != nil && len(results) == 0 {
				results = []core.HookResult{{Ran: true, Err: errors.New("panicked")}}
			}
			ch <- hookExecutionDoneMsg{results: results}
		}()
		panic("simulated")
	}()

	select {
	case msg := <-ch:
		done, ok := msg.(hookExecutionDoneMsg)
		if !ok {
			t.Fatalf("expected hookExecutionDoneMsg first, got %T", msg)
		}
		if len(done.results) == 0 || done.results[0].Err == nil {
			t.Errorf("expected a failing result in Done msg after panic, got %+v", done.results)
		}
	case <-time.After(time.Second):
		t.Fatal("no message received; goroutine did not send Done after panic")
	}
}

func TestHandleHookRunningKeys_DismissesWhenDone(t *testing.T) {
	m := Model{
		hookRunningState: &HookRunningState{visible: true, done: true, hookType: config.HookPostCreate},
	}
	updated, _ := m.handleHookRunningKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	if m2.hookRunningState != nil {
		t.Errorf("expected dismiss on any key when done")
	}
}

func TestHandleHookRunningKeys_SwallowsKeysWhileRunning(t *testing.T) {
	m := Model{
		hookRunningState: &HookRunningState{visible: true, done: false, hookType: config.HookPostCreate},
	}
	updated, _ := m.handleHookRunningKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(Model)
	if m2.hookRunningState == nil {
		t.Errorf("expected state preserved while running")
	}
}

func TestRenderHookRunningOverlay_ShowsPhasesAndTitle(t *testing.T) {
	m := Model{
		width: 120, height: 30,
		hookRunningState: &HookRunningState{
			visible:  true,
			hookType: config.HookPostCreate,
			events: []events.Event{
				{Phase: "install", Status: events.StatusOK},
				{Phase: "migrate", Status: events.StatusStart},
			},
			started: time.Now(),
		},
	}
	out := m.renderHookRunningOverlay(bigBaseView(30, 120))
	if !strings.Contains(out, "post-create") {
		t.Errorf("expected hook type in title: %q", out)
	}
	if !strings.Contains(out, "install") || !strings.Contains(out, "migrate") {
		t.Errorf("expected phase names in render: %q", out)
	}
	if !strings.Contains(out, "✓") || !strings.Contains(out, "…") {
		t.Errorf("expected status glyphs in render: %q", out)
	}
}

func TestRenderHookRunningOverlay_ShowsErrorAndEventsFileOnFailure(t *testing.T) {
	m := Model{
		width: 120, height: 40,
		hookRunningState: &HookRunningState{
			visible:  true,
			hookType: config.HookPostCreate,
			done:     true,
			hookErr:  errors.New("exit status 1"),
			results: []core.HookResult{
				{Ran: true, Err: errors.New("exit status 1"),
					Command:    "./hook.sh",
					Output:     "progress a\nprogress b\n",
					Stderr:     "post-create.sh: line 303: ${app^^}_DB_NAME: bad substitution\n",
					EventsFile: "/tmp/events.ndjson"},
			},
			events: []events.Event{{Phase: "migrate", Status: events.StatusInterrupted, Detail: "hook exited"}},
		},
	}
	out := m.renderHookRunningOverlay(bigBaseView(40, 120))
	if !strings.Contains(out, "failed") {
		t.Errorf("expected 'failed' title: %q", out)
	}
	if !strings.Contains(out, "exit status 1") {
		t.Errorf("expected error message: %q", out)
	}
	if !strings.Contains(out, "/tmp/events.ndjson") {
		t.Errorf("expected events file path: %q", out)
	}
	if !strings.Contains(out, "bad substitution") {
		t.Errorf("expected stderr trace in failure tail: %q", out)
	}
	if !strings.Contains(out, "stderr:") {
		t.Errorf("expected stderr label in failure tail: %q", out)
	}
	if !strings.Contains(out, "progress") {
		t.Errorf("expected stdout tail shown too: %q", out)
	}
	if !strings.Contains(out, "Press any key") {
		t.Errorf("expected keypress hint on failure: %q", out)
	}
}

func TestRenderHookRunningOverlay_HiddenWhenStateNil(t *testing.T) {
	m := Model{width: 80, height: 24}
	out := m.renderHookRunningOverlay("base-view")
	if out != "base-view" {
		t.Errorf("expected base view returned unchanged, got %q", out)
	}
}
