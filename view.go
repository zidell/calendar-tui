package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

// 색은 테마(어두운 배경·밝은 배경)마다 다르다. applyTheme이 정한다. 모달 테두리·선택 막대는 무채색.
var (
	focusBg, selBg                                    lipgloss.Color
	titleSt, headerSt, borderSt, plainSt, boldSt      lipgloss.Style
	sundaySt, satSt, todayLblSt, todayBdSt, helpSt    lipgloss.Style
	selTextSt, errSt, dimSt, modalSt, faintSt, linkSt lipgloss.Style
	// 한글 로케일에서도 모호폭 문자를 1칸으로 센다(터미널 기본 표시와 일치)
	rw = &runewidth.Condition{EastAsianWidth: false}
)

// themeDark는 지금 쓰는 색이 어두운 배경용인지(applyTheme이 정함).
var themeDark = true

// termDark는 터미널 배경이 어두운지(시작 때 한 번 물어 둔다, main.go). theme = "auto"일 때 쓴다.
var termDark = true

// followSystem은 터미널 배경이 시스템 모양(다크·라이트)을 따라가는지. 시작 때 물어 둔 배경과 시스템 모양이 같으면
// 따라간다고 보고, 실행 중 시스템 모양이 바뀌면 termDark도 바꾼다(appearanceMsg). 배경을 고정한 터미널 프로필이면
// 시작 때부터 어긋나 있으므로 따라가지 않는다. Bubble Tea v1은 실행 중 OSC 11 응답을 해석하지 못해 다시 묻지 못한다.
var followSystem bool

// applyTheme은 "auto"·"dark"·"light"에 맞춰 색을 정한다.
// 어두운 배경 값의 이력: 달력 선 240 → 236(밝음) → 234(배경 #171717보다 어두워 보임) → 235.
func applyTheme(theme string) {
	dark := theme == "dark" || (theme != "light" && termDark)
	themeDark = dark
	c := func(d, l string) lipgloss.Color {
		if dark {
			return lipgloss.Color(d)
		}
		return lipgloss.Color(l)
	}
	focusBg = c("250", "250") // 모달 안에서 고른 행·버튼(검정 글씨)
	selBg = c("237", "254")   // 달력에서 고른 날 배경
	titleSt = lipgloss.NewStyle().Bold(true)
	headerSt = lipgloss.NewStyle().Foreground(c("245", "242"))
	borderSt = lipgloss.NewStyle().Foreground(c("235", "252")) // 달력 선
	plainSt = lipgloss.NewStyle()
	boldSt = lipgloss.NewStyle().Bold(true)
	sundaySt = lipgloss.NewStyle().Foreground(c("203", "160"))
	satSt = sundaySt // 토요일도 일요일과 같은 빨강
	todayLblSt = lipgloss.NewStyle().Bold(true).Foreground(c("255", "232"))
	todayBdSt = lipgloss.NewStyle().Foreground(c("255", "232")) // 오늘 칸 테두리
	helpSt = lipgloss.NewStyle().Foreground(c("241", "245"))
	faintSt = lipgloss.NewStyle().Foreground(c("244", "246")) // 반복 설명 등 흐린 글씨
	selTextSt = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0"))
	errSt = lipgloss.NewStyle().Foreground(c("203", "160"))
	dimSt = lipgloss.NewStyle().Foreground(c("238", "251")) // 모달 아래 깔린 화면
	linkSt = lipgloss.NewStyle().Underline(true).Foreground(c("117", "25"))
	modalSt = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c("244", "246")).Padding(0, 1)
}

func init() { applyTheme("dark") }

// seg는 한 칸 안의 글자 조각. 조각마다 따로 렌더링해야 ANSI 코드가 중첩되지 않는다.
type seg struct {
	st   lipgloss.Style
	text string
}

// line은 조각들을 폭 w에 맞춰 자르고 채운다. bg가 있으면 배경 없는 조각과 빈칸에 깐다.
func line(w int, bg lipgloss.TerminalColor, segs ...seg) string {
	var b strings.Builder
	left := w
	for _, sg := range segs {
		if left <= 0 {
			break
		}
		t := sg.text
		if rw.StringWidth(t) > left {
			if left > 2 {
				t = rw.Truncate(t, left, "..")
			} else { // ".."도 안 들어가면 그냥 자른다(넘치면 세로선이 밀린다)
				t = rw.Truncate(t, left, "")
			}
		}
		left -= rw.StringWidth(t)
		st := sg.st
		if _, none := st.GetBackground().(lipgloss.NoColor); bg != nil && none {
			st = st.Background(bg)
		}
		b.WriteString(st.Render(t))
	}
	pad := plainSt
	if bg != nil {
		pad = pad.Background(bg)
	}
	b.WriteString(pad.Render(strings.Repeat(" ", max(0, left))))
	return b.String()
}

