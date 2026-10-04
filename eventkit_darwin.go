//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework EventKit -framework Foundation -framework CoreGraphics
#include <stdlib.h>
#import <EventKit/EventKit.h>
#import <CoreGraphics/CoreGraphics.h>

static EKEventStore *store;

extern void goStoreChanged(void); // ekchange_darwin.go

// ekWatch는 캘린더 DB가 바뀌면(다른 앱·기기 동기화 포함) Go에 알린다. 메인 런루프 없이 받도록 별도 큐에서.
static void ekWatch(void) {
	[[NSNotificationCenter defaultCenter] addObserverForName:EKEventStoreChangedNotification object:store
		queue:[NSOperationQueue new] usingBlock:^(NSNotification *n) { goStoreChanged(); }];
}

// 0 거부 · 1 허용. 처음이면 권한 창을 띄우고 답을 기다린다.
static int ekInit(void) {
	store = [EKEventStore new];
	EKAuthorizationStatus st = [EKEventStore authorizationStatusForEntityType:EKEntityTypeEvent];
	if (st == EKAuthorizationStatusFullAccess) return 1;
	if (st != EKAuthorizationStatusNotDetermined) return 0;
	dispatch_semaphore_t sem = dispatch_semaphore_create(0);
	__block BOOL ok = NO;
	[store requestFullAccessToEventsWithCompletion:^(BOOL granted, NSError *e) {
		ok = granted;
		dispatch_semaphore_signal(sem);
	}];
	dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
	return ok ? 1 : 0;
}

static char *toC(id obj) {
	NSData *d = [NSJSONSerialization dataWithJSONObject:obj options:0 error:nil];
	NSString *s = d ? [[NSString alloc] initWithData:d encoding:NSUTF8StringEncoding] : @"null";
	return strdup(s.UTF8String);
}

static NSString *hexColor(CGColorRef c) {
	if (!c) return @"";
	CGColorSpaceRef rgb = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
	CGColorRef m = CGColorCreateCopyByMatchingToColorSpace(rgb, kCGRenderingIntentDefault, c, NULL);
	CGColorSpaceRelease(rgb);
	if (!m) return @"";
	const CGFloat *p = CGColorGetComponents(m);
	NSString *s = [NSString stringWithFormat:@"#%02x%02x%02x", (int)(p[0] * 255), (int)(p[1] * 255), (int)(p[2] * 255)];
	CGColorRelease(m);
	return s;
}

static char *ekCalendars(void) {
	NSMutableArray *out = [NSMutableArray array];
	for (EKCalendar *c in [store calendarsForEntityType:EKEntityTypeEvent]) {
		[out addObject:@{@"id": c.calendarIdentifier, @"title": c.title ?: @"", @"source": c.source.title ?: @"", @"sourceId": c.source.sourceIdentifier ?: @"",
			@"color": hexColor(c.CGColor), @"writable": @(c.allowsContentModifications)}];
	}
	return toC(out);
}

// 1~4 매일·매주·매월·매년, 5 그 밖의 규칙(rule에 빈도·간격·요일을 담아 Go가 설명을 만든다, events.go ruleText).
// 구글은 매주 일정에도 시작 요일(BYDAY)을 넣어 보내므로, 시작일과 같은 요일·날짜·달 지정은 단순 규칙으로 본다.
static int repeatOf(EKEvent *e, NSDictionary **rule) {
	*rule = @{};
	if (!e.hasRecurrenceRules) return 0;
	EKRecurrenceRule *r = e.recurrenceRules.firstObject;
	NSCalendar *cal = NSCalendar.currentCalendar;
	NSDate *base = e.occurrenceDate ?: e.startDate;
	NSInteger wd = [cal component:NSCalendarUnitWeekday fromDate:base];
	NSInteger day = [cal component:NSCalendarUnitDay fromDate:base];
	NSInteger mon = [cal component:NSCalendarUnitMonth fromDate:base];
	BOOL days = r.daysOfTheWeek.count == 0 || (r.frequency == EKRecurrenceFrequencyWeekly && r.daysOfTheWeek.count == 1 &&
		r.daysOfTheWeek[0].dayOfTheWeek == wd && r.daysOfTheWeek[0].weekNumber == 0);
	BOOL mdays = r.daysOfTheMonth.count == 0 || (r.daysOfTheMonth.count == 1 && r.daysOfTheMonth[0].integerValue == day);
	BOOL months = r.monthsOfTheYear.count == 0 || (r.monthsOfTheYear.count == 1 && r.monthsOfTheYear[0].integerValue == mon);
	if (e.recurrenceRules.count == 1 && r.interval == 1 && days && mdays && months && r.setPositions.count == 0) {
		switch (r.frequency) {
		case EKRecurrenceFrequencyDaily: return 1;
		case EKRecurrenceFrequencyWeekly: return 2;
		case EKRecurrenceFrequencyMonthly: return 3;
		case EKRecurrenceFrequencyYearly: return 4;
		}
	}
	NSMutableArray *ds = [NSMutableArray array];
	for (EKRecurrenceDayOfWeek *d in r.daysOfTheWeek) [ds addObject:@[@(d.dayOfTheWeek - 1), @(d.weekNumber)]];
	*rule = @{@"freq": @(r.frequency), @"interval": @(r.interval), @"days": ds};
	return 5;
}

