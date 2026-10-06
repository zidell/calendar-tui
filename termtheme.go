package main

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

// 터미널이 알려 주는 다크·라이트 전환(모드 2031, tuidock 내장 터미널·Ghostty 등)과 실행 중 배경색 다시 묻기(OSC 11).
// Bubble Tea v1은 둘 다 해석하지 못해 입력 메시지를 여기서 직접 본다. 터미널 자체 모양 설정만 바꿔도(시스템은 그대로)
// 알림이 오므로, 알림을 주는 터미널에선 시스템 모양 감시(appearanceMsg)를 쓰지 않는다.
const (
	enable2031  = "\x1b[?2031h\x1b[?996n" // 알림 켜기 + 지금 모양 묻기(답이 오면 알림을 주는 터미널이다)
	disable2031 = "\x1b[?2031l"
	queryBg     = "\x1b]11;?\x07"
)

var (
	term2031 bool      // 터미널이 모드 2031로 다크·라이트를 알려 준다
	bgAsked  time.Time // 배경색을 다시 물은 때. 1초 안에 온 답만 받는다
)

// darkReport는 CSI ? 997 ; 1|2 n(다크|라이트)을 읽는다. Bubble Tea v1엔 비공개 타입(unknownCSISequenceMsg, []byte)으로 온다.
func darkReport(msg tea.Msg) (dark, ok bool) {
	v := reflect.ValueOf(msg)
	if v.Kind() != reflect.Slice || v.Type().Elem().Kind() != reflect.Uint8 {
		return false, false
	}
	switch string(v.Bytes()) {
	case "\x1b[?997;1n":
		return true, true
	case "\x1b[?997;2n":
		return false, true
	}
	return false, false
}

// colorScheme은 터미널이 알린 모양을 반영한다. 바뀌었으면 테마를 바로 다시 정하고(선 색은 배경을 알 때까지 고정값),
// 새 배경색을 물어 선 색을 맞춘다(bgReply).
func (m model) colorScheme(dark bool) (tea.Model, tea.Cmd) {
	logLine(fmt.Sprintf("color scheme dark=%v", dark))
	first := !term2031
	term2031 = true
	if first && dark == termDark { // 시작 때 물은 답(996). 시작 때 잰 배경 그대로
		return m, nil
	}
	termDark = dark
	applyTheme(m.db.cfg.Theme)
	bgAsked = time.Now()
	return m, func() tea.Msg {
		fmt.Print(queryBg)
		return nil
	}
}

// bgReply는 다시 물은 배경색 답(ESC ] 11 ; rgb:RRRR/GGGG/BBBB, BEL이나 ESC \로 끝남)을 먹는다. Bubble Tea v1은 이걸
// alt+] → 글자 "11;rgb:…" → alt+\(BEL이면 ctrl+g) 키로 쪼개 넘긴다. 물은 뒤 1초 안의 것만 본다.
func bgReply(k tea.KeyMsg) (eaten, got bool) {
	if time.Since(bgAsked) > time.Second {
		return false, false
	}
	s := string(k.Runes)
	switch {
	case k.String() == "alt+]" || k.String() == "alt+\\" || k.String() == "ctrl+g":
		return true, false
	case k.Type == tea.KeyRunes && strings.HasPrefix(s, "11;rgb:"):
		var hex strings.Builder
		for _, p := range strings.SplitN(s[len("11;rgb:"):], "/", 3) {
			if len(p) < 2 {
				return true, false
			}
			hex.WriteString(p[:2])
		}
		if hex.Len() != 6 {
			return true, false
		}
		termBg = "#" + hex.String()
		_, _, l := termenv.ConvertToRGB(termenv.RGBColor(termBg)).Hsl()
		termBgDark = l < 0.5
		termDark = termBgDark // 끝 문자(alt+\·ctrl+g)도 먹어야 해서 bgAsked는 그대로 둔다
		logLine("background " + termBg)
		return true, true
	}
	return false, false
}
