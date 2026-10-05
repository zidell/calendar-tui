package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// 키 입력 디버그 로그. ~/Library/Logs/calendar-tui/keys.log(맥 기준)에 쌓고 24시간 지난 줄은 켤 때 지운다.
// 입력한 글자(일정 제목 등)까지 남으므로 기본은 꺼 둔다. config.toml [debug] key_log = true로 켠다.
var keyLog *os.File

const keyLogTime = "2006-01-02T15:04:05.000"

// setKeyLog는 키 로그를 켜거나 끈다(설정이 바뀌면 다시 부른다).
func setKeyLog(on bool) {
	switch {
	case on && keyLog == nil:
		openKeyLog()
	case !on && keyLog != nil:
		keyLog.Close()
		keyLog = nil
	}
}

func openKeyLog() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, "Library", "Logs", "calendar-tui")
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	path := filepath.Join(dir, "keys.log")
	pruneKeyLog(path, time.Now().Add(-24*time.Hour))
	keyLog, _ = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// pruneKeyLog는 cutoff 이전 줄을 버린다.
func pruneKeyLog(path string, cutoff time.Time) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	var keep []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		ts, _, _ := strings.Cut(l, " ")
		if t, err := time.ParseInLocation(keyLogTime, ts, time.Local); err == nil && t.After(cutoff) {
			keep = append(keep, l)
		}
	}
	f.Close()
	out := strings.Join(keep, "\n")
	if out != "" {
		out += "\n"
	}
	os.WriteFile(path, []byte(out), 0o644)
}

// logLine은 키가 아닌 진단 기록(예: EventKit 변경 알림 수신).
func logLine(s string) {
	if keyLog != nil {
		fmt.Fprintf(keyLog, "%s %s\n", time.Now().Format(keyLogTime), s)
	}
}

func logKey(k tea.KeyMsg, top modalKind) {
	if keyLog == nil {
		return
	}
	runes := make([]string, len(k.Runes))
	for i, r := range k.Runes {
		runes[i] = fmt.Sprintf("%U", r)
	}
	fmt.Fprintf(keyLog, "%s str=%q type=%d runes=[%s] alt=%v paste=%v modal=%d\n",
		time.Now().Format(keyLogTime), k.String(), k.Type, strings.Join(runes, " "), k.Alt, k.Paste, top)
}
