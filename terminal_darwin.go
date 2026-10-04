package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// 이 프로세스가 도는 Terminal 탭(tty로 찾음)에 AppleScript로 묻거나 시킨다.
// Terminal 안에서 Terminal에 보내는 것이라 자동화 권한 창이 뜨지 않는다(2026-10-04 확인).

var myTTY = sync.OnceValue(func() string {
	cmd := exec.Command("/usr/bin/tty")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	tty := strings.TrimSpace(string(out))
	if err != nil || !strings.HasPrefix(tty, "/dev/") {
		return ""
	}
	return tty
})

// tellMyTab은 내 탭을 t, 그 창을 w로 두고 body를 실행한 결과를 돌려준다.
func tellMyTab(body string) (string, bool) {
	tty := myTTY()
	if tty == "" {
		return "", false
	}
	src := `tell application "Terminal"
	repeat with w in windows
		repeat with t in tabs of w
			if tty of t is "` + tty + `" then
				` + body + `
			end if
		end repeat
	end repeat
end tell`
	out, err := exec.Command("/usr/bin/osascript", "-e", src).Output()
	return strings.TrimSpace(string(out)), err == nil
}

// termFontSize는 내 탭의 글꼴 크기. 모르면 0.
func termFontSize() float64 {
	out, ok := tellMyTab("return font size of t")
	if !ok {
		return 0
	}
	fs, _ := strconv.ParseFloat(strings.ReplaceAll(out, ",", "."), 64)
	return fs
}

// hideWindow는 내 창만 숨긴다(Cmd+H는 Terminal 창을 모두 숨겨서). Dock의 Calendar TUI 클릭·Cmd+Tab으로
// 실행기(launcher.swift)가 창을 맨 앞으로 가져오면 다시 보인다(index 설정이 visible도 켬, 2026-10-04 확인).
func hideWindow() {
	tellMyTab("set visible of w to false\n\t\t\t\treturn")
}
