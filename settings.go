package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// settings는 설정 전부를 메모리에 들고 있다. 파일은 둘로 나눈다(config.go):
// 사용자가 고르는 것(캘린더 표시 여부)은 config.toml, 앱이 저절로 기억하는 것은 state.json.
// 숨긴 것만 적어 두므로 새로 생긴 계정·캘린더는 기본으로 보인다.
type settings struct {
	userConfig   `json:"-"` // config.toml: 표시 설정, 숨긴 계정(EKSource)·캘린더 ID
	LastCalendar string     `json:"lastCalendar"` // 마지막으로 저장한 일정의 캘린더. 새 일정에 자동 선택
	WindowCols   int        `json:"windowCols"`   // 마지막 창 크기(글자 칸). 앱 실행기가 새 창을 이 크기로 연다
	WindowRows   int        `json:"windowRows"`
	FontSize     float64    `json:"fontSize,omitempty"` // 마지막 Terminal 글꼴 크기. 종료 때 적고 실행기가 새 창에 적용한다
	View         string     `json:"view,omitempty"`     // 마지막 보기(month·week·agenda)
	dir          string     // 설정 폴더. 비면 파일을 읽고 쓰지 않는다
	readOnly     bool       // -mock: 읽기만 한다
	err          string     // config.toml 오류. 이전 값을 계속 쓰고 화면 아래에 띄운다
	cfgMod       time.Time
}

func (c *settings) hidden(list []string, id string) bool { return slices.Contains(list, id) }

func toggle(list []string, id string) []string {
	if i := slices.Index(list, id); i >= 0 {
		return slices.Delete(list, i, i+1)
	}
	return append(list, id)
}

// 설정 화면의 한 줄
type settingRow struct {
	kind int // rowSource · rowCal
	id   string
	cal  calendar
}

const (
	rowSource = iota
	rowCal
)

// settingRows는 계정마다 그 아래 캘린더를 둔다.
func (m model) settingRows() []settingRow {
	var rows []settingRow
	seen := map[string]bool{}
	for _, c := range m.db.cals {
		if seen[c.sourceID] {
			continue
		}
		seen[c.sourceID] = true
		rows = append(rows, settingRow{kind: rowSource, id: c.sourceID, cal: c})
		for _, x := range m.db.cals {
			if x.sourceID == c.sourceID {
				rows = append(rows, settingRow{kind: rowCal, id: x.id, cal: x})
			}
		}
	}
	return rows
}

// 설정 메뉴 항목. 설정이 늘면 여기에 추가한다.
func settingMenu() []string {
	return []string{L("캘린더 선택", "Calendars"), L("표시", "Display")}
}

func (m model) updateSettingMenu(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "up", "k":
		m.menuSel = max(0, m.menuSel-1)
	case "down", "j":
		m.menuSel = min(len(settingMenu())-1, m.menuSel+1)
	case "enter", "right", "l":
		switch m.menuSel {
		case 0:
			return m.openCalendars(), nil
		case 1:
			return m.openDisplay(), nil
		}
	}
	return m, nil
}

func (m model) settingMenuModal() string {
	w := m.modalWidth(40)
	var lines []string
	for i, name := range settingMenu() {
		var bg lipgloss.TerminalColor
		st := plainSt
		if i == m.menuSel {
			bg, st = focusBg, selTextSt
		}
		lines = append(lines, line(w-2, bg, seg{st, " " + name}, seg{st, "  ›"}))
	}
	return box(L("설정", "Settings"), w, lines...)
}

// openCalendars는 지금 설정을 복사해 고친다. [저장]을 눌러야 반영되고 esc는 버린다.
func (m model) openCalendars() model {
	m.setSel = 0
	m.setDraft = settings{userConfig: m.db.cfg.userConfig}
	m.setDraft.HiddenSources = slices.Clone(m.db.cfg.HiddenSources)
	m.setDraft.HiddenCalendars = slices.Clone(m.db.cfg.HiddenCalendars)
	return m.push(mCalendars)
}

func (m model) updateCalendars(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.settingRows()
	m.setSel = min(m.setSel, len(rows)) // len(rows) = [저장]
	d := &m.setDraft
	switch k.String() {
	case "up", "k":
		m.setSel = max(0, m.setSel-1)
	case "down", "j":
		m.setSel = min(len(rows), m.setSel+1)
	case "ctrl+s":
		return m.saveSettings(), nil
	case "enter", " ":
		if m.setSel == len(rows) {
			if k.String() == "enter" {
				return m.saveSettings(), nil
			}
			return m, nil
		}
		switch r := rows[m.setSel]; r.kind {
		case rowSource:
			d.HiddenSources = toggle(d.HiddenSources, r.id)
		case rowCal:
			d.HiddenCalendars = toggle(d.HiddenCalendars, r.id)
		}
	}
	return m, nil
}

func (m model) saveSettings() model {
	cfg := m.db.cfg
	cfg.HiddenSources, cfg.HiddenCalendars = m.setDraft.HiddenSources, m.setDraft.HiddenCalendars
	cfg.writeConfig(m.db.cals)
	return m.pop()
}