// timeLabel은 날짜 d에서 본 일정 시각. 여러 날에 걸치면 그날 몫만.
func timeLabel(e event, d time.Time) string {
	if e.allDay {
		return L("하루 종일", "all day")
	}
	first, last := dateOf(e.start).Equal(d), e.lastDay().Equal(d)
	switch {
	case first && last:
		return clock(e.start) + "–" + clock(e.end)
	case first:
		return clock(e.start) + "–"
	case last:
		return "–" + clock(e.end)
	}
	return L("종일", "all day")
}

// calColor는 일정의 캘린더 색. 모르면 무난한 파랑.
func (m model) calColor(e event) lipgloss.Color {
	if c := m.db.calendar(e.calID).color; c != "" {
		return lipgloss.Color(c)
	}
	return lipgloss.Color("#7aa2c8")
}

// inkOf는 캘린더 색을 글씨로 쓸 때의 색. 밝은 테마에선 바탕에 묻히지 않게 검정 쪽으로 45% 섞는다.
func inkOf(c lipgloss.Color) lipgloss.Color {
	var r, g, b int
	if themeDark {
		return c
	}
	if _, err := fmt.Sscanf(string(c), "#%02x%02x%02x", &r, &g, &b); err != nil {
		return c
	}
	k := 0.55
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", int(float64(r)*k), int(float64(g)*k), int(float64(b)*k)))
}

// textOn은 배경색 위 글자색. 밝기(0~255,000 가중합)가 기준 미만인 어두운 색만 흰 글씨, 나머지는 검정.
// 기준 이력: 140,000(55%) → 102,000(40%, 흰 글씨가 너무 자주 나온다는 피드백)
func textOn(c lipgloss.Color) lipgloss.Color {
	var r, g, b int
	if _, err := fmt.Sscanf(string(c), "#%02x%02x%02x", &r, &g, &b); err == nil && r*299+g*587+b*114 < 102000 {
		return lipgloss.Color("255")
	}
	return lipgloss.Color("0")
}

// dayStyle은 날짜 숫자 색. 주말·공휴일은 빨강, 나머지는 기본.
func (m model) dayStyle(d time.Time) lipgloss.Style {
	if _, off := m.db.holiday(d); off || d.Weekday() == time.Sunday {
		return sundaySt
	}
	if d.Weekday() == time.Saturday {
		return satSt
	}
	return plainSt
}

// cellLines는 날짜 한 칸의 내용(h줄)을 만든다.
func (m model) cellLines(date time.Time, w, h int) []string {
	selected := date.Equal(m.cursor)
	var bg lipgloss.TerminalColor
	if selected {
		bg = selBg
	}
	hol, off := m.db.holiday(date)
	head := []seg{{plainSt, " "}, {m.dayStyle(date).Bold(true), fmt.Sprintf("%d", date.Day())}}
	if date.Equal(m.today) { // 오늘은 칸 테두리를 날짜색으로 둘러 표시한다(gridView)
		head = append(head, seg{todayLblSt, L(" 오늘", " Today")})
	}
	if len(hol) > 0 { // 휴일은 일정 줄 대신 날짜 옆에. 공휴일은 빨강, 기념일은 흐리게
		hst := helpSt
		if off {
			hst = sundaySt
		}
		head = append(head, seg{hst, " " + strings.Join(hol, "·")})
	}
	out := []string{line(w, bg, head...)}

	evs := m.db.on(date)
	for i, e := range evs {
		if len(out) == h-1 && len(evs)-i > 1 {
			out = append(out, line(w, bg, seg{helpSt, fmt.Sprintf(L(" +%d개 더", " +%d more"), len(evs)-i)}))
			break
		}
		if len(out) == h {
			break
		}
		col := m.calColor(e)
		if e.allDay { // 하루 종일은 줄 전체를 캘린더 색으로
			out = append(out, line(w, col, seg{plainSt.Foreground(textOn(col)), " " + e.title}))
			continue
		}
		cst := plainSt.Foreground(inkOf(col)) // 시각·제목 모두 캘린더 색
		segs := []seg{{plainSt, " "}}
		if dateOf(e.start).Equal(date) {
			segs = append(segs, seg{cst, clock(e.start) + " "})
		}
		out = append(out, line(w, bg, append(segs, seg{cst, e.title})...))
	}
	for len(out) < h {
		out = append(out, line(w, bg))
	}
	return out
}

