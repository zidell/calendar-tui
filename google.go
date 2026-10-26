package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 구글 캘린더 API v3 백엔드. 공식 클라이언트(google.golang.org/api)는 의존성이 커서(바이너리·메모리)
// 쓰는 엔드포인트 몇 개만 net/http + JSON으로 부른다. 로그인·토큰은 googleauth.go.

const gcalAPI = "https://www.googleapis.com/calendar/v3"

// 네트워크 왕복이 화면을 멈추지 않게 받아 둔 것을 먼저 돌려주고 뒤에서 다시 받는다(stale-while-revalidate).
// 다시 받은 내용이 다르면 storeChanges로 알려 화면이 새로 읽는다(EventKit 변경 알림과 같은 길).
// 처음 보는 기간만 기다려서 받고, 달 단위로 읽으면 앞뒤 달을 미리 받아 둔다. 쓰기 뒤엔 받아 둔 일정을 버린다
// (저장한 일정을 바로 찾아 커서를 옮겨야 해서, main.go commit).
// 2026-10-26 측정(윈도우, 캘린더 8개): 달 하나 0.8초(첫 연결 4.5초), 앞뒤 1년 0.9초, 캘린더 목록 0.7초.
type googleBackend struct {
	hc  *http.Client
	api string // gcalAPI. 테스트에선 가짜 서버

	mu       sync.Mutex // 아래는 뒤에서 받는 고루틴과 같이 쓴다
	cals     []calendar
	tz       map[string]string      // 캘린더 ID → 시간대(IANA). 시간 일정의 시작·끝에 같이 보낸다(반복 일정은 필수)
	defRem   map[string][]gReminder // 캘린더 기본 알림(일정이 useDefault면 이걸 쓴다)
	calsAt   time.Time              // 캘린더 목록을 받은 때
	calsBusy bool
	cache    map[[2]int64]*gSpan // [from, to) → 받아 둔 일정
}

type gSpan struct {
	evs  []event
	at   time.Time
	busy bool
}

// refreshGap: 이보다 최근에 받은 것은 다시 받지 않는다(포커스가 잦게 오갈 때).
const refreshGap = 5 * time.Second

func notifyChanged() {
	select {
	case storeChanges <- struct{}{}:
	default:
	}
}

