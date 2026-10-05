package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 모달은 스택으로 쌓는다. esc는 언제나 맨 위 모달 하나를 닫는다(취소).
type modalKind int

const (
	mNone      modalKind = iota
	mDay                 // 날짜의 일정 목록
	mDetail              // 일정 상세
	mForm                // 일정 추가·편집
	mSpan                // 반복 일정 저장 범위
	mConfirm             // 삭제 확인
	mGoto                // 특정 월로 이동
	mSettings            // 설정 메뉴
	mCalendars           // 설정 › 캘린더 선택
	mDisplay             // 설정 › 표시
	mMove                // 일정을 다른 날로 옮기기
	mQuick               // 빠른 추가(한 줄)
	mSearch              // 검색
)

type model struct {
	cursor     time.Time
	view       int // vMonth · vWeek · vAgenda (views.go)
	today      time.Time
	db         *store
	stack      []modalKind
	daySel     int   // 일정 목록에서 선택한 행 (0 = "+ 새 일정 추가", i+1 = i번째 일정)
	detail     event // 상세로 연 일정
	detailBtn  int   // 0 편집 · 1 삭제
	confirmBtn int   // 반복 일정이면 0 이 일정만 · 1 이후 모두
	confirmErr string
	pending    event // 범위를 고르는 중인 저장
	spanBtn    int
	form       form
	gotoIn     textinput.Model
	moveIn     textinput.Model // 옮길 날짜
	gotoBtn    int             // 월 이동 창 버튼 포커스(-1 = 입력 칸)
	moveBtn    int             // 일정 이동 창 버튼 포커스(-1 = 입력 칸)
	quickIn    textinput.Model // 빠른 추가 한 줄
	quickBtn   int             // 빠른 추가 포커스: -1 입력 칸, 0 [추가], 1 [상세]
	searchIn   textinput.Model // 검색어
	searchAll  []event         // 검색 대상(열 때 읽음)
	searchSel  int
	searchErr  string
	gotoErr    string
	menuSel    int      // 설정 메뉴에서 선택한 항목
	setSel     int      // 캘린더 선택 화면에서 선택한 줄
	setFrom    int      // 설정 화면에서 [저장]으로 Tab하기 전 줄(돌아올 자리)
	setDraft   settings // 설정 화면에서 고치는 중인 사본
	width      int
	height     int
	reload     bool // 새 빌드를 감지해 종료 → 새 실행 파일로 다시 시작
}

type (
	reloadMsg  struct{}
	configMsg  struct{} // config.toml이 바뀜(파일 감시)
	refreshMsg struct{}
	changedMsg struct{} // EventKit이 알린 바깥 변경
)

// 바깥 변경은 EventKit 알림(changedMsg)으로 즉시 반영한다. 이 주기 새로고침은 알림을 놓쳤을 때를 위한 최후 수단.
func refreshTick() tea.Cmd {
	return tea.Tick(time.Hour, func(time.Time) tea.Msg { return refreshMsg{} })
}

func newModel(b backend, cfg *settings) model {
	today := dateOf(time.Now())
	m := model{cursor: today, today: today, db: newStore(b, cfg), width: 100, height: 30}
	m.gotoIn = newInput("2026-12", "", 7, 16)
	m.moveIn = newInput("2026-10-05", "", 10, 12)
	m.quickIn = newInput("", "", 200, 44)
	m.searchIn = newInput("", "", 100, 50)
	return m
}

// Init은 창 제목을 정한다(Terminal 창·탭 이름).
func (m model) Init() tea.Cmd {
	return tea.Batch(refreshTick(), tea.SetWindowTitle(windowTitle()))
}

func windowTitle() string { return L("▦ 캘린더", "▦ Calendar") }

func (m model) top() modalKind {
	if len(m.stack) == 0 {
		return mNone
	}
	return m.stack[len(m.stack)-1]
}

