package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/core"
	"github.com/langtind/gren/internal/events"
)

// HookRunningState holds state for the live hook-execution modal.
// Shown while a non-interactive hook runs; phases update in place as events
// arrive. On failure the modal persists until a keypress so the user can read
// which phase failed and find the events file for post-mortem.
type HookRunningState struct {
	visible  bool
	hookType config.HookType
	// stream carries both hookPhaseEventMsg and hookExecutionDoneMsg; closed
	// when the background goroutine returns.
	stream  <-chan tea.Msg
	events  []events.Event
	done    bool
	results []core.HookResult
	hookErr error
	started time.Time
}

// hookPhaseEventMsg is dispatched each time the observer sees a new event.
type hookPhaseEventMsg struct {
	ev events.Event
}

// hookExecutionDoneMsg is dispatched once after the hook goroutine finishes.
type hookExecutionDoneMsg struct {
	results []core.HookResult
}

// hookRunningDismissMsg dismisses the modal (sent on timer after a clean run
// or synthesized on keypress after a failed run).
type hookRunningDismissMsg struct{}

// waitForHookStream returns a tea.Cmd that blocks on ch and dispatches the
// next message into Update. Returns nil message when the channel closes —
// Bubble Tea treats nil as no-op, so the tail of a closed stream is silent.
// Callers must reissue this after each event message if they want more.
func waitForHookStream(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// startHookRun spawns a goroutine that runs the hook via the appropriate
// RunXxxHookWithApproval method and streams its events + completion to the
// returned channel. The initial waitForHookStream tea.Cmd is returned so the
// caller can thread it into Bubble Tea's command loop.
func startHookRun(wm *core.WorktreeManager, hookType config.HookType, ctx core.HookContext, autoYes bool) (<-chan tea.Msg, tea.Cmd) {
	// Buffer sized to absorb event bursts between Bubble Tea waits.
	ch := make(chan tea.Msg, 64)
	go func() {
		defer close(ch)
		wm.SetEventObserver(func(e events.Event) {
			ch <- hookPhaseEventMsg{ev: e}
		})
		defer wm.SetEventObserver(nil)

		var results []core.HookResult
		switch hookType {
		case config.HookPostCreate:
			results = wm.RunPostCreateHookWithApproval(ctx.WorktreePath, ctx.BranchName, ctx.BaseBranch, autoYes)
		case config.HookPreRemove:
			results = wm.RunPreRemoveHookWithApproval(ctx.WorktreePath, ctx.BranchName, autoYes)
		case config.HookPostRemove:
			results = wm.RunPostRemoveHookWithApproval(ctx.WorktreePath, ctx.BranchName, autoYes)
		case config.HookPreMerge:
			results = wm.RunPreMergeHookWithApproval(ctx.WorktreePath, ctx.BranchName, ctx.TargetBranch, autoYes)
		case config.HookPostMerge:
			results = wm.RunPostMergeHookWithApproval(ctx.WorktreePath, ctx.BranchName, ctx.TargetBranch, autoYes)
		case config.HookPostSwitch:
			results = wm.RunPostSwitchHookWithApproval(ctx.WorktreePath, ctx.BranchName, autoYes)
		case config.HookPostStart:
			results = wm.RunPostStartHookWithApproval(ctx.WorktreePath, ctx.BranchName, ctx.ExecuteCmd, autoYes)
		}
		ch <- hookExecutionDoneMsg{results: results}
	}()
	return ch, waitForHookStream(ch)
}

// phaseRow is a collapsed view of all events for a single (phase, app) pair.
type phaseRow struct {
	phase  string
	app    string
	status events.Status
	detail string
}

// collapsePhases reduces a flat event list to one row per (phase, app),
// keeping insertion order and the most recent status + detail.
func collapsePhases(evs []events.Event) []phaseRow {
	type key struct{ phase, app string }
	order := []key{}
	rows := map[key]*phaseRow{}
	for _, e := range evs {
		k := key{e.Phase, e.App}
		r, ok := rows[k]
		if !ok {
			r = &phaseRow{phase: e.Phase, app: e.App}
			rows[k] = r
			order = append(order, k)
		}
		r.status = e.Status
		if e.Detail != "" {
			r.detail = e.Detail
		}
	}
	out := make([]phaseRow, 0, len(order))
	for _, k := range order {
		out = append(out, *rows[k])
	}
	return out
}

func glyphForStatus(s events.Status) string {
	switch s {
	case events.StatusStart:
		return "…"
	case events.StatusOK:
		return "✓"
	case events.StatusError:
		return "✗"
	case events.StatusInterrupted:
		return "⊘"
	}
	return "?"
}

func firstFailedErr(results []core.HookResult) error {
	if f := core.FirstFailedHook(results); f != nil {
		return f.Err
	}
	return nil
}

func eventsFileFromResults(results []core.HookResult) string {
	for _, r := range results {
		if r.EventsFile != "" {
			return r.EventsFile
		}
	}
	return ""
}

// tailOutputLines returns at most n of the final lines of s, joined with \n.
// Used to surface the last bit of a failed hook's stderr+stdout without
// flooding the modal.
func tailOutputLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// lastFailedOutput returns a labeled tail of stderr and/or stdout for the
// first failed hook. stderr comes first because that's where runtime error
// traces (e.g. bash `bad substitution`) land — the original combined-output
// log buried them behind normal progress lines. Returns empty if no hook
// failed.
func lastFailedOutput(results []core.HookResult, tailN int) string {
	f := core.FirstFailedHook(results)
	if f == nil {
		return ""
	}
	var parts []string
	if tail := tailOutputLines(f.Stderr, tailN); tail != "" {
		parts = append(parts, "stderr:\n"+tail)
	}
	if tail := tailOutputLines(f.Output, tailN); tail != "" {
		parts = append(parts, "stdout:\n"+tail)
	}
	return strings.Join(parts, "\n")
}

// handleHookRunningKeys dismisses the modal on any key once the hook is done.
// While the hook is still running, keys are swallowed (no escape hatch — the
// hook has side effects and abandoning mid-run would leave partial state).
func (m Model) handleHookRunningKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.hookRunningState == nil || !m.hookRunningState.visible {
		return m, nil
	}
	if !m.hookRunningState.done {
		// Silently ignore; modal cannot be cancelled mid-run (hooks have
		// side effects and abandoning mid-run leaves partial state).
		return m, nil
	}
	m.hookRunningState = nil
	return m, nil
}