static char *ekEvents(double from, double to) {
	NSPredicate *p = [store predicateForEventsWithStartDate:[NSDate dateWithTimeIntervalSince1970:from]
		endDate:[NSDate dateWithTimeIntervalSince1970:to] calendars:nil];
	NSMutableArray *out = [NSMutableArray array];
	for (EKEvent *e in [store eventsMatchingPredicate:p]) {
		NSDictionary *rule;
		int rep = repeatOf(e, &rule);
		NSMutableArray *alarms = [NSMutableArray array];
		BOOL absolute = NO;
		for (EKAlarm *a in e.alarms) {
			if (a.absoluteDate) absolute = YES; else [alarms addObject:@(a.relativeOffset)];
		}
		[out addObject:@{@"alarms": alarms, @"alarmAbs": @(absolute), @"url": e.URL.absoluteString ?: @"", @"rule": rule, @"id": e.eventIdentifier ?: @"", @"occ": @(e.occurrenceDate.timeIntervalSince1970),
			@"start": @(e.startDate.timeIntervalSince1970), @"end": @(e.endDate.timeIntervalSince1970),
			@"allDay": @(e.allDay), @"title": e.title ?: @"", @"location": e.location ?: @"",
			@"notes": e.notes ?: @"", @"cal": e.calendar.calendarIdentifier ?: @"", @"repeat": @(rep),
			@"writable": @(e.calendar.allowsContentModifications)}];
	}
	return toC(out);
}

// find는 반복 일정이면 occ에 시작하는 회차를, 아니면 일정 자체를 찾는다.
static EKEvent *find(NSString *ident, double occ) {
	EKEvent *e = [store eventWithIdentifier:ident];
	if (!e || !e.hasRecurrenceRules) return e;
	NSDate *o = [NSDate dateWithTimeIntervalSince1970:occ];
	NSPredicate *p = [store predicateForEventsWithStartDate:[o dateByAddingTimeInterval:-1]
		endDate:[o dateByAddingTimeInterval:1] calendars:@[e.calendar]];
	for (EKEvent *x in [store eventsMatchingPredicate:p]) {
		if ([x.eventIdentifier isEqualToString:ident] && fabs(x.occurrenceDate.timeIntervalSince1970 - occ) < 1) return x;
	}
	return nil;
}

static NSString *str(NSDictionary *d, NSString *k) { id v = d[k]; return [v isKindOfClass:[NSString class]] ? v : @""; }

