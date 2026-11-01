package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCtrlAOpensAdd(t *testing.T) {
	for _, tc := range []struct {
		name string
		top  modalKind
		want modalKind
	}{{"calendar", mNone, mQuick}, {"day list", mDay, mForm}} {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(newMockBackend(), &settings{})
			if tc.top != mNone {
				m = m.push(tc.top)
			}
			next, _ := m.update(tea.KeyMsg{Type: tea.KeyCtrlA})
			if got := next.(model).top(); got != tc.want {
				t.Fatalf("Ctrl+A opened modal %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCtrlAInQuickInputMovesCursorToStart(t *testing.T) {
	m := newModel(newMockBackend(), &settings{})
	next, _ := m.openQuick()
	m = next.(model)
	m.quickIn.SetValue("한글 일정")
	m.quickIn.CursorEnd()
	next, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlA})
	got := next.(model)
	if got.top() != mQuick || got.quickIn.Value() != "한글 일정" || got.quickIn.Position() != 0 {
		t.Fatalf("Ctrl+A changed input behavior: modal %v, text %q, cursor %d", got.top(), got.quickIn.Value(), got.quickIn.Position())
	}
}