// split은 total을 n칸으로 나눈다. 남는 것은 앞쪽 칸부터 1씩 더해 꼭 채운다.
func split(total, n, minSize int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = max(minSize, total/n)
		if i < total%n && total/n >= minSize {
			out[i]++
		}
	}
	return out
}

// 좁을 때 커서가 있는 열의 최소 폭을 확보한다(사용자 지시, 2026-10-04).
//
//	단위는 모두 영문 글자 수(= 터미널 칸, 한글은 한 글자가 2).
//	똑같이 나눈 폭이 focusWidth(config.toml display.focus_min_width, 기본 30) 이상이면 그대로 둔다.
//	그보다 좁으면 다른 열에서 고르게 가져와 focusWidth를 맞춘다. 다른 열은 minCol(5자 = 한글 2자 + 여백)
//	아래로 줄이지 않고, 커서 열은 그래도 focusFloor(12자)까지는 확보한다. 더 좁으면 글꼴 크기·창 폭으로 해결한다.
//
// 이력: 고정 24칸 → 창 폭 비율 → 내용 기반 협상(강도 0~1) → 최소폭 확보(원안). 협상 방식은 "이미 충분한데도
// 넓어질 수 있다"는 우려로 버렸다.
const (
	minCol     = 5
	focusFloor = 12
)

// focusWidth는 좁을 때 커서 열 최소 폭. 0이면 넓히지 않는다.
var focusWidth = 30

// colWidths는 일곱 열의 폭. fc는 커서가 있는 열.
func colWidths(total, fc int) []int {
	out := split(total, 7, minCol)
	if focusWidth == 0 || out[fc] >= focusWidth {
		return out
	}
	want := min(focusWidth, total-6*minCol)           // 다른 열 하한이 먼저
	want = max(want, min(focusFloor, total-6*minCol)) // 그래도 커서 열 하한까지는
	if want <= out[fc] {
		return out
	}
	rest := split(total-want, 6, minCol)
	return append(append(append([]int{}, rest[:fc]...), want), rest[fc:]...)
}

// grid는 달력 격자의 크기. gridView와 마우스 클릭(mouse.go)이 같이 쓴다.
type grid struct {
	offset, days, weeks int   // 1일의 요일, 날 수, 주 수
	cws, chs            []int // 칸 폭(7열)·칸 높이(주마다)
	total               int   // 세로선 포함 전체 폭
}

// gridTop은 첫 주 칸이 시작하는 줄(여백·제목 2줄·요일·위 가로선 다음).
const gridTop = 5

func (m model) layout(h int) grid {
	y, mo := m.cursor.Year(), m.cursor.Month()
	first := time.Date(y, mo, 1, 0, 0, 0, 0, time.Local)
	days := time.Date(y, mo+1, 0, 0, 0, 0, 0, time.Local).Day()
	offset := (int(first.Weekday()) - int(weekStart) + 7) % 7 // 첫 열이 weekStart
	weeks := (offset + days + 6) / 7

	fc := (offset + m.cursor.Day() - 1) % 7
	g := grid{offset: offset, days: days, weeks: weeks, total: 8,
		cws: colWidths(m.width-8, fc),       // 세로선 8개 제외
		chs: split(h-4-(weeks+1), weeks, 2), // 여백·제목 2줄·요일, 가로선 제외
	}
	for _, w := range g.cws {
		g.total += w
	}
	return g
}

// 월 제목 양옆의 이전·다음 달 꺽쇠. 클릭하면 달을 옮긴다(mouse.go).
const titleGap = "   "

// titleLayout은 2배 크기 제목 줄에서 제목 묶음("‹   2026년 10월   ›")의 시작 위치와 폭(2배 크기 칸 기준).
func (m model) titleLayout(total int) (title string, pad, fw int) {
	title = monthTitle(m.cursor)
	fw = 2 + 2*len(titleGap) + rw.StringWidth(title)
	return title, max(0, (total/2-fw)/2), fw
}

