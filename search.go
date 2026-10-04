package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 검색(/): 열 때 오늘 앞뒤 1년 일정을 한 번 읽어 두고(searchAll), 칠 때마다 제목·장소·메모에서 찾는다.
// 결과는 다가오는 일정이 먼저(가까운 순), 그다음 지난 일정(최근 순). 고르면 그날로 가서 상세를 연다.

const searchSpan = 365 // 오늘 앞뒤 날 수

func (m model) openSearch() (tea.Model, tea.Cmd) {
	t0 := time.Now()
	evs, err := m.db.b.events(m.today.AddDate(0, 0, -searchSpan), m.today.AddDate(0, 0, searchSpan+1))
	logLine(fmt.Sprintf("search load %d events %v", len(evs), time.Since(t0).Round(time.Millisecond)))
	m.searchAll = nil
	for _, e := range evs {
		if c := m.db.calendar(e.calID); m.db.visible(c) && !c.isHoliday() {
			m.searchAll = append(m.searchAll, e)
		}
	}
	m.searchErr = ""
	if err != nil {
		m.searchErr = err.Error()
	}
	m.searchIn.SetValue("")
	m.searchIn.Placeholder = L("제목·장소·메모", "title, location, notes")
	m.searchSel = 0
	cmd := m.searchIn.Focus()
	return m.push(mSearch), cmd
}

// searchHits는 지금 입력과 맞는 일정.
func (m model) searchHits() []event {
	q := strings.ToLower(strings.TrimSpace(m.searchIn.Value()))
	if q == "" {
		return nil
	}
	var up, past []event
	for _, e := range m.searchAll {
		if !strings.Contains(strings.ToLower(e.title+"\n"+e.location+"\n"+e.memo), q) {
			continue
		}
		if e.lastDay().Before(m.today) {
			past = append(past, e)
		} else {
			up = append(up, e)
		}
	}
	sort.SliceStable(up, func(i, j int) bool { return up[i].start.Before(up[j].start) })
	sort.SliceStable(past, func(i, j int) bool { return past[i].start.After(past[j].start) })
	return append(up, past...)
}

func (m model) updateSearch(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	hits := m.searchHits()
	switch k.String() {
	case "up", "ctrl+p":
		m.searchSel = max(0, m.searchSel-1)
		return m, nil
	case "down", "ctrl+n", "tab":
		m.searchSel = min(max(0, len(hits)-1), m.searchSel+1)
		return m, nil
	case "enter":
		if m.searchSel < len(hits) {
			return m.openHit(hits[m.searchSel])
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.searchIn, cmd = m.searchIn.Update(k)
	m.searchSel = 0
	return m, cmd
}

// openHit은 검색 결과의 날로 가서 상세를 연다. esc로 닫으면 검색으로 돌아온다.
func (m model) openHit(e event) (tea.Model, tea.Cmd) {
	m = m.move(dateOf(e.start))
	m.detail, m.detailBtn = e, 0
	return m.push(mDetail), nil
}

// searchRows는 결과 목록 줄과 그 첫 줄 번호(스크롤). 그리기와 클릭이 같이 쓴다.
func (m model) searchRows() (hits []event, start, shown int) {
	hits = m.searchHits()
	start, shown = scroll(len(hits), m.searchSel, m.height-12)
	return
}

func (m model) searchModal() string {
	w := m.modalWidth(64)
	iw := w - 2
	rows := []string{m.searchIn.View(), ""}
	hits, start, shown := m.searchRows()
	switch {
	case m.searchErr != "":
		rows = append(rows, errSt.Render(m.searchErr))
	case strings.TrimSpace(m.searchIn.Value()) == "":
		rows = append(rows, helpSt.Render(fmt.Sprintf(L("오늘 앞뒤 1년, 일정 %d개에서 찾습니다", "Searching %d events within a year of today"), len(m.searchAll))))
	case len(hits) == 0:
		rows = append(rows, helpSt.Render(L("찾은 일정이 없습니다", "No matches")))
	}
	when := func(e event) string {
		w := dayLabel(e.start)
		if e.start.Year() != m.today.Year() {
			w = L(e.start.Format("2006년 "), "") + w + L("", e.start.Format(", 2006"))
		}
		if !e.allDay {
			w += " " + clock(e.start)
		}
		return w
	}
	ww := 0
	for i := start; i < start+shown; i++ {
		ww = max(ww, rw.StringWidth(when(hits[i])))
	}
	for i := start; i < start+shown; i++ {
		e := hits[i]
		var bg lipgloss.TerminalColor
		dst, tst := headerSt, plainSt.Foreground(inkOf(m.calColor(e)))
		if i == m.searchSel {
			bg, dst, tst = focusBg, selTextSt, selTextSt
		}
		w := when(e)
		rows = append(rows, line(iw, bg, seg{dst, " " + w + strings.Repeat(" ", ww-rw.StringWidth(w)+2)}, seg{tst, e.title}))
	}
	if len(hits) > 0 {
		rows = append(rows, "", helpSt.Render(fmt.Sprintf(L("%d개", "%d found"), len(hits))))
	}
	return box(L("검색", "Search"), w, rows...)
}