func (m model) calendarsModal() string {
	w := m.modalWidth(54)
	iw := w - 2
	rows := m.settingRows()
	sel := min(m.setSel, len(rows))
	cfg := &m.setDraft
	check := func(off bool) string {
		if off {
			return "[ ] "
		}
		return "[x] "
	}
	var lines []string
	for i, r := range rows {
		var bg lipgloss.TerminalColor
		st := plainSt
		if i == sel {
			bg, st = focusBg, selTextSt
		}
		srcOff := cfg.hidden(cfg.HiddenSources, r.cal.sourceID)
		switch r.kind {
		case rowSource:
			lines = append(lines, line(iw, bg, seg{st.Bold(true), " " + check(srcOff) + r.cal.source}))
		case rowCal:
			if srcOff && i != sel {
				st = helpSt // 계정을 끄면 그 아래 캘린더는 흐리게
			}
			dot := plainSt.Foreground(m.calColor(event{calID: r.id}))
			name := r.cal.title
			if !r.cal.writable {
				name += L(" (읽기 전용)", " (read-only)")
			}
			lines = append(lines, line(iw, bg, seg{st, "     " + check(cfg.hidden(cfg.HiddenCalendars, r.id))}, seg{dot, "● "}, seg{st, name}))
		}
	}
	start, n := scroll(len(lines), sel, m.height-10)
	lines = lines[start : start+n]
	btn := -1
	if sel == len(rows) {
		btn = 0
	}
	lines = append(lines, "", " "+buttons([]string{saveLabel()}, btn))
	return box(L("캘린더 선택", "Calendars"), w, strings.Join(lines, "\n"))
}

// 설정 › 표시: 언어·테마·주 시작·시각 표기. 사본을 고치고 [저장]에서 config.toml에 쓰고 반영한다.
var displayKeys = []string{"language", "theme", "week_start", "time_format", "focus_min_width"}

func (d *userConfig) displayField(key string) *string {
	switch key {
	case "language":
		return &d.Language
	case "theme":
		return &d.Theme
	case "week_start":
		return &d.WeekStart
	}
	return &d.TimeFormat
}

func displayName(key string) string {
	switch key {
	case "language":
		return L("언어", "Language")
	case "theme":
		return L("테마", "Theme")
	case "week_start":
		return L("주 시작", "Week starts")
	case "focus_min_width":
		return L("좁을 때 고른 요일 최소 폭", "Selected day min width")
	}
	return L("시각 표기", "Time format")
}

func displayValue(v string) string {
	switch v {
	case "auto":
		return L("자동", "Auto")
	case "ko":
		return "한국어"
	case "en":
		return "English"
	case "dark":
		return L("어두운 배경", "Dark")
	case "light":
		return L("밝은 배경", "Light")
	case "sunday":
		return L("일요일", "Sunday")
	case "monday":
		return L("월요일", "Monday")
	case "24h":
		return L("24시간 (15:30)", "24-hour (15:30)")
	case "12h":
		return L("12시간 (오후 3:30)", "12-hour (3:30pm)")
	}
	return v
}

func (m model) openDisplay() model {
	m.setSel = 0
	m.setDraft = settings{userConfig: m.db.cfg.userConfig}
	return m.push(mDisplay)
}

func (m model) cycleDisplay(i, dir int) model {
	key := displayKeys[i]
	if key == "focus_min_width" { // 0(넓히지 않음), 12~60을 2자씩. 끝에서 멈춘다
		w := m.setDraft.FocusWidth + 2*dir
		switch {
		case m.setDraft.FocusWidth == 0 && dir > 0:
			w = focusFloor
		case w < focusFloor:
			w = 0
		}
		m.setDraft.FocusWidth = min(60, w)
		return m
	}
	p := m.setDraft.displayField(key)
	ch := displayChoices[key]
	j := max(0, slices.Index(ch, or(*p, ch[0])))
	*p = ch[(j+dir+len(ch))%len(ch)]
	return m
}

func (m model) updateDisplay(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(displayKeys) // n = [저장]
	switch k.String() {
	case "up", "k", "shift+tab":
		m.setSel = max(0, m.setSel-1)
	case "down", "j":
		m.setSel = min(n, m.setSel+1)
	case "ctrl+s":
		return m.saveDisplay(), tea.SetWindowTitle(windowTitle())
	case "left", "h":
		if m.setSel < n {
			return m.cycleDisplay(m.setSel, -1), nil
		}
	case "right", "l", " ", "enter":
		if m.setSel == n {
			if k.String() == "enter" {
				return m.saveDisplay(), tea.SetWindowTitle(windowTitle())
			}
			return m, nil
		}
		return m.cycleDisplay(m.setSel, 1), nil
	}
	return m, nil
}

func (m model) saveDisplay() model {
	cfg := m.db.cfg
	for _, key := range displayKeys {
		if key != "focus_min_width" {
			*cfg.displayField(key) = *m.setDraft.displayField(key)
		}
	}
	cfg.FocusWidth = m.setDraft.FocusWidth
	cfg.apply()
	cfg.writeConfig(m.db.cals)
	return m.pop()
}

func (m model) displayModal() string {
	nameW := 0
	for _, key := range displayKeys {
		nameW = max(nameW, rw.StringWidth(displayName(key))+2)
	}
	w := m.modalWidth(nameW + 28)
	var lines []string
	for i, key := range displayKeys {
		var bg lipgloss.TerminalColor
		st := plainSt
		if i == m.setSel {
			bg, st = focusBg, selTextSt
		}
		name := displayName(key)
		val := ""
		if key == "focus_min_width" {
			val = L("넓히지 않음", "Off")
			if fw := m.setDraft.FocusWidth; fw > 0 {
				val = fmt.Sprintf(L("%d자 (영문 기준)", "%d chars"), fw)
			}
		} else {
			val = displayValue(or(*m.setDraft.displayField(key), displayChoices[key][0]))
		}
		lines = append(lines, line(w-2, bg, seg{st, " " + name + strings.Repeat(" ", max(1, nameW-rw.StringWidth(name)))},
			seg{st, "‹ " + val + " ›"}))
	}
	btn := -1
	if m.setSel == len(displayKeys) {
		btn = 0
	}
	lines = append(lines, "", " "+buttons([]string{saveLabel()}, btn))
	return box(L("표시", "Display"), w, lines...)
}