func (m model) push(k modalKind) model {
	m.stack = append(append([]modalKind(nil), m.stack...), k)
	return m
}

func (m model) pop() model {
	switch m.top() {
	case mGoto:
		m.gotoIn.Blur()
	case mMove:
		m.moveIn.Blur()
	case mQuick:
		m.quickIn.Blur()
	case mSearch:
		m.searchIn.Blur()
	}
	m.stack = m.stack[:len(m.stack)-1]
	return m
}

// syncToday는 날짜가 바뀌었으면(자정을 넘겨 띄워 둔 창) 오늘을 새로 잡는다. 커서가 옛 오늘에 있었으면
// 따라가서 보이는 달·주도 오늘 것으로 바뀐다. 모달이 열려 있으면 커서는 그대로 둔다(보던 일정 기준).
func (m model) syncToday() model {
	t := dateOf(time.Now())
	if t.Equal(m.today) {
		return m
	}
	if m.cursor.Equal(m.today) && m.top() == mNone {
		m.cursor = t
	}
	m.today = t
	return m
}

func (m model) move(t time.Time) model {
	m.cursor = t
	return m
}

// Update는 포커스 들어옴·나감을 처리하고 나머지는 update로 넘긴다.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.FocusMsg: // 바깥에서 바뀐 일정이 있을 수 있어 한 번 다시 읽는다(로컬 DB)
		logLine("focus in")
		m.db.invalidate()
		m = m.syncToday()
	case tea.BlurMsg:
		logLine("focus out")
	}
	return m.update(msg)
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// 크기가 바뀌면 Terminal이 기존 줄을 다시 흘려 놓아 2배 크기 줄(ESC#3/#4) 표시가 엉뚱한 줄에 남는다.
		// 바뀐 줄만 다시 그리면 그 흔적이 안 지워지므로 화면을 통째로 지우고 그린다(전체 지우기는 줄 표시도 푼다).
		return m, tea.ClearScreen
	case reloadMsg:
		m.reload = true
		return m, tea.Quit
	case refreshMsg:
		m.db.invalidate()
		m = m.syncToday()
		return m, refreshTick()
	case configMsg:
		m.db.invalidate() // config.toml도 다시 읽는다
		return m, nil
	case changedMsg:
		logLine("store changed")
		m.db.invalidate()
		return m, nil
	case tea.MouseMsg:
		if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		logLine(fmt.Sprintf("click %d,%d %v", msg.X, msg.Y, m.top()))
		return m.click(msg.X, msg.Y)
	case tea.KeyMsg:
		logKey(msg, m.top())
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.top() == mNone {
			return m.updateCal(msg)
		}
		if msg.String() == "esc" {
			return m.pop(), nil
		}
		if nm, cmd, ok := m.handleButtons(msg); ok { // Tab·←→·Enter의 버튼 그룹 규칙은 모든 모달 공통(modal.go)
			return nm, cmd
		}
		switch m.top() {
		case mDay:
			return m.updateDay(msg)
		case mDetail:
			return m.updateDetail(msg)
		case mForm:
			return m.updateForm(msg)
		case mSpan:
			return m.updateSpan(msg)
		case mConfirm:
			return m.updateConfirm(msg)
		case mGoto:
			return m.updateGoto(msg)
		case mSettings:
			return m.updateSettingMenu(msg)
		case mCalendars:
			return m.updateCalendars(msg)
		case mDisplay:
			return m.updateDisplay(msg)
		case mMove:
			return m.updateMove(msg)
		case mQuick:
			return m.updateQuick(msg)
		case mSearch:
			return m.updateSearch(msg)
		}
	}
	// 키가 아닌 메시지(커서 깜빡임)는 입력 중인 칸으로
	var cmd tea.Cmd
	switch m.top() {
	case mForm:
		if isText(m.form.focus) {
			m.form.inputs[m.form.focus], cmd = m.form.inputs[m.form.focus].Update(msg)
		}
	case mGoto:
		m.gotoIn, cmd = m.gotoIn.Update(msg)
	case mMove:
		m.moveIn, cmd = m.moveIn.Update(msg)
	case mQuick:
		m.quickIn, cmd = m.quickIn.Update(msg)
	case mSearch:
		m.searchIn, cmd = m.searchIn.Update(msg)
	}
	return m, cmd
}

