package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 빠른 추가: "내일 오후 3시 회의", "금요일 10:30-12 리뷰", "tomorrow 3pm lunch", "10/12 하루 종일 휴가" 같은 한 줄을
// 날짜·시각·길이·제목으로 나눈다. 날짜가 없으면 base(고른 날), 시각이 없으면 하루 종일, 길이가 없으면 1시간.
// 못 읽은 나머지는 모두 제목이다. 규칙은 흔한 표현만 다룬다(애매하면 폼으로 넘겨 고친다).

type quickResult struct {
	e       event
	hasTime bool
}

var (
	// 날짜
	qRelKo   = regexp.MustCompile(`(^|\s)(오늘|내일|모레|글피)(\s|$)`)
	qRelEn   = regexp.MustCompile(`(?i)(^|\s)(today|tonight|tomorrow|tmr)(\s|$)`)
	qWdKo    = regexp.MustCompile(`(^|\s)(다음\s*주|담주|이번\s*주)?\s*([월화수목금토일])요일(에)?(\s|$)`)
	qWdEn    = regexp.MustCompile(`(?i)(^|\s)(next\s+|this\s+)?(mon|tue|wed|thu|fri|sat|sun)[a-z]*(\s|$)`)
	qMDKo    = regexp.MustCompile(`(^|\s)(\d{1,2})월\s*(\d{1,2})일(에)?(\s|$)`)
	qDayKo   = regexp.MustCompile(`(^|\s)(\d{1,2})일(에)?(\s|$)`)
	qISO     = regexp.MustCompile(`(^|\s)(\d{4})-(\d{1,2})-(\d{1,2})(\s|$)`)
	qSlash   = regexp.MustCompile(`(^|\s)(\d{1,2})/(\d{1,2})(\s|$)`)
	qMonthEn = regexp.MustCompile(`(?i)(^|\s)(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\.?\s+(\d{1,2})(st|nd|rd|th)?(\s|$)`)
	qAllDay  = regexp.MustCompile(`(?i)(^|\s)(하루\s*종일|종일|all[\s-]?day)(\s|$)`)
	// 시각: 오후 3시 반, 3시 30분, 15:00, 3pm, 3:30pm. 범위 "3-5시", "15:00~16:30", "3pm-4pm"
	qTime  = `(?:(?:오전|오후)\s*)?\d{1,2}(?::\d{2}|\s*시(?:\s*\d{1,2}\s*분|\s*반)?)?(?:\s*(?:am|pm|a|p))?`
	qRange = regexp.MustCompile(`(?i)(^|\s)(?:at\s+)?(` + qTime + `)(?:\s*[-~]\s*(` + qTime + `))?(에|부터)?(\s|$)`)
	// 길이: 1시간, 30분, 1시간 30분, 2h, 90m, for 2 hours
	qDur = regexp.MustCompile(`(?i)(^|\s)(?:for\s+)?(?:(\d+)\s*(?:시간|h|hr|hrs|hours?)(?:\s*(\d+)\s*(?:분|m|min|mins|minutes?))?|(\d+)\s*(?:분|m|min|mins|minutes?))(\s*동안)?(\s|$)`)
)

// cutOut은 s에서 loc 구간을 지우고 앞뒤 공백을 하나로 남긴다.
func cutOut(s string, loc []int) string {
	return strings.TrimSpace(s[:loc[0]] + " " + s[loc[1]:])
}

