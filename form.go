package main

import (
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// 일정 추가·편집 폼의 칸. 포커스는 이 순서대로 돈다.
type field int

const (
	fTitle field = iota
	fAllDay
	fStartDate
	fStartTime
	fEndDate
	fEndTime
	fRepeat
	fAlarm
	fCalendar
	fLocation
	fURL
	fMemo
	fSave
	fieldCount
)

// formRows는 폼의 줄 순서(줄마다 첫 칸). 그리기(view.go formModal)와 클릭 판정(mouse.go)이 같이 쓴다.
var formRows = []field{fTitle, fAllDay, fStartDate, fEndDate, fRepeat, fAlarm, fCalendar, fLocation, fURL, fMemo}

// formLabelW는 폼 왼쪽 이름 열 폭("▸ " + 이름). 클릭 판정(mouse.go)도 쓴다.
const formLabelW = 12

func isText(f field) bool {
	switch f {
	case fTitle, fStartDate, fStartTime, fEndDate, fEndTime, fLocation, fURL, fMemo:
		return true
	}
	return false
}

type form struct {
	orig       event // 고치는 일정(새 일정이면 id가 빔)
	allDay     bool
	repeat     repeatKind
	repeats    []repeatKind // 고를 수 있는 반복. 원래 사용자 지정 규칙이면 그것도 포함
	customText string       // 원래 사용자 지정 규칙의 설명
	alarm      int          // 고른 알림(분)
	alarmSet   bool         // 알림 있음
	alarmOther bool         // 원래 알림이 여럿·절대 시각: 바꾸기 전까지 그대로 둔다
	calID      string
	cals       []calendar                  // 고를 수 있는 캘린더(쓰기 가능한 것)
	inputs     [fieldCount]textinput.Model // 텍스트 칸만 쓴다
	focus      field
	fresh      bool      // 날짜·시각 칸에 막 들어옴: 첫 글자가 기존 값을 덮어쓴다
	last       time.Time // 마지막으로 유효했던 시작. 시작이 바뀐 만큼 종료를 옮기는 기준
	err        string
	dup        bool  // 복제로 연 폼(저장해도 원래 일정의 상세를 바꾸지 않는다)
	from       field // [저장]으로 Tab하기 전 칸(Tab이 돌아올 자리, modal.go)
}

func newInput(placeholder, value string, limit, width int) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = placeholder
	ti.CharLimit = limit
	ti.Width = width
	ti.SetValue(value)
	return ti
}

func newForm(e event, cals []calendar) form {
	f := form{orig: e, allDay: e.allDay, repeat: e.repeat, calID: e.calID, cals: cals,
		alarm: e.alarm, alarmSet: e.alarmSet, alarmOther: e.alarmOther}
	for r := repNone; r < repeatCount; r++ {
		f.repeats = append(f.repeats, r)
	}
	if e.repeat == repCustom {
		f.repeats = append(f.repeats, repCustom)
		f.customText = e.repeatText
	}
	if f.calID == "" && len(cals) > 0 {
		f.calID = cals[0].id
	}
	st, et := clockInput(e.start), clockInput(e.end)
	if e.allDay { // 하루 종일을 끄면 쓸 기본 시각
		nine := time.Date(2000, 1, 1, 9, 0, 0, 0, time.Local)
		st, et = clockInput(nine), clockInput(nine.Add(time.Hour))
	}
	f.inputs[fTitle] = newInput(L("제목", "Title"), e.title, 80, 34)
	f.inputs[fStartDate] = newInput("2026-10-04", e.start.Format("2006-01-02"), 10, 10)
	cw := 5
	if clock12 {
		cw = 7 // "12:30pm"
	}
	f.inputs[fStartTime] = newInput(st, st, 8, cw)
	f.inputs[fEndDate] = newInput("2026-10-04", e.end.Format("2006-01-02"), 10, 10)
	f.inputs[fEndTime] = newInput(et, et, 8, cw)
	f.inputs[fLocation] = newInput(L("장소", "Location"), e.location, 80, 34)
	f.inputs[fURL] = newInput("https://", e.url, 500, 34)
	f.inputs[fMemo] = newInput(L("메모", "Notes"), e.memo, 200, 34)
	f.last, _ = f.startAt()
	return f
}

// fields는 지금 보이는 칸. 하루 종일이면 시각 칸을 건너뛴다.
func (f *form) fields() []field {
	var out []field
	for i := field(0); i < fieldCount; i++ {
		if f.allDay && (i == fStartTime || i == fEndTime) {
			continue
		}
		out = append(out, i)
	}
	return out
}

func (f *form) setFocus(to field) tea.Cmd {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
	f.focus = to
	f.fresh = to == fStartDate || to == fStartTime || to == fEndDate || to == fEndTime
	if isText(to) {
		f.inputs[to].CursorEnd()
		return f.inputs[to].Focus()
	}
	return nil
}

func (f *form) step(d int) tea.Cmd {
	fs := f.fields()
	idx := 0
	for i, x := range fs {
		if x == f.focus {
			idx = i
		}
	}
	return f.setFocus(fs[(idx+d+len(fs))%len(fs)])
}

func (f *form) cycleRepeat(d int) {
	i := 0
	for j, r := range f.repeats {
		if r == f.repeat {
			i = j
		}
	}
	f.repeat = f.repeats[(i+d+len(f.repeats))%len(f.repeats)]
}

func (f *form) repeatName(base time.Time) string {
	if f.repeat == repCustom && f.customText != "" {
		return f.customText
	}
	return f.repeat.label(base)
}