func (m model) updateCal(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q":
		return m, tea.Quit
	case "left", "h":
		return m.move(m.cursor.AddDate(0, 0, -1)), nil
	case "right", "l":
		return m.move(m.cursor.AddDate(0, 0, 1)), nil
	case "up", "k":
		if m.view == vAgenda { // 목록에선 위아래가 하루
			return m.move(m.cursor.AddDate(0, 0, -1)), nil
		}
		return m.move(m.cursor.AddDate(0, 0, -7)), nil
	case "down", "j":
		if m.view == vAgenda {
			return m.move(m.cursor.AddDate(0, 0, 1)), nil
		}
		return m.move(m.cursor.AddDate(0, 0, 7)), nil
	// 터미널은 Cmd 조합을 앱에 넘기지 않아 Shift·Option(alt) 조합으로 받는다
	case "shift+up", "shift+left", "alt+up", "alt+left", "alt+b", "[", "pgup", "p", "shift+tab":
		return m.step(-1), nil
	case "shift+down", "shift+right", "alt+down", "alt+right", "alt+f", "]", "pgdown", "n", "tab":
		return m.step(1), nil
	case "v":
		return m.setView((m.view + 1) % viewCount), nil
	case "1", "2", "3": // 1 월간 · 2 주간 · 3 목록(v 순서와 같음)
		return m.setView(map[string]int{"1": vMonth, "2": vWeek, "3": vAgenda}[k.String()]), nil
	case "t":
		return m.move(m.today), nil
	case "r":
		m.db.invalidate()
		return m, nil
	case "g":
		m.gotoIn.SetValue("")
		m.gotoErr = ""
		m.gotoBtn = -1
		cmd := m.gotoIn.Focus()
		return m.push(mGoto), cmd
	case "s":
		m.menuSel = 0
		return m.push(mSettings), nil
	case "a":
		return m.openQuick()
	case "/":
		return m.openSearch()
	case "enter":
		m.daySel = 0
		return m.push(mDay), nil
	}
	return m, nil
}

func (m model) updateDay(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	evs := m.db.on(m.cursor)
	m.daySel = min(m.daySel, len(evs))
	switch k.String() {
	case "up", "k":
		m.daySel = max(0, m.daySel-1)
	case "down", "j":
		m.daySel = min(len(evs), m.daySel+1)
	case "a", "n":
		return m.openNewForm()
	case "enter":
		if m.daySel == 0 {
			return m.openNewForm()
		}
		m.detail = evs[m.daySel-1]
		m.detailBtn = 0
		return m.push(mDetail), nil
	}
	return m, nil
}

func (m model) openNewForm() (tea.Model, tea.Cmd) {
	h := 9
	if m.cursor.Equal(m.today) && time.Now().Hour() < 23 {
		h = time.Now().Hour() + 1
	}
	return m.openNewFormAt(h)
}

// openNewFormAt은 고른 날 h시에 시작하는 새 일정 폼을 연다(주간 보기 빈 시간 클릭).
func (m model) openNewFormAt(h int) (tea.Model, tea.Cmd) {
	st := m.cursor.Add(time.Duration(h) * time.Hour)
	m.form = newForm(event{start: st, end: st.Add(time.Hour), calID: m.db.defaultCal().id}, m.db.writableCals())
	cmd := m.form.setFocus(fTitle)
	return m.push(mForm), cmd
}