func parseQuick(in string, base, today time.Time) quickResult {
	s := " " + strings.TrimSpace(in) + " "
	date := base
	if g := qISO.FindStringSubmatchIndex(s); g != nil {
		y, _ := strconv.Atoi(s[g[4]:g[5]])
		mo, _ := strconv.Atoi(s[g[6]:g[7]])
		d, _ := strconv.Atoi(s[g[8]:g[9]])
		date, s = time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local), " "+cutOut(s, []int{g[0], g[1]})+" "
	} else if g := qMDKo.FindStringSubmatchIndex(s); g != nil {
		mo, _ := strconv.Atoi(s[g[4]:g[5]])
		d, _ := strconv.Atoi(s[g[6]:g[7]])
		date, s = nextDate(today, time.Month(mo), d), " "+cutOut(s, []int{g[0], g[1]})+" "
	} else if g := qMonthEn.FindStringSubmatchIndex(s); g != nil {
		mo := monthIndex(s[g[4]:g[5]])
		d, _ := strconv.Atoi(s[g[6]:g[7]])
		date, s = nextDate(today, time.Month(mo), d), " "+cutOut(s, []int{g[0], g[1]})+" "
	} else if g := qSlash.FindStringSubmatchIndex(s); g != nil {
		mo, _ := strconv.Atoi(s[g[4]:g[5]])
		d, _ := strconv.Atoi(s[g[6]:g[7]])
		date, s = nextDate(today, time.Month(mo), d), " "+cutOut(s, []int{g[0], g[1]})+" "
	}
	if g := qRelKo.FindStringSubmatchIndex(s); g != nil {
		n := map[string]int{"오늘": 0, "내일": 1, "모레": 2, "글피": 3}[s[g[4]:g[5]]]
		date, s = today.AddDate(0, 0, n), " "+cutOut(s, []int{g[0], g[1]})+" "
	} else if g := qRelEn.FindStringSubmatchIndex(s); g != nil {
		n := 0
		if w := strings.ToLower(s[g[4]:g[5]]); w == "tomorrow" || w == "tmr" {
			n = 1
		}
		date, s = today.AddDate(0, 0, n), " "+cutOut(s, []int{g[0], g[1]})+" "
	}
	if g := qWdKo.FindStringSubmatchIndex(s); g != nil {
		wd := strings.Index("일월화수목금토", s[g[6]:g[7]]) / 3
		next := g[4] >= 0 && !strings.HasPrefix(s[g[4]:g[5]], "이번")
		date, s = weekdayFrom(today, time.Weekday(wd), next), " "+cutOut(s, []int{g[0], g[1]})+" "
	} else if g := qWdEn.FindStringSubmatchIndex(s); g != nil {
		wd := map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}[strings.ToLower(s[g[6]:g[7]])]
		next := g[4] >= 0 && strings.HasPrefix(strings.ToLower(s[g[4]:g[5]]), "next")
		date, s = weekdayFrom(today, time.Weekday(wd), next), " "+cutOut(s, []int{g[0], g[1]})+" "
	} else if g := qDayKo.FindStringSubmatchIndex(s); g != nil {
		d, _ := strconv.Atoi(s[g[4]:g[5]])
		date, s = nextDate(today, today.Month(), d), " "+cutOut(s, []int{g[0], g[1]})+" "
	}
	allDay := false
	if g := qAllDay.FindStringIndex(s); g != nil {
		allDay, s = true, " "+cutOut(s, g)+" "
	}
	r := quickResult{e: event{allDay: true, start: date, end: date}}
	if !allDay {
		for _, g := range qRange.FindAllStringSubmatchIndex(s, -1) {
			from := s[g[4]:g[5]]
			if g[6] >= 0 && strings.Trim(from, "0123456789 ") == "" { // "3-5시": 끝의 단위를 시작에도
				to := strings.ToLower(s[g[6]:g[7]])
				switch {
				case strings.Contains(to, "시"):
					from += "시"
				case strings.HasSuffix(to, "pm"), strings.HasSuffix(to, "am"):
					from += to[len(to)-2:]
				}
			}
			h, mi, ok := quickClock(from)
			if !ok {
				continue
			}
			st := time.Date(date.Year(), date.Month(), date.Day(), h, mi, 0, 0, time.Local)
			et := st.Add(time.Hour)
			if g[6] >= 0 {
				eh, em, ok := quickClock(s[g[6]:g[7]])
				if ok {
					if eh < h && eh+12 <= 23 && !strings.Contains(s[g[6]:g[7]], "오전") { // "3-5시" = 15~17시
						eh += 12
					}
					et = time.Date(date.Year(), date.Month(), date.Day(), eh, em, 0, 0, time.Local)
					if !et.After(st) {
						et = et.AddDate(0, 0, 1)
					}
				}
			}
			r.e.allDay, r.e.start, r.e.end, r.hasTime = false, st, et, true
			s = " " + cutOut(s, []int{g[0], g[1]}) + " "
			break
		}
		if r.hasTime {
			if g := qDur.FindStringSubmatchIndex(s); g != nil {
				h, mi := 0, 0
				if g[4] >= 0 {
					h, _ = strconv.Atoi(s[g[4]:g[5]])
				}
				for _, k := range []int{6, 8} {
					if g[k] >= 0 {
						mi, _ = strconv.Atoi(s[g[k]:g[k+1]])
					}
				}
				if d := time.Duration(h)*time.Hour + time.Duration(mi)*time.Minute; d > 0 {
					r.e.end = r.e.start.Add(d)
					s = " " + cutOut(s, []int{g[0], g[1]}) + " "
				}
			}
		}
	}
	title := strings.TrimSpace(s)
	for _, p := range []string{"에 ", "at ", "on "} { // 남은 조사·전치사
		title = strings.TrimPrefix(title, p)
	}
	r.e.title = strings.TrimSpace(title)
	return r
}

// quickClock은 빠른 추가의 시각. 오전·오후·am·pm이 없고 1~7시면 오후로 본다("3시 회의" = 15시).
func quickClock(s string) (int, int, bool) {
	t := strings.ToLower(strings.TrimSpace(s))
	h, m, ok := parseClock(t)
	if !ok {
		return 0, 0, false
	}
	plain := !strings.ContainsAny(t, "apm:") && !strings.Contains(t, "오전") && !strings.Contains(t, "오후")
	if plain && !strings.Contains(t, "시") { // 숫자만("3")은 시각으로 보지 않는다
		return 0, 0, false
	}
	if plain && h >= 1 && h <= 7 {
		h += 12
	}
	return h, m, true
}

// nextDate는 오늘 이후 가장 가까운 mo월 d일(지났으면 내년).
func nextDate(today time.Time, mo time.Month, d int) time.Time {
	t := time.Date(today.Year(), mo, d, 0, 0, 0, 0, time.Local)
	if t.Before(today) {
		t = t.AddDate(1, 0, 0)
	}
	return t
}

// weekdayFrom은 오늘부터 가장 가까운 그 요일(오늘 포함). next면 다음 주 그 요일.
func weekdayFrom(today time.Time, wd time.Weekday, next bool) time.Time {
	d := (int(wd) - int(today.Weekday()) + 7) % 7
	if next {
		// "다음 주 월요일": 이번 주(주 시작 기준) 다음 주의 그 요일
		ws := (int(today.Weekday()) - int(weekStart) + 7) % 7
		d = 7 - ws + (int(wd)-int(weekStart)+7)%7
	}
	return today.AddDate(0, 0, d)
}

func monthIndex(s string) int {
	return strings.Index("janfebmaraprmayjunjulaugsepoctnovdec", strings.ToLower(s[:3]))/3 + 1
}
