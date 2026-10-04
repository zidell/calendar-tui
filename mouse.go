package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// 마우스는 왼쪽 클릭만 쓴다. 클릭은 그 항목을 고르고 enter를 친 것과 같다.
// 좌표는 View와 같은 계산(layout·overlay·box)으로 되짚는다.

var enterKey = tea.KeyMsg{Type: tea.KeyEnter}

func (m model) click(x, y int) (tea.Model, tea.Cmd) {
	if m.top() == mNone {
		return m.clickCal(x, y)
	}
	// 맨 위 모달의 자리(overlay와 같은 가운데 정렬)
	h := m.height - 1
	s := m.modal(m.top())
	lines := strings.Split(s, "\n")
	fw := lipgloss.Width(s)
	x0, y0 := max(0, (m.width-fw)/2), max(0, (h-len(lines))/2)
	if x < x0 || x >= x0+fw || y < y0 || y >= y0+len(lines) {
		return m.pop(), nil // 바깥 클릭 = esc(취소)
	}
	ln := ansi.Strip(lines[y-y0])
	bx := x - x0    // 모달 안 가로 위치(테두리 포함)
	r := y - y0 - 3 // 본문 줄(위 테두리·제목·빈 줄 다음)
	c := x - x0 - 2 // 본문 안 가로 위치(테두리·여백 다음)
	switch m.top() {
	case mDay:
		evs := m.db.on(m.cursor)
		n := 1 + len(evs)
		if len(evs) == 0 {
			n = 3 // "+ 새 일정 추가", 빈 줄, "일정이 없습니다"
		}
		start, shown := scroll(n, min(m.daySel, len(evs)), m.height-8)
		if i := start + r; r >= 0 && r < shown && i <= len(evs) {
			m.daySel = i
			return m.updateDay(enterKey)
		}
	case mDetail:
		if u := m.detail.link(); u != "" && strings.Contains(ln, rw.Truncate(u, 20, "")) {
			openURL(u)
			return m, nil
		}
		if b := buttonAt(ln, bx, detailButtons()...); b >= 0 {
			m.detailBtn = b
			return m.updateDetail(enterKey)
		}
	case mConfirm:
		if b := buttonAt(ln, bx, m.confirmButtons()...); b >= 0 {
			m.confirmBtn = b
			return m.updateConfirm(enterKey)
		}
	case mSpan:
		if b := buttonAt(ln, bx, spanButtons()...); b >= 0 {
			m.spanBtn = b
			return m.updateSpan(enterKey)
		}
	case mSearch:
		hits, start, shown := m.searchRows()
		if i := r - 2; i >= 0 && i < shown { // 입력 줄·빈 줄 다음이 결과
			return m.openHit(hits[start+i])
		}
	case mQuick:
		if buttonAt(ln, bx, L("추가", "Add")) >= 0 {
			return m.updateQuick(enterKey)
		}
	case mMove:
		if buttonAt(ln, bx, goLabel()) >= 0 {
			return m.updateMove(enterKey)
		}
	case mGoto:
		if buttonAt(ln, bx, goLabel()) >= 0 {
			return m.updateGoto(enterKey)
		}
	case mSettings:
		if r >= 0 && r < len(settingMenu()) {
			m.menuSel = r
			return m.updateSettingMenu(enterKey)
		}
	case mCalendars:
		rows := m.settingRows()
		if buttonAt(ln, bx, saveLabel()) >= 0 {
			return m.saveSettings(), nil
		}
		start, shown := scroll(len(rows), min(m.setSel, len(rows)), m.height-10)
		if r >= 0 && r < shown {
			m.setSel = start + r
			return m.updateCalendars(enterKey)
		}
	case mDisplay:
		if buttonAt(ln, bx, saveLabel()) >= 0 {
			return m.saveDisplay(), tea.SetWindowTitle(windowTitle())
		}
		if r >= 0 && r < len(displayKeys) {
			m.setSel = r
			return m.cycleDisplay(r, 1), nil
		}
	case mForm:
		return m.clickForm(r, c, buttonAt(ln, bx, saveLabel()) >= 0)
	}
	return m, nil
}

