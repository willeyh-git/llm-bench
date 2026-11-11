package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/willyhetland/llm-bencher/internal/bench"
)

func TestCompletedRunShowsSummaryUntilEnter(t *testing.T) {
	result := &bench.Result{Model: "test-model", Path: "results/test.json"}
	m := Model{
		screen: screenRun,
		run:    run{state: &runState{running: true}},
	}

	updated, _ := m.Update(runEventMsg{ev: bench.Event{Kind: bench.EventDone, Data: result}})
	m = updated.(Model)
	if m.screen != screenSummary {
		t.Fatalf("screen after successful run = %v, want summary", m.screen)
	}
	if got := m.View(); !strings.Contains(got, "Summary: test-model") {
		t.Fatalf("summary view does not show the completed result: %s", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if updated.(Model).screen != screenSummary {
		t.Fatal("summary should remain visible until the user returns to the picker")
	}
}
