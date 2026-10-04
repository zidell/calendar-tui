package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// termFontSize는 이 프로세스가 도는 Terminal 탭(tty로 찾음)의 글꼴 크기. 모르면 0.
// Terminal 안에서 Terminal에 묻는 것이라 자동화 권한 창이 뜨지 않는다(2026-10-04 확인).
func termFontSize() float64 {
	cmd := exec.Command("/usr/bin/tty")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	tty := strings.TrimSpace(string(out))
	if err != nil || !strings.HasPrefix(tty, "/dev/") {
		return 0
	}
	src := `tell application "Terminal"
	repeat with w in windows
		repeat with t in tabs of w
			if tty of t is "` + tty + `" then return font size of t
		end repeat
	end repeat
end tell`
	out, err = exec.Command("/usr/bin/osascript", "-e", src).Output()
	if err != nil {
		return 0
	}
	fs, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(string(out)), ",", "."), 64)
	return fs
}
