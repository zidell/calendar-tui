package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

type repeatKind int

const (
	repNone repeatKind = iota
	repDaily
	repWeekly
	repMonthly
	repYearly
	repeatCount // 폼에서 고를 수 있는 것은 여기까지
	repCustom   // 간격·요일 지정 등 위로 표현 못 하는 규칙. 읽기만 하고 그대로 둔다
)

// span은 반복 일정을 고치거나 지울 범위.
type span int

const (
	spanThis   span = iota // 이 일정만
	spanFuture             // 이후 일정 모두
)

type calendar struct {
	id       string
	title    string
	source   string // 계정 이름 (Google, iCloud …)
	sourceID string
	color    string // "#rrggbb"
	writable bool
}

// event는 일정 한 회차. 반복 일정은 회차마다 따로 온다.
type event struct {
	id         string    // 백엔드 식별자. 반복 일정은 모든 회차가 같다
	occ        time.Time // 이 회차의 원래 시작. 반복 회차를 찾는 키
	title      string
	allDay     bool
	start      time.Time
	end        time.Time // 하루 종일이면 마지막 날 0시(그날 포함)
	repeat     repeatKind
	repeatText string // repCustom일 때 설명 ("2주마다", "매주 월·수")
	calID      string
	location   string
	memo       string
	url        string
	alarm      int  // 알림: 시작 기준 분(음수 = 전). alarmSet이 false면 알림 없음
	alarmSet   bool // 알림이 하나 있고 alarm으로 표현됨
	alarmOther bool // 알림이 여럿이거나 절대 시각이라 폼에서 못 다룸. 그대로 둔다
	writable   bool
}

// 알림 선택지(시작 기준 분). 하루 종일 일정은 그날 0시 기준이라 따로 둔다.
var (
	timedAlarms  = []int{0, -5, -10, -15, -30, -60, -1440}
	allDayAlarms = []int{9 * 60, -15 * 60, -7*1440 + 9*60} // 당일 오전 9시, 전날 오전 9시, 1주 전 오전 9시
)

// alarmLabel은 알림 설명: "10분 전" / "10 min before".
func alarmLabel(min int, allDay bool) string {
	if allDay {
		switch min {
		case 9 * 60:
			return L("당일 오전 9시", "On the day, 9am")
		case -15 * 60:
			return L("전날 오전 9시", "Day before, 9am")
		case -7*1440 + 9*60:
			return L("1주 전 오전 9시", "1 week before, 9am")
		}
	}
	switch {
	case min == 0:
		return L("일정 시작 시", "At start")
	case min%1440 == 0:
		return fmt.Sprintf(L("%d일 전", "%d day(s) before"), -min/1440)
	case min%60 == 0:
		return fmt.Sprintf(L("%d시간 전", "%d hour(s) before"), -min/60)
	case min < 0:
		return fmt.Sprintf(L("%d분 전", "%d min before"), -min)
	}
	return fmt.Sprintf(L("시작 %d분 후", "%d min after start"), min)
}

// alarmText는 일정의 알림 설명. 없으면 "".
func (e event) alarmText() string {
	switch {
	case e.alarmOther:
		return L("여러 개·사용자 지정", "Multiple / custom")
	case e.alarmSet:
		return alarmLabel(e.alarm, e.allDay)
	}
	return ""
}

var urlRe = regexp.MustCompile(`https?://[^\s<>"')\]]+`)

// link는 일정의 열 수 있는 링크: URL 칸, 없으면 장소·메모 안의 첫 링크(구글 Meet·Zoom 등).
func (e event) link() string {
	if e.url != "" {
		return e.url
	}
	if u := urlRe.FindString(e.location); u != "" {
		return u
	}
	return urlRe.FindString(e.memo)
}

// storeChanges는 백엔드 바깥(다른 앱·기기 동기화)에서 일정이 바뀌었다는 신호. 버퍼 1이라 몰린 알림은 합쳐진다.
var storeChanges = make(chan struct{}, 1)

// backend는 일정 저장소. UI는 이 인터페이스만 부른다.
type backend interface {
	calendars() []calendar
	events(from, to time.Time) ([]event, error) // [from, to)에 걸치는 회차들
	save(e event, sp span) (string, error)      // e.id가 비면 새 일정. 저장된 id를 돌려준다
	remove(e event, sp span) error
}

func dateOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// daysBetween은 a→b 날짜 차이. 서머타임과 무관하게 달력 날짜로 센다.
func daysBetween(a, b time.Time) int {
	ua := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	ub := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(ub.Sub(ua).Hours() / 24)
}

// addMonths는 달만 옮기고 일은 그 달 마지막 날로 맞춘다(10/31 → 11/30).
func addMonths(t time.Time, n int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(n), 1, 0, 0, 0, 0, time.Local)
	last := first.AddDate(0, 1, -1).Day()
	return time.Date(first.Year(), first.Month(), min(t.Day(), last), 0, 0, 0, 0, time.Local)
}

// lastDay는 일정이 걸치는 마지막 날. 시간 일정이 자정에 끝나면 전날까지.
func (e *event) lastDay() time.Time {
	d := dateOf(e.end)
	if !e.allDay && e.end.Equal(d) && e.end.After(e.start) {
		d = d.AddDate(0, 0, -1)
	}
	if d.Before(dateOf(e.start)) {
		d = dateOf(e.start)
	}
	return d
}

func (e *event) on(d time.Time) bool {
	return !d.Before(dateOf(e.start)) && !d.After(e.lastDay())
}

func (r repeatKind) matches(base, d time.Time) bool {
	switch r {
	case repDaily:
		return true
	case repWeekly:
		return d.Weekday() == base.Weekday()
	case repMonthly:
		return d.Day() == base.Day()
	case repYearly:
		return d.Month() == base.Month() && d.Day() == base.Day()
	}
	return d.Equal(base)
}

// repeatLabel은 상세·목록에 쓰는 반복 설명. 짧게면 "매주", 아니면 "매주 수요일".
func (e event) repeatLabel(short bool) string {
	if e.repeat == repCustom && e.repeatText != "" {
		return e.repeatText
	}
	if short {
		return e.repeat.short()
	}
	return e.repeat.label(e.start)
}

// short는 목록에 붙이는 짧은 반복 설명.
func (r repeatKind) short() string {
	switch r {
	case repDaily:
		return L("매일", "daily")
	case repWeekly:
		return L("매주", "weekly")
	case repMonthly:
		return L("매월", "monthly")
	case repYearly:
		return L("매년", "yearly")
	}
	return L("반복", "repeats")
}

func (r repeatKind) label(base time.Time) string {
	switch r {
	case repDaily:
		return L("매일", "Every day")
	case repWeekly:
		return fmt.Sprintf(L("매주 %s요일", "Every week on %s"), L(weekName(base.Weekday()), base.Weekday().String()))
	case repMonthly:
		return fmt.Sprintf(L("매월 %d일", "Every month on day %d"), base.Day())
	case repYearly:
		if lang == "en" {
			return "Every year on " + base.Format("Jan 2")
		}
		return fmt.Sprintf("매년 %d월 %d일", int(base.Month()), base.Day())
	case repCustom:
		return L("사용자 지정", "Custom")
	}
	return L("반복 안 함", "Never")
}

// ruleDay는 반복 규칙의 요일 지정. n이 0이 아니면 "n번째(음수는 끝에서) 그 요일".
type ruleDay struct{ wd, n int }

// ruleText는 단순하지 않은 반복 규칙 설명: "2주마다 월·수" / "Every 2 weeks on Mon, Wed".
// freq 0~3 = 일·주·월·년.
func ruleText(freq, interval int, days []ruleDay) string {
	every := []string{L("매일", "Daily"), L("매주", "Weekly"), L("매월", "Monthly"), L("매년", "Yearly")}
	unitKo := []string{"일", "주", "개월", "년"}
	unitEn := []string{"days", "weeks", "months", "years"}
	if freq < 0 || freq > 3 {
		return L("사용자 지정", "Custom")
	}
	t := every[freq]
	if interval > 1 {
		t = L(fmt.Sprintf("%d%s마다", interval, unitKo[freq]), fmt.Sprintf("Every %d %s", interval, unitEn[freq]))
	}
	if len(days) == 0 {
		return t
	}
	var ds []string
	for _, d := range days {
		n := weekName(time.Weekday(d.wd))
		switch {
		case d.n == -1:
			n = L("마지막 ", "last ") + n
		case d.n != 0:
			n = L(fmt.Sprintf("%d째 ", d.n), ordinal(d.n)+" ") + n
		}
		ds = append(ds, n)
	}
	return t + L(" ", " on ") + strings.Join(ds, L("·", ", "))
}

