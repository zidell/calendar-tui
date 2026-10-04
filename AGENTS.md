# calendar-tui 에이전트 지침

사용법은 `README.md`, 개발 지시·UI/UX 규칙·설계 근거는 이 파일에 둔다. 새로 정한 규칙·결정은 여기에 누적한다.

## 작업 방식

README(`README.md`·`README.en.md`)에는 사용법만 둔다(설치·실행·화면·조작·설정). 빌드·테스트·개발 실행·설계는 이 파일에 둔다. 두 README는 같은 내용으로 맞춘다.

- 빌드와 실행(개발):
  ```sh
  go build -o calendar .
  ./calendar          # 맥 캘린더 계정(EventKit). 권한은 실행한 터미널 앱 기준
  ./calendar -mock    # 메모리 목업 일정(설정 파일은 읽기만). 쓰기 경로 검증은 이걸로
  go test .           # 빠른 추가·시각 해석 테스트
  ```
- 필요한 것: Go, Xcode 커맨드라인 도구(cgo로 EventKit을 부른다). 다른 OS에선 빌드 태그로 목업만 붙는다.

- 코드를 고친 뒤에는 묻지 말고 항상 `./dev.sh`를 실행해 사용자가 바로 확인할 수 있게 한다.
  - 앱이 설치돼 있으면 앱이 쓰는 바이너리 `~/Library/Application Support/calendar-tui/calendar`를 바꾼다(옆에 쓰고 `mv`로 교체 — 실행 중인 파일을 덮어쓰면 프로세스가 죽을 수 있다). 그래서 앱으로 띄운 창도 제자리에서 갱신된다. 앱 번들은 건드리지 않는다(서명이 바뀌면 Terminal 제어 권한을 다시 묻는다).
  - 앱이 떠 있지 않으면 `Calendar TUI.app`을 연다(앱이 설치돼 있지 않으면 Terminal 창을 직접 띄운다).
  - 이미 떠 있으면 앱이 새 실행 파일을 감지해 그 창에서 다시 시작한다(보던 날짜 유지, 열린 모달·입력 중 내용은 사라짐). 창을 새로 열거나 프로세스를 죽이지 않는다.
- 윈도우 교차 빌드도 깨지지 않게 유지한다: `GOOS=windows GOARCH=amd64 go vet ./... && go build -o calendar.exe .`
- EventKit 권한·입력 소스처럼 Terminal.app 기준으로 확인해야 하는 실행(테스트 바이너리, 프로브)은 `scripts/term-run.sh '명령'`으로 돌린다. 끝나면 그 창을 닫아 종료된 터미널 창이 남지 않는다. `osascript ... do script`로 창을 직접 띄우지 않는다.
- 화면 확인은 Terminal 창 스크린샷으로 한다: `screencapture -x -o -l <창 id>`(창 id는 `osascript -e 'tell application "Terminal" to get id of (first window whose name contains "— calendar —")'`). tmux는 DEC 2배 크기 줄을 그리지 못하고, 입력기를 거치지 않으므로 한글 입력 확인에도 못 쓴다.
- 사용자의 실제 구글 캘린더에 테스트 일정을 만들거나 지우지 않는다. 쓰기 경로는 `-mock`으로 검증한다.
- 성능 문제는 짐작하지 말고 잰다(키 처리·View 시간, 키당 출력 바이트). 2026-10-04 측정: Update 0.03ms, View 0.6ms, 달 바뀔 때 EventKit 조회 9ms, 키당 출력 약 3KB, 대기 중 출력 0.

## 구조