// gridView는 제목·요일·달력을 h줄로 그린다.
func (m model) gridView(h int) string {
	y, mo := m.cursor.Year(), m.cursor.Month()
	g := m.layout(h)
	offset, days, weeks, cws, chs, total := g.offset, g.days, g.weeks, g.cws, g.chs, g.total

	// 오늘이 이 달에 있으면 그 칸(tw주 tc열)의 테두리만 날짜색(평일 흰색, 주말·공휴일 빨강)
	tw, tc := -1, -1
	todayBd := todayBdSt
	if m.dayStyle(m.today).GetForeground() != plainSt.GetForeground() {
		todayBd = m.dayStyle(m.today)
	}
	if m.today.Year() == y && m.today.Month() == mo {
		idx := offset + m.today.Day() - 1
		tw, tc = idx/7, idx%7
	}
	bst := func(hot bool) lipgloss.Style {
		if hot {
			return todayBd
		}
		return borderSt
	}
	// border는 above주와 below주 사이 가로선. 오늘 칸에 닿는 선분·꼭짓점만 흰색
	border := func(l, mid, r string, above, below int) string {
		near := above == tw || below == tw
		var b strings.Builder
		for j := 0; j <= 7; j++ {
			ch := mid
			if j == 0 {
				ch = l
			} else if j == 7 {
				ch = r
			}
			b.WriteString(bst(near && (j == tc || j == tc+1)).Render(ch))
			if j < 7 {
				b.WriteString(bst(near && j == tc).Render(strings.Repeat("─", cws[j])))
			}
		}
		return b.String()
	}

	var rows []string
	// 월 제목은 DEC 2배 크기 줄(윗절반 ESC#3 · 아랫절반 ESC#4)로 가운데에. 이 줄은 칸 하나가 2칸 폭이라 절반 폭 기준으로 가운데를 잡는다
	rows = append(rows, m.titleRows(total)...) // 맨 위 빈 줄은 여백
	hdr := " "
	for i := range 7 {
		wd := time.Weekday((i + int(weekStart)) % 7)
		st := headerSt
		if wd == time.Sunday {
			st = sundaySt
		} else if wd == time.Saturday {
			st = satSt
		}
		hdr += line(cws[i], nil, seg{plainSt, " "}, seg{st, weekName(wd)}) + " "
	}
	rows = append(rows, hdr, border("┌", "┬", "┐", -2, 0))
	for wk := 0; wk < weeks; wk++ {
		ch := chs[wk]
		cells := make([][]string, 7)
		for c := 0; c < 7; c++ {
			d := wk*7 + c - offset + 1
			if d < 1 || d > days {
				cells[c] = make([]string, ch)
				for i := range cells[c] {
					cells[c][i] = strings.Repeat(" ", cws[c])
				}
				continue
			}
			cells[c] = m.cellLines(time.Date(y, mo, d, 0, 0, 0, 0, time.Local), cws[c], ch)
		}
		bars := make([]string, 8)
		for j := range bars {
			bars[j] = bst(wk == tw && (j == tc || j == tc+1)).Render("│")
		}
		for i := 0; i < ch; i++ {
			var b strings.Builder
			b.WriteString(bars[0])
			for c := 0; c < 7; c++ {
				b.WriteString(cells[c][i] + bars[c+1])
			}
			rows = append(rows, b.String())
		}
		if wk < weeks-1 {
			rows = append(rows, border("├", "┼", "┤", wk, wk+1))
		}
	}
	rows = append(rows, border("└", "┴", "┘", weeks-1, -2))
	return strings.Join(rows, "\n")
}

// cut은 글자만 남은 줄에서 화면 칸 [from, to)를 잘라낸다. 걸친 넓은 글자는 빈칸으로 채운다.
func cut(s string, from, to int) string {
	var b strings.Builder
	col, n := 0, 0
	for _, r := range s {
		if col >= to {
			break
		}
		w := rw.RuneWidth(r)
		switch {
		case col >= from && col+w <= to:
			b.WriteRune(r)
			n += w
		case col+w > from:
			k := min(col+w, to) - max(col, from)
			b.WriteString(strings.Repeat(" ", k))
			n += k
		}
		col += w
	}
	if to-from > n {
		b.WriteString(strings.Repeat(" ", to-from-n))
	}
	return b.String()
}

// overlay는 배경을 흐리게 깔고 그 가운데에 fg를 얹는다.
const (
	dhTop    = "\x1b#3" // 이 줄을 2배 크기 윗절반으로
	dhBottom = "\x1b#4" // 아랫절반
)

