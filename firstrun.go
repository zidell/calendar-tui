package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 첫 실행(config.toml이 아직 없음) 때 일정 저장소를 고르는 화면. 고를 게 있는 맥에서만 띄운다
// (윈도우·리눅스는 구글뿐이라 바로 로그인). 고른 값은 config.toml [backend] type에 남는다(2026-10-26 사용자 지시).

var backendChoices = []struct{ kind, name, desc string }{
	{"eventkit", "맥에 등록한 계정 (EventKit)|Accounts on this Mac (EventKit)",
		"시스템 설정 → 인터넷 계정의 구글·iCloud 등을 그대로 쓴다|Uses Google, iCloud, etc. from System Settings → Internet Accounts"},
	{"google", "구글 계정에 직접 로그인|Sign in to Google directly",
		"브라우저로 로그인한다. 윈도우와 같은 방식|Sign in with your browser. Same as on Windows"},
}

func pick2(s string) string {
	ko, en, _ := strings.Cut(s, "|")
	return L(ko, en)
}

type chooser struct {
	sel    int
	chosen string
	w, h   int
}

func (c chooser) Init() tea.Cmd { return nil }

func (c chooser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.w, c.h = msg.Width, msg.Height
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			for i := range backendChoices { // 줄 위치는 View와 같은 계산(가운데 상자 안 3줄째부터 항목마다 3줄)
				if top := (c.h-c.boxHeight())/2 + 3 + i*3; msg.Y == top || msg.Y == top+1 {
					c.chosen = backendChoices[i].kind
					return c, tea.Quit
				}
			}
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "shift+tab":
			c.sel = (c.sel + len(backendChoices) - 1) % len(backendChoices)
		case "down", "j", "tab":
			c.sel = (c.sel + 1) % len(backendChoices)
		case "1", "2":
			c.chosen = backendChoices[msg.String()[0]-'1'].kind
			return c, tea.Quit
		case "enter":
			c.chosen = backendChoices[c.sel].kind
			return c, tea.Quit
		case "q", "esc", "ctrl+c":
			return c, tea.Quit
		}
	}
	return c, nil
}

func (c chooser) boxHeight() int { return 2 + 2 + len(backendChoices)*3 + 1 }

func (c chooser) View() string {
	var b strings.Builder
	b.WriteString(titleSt.Render(L("캘린더를 어디서 가져올까요?", "Where should calendars come from?")) + "\n\n")
	for i, ch := range backendChoices {
		name := pick2(ch.name)
		if i == c.sel {
			name = lipgloss.NewStyle().Background(focusBg).Inherit(selTextSt).Render(" " + name + " ")
		} else {
			name = " " + name + " "
		}
		b.WriteString(name + "\n " + faintSt.Render(pick2(ch.desc)) + "\n\n")
	}
	b.WriteString(helpSt.Render(L("↑↓ 고르기 · enter 확인 · q 끝내기 · 나중에 config.toml [backend]에서 바꿀 수 있다",
		"↑↓ move · enter choose · q quit · change later in config.toml [backend]")))
	return lipgloss.Place(c.w, c.h, lipgloss.Center, lipgloss.Center, modalSt.Render(b.String()))
}

// chooseBackend는 첫 실행 화면을 띄워 고른 저장소를 돌려준다. 그만두면 "".
func chooseBackend() string {
	final, err := tea.NewProgram(chooser{}, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	if err != nil {
		return ""
	}
	return final.(chooser).chosen
}