type gTime struct {
	Date     string `json:"date,omitempty"`
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type gReminder struct {
	Method  string `json:"method"`
	Minutes int    `json:"minutes"`
}

type gReminders struct {
	UseDefault bool        `json:"useDefault"`
	Overrides  []gReminder `json:"overrides"`
}

type gEvent struct {
	ID                string     `json:"id"`
	Summary           string     `json:"summary"`
	Description       string     `json:"description"`
	Location          string     `json:"location"`
	HangoutLink       string     `json:"hangoutLink"`
	RecurringEventID  string     `json:"recurringEventId"`
	Start             gTime      `json:"start"`
	End               gTime      `json:"end"`
	OriginalStartTime *gTime     `json:"originalStartTime"`
	Recurrence        []string   `json:"recurrence"`
	Reminders         gReminders `json:"reminders"`
	Source            *struct {
		URL string `json:"url"`
	} `json:"source"`
}

// 목록 응답에서 받을 칸만 고른다(응답 크기를 줄임).
const gEventFields = "id,summary,description,location,hangoutLink,recurringEventId,start,end,originalStartTime,recurrence,reminders,source"

// apiError는 API가 돌려준 실패. code로 404 등을 가른다.
type apiError struct {
	code int
	msg  string
}

func (e *apiError) Error() string { return e.msg }

func isNotFound(err error) bool {
	var ae *apiError
	return errors.As(err, &ae) && (ae.code == http.StatusNotFound || ae.code == http.StatusGone)
}

// do는 API를 부른다. in이 있으면 JSON 본문으로, out이 있으면 응답을 거기에 푼다.
func (g *googleBackend) do(method, path string, q url.Values, in, out any) error {
	u := g.api + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := g.hc.Do(req)
	if err != nil {
		return authErr(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(b, &e)
		msg := e.Error.Message
		if msg == "" {
			msg = res.Status
		}
		return &apiError{res.StatusCode, L("구글 캘린더: ", "Google Calendar: ") + msg}
	}
	if out != nil && len(b) > 0 {
		return json.Unmarshal(b, out)
	}
	return nil
}

func calPath(cal string) string { return "/calendars/" + url.PathEscape(cal) + "/events" }

func evPath(cal, id string) string { return calPath(cal) + "/" + url.PathEscape(id) }

// gCal은 캘린더 목록(calendarList)의 한 줄.
type gCal struct {
	ID               string      `json:"id"`
	Summary          string      `json:"summary"`
	SummaryOverride  string      `json:"summaryOverride"`
	BackgroundColor  string      `json:"backgroundColor"`
	AccessRole       string      `json:"accessRole"`
	Primary          bool        `json:"primary"`
	TimeZone         string      `json:"timeZone"`
	DefaultReminders []gReminder `json:"defaultReminders"`
}

// calendars는 캘린더 목록. 받아 둔 게 있으면 그걸 돌려주고 뒤에서 다시 받는다.
func (g *googleBackend) calendars() []calendar {
	g.mu.Lock()
	cals, stale := g.cals, !g.calsBusy && time.Since(g.calsAt) > refreshGap
	if cals != nil && stale {
		g.calsBusy = true
	}
	g.mu.Unlock()
	if cals == nil {
		g.fetchCals()
		g.mu.Lock()
		defer g.mu.Unlock()
		return g.cals
	}
	if stale {
		go func() {
			if g.fetchCals() {
				notifyChanged()
			}
			g.mu.Lock()
			g.calsBusy = false
			g.mu.Unlock()
		}()
	}
	return cals
}

// fetchCals는 캘린더 목록을 받는다. 실패하면(오프라인 등) 마지막 목록을 그대로 둔다. 바뀌었으면 true.
func (g *googleBackend) fetchCals() bool {
	var items []gCal
	q := url.Values{"minAccessRole": {"reader"}, "maxResults": {"250"}}
	for {
		var page struct {
			Items         []gCal `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		if g.do("GET", "/users/me/calendarList", q, nil, &page) != nil {
			return false
		}
		items = append(items, page.Items...)
		if page.NextPageToken == "" {
			break
		}
		q.Set("pageToken", page.NextPageToken)
	}
	account := ""
	for _, it := range items {
		if it.Primary {
			account = it.ID // 기본 캘린더 ID가 계정 이메일이다
		}
	}
	var out []calendar
	tz, rem := map[string]string{}, map[string][]gReminder{}
	for _, it := range items {
		title := it.SummaryOverride
		if title == "" {
			title = it.Summary
		}
		out = append(out, calendar{id: it.ID, title: title, source: "Google " + account, sourceID: "google:" + account,
			color: it.BackgroundColor, writable: it.AccessRole == "owner" || it.AccessRole == "writer"})
		tz[it.ID], rem[it.ID] = it.TimeZone, it.DefaultReminders
	}
	// 기본 캘린더 → 쓸 수 있는 것 → 이름 순(구글 목록 순서는 일정하지 않다)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.id == account) != (b.id == account) {
			return a.id == account
		}
		if a.writable != b.writable {
			return a.writable
		}
		return a.title < b.title
	})
	g.mu.Lock()
	defer g.mu.Unlock()
	changed := !reflect.DeepEqual(out, g.cals) || !reflect.DeepEqual(rem, g.defRem)
	g.cals, g.tz, g.defRem, g.calsAt = out, tz, rem, time.Now()
	return changed
}

func (g *googleBackend) tzOf(cal string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.tz[cal]
}

// dropCache는 받아 둔 일정을 버린다(쓴 뒤).
func (g *googleBackend) dropCache() {
	g.mu.Lock()
	g.cache = nil
	g.mu.Unlock()
}

// listEvents는 캘린더 하나의 [from, to) 일정을 모두 받는다. single이면 반복을 회차로 펼친 것.
func (g *googleBackend) listEvents(cal string, from, to time.Time, single bool) ([]gEvent, error) {
	q := url.Values{"timeMin": {from.Format(time.RFC3339)}, "timeMax": {to.Format(time.RFC3339)},
		"maxResults": {"2500"}, "singleEvents": {strconv.FormatBool(single)},
		"fields": {"nextPageToken,items(" + gEventFields + ")"}}
	var all []gEvent
	for {
		var page struct {
			Items         []gEvent `json:"items"`
			NextPageToken string   `json:"nextPageToken"`
		}
		if err := g.do("GET", calPath(cal), q, nil, &page); err != nil {
			return all, err
		}
		all = append(all, page.Items...)
		if page.NextPageToken == "" {
			return all, nil
		}
		q.Set("pageToken", page.NextPageToken)
	}
}

// events는 [from, to) 일정. 받아 둔 게 있으면 그걸 돌려주고 뒤에서 다시 받는다. 달 단위면 앞뒤 달도 미리 받는다.
func (g *googleBackend) events(from, to time.Time) ([]event, error) {
	key := [2]int64{from.Unix(), to.Unix()}
	g.mu.Lock()
	c := g.cache[key]
	g.mu.Unlock()
	if c != nil {
		g.revalidate(from, to, false)
	} else {
		evs, err := g.fetch(from, to)
		if err != nil {
			return evs, err
		}
		g.mu.Lock()
		if g.cache == nil {
			g.cache = map[[2]int64]*gSpan{}
		}
		c = &gSpan{evs: evs, at: time.Now()}
		g.cache[key] = c
		g.mu.Unlock()
	}
	if from.Day() == 1 && to.Equal(from.AddDate(0, 1, 0)) {
		for _, d := range []int{-1, 1} {
			f := from.AddDate(0, d, 0)
			g.revalidate(f, f.AddDate(0, 1, 0), true)
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return c.evs, nil
}

// revalidate는 그 기간을 뒤에서 다시 받는다. 받아 둔 게 없으면 prefetch일 때만 받는다(미리 받기).
// 내용이 바뀌었으면 화면에 알린다.
func (g *googleBackend) revalidate(from, to time.Time, prefetch bool) {
	key := [2]int64{from.Unix(), to.Unix()}
	g.mu.Lock()
	if g.cache == nil {
		g.cache = map[[2]int64]*gSpan{}
	}
	c := g.cache[key]
	switch {
	case c == nil && !prefetch, c != nil && (c.busy || time.Since(c.at) < refreshGap):
		g.mu.Unlock()
		return
	case c == nil:
		c = &gSpan{}
		g.cache[key] = c
	}
	c.busy = true
	g.mu.Unlock()
	go func() {
		evs, err := g.fetch(from, to)
		g.mu.Lock()
		defer g.mu.Unlock()
		c.busy = false
		if g.cache[key] != c { // 그사이 쓰기로 버려졌다
			return
		}
		if err != nil {
			if c.at.IsZero() { // 미리 받기가 실패하면 자리를 비워 나중에 기다려서 받게 한다
				delete(g.cache, key)
			}
			return
		}
		changed := !c.at.IsZero() && !reflect.DeepEqual(evs, c.evs)
		c.evs, c.at = evs, time.Now()
		if changed {
			notifyChanged()
		}
	}()
}

// fetch는 모든 캘린더를 동시에 읽는다. 반복 회차는 원본(master)의 규칙을 봐야 반복 종류를 알 수 있어,
// 회차가 있는 캘린더는 같은 기간의 원본 목록(singleEvents=false)도 받는다. 그래도 없는 원본은 하나씩 읽는다.
// 결과는 비교할 수 있게 정렬한다.
func (g *googleBackend) fetch(from, to time.Time) ([]event, error) {
	g.mu.Lock()
	cals := g.cals
	g.mu.Unlock()
	if cals == nil {
		g.fetchCals()
		g.mu.Lock()
		cals = g.cals
		g.mu.Unlock()
	}
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		out   []event
		first error
	)
	for _, c := range cals {
		wg.Add(1)
		go func(c calendar) {
			defer wg.Done()
			evs, err := g.calEvents(c, from, to)
			mu.Lock()
			defer mu.Unlock()
			out = append(out, evs...)
			if err != nil && first == nil {
				first = err
			}
		}(c)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.calID != b.calID {
			return a.calID < b.calID
		}
		return a.ref < b.ref
	})
	return out, first
}

func (g *googleBackend) calEvents(c calendar, from, to time.Time) ([]event, error) {
	items, err := g.listEvents(c.id, from, to, true)
	if err != nil {
		return nil, err
	}
	masters := map[string]gEvent{}
	need := false
	for _, x := range items {
		need = need || x.RecurringEventID != ""
	}
	if need {
		ms, err := g.listEvents(c.id, from, to, false)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			if len(m.Recurrence) > 0 {
				masters[m.ID] = m
			}
		}
	}
	out := make([]event, 0, len(items))
	for _, x := range items {
		m, ok := masters[x.RecurringEventID]
		if x.RecurringEventID != "" && !ok {
			if err := g.do("GET", evPath(c.id, x.RecurringEventID), url.Values{"fields": {"id,recurrence,start"}}, nil, &m); err == nil {
				masters[m.ID] = m
			}
		}
		out = append(out, g.toEvent(c, x, m))
	}
	return out, nil
}

// parseGTime은 구글 시각. 하루 종일이면 그날 0시(로컬).
func parseGTime(t gTime) (time.Time, bool) {
	if t.Date != "" {
		d, _ := time.ParseInLocation("2006-01-02", t.Date, time.Local)
		return d, true
	}
	d, _ := time.Parse(time.RFC3339, t.DateTime)
	return d.Local(), false
}

// toEvent는 구글 일정(회차)을 앱 일정으로. m은 반복 회차의 원본(없으면 빈 값).
func (g *googleBackend) toEvent(c calendar, x gEvent, m gEvent) event {
	e := event{id: x.ID, title: x.Summary, location: x.Location, memo: x.Description, calID: c.id, writable: c.writable}
	e.ref = c.id + "\n" + x.ID
	e.start, e.allDay = parseGTime(x.Start)
	e.end, _ = parseGTime(x.End)
	if e.allDay { // 구글은 끝 날짜를 다음 날로 준다(배타) → 마지막 날 0시
		e.end = e.end.AddDate(0, 0, -1)
		if e.end.Before(e.start) {
			e.end = e.start
		}
	}
	e.occ = e.start
	if x.OriginalStartTime != nil {
		e.occ, _ = parseGTime(*x.OriginalStartTime)
	}
	if x.RecurringEventID != "" {
		e.id = x.RecurringEventID
		base, _ := parseGTime(m.Start)
		if base.IsZero() {
			base = e.occ
		}
		e.repeat, e.repeatText = ruleKind(m.Recurrence, base)
	}
	if x.Source != nil {
		e.url = x.Source.URL
	}
	if e.url == "" {
		e.url = x.HangoutLink // 구글 Meet 링크(o로 열기)
	}
	e.alarm, e.alarmSet, e.alarmOther = g.alarmOf(c.id, x.Reminders)
	return e
}

// alarmOf는 구글 알림을 앱 알림(시작 기준 분, 음수 = 전)으로. 기본 알림이면 캘린더 기본값을 본다.
func (g *googleBackend) alarmOf(cal string, r gReminders) (alarm int, set, other bool) {
	rs := r.Overrides
	if r.UseDefault {
		g.mu.Lock()
		rs = g.defRem[cal]
		g.mu.Unlock()
	}
	switch len(rs) {
	case 0:
		return 0, false, false
	case 1:
		return -rs[0].Minutes, true, false
	}
	return 0, false, true
}

// ── 반복 규칙(RRULE) ──

var rruleFreq = map[string]repeatKind{"DAILY": repDaily, "WEEKLY": repWeekly, "MONTHLY": repMonthly, "YEARLY": repYearly}

var rruleDays = []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}

// rruleParts는 "RRULE:FREQ=WEEKLY;BYDAY=MO"의 항목들. RRULE 줄이 아니면 nil.
func rruleParts(line string) map[string]string {
	body, ok := strings.CutPrefix(line, "RRULE:")
	if !ok {
		return nil
	}
	p := map[string]string{}
	for _, kv := range strings.Split(body, ";") {
		k, v, _ := strings.Cut(kv, "=")
		p[strings.ToUpper(k)] = v
	}
	return p
}

// ruleKind는 반복 규칙을 앱의 반복 종류로. 시작일(base)과 같은 요일·날짜·달 지정은 단순 반복으로 본다
// (구글은 매주 일정에도 BYDAY를 넣는다, eventkit_darwin.go repeatOf와 같은 기준).
func ruleKind(rules []string, base time.Time) (repeatKind, string) {
	var rs []map[string]string
	for _, l := range rules {
		if p := rruleParts(l); p != nil {
			rs = append(rs, p)
		}
	}
	if len(rs) == 0 {
		return repCustom, ""
	}
	p := rs[0]
	kind, ok := rruleFreq[p["FREQ"]]
	if !ok {
		return repCustom, ""
	}
	interval, _ := strconv.Atoi(p["INTERVAL"])
	interval = max(interval, 1)
	days := strings.Split(p["BYDAY"], ",")
	if p["BYDAY"] == "" {
		days = nil
	}
	simpleDays := len(days) == 0 || (kind == repWeekly && len(days) == 1 && days[0] == rruleDays[base.Weekday()])
	simpleMDay := p["BYMONTHDAY"] == "" || p["BYMONTHDAY"] == strconv.Itoa(base.Day())
	simpleMonth := p["BYMONTH"] == "" || p["BYMONTH"] == strconv.Itoa(int(base.Month()))
	if len(rs) == 1 && interval == 1 && simpleDays && simpleMDay && simpleMonth && p["BYSETPOS"] == "" {
		return kind, ""
	}
	var rd []ruleDay
	for _, d := range days {
		code := d[max(0, len(d)-2):]
		n, _ := strconv.Atoi(strings.TrimSuffix(d, code))
		for wd, c := range rruleDays {
			if c == code {
				rd = append(rd, ruleDay{wd, n})
			}
		}
	}
	return repCustom, ruleText(int(kind-repDaily), interval, rd)
}

// simpleRule은 폼에서 고른 반복(매일~매년)의 규칙.
func simpleRule(r repeatKind) []string {
	freq := map[repeatKind]string{repDaily: "DAILY", repWeekly: "WEEKLY", repMonthly: "MONTHLY", repYearly: "YEARLY"}[r]
	return []string{"RRULE:FREQ=" + freq}
}

// withoutEnd는 RRULE 줄에서 끝(UNTIL·COUNT)을 뺀 나머지 항목 문자열.
func withoutEnd(line string) string {
	body := strings.TrimPrefix(line, "RRULE:")
	var keep []string
	for _, kv := range strings.Split(body, ";") {
		k, _, _ := strings.Cut(kv, "=")
		if k = strings.ToUpper(k); k != "UNTIL" && k != "COUNT" {
			keep = append(keep, kv)
		}
	}
	return "RRULE:" + strings.Join(keep, ";")
}

// cutRules는 반복을 회차 occ 직전에서 끝낸 규칙(이후 일정 모두 고치기·지우기). 하루 종일이면 UNTIL도 날짜로 쓴다.
func cutRules(rules []string, occ time.Time, allDay bool) []string {
	until := occ.Add(-time.Second).UTC().Format("20060102T150405Z")
	if allDay {
		until = dateOf(occ).AddDate(0, 0, -1).Format("20060102")
	}
	out := make([]string, 0, len(rules))
	for _, l := range rules {
		if rruleParts(l) != nil {
			l = withoutEnd(l) + ";UNTIL=" + until
		}
		out = append(out, l)
	}
	return out
}

// restRules는 끊은 뒤 새로 만드는 반복이 이어받을 규칙(사용자 지정 규칙을 그대로 둘 때). 횟수(COUNT)와
// 빠진 회차(EXDATE)는 새 반복에 맞지 않아 버리고, 끝 날짜(UNTIL)는 남긴다.
func restRules(rules []string) []string {
	var out []string
	for _, l := range rules {
		p := rruleParts(l)
		if p == nil {
			continue
		}
		l = withoutEnd(l)
		if u := p["UNTIL"]; u != "" {
			l += ";UNTIL=" + u
		}
		out = append(out, l)
	}
	return out
}

// ── 쓰기 ──

func splitRef(ref string) (cal, inst string) {
	cal, inst, _ = strings.Cut(ref, "\n")
	return
}

// gTimes는 앱 일정의 시작·끝을 구글 형식으로. 하루 종일 끝은 다음 날(배타).
func gTimes(e event, tz string) (gTime, gTime) {
	if e.allDay {
		return gTime{Date: e.start.Format("2006-01-02")}, gTime{Date: e.end.AddDate(0, 0, 1).Format("2006-01-02")}
	}
	return gTime{DateTime: e.start.Format(time.RFC3339), TimeZone: tz}, gTime{DateTime: e.end.Format(time.RFC3339), TimeZone: tz}
}

// apply는 폼에서 고친 칸을 받아 온 일정(raw, 모르는 칸은 그대로 둔다)에 덮는다. cur는 같은 일정을 푼 것.
func (g *googleBackend) apply(raw map[string]any, cur gEvent, e event, cal string) {
	raw["summary"], raw["location"], raw["description"] = e.title, e.location, e.memo
	tz := cur.Start.TimeZone
	if tz == "" {
		tz = g.tzOf(cal)
	}
	raw["start"], raw["end"] = gTimes(e, tz)
	if !e.alarmOther { // 알림이 바뀌었을 때만 덮는다(기본 알림을 쓰던 일정을 그대로 두려고)
		if a, set, other := g.alarmOf(cal, cur.Reminders); other || set != e.alarmSet || (set && a != e.alarm) {
			raw["reminders"] = remindersOf(e)
		}
	}
	switch {
	case e.url == "" || e.url == cur.HangoutLink: // Meet 링크는 따로 저장하지 않는다
		delete(raw, "source")
	default:
		raw["source"] = map[string]string{"title": e.url, "url": e.url}
	}
}

// remindersOf는 앱 알림을 구글 알림으로(팝업 하나, 시작 전 분). 없으면 빈 목록.
func remindersOf(e event) gReminders {
	r := gReminders{Overrides: []gReminder{}}
	if e.alarmSet {
		r.Overrides = append(r.Overrides, gReminder{"popup", max(0, -e.alarm)})
	}
	return r
}

// getRaw는 일정 하나를 모르는 칸까지 그대로(map) 받는다. 고쳐서 통째로 다시 쓰기(PUT) 위해서다.
func (g *googleBackend) getRaw(cal, id string) (map[string]any, gEvent, error) {
	var raw map[string]any
	if err := g.do("GET", evPath(cal, id), nil, nil, &raw); err != nil {
		return nil, gEvent{}, g.notFound(err)
	}
	var cur gEvent
	b, _ := json.Marshal(raw)
	json.Unmarshal(b, &cur)
	return raw, cur, nil
}

func (g *googleBackend) put(cal, id string, raw map[string]any) error {
	return g.do("PUT", evPath(cal, id), url.Values{"conferenceDataVersion": {"1"}}, raw, nil)
}

func (g *googleBackend) notFound(err error) error {
	if isNotFound(err) {
		return errors.New(L("일정을 찾을 수 없습니다", "Event not found"))
	}
	return err
}

// insert는 새 일정을 만든다. rules가 있으면 반복 일정.
func (g *googleBackend) insert(e event, rules []string) (string, error) {
	start, end := gTimes(e, g.tzOf(e.calID))
	body := map[string]any{"summary": e.title, "location": e.location, "description": e.memo,
		"start": start, "end": end, "reminders": remindersOf(e)}
	if len(rules) > 0 {
		body["recurrence"] = rules
	}
	if e.url != "" {
		body["source"] = map[string]string{"title": e.url, "url": e.url}
	}
	var out gEvent
	if err := g.do("POST", calPath(e.calID), nil, body, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// firstOcc는 occ가 반복의 첫 회차(또는 그 전)인지. 그러면 "이후 모두"는 반복 전체다.
func firstOcc(master gEvent, occ time.Time) bool {
	st, allDay := parseGTime(master.Start)
	if allDay {
		return !dateOf(occ).After(st)
	}
	return !occ.After(st)
}

// save: 새 일정은 만들고, 있던 일정은 받아서 고친 뒤 통째로 다시 쓴다(모르는 칸 — 참석자·회의 정보 등 — 보존).
// 반복 일정 "이후 모두"는 구글 API에 그 옵션이 없어, 구글 웹처럼 기존 반복을 전 회차까지로 끊고 새 반복을 만든다.
func (g *googleBackend) save(e event, sp span) (string, error) {
	defer g.dropCache()
	if e.id == "" {
		var rules []string
		if e.repeat > repNone && e.repeat < repeatCount {
			rules = simpleRule(e.repeat)
		}
		return g.insert(e, rules)
	}
	cal, inst := splitRef(e.ref)
	raw, master, err := g.getRaw(cal, e.id)
	if err != nil {
		return "", err
	}
	if len(master.Recurrence) > 0 && sp == spanThis { // 이 회차만
		if e.calID != cal {
			return "", errors.New(L("반복 일정의 한 회차만 다른 캘린더로 옮길 수는 없습니다", "Can't move a single occurrence to another calendar"))
		}
		ir, icur, err := g.getRaw(cal, inst)
		if err != nil {
			return "", err
		}
		g.apply(ir, icur, e, cal)
		return e.id, g.put(cal, inst, ir)
	}
	if len(master.Recurrence) > 0 && !firstOcc(master, e.occ) { // 이후 모두(중간 회차부터)
		rules := restRules(master.Recurrence)
		if e.repeat != repCustom {
			rules = nil
			if e.repeat != repNone {
				rules = simpleRule(e.repeat)
			}
		}
		id, err := g.insert(e, rules) // 새 반복을 먼저 만들어, 끊기가 실패해도 일정이 사라지지 않게 한다
		if err != nil {
			return "", err
		}
		_, mAll := parseGTime(master.Start)
		raw["recurrence"] = cutRules(master.Recurrence, e.occ, mAll)
		return id, g.put(cal, e.id, raw)
	}
	// 반복 없는 일정, 또는 첫 회차부터 반복 전체
	if e.calID != cal {
		if err := g.do("POST", evPath(cal, e.id)+"/move", url.Values{"destination": {e.calID}}, nil, nil); err != nil {
			return "", g.notFound(err)
		}
		cal = e.calID
	}
	g.apply(raw, master, e, cal)
	switch {
	case e.repeat == repNone:
		delete(raw, "recurrence")
	case e.repeat == repCustom:
	case len(master.Recurrence) == 0 || e.repeat != g.kindOf(master): // 종류가 같으면 끝·빠진 회차를 보존한다
		raw["recurrence"] = simpleRule(e.repeat)
	}
	return e.id, g.put(cal, e.id, raw)
}

func (g *googleBackend) kindOf(master gEvent) repeatKind {
	base, _ := parseGTime(master.Start)
	k, _ := ruleKind(master.Recurrence, base)
	return k
}

func (g *googleBackend) remove(e event, sp span) error {
	defer g.dropCache()
	cal, inst := splitRef(e.ref)
	if e.repeat == repNone {
		return g.notFound(g.do("DELETE", evPath(cal, e.id), nil, nil, nil))
	}
	if sp == spanThis { // 회차 하나를 지우면 구글이 그 회차를 취소(빠진 회차)로 둔다
		return g.notFound(g.do("DELETE", evPath(cal, inst), nil, nil, nil))
	}
	raw, master, err := g.getRaw(cal, e.id)
	if err != nil {
		return err
	}
	if len(master.Recurrence) == 0 || firstOcc(master, e.occ) {
		return g.notFound(g.do("DELETE", evPath(cal, e.id), nil, nil, nil))
	}
	_, allDay := parseGTime(master.Start)
	raw["recurrence"] = cutRules(master.Recurrence, e.occ, allDay)
	return g.put(cal, e.id, raw)
}

// googleAllDayAlarms: 구글 알림은 시작 "전" 분만 받아(0~40320) 하루 종일 일정의 "당일 오전 9시"(0시 + 9시간)를 못 쓴다.
var googleAllDayAlarms = []int{-15 * 60, -7*1440 + 9*60}
