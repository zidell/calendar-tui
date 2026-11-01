package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// shortcutKey connects Ctrl+letter to the existing shortcut in command screens.
// Text inputs keep their editing keys, and settings keep Ctrl+S for saving.
// Ctrl+M is Enter in the terminal input stream, so it cannot alias m.
func (m model) shortcutKey(k tea.KeyMsg) tea.KeyMsg {
	s := k.String()
	if len(s) != len("ctrl+a") || !strings.HasPrefix(s, "ctrl+") {
		return k
	}
	var letters string
	switch m.top() {
	case mNone:
		letters = "ahjklpnvtrgsq"
	case mDay:
		letters = "kjan"
	case mDetail:
		letters = "hledo"
	case mConfirm:
		letters = "hly"
	case mSpan:
		letters = "hl"
	case mSettings:
		letters = "kjl"
	case mCalendars:
		letters = "kj"
	case mDisplay:
		letters = "kjhl"
	}
	if strings.ContainsRune(letters, rune(s[5])) {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(s[5])}}
	}
	return k
}
