package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 보기: 월간(기본) · 주간(시간 격자) · 목록(어젠다). v로 돌린다. 마지막 보기는 state.json에 기억한다.
// 세 보기 모두 맨 위 3줄(여백·2배 크기 제목)은 같고, 제목 양옆 ‹ ›와 [ ]는 월간에선 한 달, 주간·목록에선 한 주씩 옮긴다.
const (
	vMonth = iota
	vWeek
	vAgenda
	viewCount
)

var viewNames = []string{"month", "week", "agenda"}

func viewName(v int) string {
	return []string{L("월간", "Month"), L("주간", "Week"), L("목록", "Agenda")}[v]
}

// step은 ‹ ›·[ ]로 옮기는 단위.
func (m model) step(n int) model {
	if m.view == vMonth {
		return m.move(addMonths(m.cursor, n))
	}
	return m.move(m.cursor.AddDate(0, 0, 7*n))
}

func (m model) mainView(h int) string {
	switch m.view {
	case vWeek:
		return m.weekView(h)
	case vAgenda:
		return m.agendaView(h)
	}
	return m.gridView(h)
}

// titleRows는 맨 위 3줄(여백, 2배 크기 제목 윗·아랫절반). total은 제목을 가운데 맞출 폭.
func (m model) titleRows(total int) []string {
	title, tpad, _ := m.titleLayout(total)
	head := strings.Repeat(" ", tpad) + headerSt.Render("‹") + titleGap + titleSt.Render(title) + titleGap + headerSt.Render("›")
	return []string{"", dhTop + head, dhBottom + head}
}

// ── 목록(어젠다) ──

// aRow는 목록 보기의 한 줄: 날짜 줄(ev == nil) 또는 일정 줄.
type aRow struct {
	date time.Time
	ev   *event
}

// agendaRows는 고른 날부터 일정이 있는 날을 이어 h줄까지. 고른 날은 일정이 없어도 넣는다. 최대 120일.
func (m model) agendaRows(h int) []aRow {
	var rows []aRow
	for d := m.cursor; len(rows) < h && daysBetween(m.cursor, d) < 120; d = d.AddDate(0, 0, 1) {
		evs := m.db.on(d)
		hol, _ := m.db.holiday(d)
		if len(evs) == 0 && len(hol) == 0 && !d.Equal(m.cursor) {
			continue
		}
		rows = append(rows, aRow{date: d})
		for i := range evs {
			rows = append(rows, aRow{date: d, ev: &evs[i]})
		}
	}
	if len(rows) > h {
		rows = rows[:h]
	}
	return rows
}

const agendaTop = 4 // 제목 3줄 + 빈 줄 다음부터 목록

func (m model) agendaView(h int) string {
	out := append(m.titleRows(m.width), "")
	rows := m.agendaRows(h - agendaTop)
	lw := 0 // 시각 열 폭
	for _, r := range rows {
		if r.ev != nil {
			lw = max(lw, rw.StringWidth(timeLabel(*r.ev, r.date)))
		}
	}
	for _, r := range rows {
		if r.ev == nil {
			var bg lipgloss.TerminalColor
			if r.date.Equal(m.cursor) {
				bg = selBg
			}
			segs := []seg{{plainSt, " "}, {m.dayStyle(r.date).Bold(true), dayLabel(r.date)}}
			if r.date.Equal(m.today) {
				segs = append(segs, seg{todayLblSt, L("  오늘", "  Today")})
			}
			if hol, off := m.db.holiday(r.date); len(hol) > 0 {
				st := helpSt
				if off {
					st = sundaySt
				}
				segs = append(segs, seg{st, "  " + strings.Join(hol, "·")})
			}
			if len(m.db.on(r.date)) == 0 {
				segs = append(segs, seg{helpSt, L("  일정 없음", "  No events")})
			}
			out = append(out, line(m.width, bg, segs...))
			continue
		}
		e := *r.ev
		col := m.calColor(e)
		lbl := timeLabel(e, r.date)
		segs := []seg{{plainSt, "    "}, {faintSt, lbl + strings.Repeat(" ", lw-rw.StringWidth(lbl)+2)}}
		if e.allDay {
			segs = append(segs, seg{plainSt.Background(col).Foreground(textOn(col)), " " + e.title + " "})
		} else {
			segs = append(segs, seg{plainSt.Foreground(inkOf(col)), e.title})
		}
		if e.repeat != repNone {
			segs = append(segs, seg{faintSt, " (" + e.repeatLabel(true) + ")"})
		}
		if e.location != "" {
			segs = append(segs, seg{helpSt, "  · " + e.location})
		}
		out = append(out, line(m.width, nil, segs...))
	}
	for len(out) < h {
		out = append(out, "")
	}
	return strings.Join(out[:h], "\n")
}

