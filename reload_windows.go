package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// 윈도우는 실행 중인 exe를 덮어쓸 수 없어 자동 갱신을 하지 않는다.
func watchSelf(*tea.Program) {}

func restart(time.Time) {}
