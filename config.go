package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// 설정 파일. 에이전트도 설치된 앱만 보고 찾아 고칠 수 있게 한다(github.com/zidell/agent-configuration-accessibility).
//
//	config.toml  사용자가 고르는 설정. 항목마다 설명 주석. 바뀌면 실행 중인 앱이 다시 읽는다
//	state.json   앱이 저절로 기억하는 값(창 크기·글꼴 크기·마지막 캘린더). 앱 실행기(launcher.swift)도 읽는다
//	settings.json 예전 단일 파일. 있으면 한 번 옮기고 settings.json.bak으로 남긴다
const (
	configName = "config.toml"
	stateName  = "state.json"
	legacyName = "settings.json"
)

// configDir은 설정 폴더. 맥 ~/Library/Application Support/calendar-tui, 윈도우 %APPDATA%\calendar-tui, 리눅스 ~/.config/calendar-tui.
func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "calendar-tui")
}

func (c *settings) configPath() string { return filepath.Join(c.dir, configName) }

// loadSettings는 state.json·config.toml을 읽는다. 예전 settings.json만 있으면 옮긴다.
func loadSettings() *settings {
	c := &settings{dir: configDir(), userConfig: userConfig{FocusWidth: defaultFocusWidth}}
	if c.dir == "" {
		return c
	}
	if b, err := os.ReadFile(filepath.Join(c.dir, stateName)); err == nil {
		json.Unmarshal(b, c)
	}
	legacy := filepath.Join(c.dir, legacyName)
	if _, err := os.Stat(c.configPath()); os.IsNotExist(err) {
		if b, err := os.ReadFile(legacy); err == nil {
			var old struct {
				HiddenSources, HiddenCalendars []string
			}
			json.Unmarshal(b, &old)
			json.Unmarshal(b, c) // 창 크기 등은 state.json으로
			c.HiddenSources, c.HiddenCalendars = old.HiddenSources, old.HiddenCalendars
			c.save()
			c.writeConfig(nil) // 캘린더 목록은 앱이 뜬 뒤 다시 채운다(main.go)
			os.Rename(legacy, legacy+".bak")
		}
	}
	c.reload()
	return c
}

