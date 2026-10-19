package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDockCmdAUsesQuickAdd(t *testing.T) {
	m := newModel(newMockBackend(), &settings{})
	fromDock, _ := m.update(dockKeyMsg("cmd+a"))
	fromKey, _ := m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	got, want := fromDock.(model), fromKey.(model)
	if got.top() != mQuick || got.top() != want.top() || got.quickBtn != want.quickBtn || got.quickIn.Value() != want.quickIn.Value() {
		t.Fatalf("Cmd+A state = modal %v, button %d, input %q; a state = modal %v, button %d, input %q",
			got.top(), got.quickBtn, got.quickIn.Value(), want.top(), want.quickBtn, want.quickIn.Value())
	}
}