func (f *form) cycleCalendar(d int) {
	if len(f.cals) == 0 {
		return
	}
	i := 0
	for j, c := range f.cals {
		if c.id == f.calID {
			i = j
		}
	}
	f.calID = f.cals[(i+d+len(f.cals))%len(f.cals)].id
}

// alarms는 지금 고를 수 있는 알림(하루 종일 여부에 따라 다름).
func (f *form) alarms() []int {
	if f.allDay {
		return allDayAlarms
	}
	return timedAlarms
}

// cycleAlarm은 없음 → 선택지들 → 없음 순으로 돈다. 원래 사용자 지정이었으면 그것도 한 자리.
func (f *form) cycleAlarm(d int) {
	opts := f.alarms()
	n := len(opts) + 1 // 0 = 없음
	i := 0
	if f.alarmSet {
		for j, a := range opts {
			if a == f.alarm {
				i = j + 1
			}
		}
	}
	i = (i + d + n) % n
	f.alarmOther = false
	f.alarmSet = i > 0
	if i > 0 {
		f.alarm = opts[i-1]
	}
}

func (f *form) alarmName() string {
	switch {
	case f.alarmOther:
		return L("여러 개·사용자 지정", "Multiple / custom")
	case f.alarmSet:
		return alarmLabel(f.alarm, f.allDay)
	}
	return L("없음", "None")
}

func (f *form) value(fd field) string { return strings.TrimSpace(f.inputs[fd].Value()) }

func (f *form) toggleAllDay() {
	f.allDay = !f.allDay
	if f.alarmSet && !f.alarmOther { // 하루 종일 여부가 바뀌면 알림 선택지가 달라진다: 비슷한 것으로
		if f.allDay {
			f.alarm = allDayAlarms[1]
		} else {
			f.alarm = -10
		}
	}
	f.last, _ = f.startAt()
}

// at은 날짜 칸과 시각 칸을 합친 시각. 하루 종일이면 날짜만 본다.
func (f *form) at(df, tf field) (time.Time, bool) {
	d, err := parseDate(f.value(df))
	if err != nil {
		return d, false
	}
	if f.allDay {
		return d, true
	}
	h, mi, ok := parseClock(f.value(tf))
	if !ok {
		return d, false
	}
	return time.Date(d.Year(), d.Month(), d.Day(), h, mi, 0, 0, time.Local), true
}

func (f *form) startAt() (time.Time, bool) { return f.at(fStartDate, fStartTime) }

// keepDuration은 시작이 바뀐 만큼 종료도 옮겨 길이를 유지한다(구글 캘린더와 같은 동작).
func (f *form) keepDuration() {
	st, ok := f.startAt()
	if !ok || st.Equal(f.last) {
		return
	}
	if et, ok := f.at(fEndDate, fEndTime); ok && !f.last.IsZero() {
		if f.allDay {
			et = et.AddDate(0, 0, daysBetween(f.last, st))
		} else {
			et = et.Add(st.Sub(f.last))
			f.inputs[fEndTime].SetValue(clockInput(et))
		}
		f.inputs[fEndDate].SetValue(et.Format("2006-01-02"))
	}
	f.last = st
}

func parseDate(s string) (time.Time, error) {
	s = strings.NewReplacer(".", "-", "/", "-").Replace(strings.TrimSpace(s))
	return time.ParseInLocation("2006-1-2", s, time.Local)
}

func (f *form) toEvent() (event, error) {
	e := event{id: f.orig.id, occ: f.orig.occ, title: f.value(fTitle), allDay: f.allDay, repeat: f.repeat,
		calID: f.calID, location: f.value(fLocation), memo: f.value(fMemo), url: f.value(fURL),
		alarm: f.alarm, alarmSet: f.alarmSet, alarmOther: f.alarmOther}
	if e.url != "" && !strings.Contains(e.url, "://") {
		e.url = "https://" + e.url
	}
	if e.title == "" {
		e.title = L("(제목 없음)", "(No title)")
	}
	sd, err := parseDate(f.value(fStartDate))
	if err != nil {
		return e, errors.New(L("시작 날짜는 2026-10-04 형식으로 입력하세요", "Start date: use 2026-10-04"))
	}
	ed, err := parseDate(f.value(fEndDate))
	if err != nil {
		return e, errors.New(L("종료 날짜는 2026-10-04 형식으로 입력하세요", "End date: use 2026-10-04"))
	}
	if f.allDay {
		if ed.Before(sd) {
			return e, errors.New(L("종료 날짜가 시작 날짜보다 빠릅니다", "End date is before start date"))
		}
		e.start, e.end = sd, ed
		return e, nil
	}
	sh, sm, ok := parseClock(f.value(fStartTime))
	if !ok {
		return e, errors.New(L("시작 시각은 09:00·9am 형식으로 입력하세요", "Start time: use 09:00 or 9am"))
	}
	eh, em, ok := parseClock(f.value(fEndTime))
	if !ok {
		return e, errors.New(L("종료 시각은 10:00·10am 형식으로 입력하세요", "End time: use 10:00 or 10am"))
	}
	e.start = time.Date(sd.Year(), sd.Month(), sd.Day(), sh, sm, 0, 0, time.Local)
	e.end = time.Date(ed.Year(), ed.Month(), ed.Day(), eh, em, 0, 0, time.Local)
	if !e.end.After(e.start) {
		return e, errors.New(L("종료가 시작보다 늦어야 합니다", "End must be after start"))
	}
	return e, nil
}