func overlay(bg, fg string, w, h int) string {
	bl := strings.Split(bg, "\n")
	for len(bl) < h {
		bl = append(bl, "")
	}
	keep := make([]string, len(bl)) // 2배 크기 줄 표시는 지우지 않고 다시 붙인다
	for i := range bl {
		if strings.HasPrefix(bl[i], dhTop) || strings.HasPrefix(bl[i], dhBottom) {
			keep[i] = bl[i][:3]
		}
		bl[i] = ansi.Strip(bl[i])
	}
	fl := strings.Split(fg, "\n")
	fw := lipgloss.Width(fg)
	x, y := max(0, (w-fw)/2), max(0, (h-len(fl))/2)
	for i := range bl {
		plain := bl[i]
		if i < y || i >= y+len(fl) {
			bl[i] = keep[i] + dimSt.Render(plain)
			continue
		}
		pw := rw.StringWidth(plain)
		bl[i] = dimSt.Render(cut(plain, 0, x)) + fl[i-y] + dimSt.Render(cut(plain, min(x+fw, pw), pw))
	}
	return strings.Join(bl, "\n")
}

// box는 모달 테두리. w는 테두리 안쪽 폭.
func box(title string, w int, body ...string) string {
	return modalSt.Width(w).Render(titleSt.Render(title) + "\n\n" + strings.Join(body, "\n"))
}

func buttons(labels []string, sel int) string {
	parts := make([]string, len(labels))
	for i, l := range labels {
		st := helpSt
		if i == sel {
			st = selTextSt.Background(focusBg)
		}
		parts[i] = st.Render("[ " + l + " ]")
	}
	return strings.Join(parts, "  ")
}

// modalWidth는 테두리 안쪽 폭(여백 포함). 화면이 좁으면 줄인다.
func (m model) modalWidth(want int) int { return max(20, min(want, m.width-6)) }

// dayModal은 그날 일정 목록. 폭은 기본 60칸, 제목이 길면 화면 안에서 가장 긴 줄에 맞춰 넓힌다.
func (m model) dayModal() string {
	evs := m.db.on(m.cursor)
	lw := 13 // 시각 열 폭: 가장 긴 시각 + 2
	for _, e := range evs {
		lw = max(lw, rw.StringWidth(timeLabel(e, m.cursor))+2)
	}
	need := 0
	for _, e := range evs {
		rep := 0
		if e.repeat != repNone {
			rep = rw.StringWidth(" (" + e.repeatLabel(true) + ")")
		}
		need = max(need, 1+lw+rw.StringWidth(e.title)+rep+1)
	}
	w := m.modalWidth(max(60, need+2))
	iw := w - 2
	sel := min(m.daySel, len(evs))
	var rows []string
	add := seg{helpSt, L(" + 새 일정 추가", " + New event")}
	var abg lipgloss.TerminalColor
	if sel == 0 {
		abg, add.st = focusBg, selTextSt
	}
	rows = append(rows, line(iw, abg, add))
	if len(evs) == 0 {
		rows = append(rows, "", helpSt.Render(L(" 일정이 없습니다", " No events")))
	}
	for i, e := range evs {
		i++ // 0번 줄은 "+ 새 일정 추가"
		var bg lipgloss.TerminalColor
		tst := plainSt.Foreground(inkOf(m.calColor(e)))
		nst := tst
		if i == sel {
			bg, tst, nst = focusBg, selTextSt, selTextSt
		}
		rst := faintSt // 반복 설명은 흐리게
		if i == sel {
			rst = plainSt.Foreground(lipgloss.Color("240"))
		}
		rep := ""
		if e.repeat != repNone {
			rep = " (" + e.repeatLabel(true) + ")"
		}
		lbl := timeLabel(e, m.cursor)
		rows = append(rows, line(iw, bg, seg{tst, " " + lbl + strings.Repeat(" ", max(1, lw-rw.StringWidth(lbl)))}, seg{nst, e.title}, seg{rst, rep}))
	}

	// 목록이 화면보다 길면 선택 행이 보이게 잘라 보여준다
	start, n := scroll(len(rows), sel, m.height-8)
	rows = rows[start : start+n]
	title := dayLabel(m.cursor)
	if hol, _ := m.db.holiday(m.cursor); len(hol) > 0 {
		title += " · " + strings.Join(hol, "·")
	}
	return box(title, w, rows...)
}

