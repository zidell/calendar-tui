package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRuleKind(t *testing.T) {
	mon := time.Date(2026, 10, 5, 10, 0, 0, 0, time.Local) // 월요일
	cases := []struct {
		rules []string
		kind  repeatKind
		text  string
	}{
		{[]string{"RRULE:FREQ=WEEKLY;BYDAY=MO"}, repWeekly, ""},
		{[]string{"RRULE:FREQ=DAILY;UNTIL=20261231"}, repDaily, ""},
		{[]string{"RRULE:FREQ=MONTHLY;BYMONTHDAY=5", "EXDATE;TZID=Asia/Seoul:20261105T100000"}, repMonthly, ""},
		{[]string{"RRULE:FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE"}, repCustom, "2주마다 월·수"},
		{[]string{"RRULE:FREQ=MONTHLY;BYDAY=-1FR"}, repCustom, "매월 마지막 금"},
		{[]string{"RRULE:FREQ=WEEKLY;BYDAY=TU"}, repCustom, "매주 화"},
	}
	for _, c := range cases {
		k, txt := ruleKind(c.rules, mon)
		if k != c.kind || txt != c.text {
			t.Errorf("%v → %v %q, want %v %q", c.rules, k, txt, c.kind, c.text)
		}
	}
}

func TestCutRules(t *testing.T) {
	occ := time.Date(2026, 10, 12, 10, 0, 0, 0, time.FixedZone("KST", 9*3600))
	got := cutRules([]string{"RRULE:FREQ=WEEKLY;COUNT=10;BYDAY=MO", "EXDATE:20261005T010000Z"}, occ, false)
	want := []string{"RRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20261012T005959Z", "EXDATE:20261005T010000Z"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("timed: %v, want %v", got, want)
	}
	day := time.Date(2026, 10, 12, 0, 0, 0, 0, time.Local)
	if got := cutRules([]string{"RRULE:FREQ=DAILY;UNTIL=20261231"}, day, true); got[0] != "RRULE:FREQ=DAILY;UNTIL=20261011" {
		t.Errorf("all-day: %v", got)
	}
	if got := restRules([]string{"RRULE:FREQ=DAILY;COUNT=5", "RRULE:FREQ=WEEKLY;UNTIL=20261231", "EXDATE:20261005"}); strings.Join(got, "|") != "RRULE:FREQ=DAILY|RRULE:FREQ=WEEKLY;UNTIL=20261231" {
		t.Errorf("rest: %v", got)
	}
}

func TestToEvent(t *testing.T) {
	g := &googleBackend{defRem: map[string][]gReminder{"c": {{"popup", 10}}}}
	c := calendar{id: "c", writable: true}
	var x, m gEvent
	json.Unmarshal([]byte(`{"id":"m_20261012T010000Z","recurringEventId":"m","summary":"회의",
		"start":{"dateTime":"2026-10-12T10:00:00+09:00"},"end":{"dateTime":"2026-10-12T11:00:00+09:00"},
		"originalStartTime":{"dateTime":"2026-10-12T10:00:00+09:00"},"reminders":{"useDefault":true},
		"hangoutLink":"https://meet.google.com/x"}`), &x)
	json.Unmarshal([]byte(`{"id":"m","recurrence":["RRULE:FREQ=WEEKLY;BYDAY=MO"],"start":{"dateTime":"2026-10-05T10:00:00+09:00"}}`), &m)
	e := g.toEvent(c, x, m)
	if e.id != "m" || e.ref != "c\nm_20261012T010000Z" || e.repeat != repWeekly || !e.alarmSet || e.alarm != -10 || e.url != "https://meet.google.com/x" {
		t.Errorf("instance: %+v", e)
	}
	json.Unmarshal([]byte(`{"id":"a","start":{"date":"2026-10-12"},"end":{"date":"2026-10-14"},"reminders":{"useDefault":false}}`), &x)
	e = g.toEvent(c, x, gEvent{})
	if !e.allDay || e.start.Format("2006-01-02") != "2026-10-12" || e.end.Format("2006-01-02") != "2026-10-13" || e.alarmSet {
		t.Errorf("all-day: %+v", e)
	}
	s, en := gTimes(e, "")
	if s.Date != "2026-10-12" || en.Date != "2026-10-14" {
		t.Errorf("all-day back: %v %v", s, en)
	}
}