// clickForm은 폼의 r번째 줄(formModal 순서)을 누른 것. 선택 칸은 누를 때마다 바뀐다(‹ 쪽이면 거꾸로).
func (m model) clickForm(r, c int, save bool) (tea.Model, tea.Cmd) {
	f := &m.form
	if save {
		return m.saveForm()
	}
	const labelW = formLabelW
	timeAt := labelW + f.inputs[fStartDate].Width + 1 + 2
	if r < 0 || r >= len(formRows) {
		return m, nil
	}
	fd := formRows[r]
	if (fd == fStartDate || fd == fEndDate) && !f.allDay && c >= timeAt {
		fd++ // 같은 줄의 시각 칸
	}
	cmd := f.setFocus(fd)
	dir := 1
	switch fd {
	case fAllDay:
		f.toggleAllDay()
	case fRepeat:
		if c < labelW+2 {
			dir = -1
		}
		f.cycleRepeat(dir)
	case fAlarm:
		if c < labelW+2 {
			dir = -1
		}
		f.cycleAlarm(dir)
	case fCalendar:
		if c < labelW+4 { // "● ‹ "
			dir = -1
		}
		f.cycleCalendar(dir)
	}
	return m, cmd
}

// buttonAt은 줄 ln에 buttons(labels)가 있고 가로 위치 x가 그중 하나 위면 그 번호, 아니면 -1.
func buttonAt(ln string, x int, labels ...string) int {
	want := ansi.Strip(buttons(labels, -1))
	i := strings.Index(ln, want)
	if i < 0 {
		return -1
	}
	at := rw.StringWidth(ln[:i])
	for b, l := range labels {
		w := rw.StringWidth("[ " + l + " ]")
		if x >= at && x < at+w {
			return b
		}
		at += w + 2
	}
	return -1
}

// clickCal은 달력 화면 클릭. 칸 안의 일정은 상세, 빈 곳은 새 일정, 날짜 줄은 일정 목록을 열고, 제목 양옆 꺽쇠는 달을 옮기고, 제목은 월 이동 창을 연다.
func (m model) clickCal(x, y int) (tea.Model, tea.Cmd) {
	g := m.layout(m.height - 1)
	if y == 1 || y == 2 { // 2배 크기 제목 줄. Terminal은 이 줄의 가로 위치를 2배 크기 칸 단위로 준다(키 로그로 확인)
		total := g.total
		switch m.view {
		case vAgenda:
			total = m.width
		case vWeek:
			wl := m.weekLayout(m.height - 1)
			total = wl.gutter + 8
			for _, w := range wl.cws {
				total += w
			}
		}
		_, pad, fw := m.titleLayout(total)
		switch {
		case x >= pad-1 && x <= pad+2: // ‹
			return m.step(-1), nil
		case x >= pad+fw-3 && x <= pad+fw: // ›
			return m.step(1), nil
		case x > pad+2 && x < pad+fw-3: // 가운데 "2026년 10월" → 월 이동(g)
			return m.updateCal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
		}
		return m, nil
	}
	switch m.view {
	case vAgenda:
		return m.clickAgenda(y)
	case vWeek:
		return m.clickWeek(x, y)
	}
	wk, row, top := -1, 0, gridTop
	for i, ch := range g.chs {
		if y >= top && y < top+ch {
			wk, row = i, y-top
		}
		top += ch + 1
	}
	col, left := -1, 1
	for i, cw := range g.cws {
		if x >= left && x < left+cw {
			col = i
		}
		left += cw + 1
	}
	d := wk*7 + col - g.offset + 1
	if wk < 0 || col < 0 || d < 1 || d > g.days {
		return m, nil
	}
	m = m.move(time.Date(m.cursor.Year(), m.cursor.Month(), d, 0, 0, 0, 0, time.Local))
	m.daySel = 0
	// 칸 안의 줄은 cellLines 순서: 0 날짜, 1… 일정, 칸이 모자라면 마지막 줄 "+N개 더"
	evs, ch := m.db.on(m.cursor), g.chs[wk]
	switch {
	case row == 0 || row == ch-1 && len(evs) > ch-1: // 날짜 줄·"+N개 더" → 그날 일정 목록
		return m.push(mDay), nil
	case row-1 < len(evs): // 일정 → 바로 상세
		m.detail, m.detailBtn = evs[row-1], 0
		return m.push(mDetail), nil
	}
	return m.openNewForm() // 빈 곳 → 새 일정
}
