package main

import tea "github.com/charmbracelet/bubbletea"

// 모달 버튼 그룹 — 모든 모달에 같은 규칙(AGENTS.md "모달"):
//   Tab        버튼 그룹으로. 버튼에선 다음 버튼, 마지막 버튼에서 누르면 버튼으로 오기 전 자리로 돌아간다
//   Shift+Tab  버튼에선 이전 버튼, 첫 버튼에서 누르면 원래 자리로. 버튼 밖에선 모달이 정한 동작(폼에선 이전 칸)
//   ← →        버튼에 있으면 버튼 사이 이동
//   Enter      버튼에 있으면 그 버튼 실행
//   ↑          버튼에서 나와 원래 자리로
// 버튼만 있는 모달(only: 상세·삭제 확인·반복 범위)은 돌아갈 자리가 없어 버튼 사이를 돈다.
// 모달은 buttonGroup에 버튼 글자·지금 포커스·포커스 옮기기·실행만 정의한다. 키 처리는 handleButtons 한 곳.

type btnGroup struct {
	labels []string
	cur    int                                   // 포커스된 버튼, -1이면 버튼 밖(입력·목록)
	only   bool                                  // 버튼만 있는 모달(상세·삭제 확인·반복 범위): 늘 버튼에 있다
	focus  func(m model, i int) (model, tea.Cmd) // i번 버튼으로(-1이면 버튼으로 오기 전 자리로)
	run    func(m model, i int) (tea.Model, tea.Cmd)
}

// buttonGroup은 맨 위 모달의 버튼 그룹. 버튼이 없는 모달(일정 목록·검색·설정 메뉴)은 false.
func (m model) buttonGroup() (btnGroup, bool) {
	switch m.top() {
	case mDetail:
		if !m.detail.writable {
			return btnGroup{}, false
		}
		return btnGroup{labels: detailButtons(), cur: m.detailBtn, only: true,
			focus: func(m model, i int) (model, tea.Cmd) { m.detailBtn = i; return m, nil },
			run:   func(m model, i int) (tea.Model, tea.Cmd) { return m.detailAction(i) }}, true
	case mConfirm:
		return btnGroup{labels: m.confirmButtons(), cur: m.confirmBtn, only: true,
			focus: func(m model, i int) (model, tea.Cmd) { m.confirmBtn = i; return m, nil },
			run:   func(m model, i int) (tea.Model, tea.Cmd) { m.confirmBtn = i; return m.updateConfirm(enterKey) }}, true
	case mSpan:
		return btnGroup{labels: spanButtons(), cur: m.spanBtn, only: true,
			focus: func(m model, i int) (model, tea.Cmd) { m.spanBtn = i; return m, nil },
			run:   func(m model, i int) (tea.Model, tea.Cmd) { return m.commit(m.pending, span(i)) }}, true
	case mForm:
		cur := -1
		if m.form.focus == fSave {
			cur = 0
		}
		return btnGroup{labels: []string{saveLabel()}, cur: cur,
			focus: func(m model, i int) (model, tea.Cmd) {
				if i < 0 {
					return m, m.form.setFocus(m.form.from)
				}
				if m.form.focus != fSave {
					m.form.from = m.form.focus
				}
				return m, m.form.setFocus(fSave)
			},
			run: func(m model, i int) (tea.Model, tea.Cmd) { return m.saveForm() }}, true
	case mQuick:
		return btnGroup{labels: quickButtons(), cur: m.quickBtn,
			focus: func(m model, i int) (model, tea.Cmd) {
				m.quickBtn = i
				if i < 0 {
					return m, m.quickIn.Focus()
				}
				m.quickIn.Blur()
				return m, nil
			},
			run: func(m model, i int) (tea.Model, tea.Cmd) { return m.quickAction(i) }}, true
	case mGoto:
		return btnGroup{labels: []string{goLabel()}, cur: m.gotoBtn,
			focus: func(m model, i int) (model, tea.Cmd) {
				m.gotoBtn = i
				if i < 0 {
					return m, m.gotoIn.Focus()
				}
				m.gotoIn.Blur()
				return m, nil
			},
			run: func(m model, i int) (tea.Model, tea.Cmd) { return m.goMonth() }}, true
	case mMove:
		return btnGroup{labels: []string{goLabel()}, cur: m.moveBtn,
			focus: func(m model, i int) (model, tea.Cmd) {
				m.moveBtn = i
				if i < 0 {
					return m, m.moveIn.Focus()
				}
				m.moveIn.Blur()
				return m, nil
			},
			run: func(m model, i int) (tea.Model, tea.Cmd) { return m.moveEvent() }}, true
	case mCalendars:
		n := len(m.settingRows())
		cur := -1
		if m.setSel >= n {
			cur = 0
		}
		return btnGroup{labels: []string{saveLabel()}, cur: cur,
			focus: func(m model, i int) (model, tea.Cmd) {
				if i < 0 {
					m.setSel = min(m.setFrom, max(0, n-1))
					return m, nil
				}
				if m.setSel < n {
					m.setFrom = m.setSel
				}
				m.setSel = n
				return m, nil
			},
			run: func(m model, i int) (tea.Model, tea.Cmd) { return m.saveSettings(), nil }}, true
	case mDisplay:
		n := len(displayKeys)
		cur := -1
		if m.setSel >= n {
			cur = 0
		}
		return btnGroup{labels: []string{saveLabel()}, cur: cur,
			focus: func(m model, i int) (model, tea.Cmd) {
				if i < 0 {
					m.setSel = min(m.setFrom, n-1)
					return m, nil
				}
				if m.setSel < n {
					m.setFrom = m.setSel
				}
				m.setSel = n
				return m, nil
			},
			run: func(m model, i int) (tea.Model, tea.Cmd) {
				return m.saveDisplay(), tea.SetWindowTitle(windowTitle())
			}}, true
	}
	return btnGroup{}, false
}

// handleButtons는 버튼 그룹 키를 처리한다. 처리했으면 true(모달 자체 처리로 넘기지 않음).
func (m model) handleButtons(k tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	g, ok := m.buttonGroup()
	if !ok || len(g.labels) == 0 {
		return m, nil, false
	}
	n := len(g.labels)
	in := g.cur >= 0
	step := func(d int) (tea.Model, tea.Cmd, bool) {
		nm, cmd := g.focus(m, (g.cur+d+n)%n)
		return nm, cmd, true
	}
	back := func() (tea.Model, tea.Cmd, bool) {
		nm, cmd := g.focus(m, -1)
		return nm, cmd, true
	}
	switch k.String() {
	case "tab":
		switch {
		case !in:
			nm, cmd := g.focus(m, 0)
			return nm, cmd, true
		case g.cur == n-1 && !g.only: // 마지막 버튼 → 원래 자리
			return back()
		}
		return step(1)
	case "shift+tab":
		switch {
		case in && g.cur == 0 && !g.only: // 첫 버튼 → 원래 자리
			return back()
		case in:
			return step(-1)
		}
	case "left", "right":
		if in {
			if k.String() == "left" {
				return step(-1)
			}
			return step(1)
		}
	case "enter":
		if in {
			nm, cmd := g.run(m, g.cur)
			return nm, cmd, true
		}
	case "up":
		if in && !g.only {
			return back()
		}
	}
	return m, nil, false
}