func ordinal(n int) string {
	switch n {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	}
	return fmt.Sprintf("%dth", n)
}

// store는 백엔드 앞의 달 단위 캐시.
type store struct {
	b      backend
	cfg    *settings
	cals   []calendar
	months map[string][]event // "2006-01" → 그 달에 걸치는 회차
	err    string
}

func newStore(b backend, cfg *settings) *store {
	s := &store{b: b, cfg: cfg}
	s.invalidate()
	return s
}

// invalidate는 캐시를 비운다. 저장·삭제 뒤와 주기적 새로고침 때 부른다.
func (s *store) invalidate() {
	s.cfg.reload() // config.toml을 바깥(에이전트·편집기)에서 고쳤으면 다시 읽는다
	s.months = map[string][]event{}
	s.cals = s.b.calendars()
}

func (s *store) calendar(id string) calendar {
	for _, c := range s.cals {
		if c.id == id {
			return c
		}
	}
	return calendar{}
}

// visible은 설정에서 숨기지 않은 캘린더인지.
func (s *store) visible(c calendar) bool {
	return !s.cfg.hidden(s.cfg.HiddenSources, c.sourceID) && !s.cfg.hidden(s.cfg.HiddenCalendars, c.id)
}

// writableCals는 새 일정을 넣을 수 있는(쓰기 가능하고 보이는) 캘린더.
func (s *store) writableCals() []calendar {
	var out []calendar
	for _, c := range s.cals {
		if c.writable && s.visible(c) {
			out = append(out, c)
		}
	}
	return out
}

// defaultCal은 새 일정에 미리 고를 캘린더. 마지막으로 쓴 캘린더, 없거나 못 쓰면 첫 캘린더.
func (s *store) defaultCal() calendar {
	cals := s.writableCals()
	for _, c := range cals {
		if c.id == s.cfg.LastCalendar {
			return c
		}
	}
	if len(cals) > 0 {
		return cals[0]
	}
	return calendar{}
}

func (s *store) month(t time.Time) []event {
	mk := t.Format("2006-01")
	if evs, ok := s.months[mk]; ok {
		return evs
	}
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
	evs, err := s.b.events(first, first.AddDate(0, 1, 0))
	s.err = ""
	if err != nil {
		s.err = err.Error()
	}
	s.months[mk] = evs
	return evs
}

// isHoliday는 휴일 캘린더인지(이름으로 판단: 구글 "대한민국의 휴일", 구독 "대한민국 공휴일" 등).
func (c calendar) isHoliday() bool {
	t := strings.ToLower(c.title)
	return strings.Contains(t, "휴일") || strings.Contains(t, "holiday")
}

// holiday는 날짜 d의 휴일 이름들과 쉬는 날인지. 휴일 캘린더가 여럿이면(같은 휴일이 겹침) 목록에서 앞선 캘린더 것만 쓴다.
// 구글 휴일 캘린더는 메모에 "공휴일"/"기념일"을 적어 주므로, "기념일"로 시작하면 쉬는 날이 아닌 기념일로 본다.
func (s *store) holiday(d time.Time) (names []string, off bool) {
	for _, c := range s.cals {
		if !c.isHoliday() || !s.visible(c) {
			continue
		}
		for _, e := range s.month(d) {
			if e.calID != c.id || !e.on(d) {
				continue
			}
			names = append(names, e.title)
			if !strings.HasPrefix(e.memo, "기념일") {
				off = true
			}
		}
		if len(names) > 0 {
			return names, off
		}
	}
	return nil, false
}

// on은 날짜 d의 일정(휴일 캘린더 제외). 하루 종일 먼저, 그다음 시작 시각·제목 순.
func (s *store) on(d time.Time) []event {
	var out []event
	for _, e := range s.month(d) {
		if c := s.calendar(e.calID); e.on(d) && s.visible(c) && !c.isHoliday() {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.allDay != b.allDay {
			return a.allDay
		}
		if !a.start.Equal(b.start) {
			return a.start.Before(b.start)
		}
		return a.title < b.title
	})
	return out
}

// find는 날짜 d에서 id인 일정을 찾는다.
func (s *store) find(d time.Time, id string) (event, int, bool) {
	for i, e := range s.on(d) {
		if e.id == id {
			return e, i, true
		}
	}
	return event{}, 0, false
}