// scroll은 n줄 목록을 화면에 maxRows줄(최소 3)만 보일 때 선택 행 sel이 보이는 시작 줄과 보이는 줄 수.
func scroll(n, sel, maxRows int) (start, shown int) {
	maxRows = max(3, maxRows)
	if n <= maxRows {
		return 0, n
	}
	return min(max(0, sel-maxRows+1), n-maxRows), maxRows
}

// calLabel은 "● 회사 · Google"처럼 캘린더 색 점을 붙인 이름.
func (m model) calLabel(id string) string {
	c := m.db.calendar(id)
	if c.id == "" {
		return ""
	}
	return plainSt.Foreground(m.calColor(event{calID: id})).Render("●") + " " + c.title + helpSt.Render(" · "+c.source)
}

func (m model) detailModal() string {
	w := m.modalWidth(54)
	e := m.detail
	st, et := e.start, e.end
	var when string
	switch {
	case e.allDay && dateOf(st).Equal(dateOf(et)):
		when = dayLabel(st) + L(" · 하루 종일", " · all day")
	case e.allDay:
		when = dayLabel(st) + " – " + dayLabel(et) + L(" · 하루 종일", " · all day")
	case dateOf(st).Equal(dateOf(et)):
		when = dayLabel(st) + " " + clock(st) + " – " + clock(et)
	default:
		when = dayLabel(st) + " " + clock(st) + " – " + dayLabel(et) + " " + clock(et)
	}
	label := func(s string) string { return headerSt.Render(s + strings.Repeat(" ", max(1, 10-rw.StringWidth(s)))) }
	rows := []string{boldSt.Render(e.title), "", label(L("일시", "When")) + when}
	if e.repeat != repNone {
		rows = append(rows, label(L("반복", "Repeat"))+e.repeatLabel(false))
	}
	if c := m.calLabel(e.calID); c != "" {
		rows = append(rows, label(L("캘린더", "Calendar"))+c)
	}
	if a := e.alarmText(); a != "" {
		rows = append(rows, label(L("알림", "Alert"))+a)
	}
	if e.location != "" {
		rows = append(rows, label(L("장소", "Location"))+e.location)
	}
	if u := e.link(); u != "" {
		rows = append(rows, label(L("링크", "Link"))+linkSt.Render(rw.Truncate(u, w-12, "…"))+helpSt.Render(L("  o 열기", "  o open")))
	}
	if e.memo != "" {
		rows = append(rows, label(L("메모", "Notes"))+e.memo)
	}
	if e.writable {
		rows = append(rows, "", buttons(detailButtons(), m.detailBtn))
	} else {
		rows = append(rows, "", helpSt.Render(L("읽기 전용 캘린더라 고칠 수 없습니다 · c 복제", "Read-only calendar · c duplicate")))
	}
	return box(L("일정 상세", "Event"), w, rows...)
}

func (m model) formModal() string {
	f := &m.form
	w := m.modalWidth(56)
	label := func(s string, fds ...field) string {
		st := headerSt
		mark := "  "
		for _, fd := range fds {
			if f.focus == fd {
				st, mark = titleSt, "▸ "
			}
		}
		return st.Render(mark + s + strings.Repeat(" ", max(1, formLabelW-2-rw.StringWidth(s))))
	}
	input := func(fd field) string {
		return lipgloss.NewStyle().Width(f.inputs[fd].Width + 1).Render(f.inputs[fd].View())
	}
	choice := func(fd field, s string) string {
		if f.focus == fd {
			return selTextSt.Background(focusBg).Render(s)
		}
		return s
	}
	check := "[ ]"
	if f.allDay {
		check = "[x]"
	}
	base, err := parseDate(f.value(fStartDate))
	if err != nil {
		base = m.cursor
	}
	rows := []string{
		label(L("제목", "Title"), fTitle) + input(fTitle),
		label(L("하루 종일", "All day"), fAllDay) + choice(fAllDay, check),
	}
	if f.allDay {
		rows = append(rows,
			label(L("시작", "Starts"), fStartDate)+input(fStartDate),
			label(L("종료", "Ends"), fEndDate)+input(fEndDate))
	} else {
		rows = append(rows,
			label(L("시작", "Starts"), fStartDate, fStartTime)+input(fStartDate)+"  "+input(fStartTime),
			label(L("종료", "Ends"), fEndDate, fEndTime)+input(fEndDate)+"  "+input(fEndTime))
	}
	cal := m.db.calendar(f.calID)
	rows = append(rows,
		label(L("반복", "Repeat"), fRepeat)+choice(fRepeat, "‹ "+f.repeatName(base)+" ›"),
		label(L("알림", "Alert"), fAlarm)+choice(fAlarm, "‹ "+f.alarmName()+" ›"),
		label(L("캘린더", "Calendar"), fCalendar)+plainSt.Foreground(m.calColor(event{calID: f.calID})).Render("● ")+
			choice(fCalendar, "‹ "+cal.title+" · "+cal.source+" ›"),
		label(L("장소", "Location"), fLocation)+input(fLocation),
		label("URL", fURL)+input(fURL),
		label(L("메모", "Notes"), fMemo)+input(fMemo),
		"",
		"  "+buttons([]string{saveLabel()}, int(f.focus-fSave)),
	)
	if f.err != "" {
		rows = append(rows, "", errSt.Render("  "+f.err))
	}
	title := L("새 일정", "New event")
	switch {
	case f.dup:
		title = L("일정 복제", "Duplicate event")
	case f.orig.id != "":
		title = L("일정 편집", "Edit event")
	}
	return box(title, w, rows...)
}