// 상세의 버튼: 0 편집 · 1 복제 · 2 이동 · 3 삭제 (view.go detailButtons)
func (m model) updateDetail(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.String() == "o" { // 링크 열기는 읽기 전용 일정에서도
		openURL(m.detail.link())
		return m, nil
	}
	if !m.detail.writable {
		if k.String() == "c" { // 복제는 읽기 전용도 된다(내 캘린더로)
			return m.detailAction(1)
		}
		return m, nil
	}
	n := len(detailButtons())
	switch k.String() {
	case "h":
		m.detailBtn = (m.detailBtn + n - 1) % n
	case "l":
		m.detailBtn = (m.detailBtn + 1) % n
	case "e":
		return m.detailAction(0)
	case "c":
		return m.detailAction(1)
	case "m":
		return m.detailAction(2)
	case "d", "delete", "backspace":
		return m.detailAction(3)
	case "enter":
		return m.detailAction(m.detailBtn)
	}
	return m, nil
}

func (m model) detailAction(act int) (tea.Model, tea.Cmd) {
	switch act {
	case 0:
		m.form = newForm(m.detail, m.db.writableCals())
		cmd := m.form.setFocus(fTitle)
		return m.push(mForm), cmd
	case 1: // 복제: 이 회차를 반복 없는 새 일정으로. 캘린더가 읽기 전용이면 기본 캘린더로
		e := m.detail
		e.id, e.occ, e.repeat, e.repeatText = "", time.Time{}, repNone, ""
		if !e.writable {
			e.calID = m.db.defaultCal().id
		}
		m.form = newForm(e, m.db.writableCals())
		m.form.dup = true
		cmd := m.form.setFocus(fTitle)
		return m.push(mForm), cmd
	case 2:
		m.form.dup = false // commit이 이전 복제 폼 표시를 보지 않게
		m.moveBtn = -1
		m.moveIn.SetValue(m.detail.start.Format("2006-01-02"))
		m.moveIn.CursorEnd()
		m.gotoErr = ""
		cmd := m.moveIn.Focus()
		return m.push(mMove), cmd
	case 3:
		m.confirmBtn, m.confirmErr = 0, ""
		return m.push(mConfirm), nil
	}
	return m, nil
}

// updateMove는 일정을 입력한 날로 옮긴다. 시각·길이는 그대로. 반복 일정이면 범위를 고른다.
func (m model) updateMove(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.moveBtn >= 0 {
		return m, nil
	}
	if k.String() != "enter" {
		var cmd tea.Cmd
		m.moveIn, cmd = m.moveIn.Update(k)
		m.gotoErr = ""
		return m, cmd
	}
	return m.moveEvent()
}

// moveEvent는 일정을 입력한 날로 옮긴다([이동]).
func (m model) moveEvent() (tea.Model, tea.Cmd) {
	d, err := parseDate(m.moveIn.Value())
	if err != nil {
		m.gotoErr = L("예: 2026-10-05", "e.g. 2026-10-05")
		return m, nil
	}
	e := m.detail
	off := daysBetween(e.start, d)
	e.start, e.end = e.start.AddDate(0, 0, off), e.end.AddDate(0, 0, off)
	if e.repeat != repNone {
		m.pending, m.spanBtn = e, 0
		return m.push(mSpan), nil
	}
	return m.commit(e, spanThis)
}

func (m model) updateConfirm(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	recurring := m.detail.repeat != repNone
	switch k.String() {
	case "h", "l":
		if recurring {
			m.confirmBtn = 1 - m.confirmBtn
		}
		return m, nil
	case "y", "enter":
	default:
		return m, nil
	}
	if err := m.db.b.remove(m.detail, span(m.confirmBtn)); err != nil {
		m.confirmErr = err.Error()
		return m, nil
	}
	m.db.invalidate()
	return m.pop().pop(), nil // 확인창과 상세를 닫는다(목록에서 열었으면 목록으로)
}

