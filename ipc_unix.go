//go:build !windows

package main

import (
	"net"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// 앱 실행기(launcher.swift)와 주고받는 로컬 소켓(유닉스 데이터그램, 설정 폴더).
//   calendar.sock  실행기 → 캘린더: "key a"   (캘린더 창이 포커스일 때 Cmd+글자를 실행기가 받아 넘김)
//   launcher.sock  캘린더 → 실행기: "focus 1" / "focus 0" (그동안만 실행기가 Cmd+글자를 등록)
// Terminal은 Cmd 조합을 앱에 넘기지 않고, 입력기는 Cmd 조합을 조합하지 않으므로 한글 상태에서도 단축키가 먹는다.

func sockPath(name string) string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, name)
	}
	return ""
}

// listenCmdKeys는 실행기가 넘기는 키를 받아 프로그램에 보낸다.
func listenCmdKeys(p *tea.Program) {
	path := sockPath("calendar.sock")
	if path == "" {
		return
	}
	os.Remove(path)
	c, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		return
	}
	go func() {
		buf := make([]byte, 64)
		for {
			n, _, err := c.ReadFromUnix(buf)
			if err != nil {
				return
			}
			if s := string(buf[:n]); len(s) > 4 && s[:4] == "key " {
				p.Send(cmdKeyMsg{s[4:]})
			}
		}
	}()
}

// tellLauncher는 실행기에 한 줄 보낸다. 실행기가 없으면(직접 실행) 조용히 넘어간다.
func tellLauncher(msg string) {
	path := sockPath("launcher.sock")
	if path == "" {
		return
	}
	c, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		return
	}
	c.Write([]byte(msg))
	c.Close()
}