// 성공하면 {"id"}, 실패하면 {"err"}(빈 문자열이면 Go가 기본 문구, "notfound"면 일정 없음)
static char *ekSave(const char *js) {
	NSDictionary *d = [NSJSONSerialization JSONObjectWithData:[NSData dataWithBytes:js length:strlen(js)] options:0 error:nil];
	NSString *ident = str(d, @"id");
	EKEvent *e = ident.length ? find(ident, [d[@"occ"] doubleValue]) : [EKEvent eventWithEventStore:store];
	if (!e) return toC(@{@"err": @"notfound"});
	EKCalendar *c = str(d, @"cal").length ? [store calendarWithIdentifier:str(d, @"cal")] : store.defaultCalendarForNewEvents;
	if (c) e.calendar = c;
	e.title = str(d, @"title");
	e.allDay = [d[@"allDay"] boolValue];
	e.startDate = [NSDate dateWithTimeIntervalSince1970:[d[@"start"] doubleValue]];
	e.endDate = [NSDate dateWithTimeIntervalSince1970:[d[@"end"] doubleValue]];
	e.location = str(d, @"location").length ? str(d, @"location") : nil;
	e.notes = str(d, @"notes").length ? str(d, @"notes") : nil;
	e.URL = str(d, @"url").length ? [NSURL URLWithString:str(d, @"url")] : nil;
	if (![d[@"keepAlarms"] boolValue]) { // 폼에서 고른 알림 하나(없으면 지움). 여럿·절대 시각이면 그대로 둔다
		e.alarms = nil;
		if ([d[@"alarm"] isKindOfClass:[NSNumber class]]) [e addAlarm:[EKAlarm alarmWithRelativeOffset:[d[@"alarm"] doubleValue]]];
	}
	int rep = [d[@"repeat"] intValue]; // -1이면 반복 규칙을 건드리지 않는다
	if (rep == 0) e.recurrenceRules = nil;
	if (rep >= 1 && rep <= 4) {
		EKRecurrenceFrequency f[] = {EKRecurrenceFrequencyDaily, EKRecurrenceFrequencyWeekly,
			EKRecurrenceFrequencyMonthly, EKRecurrenceFrequencyYearly};
		e.recurrenceRules = @[[[EKRecurrenceRule alloc] initRecurrenceWithFrequency:f[rep - 1] interval:1 end:nil]];
	}
	NSError *err = nil;
	EKSpan sp = [d[@"span"] intValue] == 1 ? EKSpanFutureEvents : EKSpanThisEvent;
	if (![store saveEvent:e span:sp commit:YES error:&err]) return toC(@{@"err": err.localizedDescription ?: @"failed"});
	return toC(@{@"id": e.eventIdentifier ?: @""});
}

static char *ekRemove(const char *ident, double occ, int span) {
	EKEvent *e = find([NSString stringWithUTF8String:ident], occ);
	if (!e) return strdup("notfound");
	NSError *err = nil;
	if (![store removeEvent:e span:(span == 1 ? EKSpanFutureEvents : EKSpanThisEvent) commit:YES error:&err])
		return strdup((err.localizedDescription ?: @"failed").UTF8String);
	return strdup("");
}

// 다른 앱·동기화로 바뀐 내용을 다시 읽게 한다
static void ekRefresh(void) { [store refreshSourcesIfNecessary]; }
*/
import "C"

import (
	"encoding/json"
	"errors"
	"time"
	"unsafe"
)

type ekBackend struct{}

// newEventKit은 캘린더 권한을 확인하고(처음이면 묻고) 백엔드를 돌려준다.
func newEventKit() (backend, error) {
	if C.ekInit() != 1 {
		return nil, errors.New(L("캘린더 접근 권한이 없습니다. 시스템 설정 → 개인정보 보호 및 보안 → 캘린더에서 터미널을 '전체 접근'으로 허용하세요.",
			"No calendar access. Allow Terminal \"Full Access\" in System Settings → Privacy & Security → Calendars."))
	}
	C.ekWatch()
	return ekBackend{}, nil
}

func goJSON(p *C.char, v any) error {
	defer C.free(unsafe.Pointer(p))
	return json.Unmarshal([]byte(C.GoString(p)), v)
}

func unix(f float64) time.Time {
	return time.Unix(0, int64(f*1e9)).Local()
}

func (ekBackend) calendars() []calendar {
	var raw []struct {
		ID, Title, Source, SourceID, Color string
		Writable                           bool
	}
	if goJSON(C.ekCalendars(), &raw) != nil {
		return nil
	}
	out := make([]calendar, len(raw))
	for i, r := range raw {
		out[i] = calendar{r.ID, r.Title, r.Source, r.SourceID, r.Color, r.Writable}
	}
	return out
}

