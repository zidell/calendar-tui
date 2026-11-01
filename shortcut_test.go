package main

import (
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCalendarCtrlShortcutsMatchLetters(t *testing.T) {
	for _, letter := range "ahjklpnvtrgsq" {
		t.Run(string(letter), func(t *testing.T) {
			m := newModel(newMockBackend(), &settings{readOnly: true})
			m.cursor = m.today.AddDate(0, 0, 3)
			gotModel, gotCmd := m.update(tea.KeyMsg{Type: tea.KeyType(letter - 'a' + 1)})
			wantModel, wantCmd := m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{letter}})
			got, want := gotModel.(model), wantModel.(model)
			if !got.cursor.Equal(want.cursor) || got.view != want.view || !reflect.DeepEqual(got.stack, want.stack) {
				t.Fatalf("Ctrl+%c: cursor %v, view %d, stack %v; letter: %v, %d, %v",
					letter, got.cursor, got.view, got.stack, want.cursor, want.view, want.stack)
			}
			if letter == 'q' {
				if gotCmd == nil || wantCmd == nil {
					t.Fatal("quit shortcut did not return a command")
				}
				if _, ok := gotCmd().(tea.QuitMsg); !ok {
					t.Fatal("Ctrl+Q did not quit")
				}
			}
		})
	}
}

func TestCtrlShortcutsInModals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		top   modalKind
		key   tea.KeyType
		check func(model) bool
	}{
		{"edit", mDetail, tea.KeyCtrlE, func(m model) bool { return m.top() == mForm }},
		{"delete confirmation", mDetail, tea.KeyCtrlD, func(m model) bool { return m.top() == mConfirm }},
		{"previous detail button", mDetail, tea.KeyCtrlH, func(m model) bool { return m.detailBtn == 3 }},
		{"next detail button", mDetail, tea.KeyCtrlL, func(m model) bool { return m.detailBtn == 1 }},
		{"settings down", mSettings, tea.KeyCtrlJ, func(m model) bool { return m.menuSel == 1 }},
		{"settings open", mSettings, tea.KeyCtrlL, func(m model) bool { return m.top() == mCalendars }},
		{"calendar selection down", mCalendars, tea.KeyCtrlJ, func(m model) bool { return m.setSel == 1 }},
		{"display down", mDisplay, tea.KeyCtrlJ, func(m model) bool { return m.setSel == 1 }},
		{"repeat range", mSpan, tea.KeyCtrlL, func(m model) bool { return m.spanBtn == 1 }},
		{"delete range", mConfirm, tea.KeyCtrlL, func(m model) bool { return m.confirmBtn == 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(newMockBackend(), &settings{readOnly: true})
			m.detail = event{title: "테스트", start: m.today, end: m.today.Add(time.Hour), calID: mockCal, writable: true, repeat: repWeekly}
			m = m.push(tc.top)
			next, _ := m.update(tea.KeyMsg{Type: tc.key})
			if !tc.check(next.(model)) {
				t.Fatalf("Ctrl shortcut did not perform %s", tc.name)
			}
		})
	}
}

func TestCtrlSStillSavesFormAndSettings(t *testing.T) {
	for _, top := range []modalKind{mForm, mCalendars, mDisplay} {
		t.Run(string(rune('0'+top)), func(t *testing.T) {
			b := newMockBackend()
			m := newModel(b, &settings{readOnly: true})
			var next tea.Model
			switch top {
			case mForm:
				next, _ = m.openNewForm()
				m = next.(model)
				m.form.inputs[fTitle].SetValue("저장 확인")
			case mCalendars:
				m = m.openCalendars()
			case mDisplay:
				m = m.openDisplay()
			}
			next, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlS})
			if next.(model).top() != mNone {
				t.Fatal("Ctrl+S did not save and close")
			}
			if top == mForm {
				found := false
				for _, e := range b.masters {
					found = found || e.title == "저장 확인"
				}
				if !found {
					t.Fatal("Ctrl+S did not save event")
				}
			}
		})
	}
}

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

func TestSearchCtrlNavigationKeepsQuery(t *testing.T) {
	m := newModel(newMockBackend(), &settings{readOnly: true})
	m = m.push(mSearch)
	m.searchIn.SetValue("회의")
	m.searchAll = []event{
		{title: "회의 1", start: m.today, end: m.today},
		{title: "회의 2", start: m.today.AddDate(0, 0, 1), end: m.today.AddDate(0, 0, 1)},
	}
	for _, tc := range []struct {
		key  tea.KeyType
		want int
	}{{tea.KeyCtrlN, 1}, {tea.KeyCtrlP, 0}} {
		next, _ := m.update(tea.KeyMsg{Type: tc.key})
		m = next.(model)
		if m.searchSel != tc.want || m.searchIn.Value() != "회의" {
			t.Fatalf("search selection = %d, query = %q", m.searchSel, m.searchIn.Value())
		}
	}
}
