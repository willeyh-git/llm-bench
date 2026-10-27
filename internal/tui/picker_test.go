package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/willyhetland/llm-bencher/internal/server"
)

var testModels = []server.ModelInfo{
	{ID: "Devstral Small 2 24B", Status: "unloaded"},
	{ID: "Flint 40B A2.5B", Status: "loaded"},
	{ID: "Gemma-4-12B", Status: "unloaded"},
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func loadedPicker(t *testing.T) picker {
	t.Helper()
	p := newPicker("http://example.test/v1")
	p, _ = p.Update(modelsLoadedMsg{models: testModels})
	return p
}

func press(t *testing.T, p picker, k string) (picker, tea.Cmd) {
	t.Helper()
	return p.Update(key(k))
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestPickerMovesWithJK(t *testing.T) {
	p := loadedPicker(t)
	p, _ = press(t, p, "j")
	p, _ = press(t, p, "j")
	if p.cursor != 2 {
		t.Fatalf("after j j cursor = %d, want 2", p.cursor)
	}
	p, _ = press(t, p, "j")
	if p.cursor != 2 {
		t.Fatalf("cursor moved past last item: %d", p.cursor)
	}
	p, _ = press(t, p, "k")
	if p.cursor != 1 {
		t.Fatalf("after k cursor = %d, want 1", p.cursor)
	}
}

func TestPickerMovesWithArrows(t *testing.T) {
	p := loadedPicker(t)
	p, _ = press(t, p, "down")
	if p.cursor != 1 {
		t.Fatalf("after down cursor = %d, want 1", p.cursor)
	}
	p, _ = press(t, p, "up")
	p, _ = press(t, p, "up")
	if p.cursor != 0 {
		t.Fatalf("cursor moved above first item: %d", p.cursor)
	}
}

func TestPickerEnterReturnsChosenModel(t *testing.T) {
	p := loadedPicker(t)
	p, _ = press(t, p, "j")
	p, cmd := press(t, p, "enter")
	if p.chosen != testModels[1].ID {
		t.Fatalf("chosen = %q, want %q", p.chosen, testModels[1].ID)
	}
	if cmd != nil {
		t.Fatal("enter must not quit the program; the app switches to the run screen")
	}
}

func TestPickerEscAndQQuitWithoutSelection(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		p := loadedPicker(t)
		p, cmd := press(t, p, k)
		if !isQuit(cmd) {
			t.Errorf("%s should quit", k)
		}
		if p.chosen != "" {
			t.Errorf("%s should not select a model, got %q", k, p.chosen)
		}
	}
}

func TestPickerErrorShowsRetryAndQuit(t *testing.T) {
	p := newPicker("http://example.test/v1")
	p, _ = p.Update(modelsFailedMsg{err: errors.New("connection refused")})
	if p.err == nil || p.loading {
		t.Fatalf("expected error state, got loading=%v err=%v", p.loading, p.err)
	}
	p, _ = press(t, p, "enter")
	if p.chosen != "" {
		t.Fatal("enter must not select in error state")
	}
	p, cmd := press(t, p, "r")
	if !p.loading || p.err != nil {
		t.Fatalf("retry should start loading, got loading=%v err=%v", p.loading, p.err)
	}
	if cmd == nil {
		t.Fatal("retry should return a fetch command")
	}
	p, _ = p.Update(modelsFailedMsg{err: errors.New("still down")})
	_, cmd = press(t, p, "q")
	if !isQuit(cmd) {
		t.Fatal("q should quit in error state")
	}
}

func TestPickerViewListsModelsWithStatus(t *testing.T) {
	p := loadedPicker(t)
	view := p.View()
	for _, m := range testModels {
		if !strings.Contains(view, m.ID) {
			t.Errorf("view missing model %q", m.ID)
		}
	}
	if !strings.Contains(view, "loaded") {
		t.Error("view missing status column")
	}
}