func (ekBackend) events(from, to time.Time) ([]event, error) {
	C.ekRefresh()
	var raw []struct {
		ID                     string
		Occ, Start, End        float64
		AllDay                 bool
		Title, Location, Notes string
		Cal                    string
		Repeat                 int
		URL                    string
		Alarms                 []float64
		AlarmAbs               bool
		Rule                   struct {
			Freq, Interval int
			Days           [][2]int
		}
		Writable bool
	}
	if err := goJSON(C.ekEvents(C.double(from.Unix()), C.double(to.Unix())), &raw); err != nil {
		return nil, err
	}
	out := make([]event, len(raw))
	for i, r := range raw {
		e := event{id: r.ID, occ: unix(r.Occ), title: r.Title, allDay: r.AllDay, start: unix(r.Start), end: unix(r.End),
			repeat: repeatKind(r.Repeat), calID: r.Cal, location: r.Location, memo: r.Notes, writable: r.Writable}
		e.url = r.URL
		switch {
		case r.AlarmAbs || len(r.Alarms) > 1:
			e.alarmOther = true
		case len(r.Alarms) == 1:
			e.alarm, e.alarmSet = int(r.Alarms[0]/60), true
		}
		if e.allDay { // EventKit은 마지막 날 23:59:59로 준다 → 마지막 날 0시
			e.start, e.end = dateOf(e.start), dateOf(e.end.Add(-time.Second))
			if e.end.Before(e.start) {
				e.end = e.start
			}
		}
		if e.repeat == 5 {
			var days []ruleDay
			for _, d := range r.Rule.Days {
				days = append(days, ruleDay{d[0], d[1]})
			}
			e.repeat, e.repeatText = repCustom, ruleText(r.Rule.Freq, r.Rule.Interval, days)
		}
		out[i] = e
	}
	return out, nil
}

func (ekBackend) save(e event, sp span) (string, error) {
	end := e.end
	if e.allDay { // 마지막 날 0시 → EventKit 방식(마지막 날 23:59:59)
		end = e.end.AddDate(0, 0, 1).Add(-time.Second)
	}
	rep := int(e.repeat)
	if e.repeat == repCustom || (e.id != "" && sp == spanThis) {
		rep = -1
	}
	in, _ := json.Marshal(map[string]any{
		"id": e.id, "occ": float64(e.occ.UnixNano()) / 1e9, "cal": e.calID, "title": e.title, "allDay": e.allDay,
		"start": float64(e.start.Unix()), "end": float64(end.Unix()), "location": e.location, "notes": e.memo,
		"repeat": rep, "span": int(sp), "url": e.url, "keepAlarms": e.alarmOther, "alarm": alarmArg(e),
	})
	cs := C.CString(string(in))
	defer C.free(unsafe.Pointer(cs))
	var res struct{ ID, Err string }
	if err := goJSON(C.ekSave(cs), &res); err != nil {
		return "", err
	}
	if res.Err != "" {
		return "", ekErr(res.Err, L("저장 실패", "Save failed"))
	}
	return res.ID, nil
}

// alarmArg는 저장할 알림(초). 없으면 nil.
func alarmArg(e event) any {
	if !e.alarmSet {
		return nil
	}
	return e.alarm * 60
}

func (ekBackend) remove(e event, sp span) error {
	cs := C.CString(e.id)
	defer C.free(unsafe.Pointer(cs))
	p := C.ekRemove(cs, C.double(float64(e.occ.UnixNano())/1e9), C.int(sp))
	defer C.free(unsafe.Pointer(p))
	if msg := C.GoString(p); msg != "" {
		return ekErr(msg, L("삭제 실패", "Delete failed"))
	}
	return nil
}

// ekErr는 EventKit 쪽 오류를 화면 언어로. 시스템 오류 문구(localizedDescription)는 이미 시스템 언어라 그대로 둔다.
func ekErr(msg, failed string) error {
	switch msg {
	case "notfound":
		return errors.New(L("일정을 찾을 수 없습니다", "Event not found"))
	case "failed":
		return errors.New(failed)
	}
	return errors.New(msg)
}