func (m model) spanModal() string {
	rows := []string{L("반복 일정입니다. 어디까지 바꿀까요?", "This is a repeating event. Change:"), "", buttons(spanButtons(), m.spanBtn)}
	return box(L("반복 일정 저장", "Save repeating event"), m.modalWidth(48), rows...)
}

func (m model) confirmModal() string {
	rows := []string{fmt.Sprintf(L("‘%s’ 일정을 삭제할까요?", "Delete “%s”?"), m.detail.title), ""}
	rows = append(rows, buttons(m.confirmButtons(), m.confirmBtn))
	if m.confirmErr != "" {
		rows = append(rows, "", errSt.Render(m.confirmErr))
	}
	return box(L("일정 삭제", "Delete event"), m.modalWidth(52), rows...)
}

func (m model) quickModal() string {
	w := m.modalWidth(56)
	rows := []string{m.quickIn.View(), ""}
	if strings.TrimSpace(m.quickIn.Value()) != "" {
		e := m.quickEvent()
		when := dayLabel(e.start) + L(" · 하루 종일", " · all day")
		if !e.allDay {
			when = dayLabel(e.start) + " " + clock(e.start) + " – " + clock(e.end)
		}
		rows = append(rows, headerSt.Render("→ ")+when, headerSt.Render("  ")+boldSt.Render(e.title)+"  "+m.calLabel(e.calID))
	} else {
		rows = append(rows, helpSt.Render(L("날짜를 안 쓰면 고른 날, 시각을 안 쓰면 하루 종일", "No date = selected day, no time = all day")), "")
	}
	if m.gotoErr != "" {
		rows = append(rows, errSt.Render(m.gotoErr))
	}
	rows = append(rows, "", buttons(quickButtons(), m.quickBtn))
	return box(L("빠른 추가", "Quick add"), w, rows...)
}

func (m model) moveModal() string {
	rows := []string{helpSt.Render(L("옮길 날짜 (시각·길이는 그대로)", "New date (time and length kept)")), m.moveIn.View()}
	if m.gotoErr != "" {
		rows = append(rows, errSt.Render(m.gotoErr))
	}
	rows = append(rows, "", buttons([]string{goLabel()}, m.moveBtn))
	return box(L("일정 이동", "Move event"), m.modalWidth(40), rows...)
}

func (m model) gotoModal() string {
	rows := []string{m.gotoIn.View()}
	if m.gotoErr != "" {
		rows = append(rows, errSt.Render(m.gotoErr))
	}
	rows = append(rows, "", buttons([]string{goLabel()}, m.gotoBtn))
	return box(L("월 이동", "Go to month"), m.modalWidth(32), rows...)
}

func (m model) modal(k modalKind) string {
	switch k {
	case mDay:
		return m.dayModal()
	case mDetail:
		return m.detailModal()
	case mForm:
		return m.formModal()
	case mSpan:
		return m.spanModal()
	case mConfirm:
		return m.confirmModal()
	case mGoto:
		return m.gotoModal()
	case mSettings:
		return m.settingMenuModal()
	case mCalendars:
		return m.calendarsModal()
	case mDisplay:
		return m.displayModal()
	case mMove:
		return m.moveModal()
	case mQuick:
		return m.quickModal()
	case mSearch:
		return m.searchModal()
	}
	return ""
}