func (m model) updateForm(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.form
	switch k.String() {
	case "ctrl+s":
		return m.saveForm()
	case "down":
		return m, f.step(1)
	case "shift+tab", "up":
		return m, f.step(-1)
	case "enter":
		switch f.focus {
		case fSave:
			return m.saveForm()
		case fAllDay:
			f.toggleAllDay()
			return m, nil
		case fRepeat:
			f.cycleRepeat(1)
			return m, nil
		case fAlarm:
			f.cycleAlarm(1)
			return m, nil
		case fCalendar:
			f.cycleCalendar(1)
			return m, nil
		}
		return m, f.step(1)
	case " ", "left", "right":
		dir := 1
		if k.String() == "left" {
			dir = -1
		}
		switch f.focus {
		case fAllDay:
			f.toggleAllDay()
			return m, nil
		case fRepeat:
			f.cycleRepeat(dir)
			return m, nil
		case fAlarm:
			f.cycleAlarm(dir)
			return m, nil
		case fCalendar:
			f.cycleCalendar(dir)
			return m, nil
		}
	}
	if !isText(f.focus) {
		return m, nil
	}
	if f.fresh && k.Type == tea.KeyRunes {
		f.inputs[f.focus].SetValue("")
	}
	f.fresh = false
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(k)
	if f.focus == fStartDate || f.focus == fStartTime {
		f.keepDuration()
	}
	f.err = ""
	return m, cmd
}

func (m model) saveForm() (tea.Model, tea.Cmd) {
	e, err := m.form.toEvent()
	if err != nil {
		m.form.err = err.Error()
		return m, nil
	}
	// 반복 일정을 고치면 범위를 먼저 고른다
	if e.id != "" && m.form.orig.repeat != repNone {
		m.pending, m.spanBtn = e, 0
		return m.push(mSpan), nil
	}
	return m.commit(e, spanThis)
}

func (m model) updateSpan(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "h", "l":
		m.spanBtn = 1 - m.spanBtn
	case "enter":
		return m.commit(m.pending, span(m.spanBtn))
	}
	return m, nil
}

// commit은 저장하고 폼(과 범위 선택창)을 닫는다. 실패하면 폼에 오류를 띄운다.
func (m model) commit(e event, sp span) (tea.Model, tea.Cmd) {
	if m.top() == mSpan {
		m = m.pop()
	}
	id, err := m.db.b.save(e, sp)
	if err != nil {
		m.form.err = err.Error()
		return m, nil
	}
	m.db.invalidate()
	m.db.cfg.LastCalendar = e.calID
	m.db.cfg.save()
	m = m.pop() // 폼(또는 이동 창)
	// 다른 날로 만들었거나 옮겼으면 그날로 따라가서 보여준다
	if _, _, ok := m.db.find(m.cursor, id); !ok {
		m = m.move(dateOf(e.start))
	}
	x, i, ok := m.db.find(m.cursor, id)
	if m.top() == mDetail { // 상세를 새 내용으로. 못 찾으면 닫는다
		if ok && !m.form.dup {
			m.detail = x
		} else if !ok {
			m = m.pop()
		}
		return m, nil
	}
	if ok {
		m.daySel = i + 1
	}
	return m, nil
}

// parseMonth는 "2026-12", "2026.12", "202612", "2026 12", "12"(올해), "2027"(같은 달)을 받는다.
func parseMonth(s string, cur time.Time) (time.Time, bool) {
	s = strings.NewReplacer("-", " ", ".", " ", "/", " ", "년", " ", "월", " ").Replace(s)
	fs := strings.Fields(s)
	y, mo := cur.Year(), int(cur.Month())
	num := func(x string) (int, bool) { n, err := strconv.Atoi(x); return n, err == nil }
	var ok bool
	switch {
	case len(fs) == 1 && len(fs[0]) == 6:
		if y, ok = num(fs[0][:4]); ok {
			mo, ok = num(fs[0][4:])
		}
	case len(fs) == 1 && len(fs[0]) == 4:
		y, ok = num(fs[0])
	case len(fs) == 1:
		mo, ok = num(fs[0])
	case len(fs) == 2:
		if y, ok = num(fs[0]); ok {
			mo, ok = num(fs[1])
		}
	}
	if !ok || mo < 1 || mo > 12 || y < 1 {
		return time.Time{}, false
	}
	first := time.Date(y, time.Month(mo), 1, 0, 0, 0, 0, time.Local)
	return first.AddDate(0, 0, min(cur.Day(), first.AddDate(0, 1, -1).Day())-1), true
}