// renderHookRunningOverlay draws the live hook modal over baseView.
func (m Model) renderHookRunningOverlay(baseView string) string {
	if m.hookRunningState == nil || !m.hookRunningState.visible {
		return baseView
	}
	st := m.hookRunningState

	// Title + border color shift based on outcome.
	borderColor := ColorPrimary
	titleColor := ColorPrimary
	title := fmt.Sprintf("Running %s hook…", st.hookType)
	switch {
	case st.done && st.hookErr != nil:
		borderColor = ColorError
		titleColor = ColorError
		title = fmt.Sprintf("%s hook failed", st.hookType)
	case st.done:
		borderColor = ColorSuccess
		titleColor = ColorSuccess
		title = fmt.Sprintf("%s hook completed", st.hookType)
	}

	var content strings.Builder
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(titleColor)
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	phases := collapsePhases(st.events)
	if len(phases) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
		content.WriteString(mutedStyle.Render("  (waiting for first phase…)"))
		content.WriteString("\n")
	} else {
		detailStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
		for _, p := range phases {
			content.WriteString("  ")
			content.WriteString(glyphForStatus(p.status))
			content.WriteString(" ")
			if p.app != "" {
				content.WriteString(p.app + " / ")
			}
			content.WriteString(p.phase)
			if p.detail != "" {
				content.WriteString("  — ")
				content.WriteString(detailStyle.Render(p.detail))
			}
			content.WriteString("\n")
		}
	}

	if st.done && st.hookErr != nil {
		errStyle := lipgloss.NewStyle().Foreground(ColorError)
		mutedStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
		content.WriteString("\n")
		content.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", st.hookErr)))
		content.WriteString("\n")
		if tail := lastFailedOutput(st.results, 5); tail != "" {
			content.WriteString("\n")
			content.WriteString(mutedStyle.Render("Last output:"))
			content.WriteString("\n")
			// Indent each tail line so it's visually inside the modal.
			for _, line := range strings.Split(tail, "\n") {
				content.WriteString("  ")
				content.WriteString(mutedStyle.Render(line))
				content.WriteString("\n")
			}
		}
		if file := eventsFileFromResults(st.results); file != "" {
			content.WriteString("\n")
			content.WriteString(mutedStyle.Render("Event log: " + file))
			content.WriteString("\n")
		}
	}

	if st.done {
		hint := lipgloss.NewStyle().Foreground(ColorTextMuted).Italic(true)
		content.WriteString("\n")
		content.WriteString(hint.Render("Press any key to continue"))
	}

	modalWidth := 70
	if m.width > 0 && m.width-10 < modalWidth {
		modalWidth = m.width - 10
		if modalWidth < 40 {
			modalWidth = 40
		}
	}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(modalWidth)

	return m.centerOverlay(baseView, modalStyle.Render(content.String()))
}