// clickAgenda: 일정 줄 → 상세, 날짜 줄 → 그날 일정 목록.
func (m model) clickAgenda(y int) (tea.Model, tea.Cmd) {
	rows := m.agendaRows(m.height - 1 - agendaTop)
	i := y - agendaTop
	if i < 0 || i >= len(rows) {
		return m, nil
	}
	m = m.move(rows[i].date)
	if e := rows[i].ev; e != nil {
		m.detail, m.detailBtn = *e, 0
		return m.push(mDetail), nil
	}
	m.daySel = 0
	return m.push(mDay), nil
}

// ── 주간(시간 격자) ──

// weekLayout은 주간 보기의 자리. 그리기와 클릭이 같이 쓴다.
type weekLayout struct {
	days      []time.Time
	gutter    int   // 왼쪽 시각 열 폭
	cws       []int // 날짜 열 폭
	allDay    int   // 하루 종일 줄 수
	hourTop   int   // 시간 격자가 시작하는 줄
	hours     int   // 보이는 시간 수(한 시간 = 한 줄)
	startHour int
	lanes     [7][]weekBlock // 날마다 시간 일정 자리
}

type weekBlock struct {
	e          event
	from, to   int // 줄 [from, to)
	lane, nlan int
}

// hourLabel은 시각 열 글자: "15:00" / "오후 3시" / "3pm".
func hourLabel(h int) string {
	t := time.Date(2000, 1, 1, h, 0, 0, 0, time.Local)
	switch {
	case !clock12:
		return t.Format("15:04")
	case lang == "en":
		return t.Format("3pm")
	case h < 12:
		return "오전 " + t.Format("3") + "시"
	}
	return "오후 " + t.Format("3") + "시"
}

func (m model) weekLayout(h int) weekLayout {
	var wl weekLayout
	first := m.cursor.AddDate(0, 0, -((int(m.cursor.Weekday()) - int(weekStart) + 7) % 7))
	for i := range 7 {
		wl.days = append(wl.days, first.AddDate(0, 0, i))
	}
	for hr := range 24 {
		wl.gutter = max(wl.gutter, rw.StringWidth(hourLabel(hr))+2)
	}
	fc := daysBetween(first, m.cursor)
	wl.cws = colWidths(m.width-wl.gutter-8, fc)
	earliest, latest := 24*60, 0
	for i, d := range wl.days {
		n := 0
		for _, e := range m.db.on(d) {
			if e.allDay {
				n++
				continue
			}
			s, en := minutesOn(e, d)
			earliest, latest = min(earliest, s), max(latest, en)
		}
		wl.allDay = max(wl.allDay, n)
		_ = i
	}
	wl.allDay = min(wl.allDay, 3)
	wl.hourTop = 3 + 1 + 1 // 제목 3줄, 날짜 줄, 가로선
	if wl.allDay > 0 {
		wl.hourTop += wl.allDay + 1
	}
	wl.hours = max(1, min(24, h-wl.hourTop-1))
	// 보일 시간: 기본은 8시부터. 일정이 더 이르면 그 시각부터, 늦게 끝나면 끝이 보이게 내린다
	start := min(8, earliest/60)
	if latest > 0 && (latest+59)/60 > start+wl.hours {
		start = max(min(earliest/60, 24-wl.hours), (latest+59)/60-wl.hours)
	}
	wl.startHour = max(0, min(start, 24-wl.hours))
	for i, d := range wl.days {
		var blocks []weekBlock
		for _, e := range m.db.on(d) {
			if e.allDay {
				continue
			}
			s, en := minutesOn(e, d)
			from, to := s/60-wl.startHour, (en+59)/60-wl.startHour
			if to <= from {
				to = from + 1
			}
			if to <= 0 || from >= wl.hours {
				continue
			}
			blocks = append(blocks, weekBlock{e: e, from: max(0, from), to: min(wl.hours, to)})
		}
		sort.SliceStable(blocks, func(a, b int) bool { return blocks[a].from < blocks[b].from })
		var laneEnd []int // 겹치면 옆 칸(lane)으로
		for j := range blocks {
			placed := false
			for l, end := range laneEnd {
				if end <= blocks[j].from {
					blocks[j].lane, laneEnd[l], placed = l, blocks[j].to, true
					break
				}
			}
			if !placed {
				blocks[j].lane = len(laneEnd)
				laneEnd = append(laneEnd, blocks[j].to)
			}
		}
		for j := range blocks {
			blocks[j].nlan = len(laneEnd)
		}
		wl.lanes[i] = blocks
	}
	return wl
}