| 파일 | 역할 |
|---|---|
| `main.go` | 모델, 모달 스택, 키 처리, 실행 진입점 |
| `view.go` | 월간 달력·모달 그리기, 테마(색), 오버레이, 커서 열 넓히기(`colWidths`) |
| `views.go` | 보기 전환(월간·주간·목록), 주간 시간 격자, 목록(어젠다) 그리기와 클릭 |
| `i18n.go` | 화면 언어(`L("한국어", "English")`), 날짜·시각 표기, 시각 해석(`parseClock`) |
| `quick.go` | 빠른 추가 한 줄 해석(`parseQuick`, 테스트 `quick_test.go`) |
| `search.go` | 검색(앞뒤 1년) |
| `mouse.go` | 마우스 클릭 판정 |
| `modal.go` | 모달 버튼 그룹 키 규칙(Tab·Shift+Tab·←→·Enter·↑) 한 곳 처리 |
| `form.go` | 일정 추가·편집 폼 |
| `events.go` | 일정·캘린더 타입, `backend` 인터페이스, 달 단위 캐시(`store`), 휴일 판정 |
| `eventkit_darwin.go` | EventKit 구현체(cgo, Objective-C) |
| `mock.go` | 메모리 목업 구현체(반복·예외·분할 포함) |
| `settings.go` | 설정 메뉴, 캘린더 선택 |
| `config.go` | 설정 파일: `config.toml`(사용자 설정)·`state.json`(앱이 기억하는 값), 예전 `settings.json` 옮기기 |
| `assets/readme.txt` | 앱 번들 `Contents/Resources/readme.txt`(설치 사용자·에이전트용 설정 안내) |
| `reload_*.go` | 새 빌드 감지 → 제자리 재시작 |
| `terminal_darwin.go` | 내 Terminal 탭(tty)에 AppleScript: 글꼴 크기 읽기, 창 숨기기(`Ctrl+H`) |
| `keylog.go` | 키 입력 로그(기본 꺼짐, `config.toml` `[debug] key_log`. 입력한 글자도 남아서 공개 전 기본값을 끔) |
| `dev.sh`, `scripts/term-run.sh` | 개발용 실행 |
| `scripts/package.sh` | 앱 번들(`dist/Calendar TUI.app`) 만들기, `--install`이면 `/Applications`에 설치하고 Dock에 고정 |
| `scripts/release.sh` | 릴리스: `scripts/release.sh v0.1.1` → 확인 후 태그만 푸시. 빌드·업로드는 Actions |
| `.github/workflows/release.yml` | `v*` 태그 푸시 → macOS 러너에서 테스트·유니버설 빌드 → Releases에 `Calendar-TUI-macos.zip`. 수동 실행은 빌드만(아티팩트) |
| `docs/` | GitHub Pages(https://zidell.github.io/calendar-tui/): 소개 페이지 `index.html`, 설치 스크립트 `install.sh`(`curl … \| bash`) |
| `scripts/launcher.swift` | 앱 실행기(상주): Dock 실행 점, Dock 클릭·Cmd+Tab 때 캘린더 창 앞으로, 캘린더 종료 시 같이 종료 |
| `scripts/make-icon.swift`, `assets/icon-1024.png` | 앱 아이콘 생성기와 결과(어두운 바탕·빨간 머리띠·달력 격자·오늘 칸 흰 테두리) |

- UI는 `backend` 인터페이스(기간 조회 / 저장 / 삭제, 반복 범위 `span`)만 부른다. 나중에 Google API·CalDAV 구현체를 같은 자리에 붙인다.
- 일정은 회차 단위(`event.occ`가 회차 키). 반복 규칙 펼치기는 백엔드가 한다.

## UI/UX 규칙 (사용자 지시)

### 모달

- 모달은 스택으로 쌓는다(일정 목록 → 상세 → 편집 → 반복 범위 / 삭제 확인). 아래 화면은 흐리게 깐다.
- `esc`는 전역으로 맨 위 모달 하나를 닫는다 = 취소. 그래서 **닫기·취소 버튼은 두지 않는다.**
- **버튼 그룹 키 규칙(모든 모달 공통, 사용자 지시: 키보드만으로 리듬감 있게).** `Tab`은 다음 칸이 아니라 버튼 그룹으로 간다. 버튼 그룹에서 `Tab`을 계속 누르면 다음 버튼으로, 마지막 버튼에서 누르면 **버튼으로 오기 전 자리(그 칸·그 줄·입력 칸)로 돌아간다**(사용자 지시). `Shift+Tab`은 반대(첫 버튼에서 누르면 원래 자리로), `← →`도 버튼 사이 이동, `Enter`는 그 버튼 실행, `↑`도 원래 자리로. 버튼만 있는 모달(상세·삭제 확인·반복 범위)은 돌아갈 자리가 없어 버튼 사이를 돈다. 입력 칸이 하나뿐인 모달(빠른 추가·월 이동·일정 이동)은 입력 칸에서 `Enter`만 쳐도 첫 버튼을 실행한다. 구현은 `modal.go` 한 곳(`buttonGroup`이 모달마다 버튼 글자·포커스·실행을 정의, `handleButtons`가 키 처리). 새 모달을 만들면 `buttonGroup`에 넣는다.
- 모달이 없는 달력 화면에선 `Tab`/`Shift+Tab`이 다음/이전(월간은 달, 주간·목록은 주).
- 무언가를 바꾸는 모달은 **모두 확인 버튼을 눌러야 확정**한다(`[저장]`·`[삭제]`·`[이동]`). 바꾸는 즉시 반영하는 화면을 만들지 않는다(설정도 사본을 고치고 `[저장]`에서 반영).
- 달력 화면의 `esc`는 아무 동작도 하지 않는다(연타로 앱이 꺼지지 않게). 종료는 `q`.
- 모달 바깥(흐린 영역) 클릭 = `esc`(맨 위 모달 하나 닫기·취소). 폼도 예외 없이 닫는다(사용자 지시: 마우스로 쓸 땐 esc를 안 누르니까).
- 일정 목록은 맨 위가 `+ 새 일정 추가`이고 기본 선택. 폭은 기본 60칸, 제목이 길면 화면 안에서 가장 긴 줄에 맞춰 넓힌다(처음엔 52칸 고정이라 긴 제목이 잘렸다).
- 반복 일정은 ↻ 같은 기호 대신 제목 뒤 흐린 글씨 `(매주)`·`(매월)`. 복잡한 규칙은 `(2주마다)`·`(매주 월·수)`처럼 풀어 쓴다.
- 반복 일정 저장·삭제는 Apple 캘린더처럼 **이 일정만 / 이후 일정 모두** 두 가지.
- 폼: 칸 이동은 `↑ ↓`(`Shift+Tab`도 이전 칸). `Tab`은 위 버튼 그룹 규칙대로 `[저장]`으로.
- 폼: 시작을 바꾸면 종료도 같은 만큼 옮긴다(구글 캘린더 동작). 날짜·시각 칸에 들어가 첫 글자를 치면 기존 값을 덮어쓴다.
- 새 일정의 캘린더는 마지막으로 저장한 일정의 캘린더를 자동 선택. "기본 캘린더" 설정은 두지 않는다.
- 설정은 메뉴 구조(`s` → 항목 → 하위 화면). 항목은 "캘린더 선택"·"표시"(언어·테마·주 시작·시각 표기·좁을 때 고른 요일 최소 폭).
- 상세 버튼은 `[편집] [복제] [이동] [삭제]`(키 e·c·m·d), 링크는 `o`. 복제는 그 회차를 반복 없는 새 일정으로, 이동은 날짜만 바꾼다(반복이면 범위 창). 저장 후 일정이 다른 날로 갔으면 커서가 따라간다(`commit`).
- 빠른 추가(`a`)는 미리보기 후 `[추가]`/Enter로 확정(바로 저장하지 않음). 버튼은 `[추가] [상세]`(`[상세]` = 해석한 내용을 채운 폼). 처음엔 `Tab`이 바로 폼으로 넘어갔는데 헷갈린다는 지적으로 버튼 그룹 규칙을 따르게 바꿨다. 검색(`/`)은 결과를 고르면 상세를 검색 위에 쌓아 esc로 검색에 돌아온다.

### 색·모양

- 강조색(핑크 등)을 쓰지 않는다. 모달 테두리·선택 막대는 무채색(테두리 244, 선택 배경 250 + 검정 글씨).
- 달력 선: 256색 `235`. 이력: 240 → 236(밝다) → 234(배경 #171717보다 어두워 보임) → 235. Terminal.app(macOS 15)은 256색뿐이라 235와 236 사이 값은 없다.
- 날짜 숫자는 굵게만(역상 금지 — 위아래 줄과 붙어 보임). 일·토요일·공휴일은 빨강.
- 오늘: 칸 테두리만 날짜색으로(평일 흰색, 주말·공휴일 빨강) + "오늘". 칸 전체 역상은 쓰지 않는다(칸 안 캘린더 색 글씨가 안 보이고 선택 배경과 겹침).
- 시간 일정: 시각·제목 모두 캘린더 색 글씨. 하루 종일: 줄 전체 배경을 캘린더 색, 글씨는 검정이고 아주 어두운 색(밝기 가중합 < 102,000, 약 40%)에서만 흰색. 이력: 55% → 40%(흰 글씨가 너무 자주 나옴).
- 휴일 캘린더(이름에 "휴일"·"holiday")는 일정 줄로 찍지 않고 날짜 옆에 이름만. 공휴일은 날짜·이름 빨강, 기념일(구글 휴일 캘린더 메모가 "기념일"로 시작)은 이름만 흐리게. 휴일 캘린더가 여럿이면 목록에서 앞선 캘린더 것만 쓴다(중복 방지).
- 월 제목은 가운데, DEC 2배 크기 줄(`ESC#3`/`ESC#4`). 맨 위에 빈 줄 하나 여백.

### 키

- `Ctrl+H`는 캘린더 창만 숨긴다(사용자 요청: `Cmd+H`는 Terminal 앱 숨기기라 다른 터미널 창까지 사라진다. `Cmd+H` 자체는 Terminal이 먼저 받아 앱에 오지 않는다). 내 탭(tty)의 창을 AppleScript로 `visible false`(`terminal_darwin.go` `hideWindow`). 실행기가 Dock 클릭·Cmd+Tab 때 하는 `set index` + `activate`가 숨긴 창도 다시 보이게 해서 실행기는 고치지 않았다(2026-10-04 확인). 글자 칸에선 `Ctrl+H`가 지우기라 그대로 둔다.
- `Cmd+글자`도 단축키로 쓴다(사용자 지시: "터미널의 탈을 썼지만 단독 앱처럼"). 입력기는 Cmd 조합을 조합하지 않아 한글·구름 상태와 상관없이 먹는다. Terminal은 Cmd 조합을 앱에 안 넘기므로 실행기가 받는다: 캘린더가 포커스 들어옴·나감을 `launcher.sock`에 `focus 1/0`으로 알리고, 실행기는 그동안만 Cmd+글자(C·V·H·M 제외)를 Carbon 단축키로 등록해 받으면 `calendar.sock`에 `key a`로 넘긴다(`ipc_unix.go`, `launcher.swift` `HotKeys`). `Cmd+Q`·`Cmd+W`는 캘린더 종료, `Cmd+F` 검색, `Cmd+,` 설정, 글자 칸에선 무시. Terminal은 포커스가 바뀔 때만 알려 주므로, 이미 포커스된 창에서 시작(재설치·제자리 재시작)하면 신호가 없어 Cmd가 경고음만 냈다 → 시작할 때 내 창이 맨 앞인지 AppleScript로 물어 `FocusMsg`를 만든다(`windowIsFront`). 입력 소스 자동 전환으로 끝내 못 잡은 한글 문제(아래)의 해법이다. 입력 소스를 ABC 등 다른 입력기로 바꾸는 안은 사용자가 거부(번거로움).
- `Cmd+H`도 캘린더 창만 숨긴다: 실행기(`launcher.swift` `CmdH`)가 Terminal이 맨 앞일 때만 Carbon `RegisterEventHotKey`로 `Cmd+H`를 가로채(손쉬운 사용 권한 불필요), 맨 앞 창이 캘린더면 그 창만 `visible false`, 아니면 원래처럼 Terminal 숨기기. 다른 앱이 앞에 오면 등록을 풀어 그 앱의 `Cmd+H`는 그대로. 사용자는 `Ctrl+H`보다 손에 익은 `Cmd+H`를 눌렀다(2026-10-04).
- 한 글자 단축키를 유지한다. Cmd 조합은 터미널이 앱에 넘기지 않아 못 쓴다(달 이동을 Cmd+방향키로 하려다 Shift·Option+방향키로 바꿈).
- 보기: `v`로 월간 → 주간 → 목록. `1` `2` `3`(`Cmd+1~3`)은 월간·주간·목록으로 바로(사용자 지시: 월간이 1. 처음엔 macOS 캘린더 관례대로 목록·주간·월간이었다). `Cmd+V`는 붙여넣기라 못 씀. `[` `]`·제목 꺽쇠는 월간이면 한 달, 주간·목록이면 한 주. 목록 보기에선 `↑ ↓`도 하루. 마지막 보기는 `state.json` `view`.
- 마우스(`mouse.go`): 왼쪽 클릭만. 클릭 = 그 항목을 고르고 `enter`(키 처리 함수를 그대로 부른다). 달력 칸은 누른 줄로 가른다(`cellLines` 순서): 일정 → 바로 상세(목록 없이), 빈 곳 → 새 일정, 날짜 줄·`+N개 더` → 일정 목록(사용자 지시: enter 흉내보다 바로 열기). 월 제목 양옆 `‹` `›` → 달 이동, 가운데 제목 → 월 이동 창(`g`). 2배 크기 제목 줄의 클릭 x는 Terminal이 2배 크기 칸 단위로 준다(화면 칸의 절반. 화면 칸으로 보고 2로 나눴다가 안 눌렸다, 키 로그로 확인). 좌표는 View와 같은 계산(`layout`·`titleLayout`·`scroll`, 모달은 `overlay`의 가운데 정렬)으로 되짚고, 버튼은 그 줄에서 `buttons()` 글자를 찾아 판정한다. 휠은 쓰지 않는다(트랙패드 관성으로 달이 마구 넘어감). 클릭은 키 로그에 `click x,y`로 남는다.
- 한글 입력 상태에선 한 글자 단축키가 입력기에 붙잡혀(조합 중인 자모는 다음 키나 포커스 이동 때에야 넘어온다) 앱에 오지 않는다. 해법은 위 `Cmd+글자`이고, 앱은 입력 소스를 건드리지 않는다(2026-10-04 사용자 결정). 이력: 단축키 화면은 영문, 글자 칸은 원래 입력 소스로 앱이 자동 전환했다(Carbon TIS, `ime*.go`). 그러나 구름(세벌식 390)은 시스템엔 영문(`Gureum.system`)으로 보고되면서 내부는 한글 모드로 남아 키를 조합하는 불일치가 생겼고(키 로그로 확인), 강제 한글→영문 재전환·포커스 직후 재확인·한글 키 버리고 재전환까지 넣어도 다른 탭에 갔다 오면 재발해서 전부 걷어냈다. 프로그램으로 입력 소스를 바꾸는 것 자체가 구름 내부 모드와 어긋나는 원인일 수 있다(추정). 조합 중 마지막 글자에서 Enter를 치면 확정된 글자만 오고 Enter는 사라져 한 번 더 쳐야 한다(띄어쓰기는 확정 뒤 그대로 온다). 원시 바이트 프로브로 확인: 터미널 모드(대체 화면·마우스·포커스·붙여넣기)와 무관하고, 구름·맥 기본 입력기, Terminal.app·Ghostty, Claude Code 모두 같다(2026-10-04). 앱엔 조합 중 글자가 오지 않아(터미널이 덧그리기만 함) 브라우저처럼 조합 값을 읽는 우회도, 확정만 온 것을 Enter로 보는 우회도(입력 소스 전환·창 이동 확정과 구별 불가) 못 한다.

## 기술 결정과 근거

### 왜 터미널 TUI인가

목표는 빠른 개발 + 가벼움(SSH·원격은 목적 아님). 개인용 "작은 도구 여러 개를 각자 터미널 창에 띄워 쓰는" 구상의 첫 도구다.

- 도구당 증가분: Terminal 창 9MB + 프로세스 5~10MB ≈ 15MB(측정). 네이티브 GUI는 도구마다 20MB대부터.
- 문자 격자라 레이아웃·패키징 고민이 적고, Go 바이너리 하나로 나간다.

| 대상 (2026-10-04, `top` MEM) | 메모리 |
|---|---|
| calendar-tui (목업) | 약 4.5~10MB |
| Terminal.app 창 1개 추가 | +9MB |
| 텍스트 그리는 빈 AppKit 창 앱 | 19MB |
| ghostty (여러 탭) | 575MB |

| 대안 | 도구당 메모리 | 판단 |
|---|---|---|
| **TUI + Terminal 창 (채택)** | ~15MB | 가장 가볍고 빠른 개발. Dock/Cmd+Tab에선 "터미널" |
| Swift 네이티브 | ~20~35MB (추정) | 맥 전용, 개발 느림 |
| Swift 앱 + SwiftTerm + TUI | ~35~50MB (추정) | 비추천 |
| Rust egui/iced, Qt | 수십 MB 이상 | 과함 |

### EventKit 연동

- 맥에 등록된 계정을 EventKit으로 쓴다. 앱 등록·OAuth·심사가 필요 없다. 권한은 Terminal 앱 기준이고, Info.plist 없이 CLI에서 `requestFullAccessToEventsWithCompletion`으로 허용 창이 떴다(macOS 15, 확인).
- 하루 종일 일정 끝은 EventKit이 마지막 날 23:59:59로 준다 → 앱 안에선 마지막 날 0시로 바꾸고, 저장 때 되돌린다.
- 구글 매주 일정은 시작 요일(BYDAY)을 넣어 보내므로, 시작일과 같은 요일·날짜·달 지정은 단순 반복으로 본다.
- 달 단위 캐시. 변경 알림·포커스 복귀·`r`·저장 후·1시간 주기로 무효화(아래 "바깥 변경 반영").
- 빌드: cgo(Objective-C) → Xcode 커맨드라인 도구 필요. 윈도우 빌드에선 빌드 태그로 목업만.

### 앱 패키징 (Dock 1안 구현)

- `Calendar TUI.app`은 실행기다. `Contents/MacOS/launcher`(`scripts/launcher.swift`, AppKit만, 창 없음)가 Terminal에서 제목에 `▦ 캘린더`가 든 창을 찾아 앞으로 가져오고, 없으면 새 창에서 캘린더를 실행한다. Terminal을 처음 켜는 경우엔 생기는 빈 창을 그대로 쓴다.
- 실행기는 캘린더가 떠 있는 동안 같이 상주한다(Dock 실행 점이 보이도록 — 처음엔 창을 띄우고 바로 끝나 점이 꺼졌다). Dock 아이콘 재클릭(`applicationShouldHandleReopen`)·Cmd+Tab(`applicationDidBecomeActive`) 때 캘린더 창을 앞으로. 캘린더 pid의 종료를 `DispatchSource.makeProcessSource`로 받아 같이 끝난다(폴링 없음). Dock에서 실행기를 종료하면 캘린더에 SIGTERM. 실측 phys_footprint 14MB.
- 실행기 메모리(2026-10-04 실측): 14MB 중 12MB는 빈 앱 번들(Dock 아이콘만)의 하한선이고, 나머지 2MB는 상주하는 AppleScript 엔진(`NSAppleScript`)이다. `osascript` 자식 프로세스로 바꾸면 12MB가 되지만 이득이 작고 활성화마다 프로세스를 띄워야 해서 그대로 둔다. 더 줄이려면 상주를 포기해야 한다(Dock 실행 점·Cmd+Tab 복귀를 잃음).
- 창 크기 기억: 캘린더가 `WindowSizeMsg`마다(값이 바뀔 때만) 글자 칸 수를 `state.json`의 `windowCols`·`windowRows`에 적고, 실행기가 새 창을 그 크기(`number of columns/rows`)로 연다. 기억한 값이 없으면 화면을 거의 채운다. 위치는 기억하지 않는다(사용자 요청은 크기). 글꼴 크기(Cmd +/-)도 기억한다: 캘린더가 종료 직전 자기 탭(tty로 찾음)의 `font size`를 AppleScript로 읽어 `fontSize`에 적고(`terminal_darwin.go` `termFontSize`, 제자리 재시작 땐 건너뜀), 실행기가 새 창에 글꼴 → 칸 수 순서로 적용한다. Terminal 안에서 Terminal에 묻는 것이라 자동화 권한 창이 뜨지 않았다(2026-10-04 확인). 실행기가 앞으로 가져올 때 읽는 안은 마지막 변경을 놓칠 수 있어 버렸다.
- 캘린더 바이너리는 번들 밖 `~/Library/Application Support/calendar-tui/calendar`(설치 때 복사, `dev.sh`가 교체). 없으면 번들 안 `Resources/calendar`.
- 창 제목은 앱이 `tea.SetWindowTitle("▦ 캘린더")`로 정한다(`main.go` `windowTitle`). 처음엔 `calendar-tui`였는데 폴더명이 같은 다른 Terminal 창(Claude 세션)이 걸려 바꿨다.
- 새 창은 화면 크기(AppleScriptObjC `NSScreen`)에 맞춰 키운다. Finder에 화면 크기를 물으면 자동화 권한 창이 하나 더 떠서 쓰지 않는다.
- 캘린더 권한은 여전히 Terminal 기준(창이 Terminal 소속). 실행기는 Terminal 자동화 권한만 필요(`NSAppleEventsUsageDescription`).
- 사용자에게 중요한 건 "Dock에서 따로 눌러 실행"이다(창이 Terminal 소속인 건 상관없음). 실행기는 창을 띄우고 바로 끝나 Dock에 남지 않으므로 `package.sh --install`이 Dock에 고정한다(`com.apple.dock persistent-apps`에 추가 후 `killall Dock`, 이미 있으면 건너뜀).
- 버린 방식: 자체 창에 SwiftTerm을 넣은 Swift 앱(Dock·Cmd+Tab 독립). 실측 phys_footprint 껍데기 101MB + TUI 16MB로, Terminal 창 방식(+9MB)보다 훨씬 무거워 "배보다 배꼽"이라 되돌렸다(2026-10-04). 정말 독립 창이 필요해지면 껍데기를 얹지 말고 네이티브로 새로 만든다.
- 번들은 ad-hoc 서명(`codesign -s -`). 키 로그를 번들 안에 쓰면 서명이 깨져서 `~/Library/Logs/calendar-tui/`로 옮겼다.

### 설정 파일 (에이전트 접근성)

[agent-configuration-accessibility](https://github.com/zidell/agent-configuration-accessibility) 컨벤션을 따른다(2026-10-04 사용자 지시).

- 사용자 설정은 `config.toml`(TOML, 항목마다 설명 주석), 앱이 저절로 기억하는 값(창 크기·글꼴·마지막 보기·마지막 캘린더)은 `state.json`. 하나로 두면 창 크기를 바꿀 때마다 앱이 파일을 다시 써서 바깥 편집을 덮기 때문에 나눴다.
- 표시 여부는 `[accounts]`·`[calendars]` 아래 `"ID" = true/false  # 이름`. ID는 EventKit 것이라 이름 주석을 앱이 채운다(시작 때 목록이 바뀌었으면, 설정 [저장] 때 다시 씀). 지금 목록에 없는 숨김 ID도 남긴다.
- 바깥 편집 반영: 맥·리눅스는 자동 재시작 감시 루프(0.3초)에서 `config.toml` mtime도 같이 봐서 즉시, 그 밖엔 `invalidate`(포커스 복귀·`r`·1시간)에서 다시 읽는다. 감시 루프를 새로 두지 않았다.
- 잘못된 값(타입·모르는 항목)은 이전 값을 유지하고 화면 맨 아래에 오류. 그동안은 앱이 파일을 다시 쓰지 않는다(고치던 내용 보호).
- `--config-path`·`--check-config`는 EventKit 권한 요청·옮기기·쓰기 전에 끝난다(처음엔 설정을 읽다가 옮기기까지 해버려 고쳤다). `-mock`은 설정 파일을 읽기만 한다(목업 창이 실제 창 크기를 덮어쓴 적이 있다).
- 설정 항목을 늘리면 `tomlConfig`·`renderConfig`(설명 주석)·`assets/readme.txt`·`usage()`를 같이 고친다.

### 공개용 일반 기능 (2026-10-04)

사용자가 "내가 쓸 기능은 다 됐고, 공개를 위해 일반 사용자에게 필요할 기능을 이 컨셉·UX 위에 보완"하라고 해서 후보를 묻고 고른 것: 알림, URL·회의 링크, 복제·이동, 빠른 추가, 검색, 주 시작 요일·12/24시간, 목록 보기, 주간 보기, 영어 UI, 밝은 배경 테마. (인텔 맥 유니버설 빌드는 고르지 않음)

- 다국어: 키 표 없이 쓰는 자리에서 `L(ko, en)`. 언어 auto는 `LC_ALL`·`LC_MESSAGES`·`LANG`, 없으면 `AppleLanguages`. 사용자 지정 반복 규칙 설명은 EventKit(ObjC)에서 구조(빈도·간격·요일)만 받아 Go가 만든다(`ruleText`). 창 제목도 `▦ 캘린더`/`▦ Calendar` — 실행기는 둘 다 찾는다(번들 재설치 전엔 옛 실행기가 한국어 제목만 찾음).
- 테마: 시작 때 `lipgloss.HasDarkBackground()`(OSC 11)로 배경을 물어 auto를 정한다(Terminal.app Novel 프로필에서 밝음으로 잡히는 것 확인. Basic 프로필은 시스템 다크 모드를 따라 어둡게 나옴). 밝은 테마에선 캘린더 색 글씨를 검정 쪽으로 45% 섞는다(`inkOf`, 연한 색이 흰 바탕에 묻힘).
- 알림: 폼에서 알림 하나만 다룬다(시작 기준 분, 하루 종일은 0시 기준). 원래 여러 개·절대 시각이면 "여러 개·사용자 지정"으로 보이고 바꾸기 전까지 그대로(`keepAlarms`). 알림은 macOS·폰이 띄우므로 앱이 꺼져 있어도 온다.
- 링크: URL 칸, 없으면 장소·메모 안의 첫 http(s) 링크(구글 Meet은 EventKit에 URL로 안 오고 메모에 들어 있다).
- 빠른 추가 해석 규칙: 날짜(ISO·`10월 12일`·`10/12`·`oct 12`·오늘/내일/모레/글피·today/tomorrow·요일·다음 주 요일·`12일`), 시각(`오후 3시 반`·`15:00`·`3pm`, 범위 `3-5시`·`10:30-12`), 길이(`2시간`·`for 15m`), `하루 종일`. 오전·오후 없는 1~7시는 오후. 숫자만 있는 건 시각으로 안 본다(`3층 회의실`).
- 검색: 열 때 앞뒤 1년을 EventKit에서 한 번 읽는다(키 로그에 `search load N events 시간`). 반복 일정은 회차마다 나온다.
- 주간 보기: 한 시간 = 한 줄. 24시간이 다 안 들어가면 8시부터(일정이 이르면 그 시각부터, 늦게 끝나면 끝이 보이게). 겹치는 일정은 옆 칸(lane)으로 나눈다. 열 폭은 월간과 같은 `colWidths`.
- 커서 열 넓히기(`colWidths`): 단위는 영문 글자 수(터미널 칸). 똑같이 나눈 폭이 `focus_min_width`(기본 30, 2026-10-04 20 → 30 사용자 지시) 이상이면 그대로, 좁으면 다른 열에서 가져와 맞춘다. 다른 열 하한 5(한글 2자 + 여백), 커서 열 하한 12. 이력: 고정 24칸 → 창 폭 비율 → 내용 기반 협상(강도 0~1) → 최소폭 확보로 확정(사용자: 협상 방식은 이미 다 보이는데도 넓어질 수 있다는 우려, "그 이상은 글꼴·창 폭 문제").
- `line()`은 남은 폭이 2 이하면 `..` 없이 자른다(좁은 칸에서 `..`이 넘쳐 세로선이 밀렸다).

### 바깥 변경 반영

- EventKit `EKEventStoreChangedNotification`을 별도 `NSOperationQueue`에서 받아(메인 런루프가 Go 이벤트 루프에 묶여 있어서) `storeChanges` 채널 → `changedMsg`로 캐시를 비운다. 몰린 알림은 버퍼 1 채널로 합친다.
- 주기 새로고침은 최후 수단으로 1시간(사용자 지시: 폴링은 자원을 쓰므로 최소화). 이전엔 30초.
- 터미널 포커스가 돌아올 때도 캐시를 비운다(로컬 DB 읽기라 네트워크 없음).
- 오늘 날짜도 포커스 복귀·1시간 주기마다 다시 잡는다(`syncToday`, 사용자 지시: 자정을 넘겨 띄워 둔 창에서 어제가 오늘로 남았다). 커서가 옛 오늘에 있었고 모달이 없으면 커서도 따라가 보이는 달·주가 바뀐다.
- 알림 수신은 키 로그에 `store changed`로 남는다.

### 개발용 자동 재시작

- 앱(맥·리눅스)은 자기 실행 파일 mtime을 0.3초마다 본다. 바뀌고 한 번 더 같으면(쓰기 끝) 종료 후 `exec`로 새 실행 파일로 바꾼다. 보던 날짜는 `CAL_CURSOR`로 넘긴다. 윈도우는 실행 중 exe를 덮어쓸 수 없어 없음.

### 큰 월 제목: DEC 2배 크기 줄

- `ESC#3`(윗절반) · `ESC#4`(아랫절반)에 같은 글자를 쓴다. 한글까지 커진다. Terminal.app(macOS 15)에서 확인. 이 줄은 칸이 2칸 폭이라 화면 절반 폭 기준으로 가운데를 잡는다.
- 모달 오버레이가 배경 ANSI를 지울 때 이 표시는 다시 붙인다(`overlay`).
- 창 크기를 바꾸면 Terminal이 기존 줄을 다시 흘려 놓아 2배 크기 표시가 엉뚱한 줄(가로선 등)에 남고, Bubble Tea는 바뀐 줄만 다시 그려 흔적이 남았다. `WindowSizeMsg`마다 `tea.ClearScreen`으로 통째로 지우고 그린다(전체 지우기는 줄의 2배 크기 표시도 푼다, 2026-10-04 재현·확인).
- 반칸 블록 숫자 안은 한글을 못 그려 버렸다.

### 줄 간격·글자 크기·색

- TUI는 줄 높이를 못 정한다. 1픽셀 간격 요청 → 불가. Terminal.app 줄 간격은 선 문자가 끊길 수 있고(미확인), Ghostty `adjust-cell-height`는 선을 이어 그린다(알려진 동작). 앱 안 빈 줄은 칸당 일정 수가 줄어 쓰지 않음.
- Terminal.app(macOS 15 Sequoia)은 256색. 트루컬러는 macOS 26 Tahoe Terminal부터로 알려짐(미확인). Ghostty·iTerm2는 트루컬러.

### 이미지·첨부 (미구현 방침)

- TUI 안에서 그리지 않고 OS 기본 앱으로 연다(`open`/`start`, 잠깐 보기는 `qlmanage -p`). Terminal.app은 kitty·iTerm2·Sixel 이미지 프로토콜을 지원하지 않고, Bubble Tea 재그리기와도 충돌한다.

### 배포 (2026-10-04)

- 저장소 공개, MIT. 설치는 `curl -fsSL https://zidell.github.io/calendar-tui/install.sh | bash` 한 줄: 최신 릴리스 zip을 받아 `/Applications`에 넣고 실행 파일을 `~/Library/Application Support/calendar-tui/`에 두고 Dock에 고정한다. 다시 실행하면 업데이트(실행 중이면 새 실행 파일을 감지해 제자리 재시작).
- 공증(연 $99) 없이 되는 이유: 브라우저로 받은 파일엔 격리 표시(`com.apple.quarantine`)가 붙어 Gatekeeper가 막지만 `curl`로 받은 파일엔 붙지 않는다. 번들은 ad-hoc 서명이라 Apple Silicon에서도 실행된다. 그래서 zip을 브라우저로 받아 여는 안내는 하지 않는다.
- 릴리스는 GitHub Actions가 만든다(사용자 지시, 2026-10-04: 미리 빌드한 릴리스가 사용자 맥 빌드보다 낫고, 릴리스 수고는 Actions로 없앤다). `scripts/release.sh vX.Y.Z`는 깨끗한 작업 트리·푸시된 main을 확인하고 태그만 푸시한다. v0.1.0은 로컬에서 만들었다.
- 릴리스는 유니버설(arm64 + x86_64, `UNIVERSAL=1`): Go는 `CC="clang -arch x86_64"`로 cgo 교차 빌드 후 `lipo`, 실행기는 `swiftc -target`. 인텔 실기기 확인은 못 했다. 버전은 `-ldflags -X main.version`과 Info.plist에 같이 넣는다(`calendar --version`).
- 설치 스크립트 시험은 `ZIP_URL=file://… APP_DIR=… SUPPORT_DIR=… NO_DOCK=1 bash docs/install.sh`로 임시 폴더에(실제 설치를 건드리면 Terminal 제어 권한을 다시 묻는다).
- 윈도우는 배포하지 않는다: 윈도우엔 EventKit처럼 시스템 계정 일정을 앱에 주는 창구가 사실상 없다(WinRT `AppointmentStore`는 패키지 신원이 필요하고, 데이터를 채우던 "메일 및 일정" 앱이 새 Outlook으로 바뀜 — 미확인 지식). 목업만 보이는 앱을 내놓지 않으려고 뺐다. 윈도우는 Google API·Graph 백엔드가 생기면 다시 본다.

### Dock 패키징 (미구현, 1안 채택)

- 도구마다 작은 실행기 `.app`: 그 도구의 Terminal 창(창 제목으로 식별)이 있으면 앞으로, 없으면 새로 실행. Cmd+Tab에는 여전히 "터미널" 하나.

## 로드맵

### 2단계: API 연동 + Windows

UI는 그대로 두고 `backend` 구현체를 추가한다.

- **Google Calendar API v3**: 데스크톱 앱 OAuth 클라이언트 ID/secret을 바이너리에 포함, 로컬 루프백으로 토큰 수신. 디바이스 코드 방식은 Calendar 스코프 미지원. 캘린더는 민감 권한이라 공개 배포 시 심사(미심사면 경고 + 100명 제한, 테스트 모드는 토큰 7일 만료). 기본 클라이언트 ID 내장 + 사용자 지정 허용. 실시간 반영은 주기적 재조회 + sync token.
- **iCloud**: CalDAV + 앱 암호(심사 불필요). Fastmail·Nextcloud도 거의 그대로.
- **Outlook**: Microsoft Graph, 디바이스 코드 로그인 가능.
- 반복 범위: Google API는 "이후 일정 모두" 옵션이 없어 기존 반복을 전날까지 `UNTIL`로 끊고 새 반복을 만든다(구글 웹도 동일).
- 참고: Gmail은 제한 권한이라 공개 배포 시 유료 보안 감사. 본인만 쓰면 앱 비밀번호 + IMAP.

### 남은 일

- 실제 구글 캘린더 쓰기 경로(추가·수정·삭제·반복 범위·알림·URL)는 사용자가 직접 확인 전(목업으로만 검증).
- 실행기 번들 재설치(영문 창 제목 찾기 반영) — 설치하면 Terminal 제어 권한을 한 번 더 묻는다.
