package main

import (
	"errors"
	"strconv"
	"time"
)

// mockBackend는 메모리 목업. EventKit이 없는 환경(윈도우 등)과 `-mock` 실행에서 쓴다.
type mockBackend struct {
	masters map[string]*mockEvent
	loaded  map[string]bool // 목업을 채운 달 "2006-01"
	nextID  int
}

type mockEvent struct {
	event
	exdates map[string]bool // 빠진 회차 "2006-01-02"
	until   time.Time       // 0이 아니면 이날까지만 반복
}

const mockCal = "mock"

func newMockBackend() *mockBackend {
	b := &mockBackend{masters: map[string]*mockEvent{}, loaded: map[string]bool{}}
	b.add(event{title: "주간 회의", repeat: repWeekly,
		start: time.Date(2026, 1, 5, 10, 0, 0, 0, time.Local), end: time.Date(2026, 1, 5, 11, 0, 0, 0, time.Local)})
	b.add(event{title: "월세 납부", allDay: true, repeat: repMonthly,
		start: time.Date(2026, 1, 25, 0, 0, 0, 0, time.Local), end: time.Date(2026, 1, 25, 0, 0, 0, 0, time.Local)})
	return b
}

func (b *mockBackend) add(e event) string {
	b.nextID++
	e.id = strconv.Itoa(b.nextID)
	e.calID, e.writable = mockCal, true
	b.masters[e.id] = &mockEvent{event: e, exdates: map[string]bool{}}
	return e.id
}

func (b *mockBackend) calendars() []calendar {
	return []calendar{{id: mockCal, title: "목업", source: "로컬", sourceID: "local", color: "#7aa2c8", writable: true}}
}

func (b *mockBackend) events(from, to time.Time) ([]event, error) {
	for t := dateOf(from); t.Before(to); t = addMonths(t, 1) {
		b.loadMonth(t)
	}
	var out []event
	for _, m := range b.masters {
		sd := dateOf(m.start)
		span := daysBetween(sd, m.lastDay())
		for c := dateOf(from).AddDate(0, 0, -span); c.Before(to); c = c.AddDate(0, 0, 1) {
			if c.Before(sd) || !m.repeat.matches(sd, c) || m.exdates[c.Format("2006-01-02")] ||
				(!m.until.IsZero() && c.After(m.until)) {
				continue
			}
			e := m.event
			off := daysBetween(sd, c)
			e.start, e.end = m.start.AddDate(0, 0, off), m.end.AddDate(0, 0, off)
			e.occ = e.start
			out = append(out, e)
		}
	}
	return out, nil
}

// split은 반복 일정을 회차 occ 직전에서 끊는다. occ가 첫 회차면 끊을 게 없어 false.
func (m *mockEvent) split(occ time.Time) bool {
	if !dateOf(occ).After(dateOf(m.start)) {
		return false
	}
	m.until = dateOf(occ).AddDate(0, 0, -1)
	return true
}

func (b *mockBackend) save(e event, sp span) (string, error) {
	if e.id == "" {
		return b.add(e), nil
	}
	m := b.masters[e.id]
	if m == nil {
		return "", errors.New(L("일정을 찾을 수 없습니다", "Event not found"))
	}
	switch {
	case m.repeat == repNone:
	case sp == spanThis:
		m.exdates[dateOf(e.occ).Format("2006-01-02")] = true
		e.id, e.repeat = "", repNone
		return b.add(e), nil
	case m.split(e.occ):
		e.id = ""
		return b.add(e), nil
	}
	if e.repeat == repCustom {
		e.repeat = m.repeat
	}
	ex := m.exdates
	*m = mockEvent{event: e, exdates: ex}
	m.calID, m.writable = mockCal, true
	return e.id, nil
}

func (b *mockBackend) remove(e event, sp span) error {
	m := b.masters[e.id]
	if m == nil {
		return errors.New(L("일정을 찾을 수 없습니다", "Event not found"))
	}
	switch {
	case m.repeat == repNone:
	case sp == spanThis:
		m.exdates[dateOf(e.occ).Format("2006-01-02")] = true
		return nil
	case m.split(e.occ):
		return nil
	}
	delete(b.masters, e.id)
	return nil
}

// loadMonth는 처음 보는 달에 목업 일정을 채운다.
func (b *mockBackend) loadMonth(t time.Time) {
	mk := t.Format("2006-01")
	if b.loaded[mk] {
		return
	}
	b.loaded[mk] = true
	y, mo := t.Year(), t.Month()
	seed := int(mo) + y
	all := []struct {
		h, m  int
		title string
	}{
		{9, 0, "팀 스탠드업"}, {10, 30, "디자인 리뷰"}, {12, 0, "점심 약속"}, {14, 0, "고객 미팅"},
		{16, 0, "코드 리뷰"}, {19, 0, "운동"}, {20, 0, "저녁 식사"},
	}
	days := time.Date(y, mo+1, 0, 0, 0, 0, 0, time.Local).Day()
	for d := 1; d <= days; d++ {
		if (d+seed)%13 == 0 && d < days {
			b.add(event{title: "휴가", allDay: true,
				start: time.Date(y, mo, d, 0, 0, 0, 0, time.Local), end: time.Date(y, mo, d+1, 0, 0, 0, 0, time.Local)})
		}
		if (d*7+seed)%3 != 0 {
			continue
		}
		for i := 0; i < (d+seed)%3+1; i++ {
			a := all[(d+i*2+seed)%len(all)]
			st := time.Date(y, mo, d, a.h, a.m, 0, 0, time.Local)
			b.add(event{title: a.title, start: st, end: st.Add(time.Hour)})
		}
	}
}