// save는 앱이 기억하는 값을 state.json에 쓴다(창 크기가 바뀔 때 등). config.toml은 건드리지 않는다.
func (c *settings) save() {
	if c.dir == "" || c.readOnly {
		return
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	writeAtomic(filepath.Join(c.dir, stateName), b)
}

// tomlConfig는 config.toml의 모양. 표시 여부 값 true = 보임, false = 숨김.
type tomlConfig struct {
	Display struct {
		Language   string `toml:"language"`
		Theme      string `toml:"theme"`
		WeekStart  string `toml:"week_start"`
		TimeFormat string `toml:"time_format"`
		FocusWidth *int   `toml:"focus_min_width"`
	} `toml:"display"`
	Debug struct {
		KeyLog bool `toml:"key_log"`
	} `toml:"debug"`
	Accounts  map[string]bool `toml:"accounts"`
	Calendars map[string]bool `toml:"calendars"`
}

const defaultFocusWidth = 30

// userConfig는 config.toml에서 읽은 사용자 설정.
type userConfig struct {
	Language, Theme, WeekStart, TimeFormat string
	FocusWidth                             int  // 좁을 때 커서 열 최소 폭(영문 글자 수). 0 = 넓히지 않음
	KeyLog                                 bool // 키 입력 로그(진단용)
	HiddenSources, HiddenCalendars         []string
}

// 표시 설정의 허용값. 첫 값이 기본값.
var displayChoices = map[string][]string{
	"language":    {"auto", "ko", "en"},
	"theme":       {"auto", "dark", "light"},
	"week_start":  {"sunday", "monday"},
	"time_format": {"24h", "12h"},
}

// parseConfig는 config.toml을 읽는다. 모르는 항목·잘못된 타입·허용값 밖의 값은 오류.
func parseConfig(path string) (userConfig, error) {
	var t tomlConfig
	var u userConfig
	md, err := toml.DecodeFile(path, &t)
	if err != nil {
		return u, fmt.Errorf("%s: %v", configName, err)
	}
	if un := md.Undecoded(); len(un) > 0 {
		keys := make([]string, len(un))
		for i, k := range un {
			keys[i] = k.String()
		}
		return u, fmt.Errorf("%s: %s (%s)", configName, L("모르는 항목 ", "unknown key ")+strings.Join(keys, ", "),
			L("허용: [display]·[debug]·[accounts]·[calendars]", "allowed: [display], [debug], [accounts], [calendars]"))
	}
	pick := func(key, v string) (string, error) {
		ch := displayChoices[key]
		if v == "" {
			return ch[0], nil
		}
		if !slices.Contains(ch, v) {
			return "", fmt.Errorf("%s: display.%s = %q (%s %s)", configName, key, v, L("허용값", "allowed"), strings.Join(ch, " / "))
		}
		return v, nil
	}
	d := t.Display
	for _, x := range []struct {
		key string
		v   *string
		out *string
	}{{"language", &d.Language, &u.Language}, {"theme", &d.Theme, &u.Theme},
		{"week_start", &d.WeekStart, &u.WeekStart}, {"time_format", &d.TimeFormat, &u.TimeFormat}} {
		if *x.out, err = pick(x.key, *x.v); err != nil {
			return u, err
		}
	}
	u.FocusWidth = defaultFocusWidth
	if fw := d.FocusWidth; fw != nil {
		if *fw != 0 && (*fw < focusFloor || *fw > 60) {
			return u, fmt.Errorf("%s: display.focus_min_width = %d (%s)", configName, *fw,
				L("허용값 0(넓히지 않음) 또는 12~60", "allowed 0 (off) or 12-60"))
		}
		u.FocusWidth = *fw
	}
	hidden := func(m map[string]bool) []string {
		var out []string
		for id, show := range m {
			if !show {
				out = append(out, id)
			}
		}
		slices.Sort(out)
		return out
	}
	u.HiddenSources, u.HiddenCalendars = hidden(t.Accounts), hidden(t.Calendars)
	u.KeyLog = t.Debug.KeyLog
	return u, nil
}

// reload는 config.toml이 바뀌었으면 다시 읽는다. 잘못됐으면 이전 값을 그대로 두고 err에 적는다. 바뀌었으면 true.
func (c *settings) reload() bool {
	if c.dir == "" {
		return false
	}
	st, err := os.Stat(c.configPath())
	if err != nil {
		return false // 아직 없음. 앱이 뜬 뒤 writeConfig가 만든다
	}
	if st.ModTime().Equal(c.cfgMod) {
		return false
	}
	c.cfgMod = st.ModTime()
	u, err := parseConfig(c.configPath())
	if err != nil {
		c.err = err.Error()
		return true
	}
	c.err = ""
	c.userConfig = u
	c.apply()
	return true
}

// apply는 표시 설정을 화면에 반영한다(언어·테마·주 시작·시각 표기).
func (c *settings) apply() {
	lang = c.Language
	if lang == "auto" || lang == "" {
		lang = systemLang()
	}
	weekStart = time.Sunday
	if c.WeekStart == "monday" {
		weekStart = time.Monday
	}
	clock12 = c.TimeFormat == "12h"
	focusWidth = c.FocusWidth
	if !c.readOnly {
		setKeyLog(c.KeyLog)
	}
	applyTheme(c.Theme)
}

// writeConfig는 지금 값과 캘린더 목록으로 config.toml을 다시 쓴다(설명 주석 포함). 내용이 같으면 쓰지 않는다.
// 파일이 잘못된 상태(err)면 사용자가 고치던 것을 덮지 않도록 쓰지 않는다.
func (c *settings) writeConfig(cals []calendar) {
	if c.dir == "" || c.err != "" || c.readOnly {
		return
	}
	b := c.renderConfig(cals)
	if old, err := os.ReadFile(c.configPath()); err == nil && bytes.Equal(old, b) {
		return
	}
	writeAtomic(c.configPath(), b)
	if st, err := os.Stat(c.configPath()); err == nil {
		c.cfgMod = st.ModTime()
	}
}

func (c *settings) renderConfig(cals []calendar) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, `# calendar-tui 환경설정 (TOML, # 뒤는 주석)
#
# 위치: %s
#       어디인지 모르겠으면: calendar --config-path
# 적용: 저장하면 실행 중인 앱이 바로 다시 읽는다(윈도우는 창으로 돌아올 때). 앱을 끌 필요 없다.
# 검사: calendar --check-config  — 잘못된 값이 있으면 앱은 이전 값을 계속 쓰고 화면 맨 아래에 오류를 띄운다.
# 앱의 설정 화면(s)에서 [저장]하거나 앱이 시작할 때 캘린더 목록이 바뀌었으면 앱이 이 파일을 다시 쓴다.
#   이 안내 주석과 이름 주석은 다시 만들어지지만, 직접 단 주석은 사라진다.
# 창 크기·글꼴 크기·마지막 보기·마지막으로 쓴 캘린더는 앱이 저절로 기억하는 값이라 같은 폴더 state.json에 있다.
# 자격 증명은 없다. 캘린더 계정은 macOS 시스템 설정 → 인터넷 계정이 관리한다.

# ── 표시 ──
# 앱의 설정 화면(s → 표시)에서도 바꿀 수 있다.
[display]
# 화면 언어. "auto" = 시스템 언어(한국어면 한국어, 그 밖엔 영어) / "ko" / "en". 기본값 "auto".
language = %q
# 색. "auto" = 터미널 배경 밝기를 보고 고름 / "dark" = 어두운 배경용 / "light" = 밝은 배경용. 기본값 "auto".
theme = %q
# 한 주의 시작 요일. "sunday" / "monday". 기본값 "sunday". 월간·주간 보기의 첫 열이 바뀐다.
week_start = %q
# 시각 표기. "24h" = 15:30 / "12h" = 오후 3:30·3:30pm. 기본값 "24h". 폼에선 3:30pm·15:30 모두 받는다.
time_format = %q
# 좁을 때 고른 요일 최소 폭. 단위: 영문 글자 수(한글은 한 글자가 2). 정수 0 또는 12~60, 기본값 30.
# 월간 보기에서 일곱 열을 똑같이 나눈 폭이 이보다 좁으면, 커서가 있는 요일 열을 이 폭까지 넓히고 다른 열을 줄인다.
# 다른 열은 5자(한글 2자 + 여백) 아래로 줄이지 않고, 커서 열은 최소 12자를 확보한다. 0이면 넓히지 않는다.
# 그래도 좁으면 글꼴 크기를 줄이거나 창을 넓힌다.
focus_min_width = %d

# ── 진단 ──
[debug]
# 키 입력 로그. true면 누른 키·클릭을 ~/Library/Logs/calendar-tui/keys.log에 남긴다(24시간 지난 줄은 지움).
# 단축키가 안 먹을 때 원인을 보려는 용도. 입력한 글자(일정 제목 등)도 남으므로 기본값 false.
key_log = %t

# ── 계정 표시 여부 ──
# 키: 계정 ID(따옴표째 그대로 둔다). 값: true = 보임 / false = 숨김(그 계정의 캘린더를 모두 숨김). 기본값 true.
# 목록은 앱이 시작할 때 맥에 등록된 계정으로 채운다. 목록에 없는 계정은 보인다.
[accounts]
`, c.configPath(), or(c.Language, "auto"), or(c.Theme, "auto"), or(c.WeekStart, "sunday"), or(c.TimeFormat, "24h"), c.FocusWidth, c.KeyLog)
	type row struct{ id, name string }
	var accts, calRows []row
	seen := map[string]bool{}
	for _, x := range cals {
		if !seen[x.sourceID] {
			seen[x.sourceID] = true
			accts = append(accts, row{x.sourceID, x.source})
		}
		name := x.title + " · " + x.source
		if !x.writable {
			name += " (읽기 전용)"
		}
		calRows = append(calRows, row{x.id, name})
	}
	// 숨겨 뒀는데 지금 목록에 없는 것도 남긴다(계정이 잠깐 빠졌을 때 설정을 잃지 않게)
	keep := func(rows []row, hidden []string) []row {
		for _, id := range hidden {
			if !slices.ContainsFunc(rows, func(r row) bool { return r.id == id }) {
				rows = append(rows, row{id, "(지금 목록에 없음)"})
			}
		}
		return rows
	}
	write := func(rows []row, hidden []string) {
		for _, r := range rows {
			fmt.Fprintf(&b, "%s = %t  # %s\n", tomlKey(r.id), !slices.Contains(hidden, r.id), r.name)
		}
	}
	write(keep(accts, c.HiddenSources), c.HiddenSources)
	b.WriteString(`
# ── 캘린더 표시 여부 ──
# 키: 캘린더 ID(따옴표째 그대로 둔다). 값: true = 보임 / false = 숨김. 기본값 true. 주석은 "이름 · 계정".
# 숨긴 캘린더는 달력에 안 보이고 새 일정의 캘린더로도 고를 수 없다. 이름에 "휴일"·"holiday"가 든 캘린더는
# 일정 줄 대신 날짜 옆에 휴일 이름으로 보인다.
[calendars]
`)
	write(keep(calRows, c.HiddenCalendars), c.HiddenCalendars)
	return []byte(b.String())
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// tomlKey는 따옴표 친 TOML 키. ID에 쓰일 만한 글자만 이스케이프한다.
func tomlKey(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`).Replace(s) + `"`
}

// writeAtomic은 옆에 쓰고 이름을 바꿔, 읽는 쪽이 반쯤 쓴 파일을 보지 않게 한다.
func writeAtomic(path string, b []byte) {
	os.MkdirAll(filepath.Dir(path), 0o755)
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, path)
	}
}

// configModTime은 config.toml 수정 시각(파일 감시용). 없으면 0.
func configModTime() time.Time {
	if dir := configDir(); dir != "" {
		if st, err := os.Stat(filepath.Join(dir, configName)); err == nil {
			return st.ModTime()
		}
	}
	return time.Time{}
}