func (m model) help() string {
	switch m.top() {
	case mNone:
		unit := L("달", "month")
		if m.view != vMonth {
			unit = L("주", "week")
		}
		return fmt.Sprintf(L(" ←↑↓→ 이동 · tab·[ ] 다음/이전 %s · v·1·2·3 보기(%s) · a 빠른 추가 · / 검색 · enter 일정 · g 월 이동 · t 오늘 · s 설정 · q 종료",
			" ←↑↓→ move · tab·[ ] next/prev %s · v·1·2·3 view (%s) · a quick add · / search · enter events · g go to · t today · s settings · q quit"), unit, viewName(m.view))
	case mDay:
		return L(" ↑↓ 선택 · enter 열기 · a 새 일정 · esc 닫기", " ↑↓ select · enter open · a new · esc close")
	case mDetail:
		return L(" tab·←→ 선택 · enter 실행 · e 편집 · c 복제 · m 이동 · d 삭제 · o 링크 열기 · esc 닫기",
			" tab·←→ select · enter run · e edit · c duplicate · m move · d delete · o open link · esc close")
	case mMove:
		return L(" enter 옮기기 · tab 버튼으로 · esc 취소", " enter move · tab to button · esc cancel")
	case mQuick:
		if m.quickBtn >= 0 {
			return L(" tab·←→ 버튼 이동 · enter 실행 · ↑ 입력으로 · esc 취소", " tab·←→ next button · enter run · ↑ back to input · esc cancel")
		}
		return L(" enter 추가 · tab 버튼으로 · esc 취소", " enter add · tab to buttons · esc cancel")
	case mSearch:
		return L(" ↑↓ 선택 · enter 열기 · esc 닫기", " ↑↓ select · enter open · esc close")
	case mForm:
		return L(" ↑↓ 칸 이동 · tab 버튼으로 · ←→/space 바꾸기 · enter 다음 · ctrl+s 저장 · esc 취소",
			" ↑↓ field · tab to buttons · ←→/space change · enter next · ctrl+s save · esc cancel")
	case mSpan:
		return L(" ←→ 선택 · enter 저장 · esc 돌아가기", " ←→ select · enter save · esc back")
	case mConfirm:
		return L(" ←→ 선택 · enter 삭제 · esc 취소", " ←→ select · enter delete · esc cancel")
	case mGoto:
		return L(" enter 이동 · tab 버튼으로 · esc 취소", " enter go · tab to button · esc cancel")
	case mSettings:
		return L(" ↑↓ 선택 · enter 열기 · esc 닫기", " ↑↓ select · enter open · esc close")
	case mCalendars:
		return L(" ↑↓ 선택 · space/enter 켜고 끄기 · [저장] 또는 ctrl+s 저장 · esc 취소",
			" ↑↓ select · space/enter toggle · [Save] or ctrl+s · esc cancel")
	case mDisplay:
		return L(" ↑↓ 선택 · ←→/space 바꾸기 · [저장] 또는 ctrl+s 저장 · esc 취소",
			" ↑↓ select · ←→/space change · [Save] or ctrl+s · esc cancel")
	}
	return ""
}

// 버튼 글자. 그리기와 클릭 판정(mouse.go)이 같이 쓴다.
func detailButtons() []string {
	return []string{L("편집", "Edit"), L("복제", "Duplicate"), L("이동", "Move"), L("삭제", "Delete")}
}
func spanButtons() []string {
	return []string{L("이 일정만", "This event"), L("이후 일정 모두", "All future events")}
}
func saveLabel() string { return L("저장", "Save") }
func quickButtons() []string {
	return []string{L("추가", "Add"), L("상세", "Details")}
}
func goLabel() string { return L("이동", "Go") }

func (m model) confirmButtons() []string {
	if m.detail.repeat != repNone {
		return []string{L("이 일정만 삭제", "This event"), L("이후 일정 모두 삭제", "All future events")}
	}
	return []string{L("삭제", "Delete")}
}

func (m model) View() string {
	h := m.height - 1 // 맨 아랫줄은 도움말
	out := m.mainView(h)
	for _, k := range m.stack {
		out = overlay(out, m.modal(k), m.width, h)
	}
	help := []seg{{helpSt, m.help()}}
	if m.db.err != "" {
		help = append([]seg{{errSt, " " + m.db.err}}, help...)
	}
	if m.db.cfg.err != "" {
		help = append([]seg{{errSt, " " + m.db.cfg.err}}, help...)
	}
	return out + "\n" + line(m.width, nil, help...)
}
