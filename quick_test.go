package main

import (
	"testing"
	"time"
)

func TestParseQuick(t *testing.T) {
	today := time.Date(2026, 10, 4, 0, 0, 0, 0, time.Local) // 일요일
	base := time.Date(2026, 10, 7, 0, 0, 0, 0, time.Local)
	d := func(mo, day, h, mi int) time.Time {
		return time.Date(2026, time.Month(mo), day, h, mi, 0, 0, time.Local)
	}
	cases := []struct {
		in         string
		title      string
		start, end time.Time
		allDay     bool
	}{
		{"내일 오후 3시 회의", "회의", d(10, 5, 15, 0), d(10, 5, 16, 0), false},
		{"내일 3시에 회의", "회의", d(10, 5, 15, 0), d(10, 5, 16, 0), false},
		{"오전 9시 반 운동", "운동", d(10, 7, 9, 30), d(10, 7, 10, 30), false},
		{"금요일 10:30-12:00 디자인 리뷰", "디자인 리뷰", d(10, 9, 10, 30), d(10, 9, 12, 0), false},
		{"10월 12일 하루 종일 휴가", "휴가", d(10, 12, 0, 0), d(10, 12, 0, 0), true},
		{"치과", "치과", d(10, 7, 0, 0), d(10, 7, 0, 0), true},
		{"3-5시 워크숍", "워크숍", d(10, 7, 15, 0), d(10, 7, 17, 0), false},
		{"모레 14:00 2시간 세미나", "세미나", d(10, 6, 14, 0), d(10, 6, 16, 0), false},
		{"다음주 월요일 9시 주간 회의", "주간 회의", d(10, 12, 9, 0), d(10, 12, 10, 0), false},
		{"tomorrow 3pm lunch with Kim", "lunch with Kim", d(10, 5, 15, 0), d(10, 5, 16, 0), false},
		{"oct 20 all day offsite", "offsite", d(10, 20, 0, 0), d(10, 20, 0, 0), true},
		{"fri 9:30am standup for 15m", "standup", d(10, 9, 9, 30), d(10, 9, 9, 45), false},
		{"3층 회의실 점검", "3층 회의실 점검", d(10, 7, 0, 0), d(10, 7, 0, 0), true},
		{"11/3 19:00 저녁", "저녁", d(11, 3, 19, 0), d(11, 3, 20, 0), false},
	}
	for _, c := range cases {
		r := parseQuick(c.in, base, today)
		if r.e.title != c.title || !r.e.start.Equal(c.start) || !r.e.end.Equal(c.end) || r.e.allDay != c.allDay {
			t.Errorf("%q → %q %v–%v allDay=%v; want %q %v–%v %v", c.in, r.e.title, r.e.start.Format("01-02 15:04"), r.e.end.Format("01-02 15:04"), r.e.allDay,
				c.title, c.start.Format("01-02 15:04"), c.end.Format("01-02 15:04"), c.allDay)
		}
	}
}

func TestParseClock(t *testing.T) {
	for in, want := range map[string][2]int{"15:30": {15, 30}, "9:05": {9, 5}, "3pm": {15, 0}, "3:30 PM": {15, 30}, "12am": {0, 0},
		"12:15pm": {12, 15}, "오후 3:30": {15, 30}, "오전 9시": {9, 0}, "오후 3시 반": {15, 30}, "10시 20분": {10, 20}} {
		h, m, ok := parseClock(in)
		if !ok || h != want[0] || m != want[1] {
			t.Errorf("%q → %d:%02d ok=%v, want %d:%02d", in, h, m, ok, want[0], want[1])
		}
	}
	for _, bad := range []string{"25:00", "abc", "3:75"} {
		if _, _, ok := parseClock(bad); ok {
			t.Errorf("%q should fail", bad)
		}
	}
}