func (m model) updateGoto(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.gotoBtn >= 0 {
		return m, nil
	}
	if k.String() == "enter" {
		return m.goMonth()
	}
	var cmd tea.Cmd
	m.gotoIn, cmd = m.gotoIn.Update(k)
	m.gotoErr = ""
	return m, cmd
}

// goMonth는 입력한 달로 간다([이동]).
func (m model) goMonth() (tea.Model, tea.Cmd) {
	t, ok := parseMonth(m.gotoIn.Value(), m.cursor)
	if !ok {
		m.gotoErr = L("예: 2026-12, 202612, 12", "e.g. 2026-12, 202612, 12")
		return m, nil
	}
	return m.pop().move(t), nil
}

// version은 릴리스 빌드 때 -ldflags "-X main.version=v0.1.0"으로 넣는다(.github/workflows/release.yml).
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "버전을 출력하고 끝낸다")
	useMock := flag.Bool("mock", false, "EventKit 대신 메모리 목업 일정 사용(설정 파일은 읽기만 함)")
	configPath := flag.Bool("config-path", false, "설정 파일(config.toml) 절대 경로를 출력하고 끝낸다. 파일이 아직 없으면 처음 실행할 때 만들어질 경로")
	checkConfig := flag.Bool("check-config", false, "config.toml을 검사해 오류를 출력하고 끝낸다(정상이면 종료 코드 0)")
	flag.Usage = usage
	flag.Parse()
	// 경로·검사 명령은 설정을 읽기만 하고(옮기기·쓰기 없이) 권한 요청 전에 끝낸다
	path := (&settings{dir: configDir()}).configPath()
	switch {
	case *showVersion:
		fmt.Println("calendar-tui", version)
		return
	case *configPath:
		fmt.Println(path)
		return
	case *checkConfig:
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Println("아직 없음(앱을 처음 실행할 때 만들어진다):", path)
			return
		}
		if _, err := parseConfig(path); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("정상:", path)
		return
	}
	cfg := loadSettings()
	cfg.readOnly = *useMock              // 목업은 설정을 읽기만 한다(실제 창 크기·표시 설정을 덮지 않게)
	if or(cfg.Theme, "auto") == "auto" { // 터미널에 배경색을 묻는다(OSC 11). 프로그램이 입력을 잡기 전에 한 번만
		termDark = lipgloss.HasDarkBackground()
	}
	cfg.apply()
	var b backend = newMockBackend()
	if !*useMock && runtime.GOOS == "darwin" {
		ek, err := newEventKit()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		b = ek
	}
	m := newModel(b, cfg)
	if i := slices.Index(viewNames, cfg.View); i >= 0 {
		m.view = i
	}
	cfg.writeConfig(m.db.cals) // 처음이면 만들고, 계정·캘린더 목록이 바뀌었으면 갱신
	if t, err := parseDate(os.Getenv("CAL_CURSOR")); err == nil {
		m = m.move(t)
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithReportFocus(), tea.WithMouseCellMotion())
	go watchSelf(p)
	go func() {
		for range storeChanges {
			p.Send(changedMsg{})
		}
	}()
	final, err := p.Run()
	fm, _ := final.(model)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if fm.reload {
		restart(fm.cursor)
	}
}

