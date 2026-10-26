package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// 윈도우 개발용 자동 재시작. 실행 중인 exe는 덮어쓸 수 없지만 이름은 바꿀 수 있어, scripts/dev.ps1이 옛 파일을
// 다른 이름으로 옮기고 새 빌드를 같은 자리에 둔다. 시작 때 잡아 둔 경로의 mtime을 0.3초마다 보다가 바뀌면 다시 시작한다.
// 같은 주기에 config.toml도 본다(맥·리눅스 reload_unix.go와 같음). 시스템 모양 감시는 윈도우에 없다.

var selfPath string // 시작 때의 실행 파일 경로(이름이 바뀐 뒤 os.Executable은 옛 이름을 줄 수 있다)

func watchSelf(p *tea.Program) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return
	}
	selfPath = exe
	mtime := func() time.Time {
		if st, err := os.Stat(exe); err == nil && st.Size() > 0 {
			return st.ModTime()
		}
		return time.Time{}
	}
	orig, prev := mtime(), time.Time{}
	cfgMod := configModTime()
	for range time.Tick(300 * time.Millisecond) {
		if t := configModTime(); !t.Equal(cfgMod) {
			cfgMod = t
			p.Send(configMsg{})
		}
		cur := mtime()
		// 바뀐 뒤 한 번 더 같은 값이면(복사 끝남) 다시 시작
		if !cur.IsZero() && !cur.Equal(orig) && cur.Equal(prev) {
			p.Send(reloadMsg{})
			return
		}
		prev = cur
	}
}

// restart는 윈도우에 exec가 없어 새 실행 파일을 같은 콘솔(같은 터미널 창, tuidock 창 포함)에서 자식으로 띄우고
// 끝날 때까지 기다린다. 창을 띄운 프로세스가 살아 있어야 창이 닫히지 않기 때문이다. 다시 시작할 때마다
// 기다리는 옛 프로세스가 하나씩 남는다(개발 중에만 생김, 창을 닫으면 모두 끝남).
func restart(cursor time.Time) {
	if selfPath == "" {
		return
	}
	signal.Ignore(os.Interrupt) // 콘솔 Ctrl+C 신호로 기다리던 쪽이 먼저 죽지 않게(새 프로세스가 처리)
	c := exec.Command(selfPath, os.Args[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Env = append(os.Environ(), "CAL_CURSOR="+cursor.Format("2006-01-02"))
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		fmt.Println("다시 시작 실패:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