// fakeCal은 요청을 기록하고 일정 하나(반복 원본 m)를 돌려주는 가짜 API.
type fakeCal struct {
	reqs   []string
	bodies map[string]map[string]any
}

func (f *fakeCal) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.Path
	f.reqs = append(f.reqs, key)
	if b, _ := io.ReadAll(r.Body); len(b) > 0 {
		var m map[string]any
		json.Unmarshal(b, &m)
		f.bodies[key] = m
	}
	switch key {
	case "GET /calendars/c/events/m":
		io.WriteString(w, `{"id":"m","summary":"회의","attendees":[{"email":"a@b.c"}],"recurrence":["RRULE:FREQ=WEEKLY;BYDAY=MO"],
			"start":{"dateTime":"2026-10-05T10:00:00+09:00","timeZone":"Asia/Seoul"},"end":{"dateTime":"2026-10-05T11:00:00+09:00","timeZone":"Asia/Seoul"},
			"reminders":{"useDefault":true}}`)
	case "POST /calendars/c/events":
		io.WriteString(w, `{"id":"n"}`)
	default:
		io.WriteString(w, `{}`)
	}
}

func TestSaveFutureSplits(t *testing.T) {
	f := &fakeCal{bodies: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	g := &googleBackend{hc: srv.Client(), api: srv.URL, tz: map[string]string{"c": "Asia/Seoul"}}
	kst := time.FixedZone("KST", 9*3600)
	occ := time.Date(2026, 10, 12, 10, 0, 0, 0, kst)
	e := event{id: "m", ref: "c\nm_20261012T010000Z", occ: occ, calID: "c", title: "새 회의", repeat: repWeekly,
		start: occ.Add(time.Hour), end: occ.Add(2 * time.Hour)}
	id, err := g.save(e, spanFuture)
	if err != nil || id != "n" {
		t.Fatalf("save: %q %v", id, err)
	}
	want := "GET /calendars/c/events/m|POST /calendars/c/events|PUT /calendars/c/events/m"
	if got := strings.Join(f.reqs, "|"); got != want {
		t.Fatalf("requests %s, want %s", got, want)
	}
	put := f.bodies["PUT /calendars/c/events/m"]
	if r := put["recurrence"].([]any)[0]; r != "RRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20261012T005959Z" {
		t.Errorf("cut rule %v", r)
	}
	if put["attendees"] == nil || put["summary"] != "회의" { // 원본은 끊기만 하고 나머지는 그대로
		t.Errorf("master changed: %v", put)
	}
	ins := f.bodies["POST /calendars/c/events"]
	if ins["summary"] != "새 회의" || ins["recurrence"].([]any)[0] != "RRULE:FREQ=WEEKLY" {
		t.Errorf("insert %v", ins)
	}
}

func TestSaveKeepsUnknownFields(t *testing.T) {
	f := &fakeCal{bodies: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	g := &googleBackend{hc: srv.Client(), api: srv.URL, tz: map[string]string{"c": "Asia/Seoul"}}
	kst := time.FixedZone("KST", 9*3600)
	occ := time.Date(2026, 10, 5, 10, 0, 0, 0, kst) // 첫 회차 → 반복 전체를 고친다
	e := event{id: "m", ref: "c\nm_20261005T010000Z", occ: occ, calID: "c", title: "이름만", repeat: repWeekly,
		start: occ, end: occ.Add(time.Hour)}
	if _, err := g.save(e, spanFuture); err != nil {
		t.Fatal(err)
	}
	put := f.bodies["PUT /calendars/c/events/m"]
	if put["summary"] != "이름만" || put["attendees"] == nil || put["recurrence"].([]any)[0] != "RRULE:FREQ=WEEKLY;BYDAY=MO" {
		t.Errorf("put %v", put)
	}
	if _, ok := put["reminders"]; !ok || put["reminders"].(map[string]any)["useDefault"] != true {
		t.Errorf("기본 알림을 건드림: %v", put["reminders"])
	}
}
