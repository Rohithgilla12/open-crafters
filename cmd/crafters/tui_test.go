package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestTUIFlow drives the dashboard model through home → language picker →
// grading without a terminal, feeding the stage messages the grading goroutine
// would normally stream. It asserts the model never panics and tracks state.
// (We never execute the grading command, so the real grader/program.Send is
// not involved.)
func TestTUIFlow(t *testing.T) {
	m := newModel()
	if len(m.rows) != len(challenges) {
		t.Fatalf("expected %d rows, got %d", len(challenges), len(m.rows))
	}
	if m.View() == "" {
		t.Fatal("home view is empty")
	}

	step := func(msg tea.Msg) {
		next, _ := m.Update(msg)
		m = next.(*model)
		if m.View() == "" {
			t.Fatalf("empty view after %T", msg)
		}
	}

	step(tea.WindowSizeMsg{Width: 80, Height: 24})
	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // move cursor down
	step(tea.KeyMsg{Type: tea.KeyEnter})                     // select a challenge

	// With no solution dir in the test's cwd, selecting opens the language
	// picker.
	if m.scr != screenLang {
		t.Fatalf("expected language screen, got %v", m.scr)
	}

	// Choosing a language enters the grading screen. We ignore the returned
	// command (it would launch the real grader), then simulate its events.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(*model)
	if m.scr != screenGrading {
		t.Fatalf("expected grading screen, got %v", m.scr)
	}
	if len(m.state) != len(m.stages) {
		t.Fatalf("stage state not initialised: %d states for %d stages", len(m.state), len(m.stages))
	}

	first := m.stages[0].Slug
	step(stageStartMsg{slug: first})
	if m.state[0] != stateRunning {
		t.Fatalf("stage %q should be running", first)
	}
	step(stageEndMsg{slug: first, ok: true})
	if m.state[0] != statePassed {
		t.Fatalf("stage %q should be passed", first)
	}

	second := m.stages[1].Slug
	step(stageEndMsg{slug: second, ok: false, msg: "expected X, got Y"})
	if m.state[1] != stateFailed || !strings.Contains(m.View(), "expected X") {
		t.Fatal("failure message not surfaced")
	}

	step(gradeDoneMsg{err: nil, dir: "my-wal"})
	if !m.done {
		t.Fatal("grading should be marked done")
	}
}

func TestBarAndWrap(t *testing.T) {
	if got := bar(0, 9, 6); !strings.Contains(got, "░") {
		t.Fatalf("empty bar should be all dim, got %q", got)
	}
	if bar(3, 0, 6) != "" {
		t.Fatal("zero-total bar should be empty")
	}
	w := wrap("one two three four five six", 10)
	if !strings.Contains(w, "\n") {
		t.Fatal("expected wrapping to insert a newline")
	}
}