// minutesOn은 일정이 날짜 d 안에서 차지하는 [시작, 끝) 분.
func minutesOn(e event, d time.Time) (int, int) {
	s, en := 0, 24*60
	if dateOf(e.start).Equal(d) {
		s = e.start.Hour()*60 + e.start.Minute()
	}
	if dateOf(e.end).Equal(d) {
		en = e.end.Hour()*60 + e.end.Minute()
	}
	return s, max(en, s+1)
}

// laneX는 폭 w를 n칸으로 나눈 l번째 칸의 시작과 폭.
func laneX(w, n, l int) (int, int) {
	sw := w / n
	x := sw * l
	if l == n-1 {
		sw = w - x
	}
	return x, sw
}

func (m model) weekView(h int) string {
	wl := m.weekLayout(h)
	total := wl.gutter + 8
	for _, w := range wl.cws {
		total += w
	}
	out := m.titleRows(total)
	bar := borderSt.Render("│")
	border := func(l, mid, r string) string {
		var b strings.Builder
		b.WriteString(strings.Repeat(" ", wl.gutter-1) + borderSt.Render(l))
		for i, w := range wl.cws {
			b.WriteString(borderSt.Render(strings.Repeat("─", w)))
			if i < 6 {
				b.WriteString(borderSt.Render(mid))
			}
		}
		return b.String() + borderSt.Render(r)
	}
	bgOf := func(i int) lipgloss.TerminalColor {
		if wl.days[i].Equal(m.cursor) {
			return selBg
		}
		return nil
	}
	// 날짜 줄
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", wl.gutter-1) + " ")
	for i, d := range wl.days {
		segs := []seg{{plainSt, " "}, {m.dayStyle(d).Bold(true), weekName(d.Weekday()) + " " + fmt.Sprint(d.Day())}}
		if d.Equal(m.today) {
			segs = append(segs, seg{todayLblSt, L(" 오늘", " Today")})
		}
		if hol, off := m.db.holiday(d); len(hol) > 0 {
			st := helpSt
			if off {
				st = sundaySt
			}
			segs = append(segs, seg{st, " " + strings.Join(hol, "·")})
		}
		b.WriteString(line(wl.cws[i], bgOf(i), segs...) + " ")
	}
	out = append(out, b.String(), border("├", "┼", "┤"))
	// 하루 종일 줄
	if wl.allDay > 0 {
		for r := range wl.allDay {
			b.Reset()
			b.WriteString(strings.Repeat(" ", wl.gutter-1) + bar)
			for i, d := range wl.days {
				var all []event
				for _, e := range m.db.on(d) {
					if e.allDay {
						all = append(all, e)
					}
				}
				switch {
				case r == wl.allDay-1 && len(all) > wl.allDay:
					b.WriteString(line(wl.cws[i], bgOf(i), seg{helpSt, fmt.Sprintf(L(" +%d개 더", " +%d more"), len(all)-r)}))
				case r < len(all):
					col := m.calColor(all[r])
					b.WriteString(line(wl.cws[i], col, seg{plainSt.Foreground(textOn(col)), " " + all[r].title}))
				default:
					b.WriteString(line(wl.cws[i], bgOf(i)))
				}
				b.WriteString(bar)
			}
			out = append(out, b.String())
		}
		out = append(out, border("├", "┼", "┤"))
	}
	// 시간 격자
	for r := range wl.hours {
		b.Reset()
		lbl := hourLabel(wl.startHour + r)
		b.WriteString(helpSt.Render(strings.Repeat(" ", wl.gutter-1-rw.StringWidth(lbl))+lbl) + bar)
		for i := range wl.days {
			cells := make([]string, 0, 4)
			var lanes []*weekBlock
			n := 1
			for j := range wl.lanes[i] {
				bl := &wl.lanes[i][j]
				n = bl.nlan
				if r >= bl.from && r < bl.to {
					lanes = append(lanes, bl)
				}
			}
			for l := range n {
				_, sw := laneX(wl.cws[i], n, l)
				var hit *weekBlock
				for _, bl := range lanes {
					if bl.lane == l {
						hit = bl
					}
				}
				if hit == nil {
					cells = append(cells, line(sw, bgOf(i)))
					continue
				}
				col := m.calColor(hit.e)
				txt := ""
				if r == hit.from {
					txt = " " + clock(hit.e.start) + " " + hit.e.title
				} else if r == hit.from+1 && sw < rw.StringWidth(" "+clock(hit.e.start)+" "+hit.e.title) {
					txt = " " + hit.e.title // 첫 줄에 다 못 쓴 제목은 둘째 줄에
				}
				cells = append(cells, line(sw, col, seg{plainSt.Foreground(textOn(col)), txt}))
			}
			b.WriteString(strings.Join(cells, "") + bar)
		}
		out = append(out, b.String())
	}
	out = append(out, border("└", "┴", "┘"))
	for len(out) < h {
		out = append(out, "")
	}
	return strings.Join(out[:h], "\n")
}