func usage() {
	fmt.Fprint(flag.CommandLine.Output(), `calendar-tui — 터미널 월간 캘린더 (macOS 캘린더 계정을 EventKit으로 읽고 쓴다)

사용: calendar-tui [옵션]

설정:
  사용자 설정은 config.toml 하나다(언어·테마·주 시작·시각 표기·고른 요일 최소 폭, 키 입력 로그,
  계정·캘린더 표시 여부). 항목마다 의미·허용값·기본값 주석이 붙어 있다.
  위치는 calendar-tui --config-path 로 확인한다(맥 ~/Library/Application Support/calendar-tui/config.toml).
  처음 실행할 때 만들어지고 맥에 등록된 계정·캘린더 목록이 채워진다.
  [accounts]·[calendars]의 키는 ID, 값은 true(보임) / false(숨김), 줄 끝 주석이 이름이다.
  고쳐서 저장하면 실행 중인 앱이 바로 다시 읽는다. 검사: calendar-tui --check-config
  (잘못된 값이면 앱은 이전 값을 쓰고 화면 맨 아래에 오류를 띄운다. 앱 안 설정 화면도 같은 파일을 쓴다)
  마지막 보기·마지막으로 쓴 캘린더는 앱이 저절로 기억하며 같은 폴더 state.json에 있다.

옵션:
`)
	flag.PrintDefaults()
}

// openURL은 링크를 기본 브라우저로 연다.
func openURL(u string) {
	if u == "" {
		return
	}
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", u)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		c = exec.Command("xdg-open", u)
	}
	c.Start()
}

// openQuick은 빠른 추가 창을 연다. 날짜를 안 쓰면 고른 날에 만든다.
func (m model) openQuick() (tea.Model, tea.Cmd) {
	m.quickIn.SetValue("")
	m.quickIn.Placeholder = L("예: 내일 오후 3시 회의 · 금 10:30-12 리뷰", "e.g. tomorrow 3pm lunch · fri 10:30-12 review")
	m.gotoErr = ""
	m.quickBtn = -1
	cmd := m.quickIn.Focus()
	return m.push(mQuick), cmd
}

// quickEvent는 빠른 추가 입력을 일정으로. 캘린더는 마지막으로 쓴 캘린더.
func (m model) quickEvent() event {
	r := parseQuick(m.quickIn.Value(), m.cursor, m.today)
	r.e.calID = m.db.defaultCal().id
	if r.e.title == "" {
		r.e.title = L("(제목 없음)", "(No title)")
	}
	return r.e
}

// updateQuick: 입력 칸에서 enter = 바로 추가(한 줄이라). Tab·버튼 이동은 전역 버튼 그룹 규칙(modal.go).
// (처음엔 tab이 바로 폼으로 넘어갔는데 헷갈린다는 사용자 지적으로 [추가] [상세] 버튼으로 바꿈)
func (m model) updateQuick(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.quickBtn >= 0 { // 버튼에 있을 때 다른 키는 무시
		return m, nil
	}
	if k.String() == "enter" {
		return m.quickAction(0)
	}
	var cmd tea.Cmd
	m.quickIn, cmd = m.quickIn.Update(k)
	m.gotoErr = ""
	return m, cmd
}

// quickAction: 0 [추가] = 바로 저장, 1 [상세] = 해석한 내용을 채운 폼으로.
func (m model) quickAction(b int) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(m.quickIn.Value()) == "" && b == 0 {
		return m, nil
	}
	e := m.quickEvent()
	if b == 1 {
		m = m.pop()
		m.form = newForm(e, m.db.writableCals())
		cmd := m.form.setFocus(fTitle)
		return m.push(mForm), cmd
	}
	if e.calID == "" {
		m.gotoErr = L("쓸 수 있는 캘린더가 없습니다", "No writable calendar")
		return m, nil
	}
	return m.commit(e, spanThis)
}

// setView는 보기를 바꾸고 기억한다(state.json).
func (m model) setView(v int) model {
	m.view = v
	m.db.cfg.View = viewNames[v]
	m.db.cfg.save()
	return m
}
