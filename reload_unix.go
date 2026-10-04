//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// watchSelf는 실행 파일이 새로 빌드되면 reloadMsg를 보낸다. 개발 중 띄워둔 창이 바로 갱신되게 하려는 것.
// 같은 주기에 config.toml도 보고 바뀌면 configMsg를 보낸다(새 감시 루프를 두지 않으려고 여기서 같이 본다).
func watchSelf(p *tea.Program) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return
	}
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
		// 바뀐 뒤 한 번 더 같은 값이면(쓰기 끝남) 다시 시작
		if !cur.IsZero() && !cur.Equal(orig) && cur.Equal(prev) {
			p.Send(reloadMsg{})
			return
		}
		prev = cur
	}
}

// restart는 새 실행 파일로 프로세스를 바꿔치운다. 보던 날짜는 환경변수로 넘긴다.
func restart(cursor time.Time) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err == nil {
		os.Setenv("CAL_CURSOR", cursor.Format("2006-01-02"))
		err = syscall.Exec(exe, os.Args, os.Environ())
	}
	fmt.Println("다시 시작 실패:", err)
	os.Exit(1)
}