// clickWeek: 시간 일정 → 상세, 빈 시간 → 그 시각 새 일정, 날짜 줄 → 그날 일정 목록, 하루 종일 줄 → 그 일정 상세.
func (m model) clickWeek(x, y int) (tea.Model, tea.Cmd) {
	wl := m.weekLayout(m.height - 1)
	day, cx := -1, 0
	left := wl.gutter
	for i, w := range wl.cws {
		if x >= left && x < left+w {
			day, cx = i, x-left
		}
		left += w + 1
	}
	if day < 0 {
		return m, nil
	}
	d := wl.days[day]
	m = m.move(d)
	switch {
	case y == 3:
		m.daySel = 0
		return m.push(mDay), nil
	case wl.allDay > 0 && y >= 5 && y < 5+wl.allDay:
		var all []event
		for _, e := range m.db.on(d) {
			if e.allDay {
				all = append(all, e)
			}
		}
		if r := y - 5; r < len(all) && !(r == wl.allDay-1 && len(all) > wl.allDay) {
			m.detail, m.detailBtn = all[r], 0
			return m.push(mDetail), nil
		}
		m.daySel = 0
		return m.push(mDay), nil
	case y >= wl.hourTop && y < wl.hourTop+wl.hours:
		r := y - wl.hourTop
		for _, bl := range wl.lanes[day] {
			lx, sw := laneX(wl.cws[day], bl.nlan, bl.lane)
			if r >= bl.from && r < bl.to && cx >= lx && cx < lx+sw {
				m.detail, m.detailBtn = bl.e, 0
				return m.push(mDetail), nil
			}
		}
		return m.openNewFormAt(wl.startHour + r)
	}
	return m, nil
}
