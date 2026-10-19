package main

import (
	"fmt"
	"testing"
)

func TestStartupAsksForMissingBackground(t *testing.T) {
	old2031, oldDark, oldBg, oldBgDark, oldAsked := term2031, termDark, termBg, termBgDark, bgAsked
	t.Cleanup(func() {
		term2031, termDark, termBg, termBgDark, bgAsked = old2031, oldDark, oldBg, oldBgDark, oldAsked
	})
	term2031, termDark, termBg, termBgDark = false, true, "", false
	m := newModel(newMockBackend(), &settings{})
	_, cmd := m.colorScheme(true)
	if cmd == nil {
		t.Fatal("startup skipped OSC 11 despite missing background color")
	}
}

func TestTuidockLineVisibleWithoutBackgroundReply(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "tuidock")
	t.Setenv("COLORTERM", "truecolor")
	oldBg, oldBgDark := termBg, termBgDark
	t.Cleanup(func() { termBg, termBgDark = oldBg, oldBgDark })
	termBg, termBgDark = "", false
	if got := fmt.Sprint(lineColor(true)); got != "#4a4a4a" {
		t.Fatalf("dark tuidock line color = %s, want #4a4a4a", got)
	}
}
