package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 화면 언어. 한국어(ko)와 영어(en)만. config.toml [display] language = auto면 시스템 언어로 정한다.
// 문자열은 쓰는 자리에서 L("한국어", "English")로 고른다(따로 키 표를 두면 두 곳을 맞춰야 해서).
var lang = "ko"

func L(ko, en string) string {
	if lang == "en" {
		return en
	}
	return ko
}

// systemLang은 LC_ALL·LC_MESSAGES·LANG, 비었으면 macOS 언어 설정(AppleLanguages)의 첫 언어로 고른다.
func systemLang() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" && v != "C" && v != "POSIX" {
			return pickLang(v)
		}
	}
	if out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output(); err == nil {
		s := strings.Trim(strings.TrimSpace(string(out)), "()\n ")
		first, _, _ := strings.Cut(s, ",")
		return pickLang(strings.Trim(strings.TrimSpace(first), `"`))
	}
	return "en"
}

func pickLang(s string) string {
	if strings.HasPrefix(strings.ToLower(s), "ko") {
		return "ko"
	}
	return "en"
}

// 주 시작 요일(일요일·월요일)과 시각 표기(24시간·12시간). config.toml [display].
var (
	weekStart = time.Sunday
	clock12   = false
)

func weekName(wd time.Weekday) string {
	if lang == "en" {
		return wd.String()[:3]
	}
	return []string{"일", "월", "화", "수", "목", "금", "토"}[wd]
}

// monthTitle은 "2026년 10월" / "October 2026".
func monthTitle(t time.Time) string {
	if lang == "en" {
		return t.Format("January 2006")
	}
	return fmt.Sprintf("%d년 %d월", t.Year(), int(t.Month()))
}

// dayLabel은 "10월 4일 (일)" / "Sun, Oct 4".
func dayLabel(d time.Time) string {
	if lang == "en" {
		return d.Format("Mon, Jan 2")
	}
	return fmt.Sprintf("%d월 %d일 (%s)", int(d.Month()), d.Day(), weekName(d.Weekday()))
}

// clock은 시각 표기. 24시간 "15:04", 12시간 "오후 3:04" / "3:04pm".
func clock(t time.Time) string {
	if !clock12 {
		return t.Format("15:04")
	}
	if lang == "en" {
		return t.Format("3:04pm")
	}
	if t.Hour() < 12 {
		return "오전 " + t.Format("3:04")
	}
	return "오후 " + t.Format("3:04")
}

// clockInput은 폼 시각 칸에 넣을 값. 12시간이면 "3:04pm"처럼 영문 오전·오후를 쓴다(입력기 전환 없이 칠 수 있게).
func clockInput(t time.Time) string {
	if clock12 {
		return t.Format("3:04pm")
	}
	return t.Format("15:04")
}

var clockRe = regexp.MustCompile(`^(오전|오후|am|pm|a\.m\.|p\.m\.)?\s*(\d{1,2})(?:(?::|\s*시)\s*(?:(\d{1,2})\s*분?|(반))?)?\s*(오전|오후|am|pm|a|p|a\.m\.|p\.m\.)?$`)

// parseClock은 "15:04", "9:30", "3pm", "3:30 PM", "오후 3:30", "오후 3시 반" 등을 시·분으로 읽는다.
func parseClock(s string) (h, m int, ok bool) {
	g := clockRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if g == nil {
		return 0, 0, false
	}
	h, _ = strconv.Atoi(g[2])
	if g[3] != "" {
		m, _ = strconv.Atoi(g[3])
	}
	if g[4] != "" {
		m = 30
	}
	mer := g[1] + g[5]
	switch {
	case strings.HasPrefix(mer, "오후") || strings.HasPrefix(mer, "p"):
		if h < 12 {
			h += 12
		}
	case strings.HasPrefix(mer, "오전") || strings.HasPrefix(mer, "a"):
		if h == 12 {
			h = 0
		}
	}
	if h > 23 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}
