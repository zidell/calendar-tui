calendar-tui (Calendar TUI.app) — 설치 사용자·에이전트용 안내
================================================================

터미널(Terminal.app) 창에서 도는 월간 캘린더. macOS 시스템 설정 → 인터넷 계정에 등록된
구글·iCloud 캘린더를 EventKit으로 읽고 쓴다. 이 앱(Calendar TUI.app)은 실행기이고,
캘린더 본체는 아래 실행 파일이다.

  실행 파일: ~/Library/Application Support/calendar-tui/calendar
             (없으면 이 번들 안 Contents/Resources/calendar)

1. 설정 파일
------------
사용자 설정은 config.toml 하나다. 바꿀 수 있는 것:
  [display]   language(auto/ko/en), theme(auto/dark/light), week_start(sunday/monday),
              time_format(24h/12h), focus_min_width(좁을 때 고른 요일 최소 폭, 영문 글자 수, 0 또는 12~60)
  [debug]     key_log(키 입력 로그, 기본 false)
  [accounts]  계정 표시 여부
  [calendars] 캘린더 표시 여부

  ~/Library/Application Support/calendar-tui/config.toml

경로 확인:  <실행 파일> --config-path
(윈도우는 %APPDATA%\calendar-tui\config.toml, 리눅스는 ~/.config/calendar-tui/config.toml)

- 형식: TOML. # 뒤는 주석이고, 파일 첫머리와 항목마다 의미·허용값·기본값이 적혀 있다.
- [accounts]·[calendars]의 키는 계정·캘린더 ID, 값은 true(보임) / false(숨김). 각 줄 끝 주석이 이름이다.
  예) "회사 캘린더 숨겨줘" → [calendars]에서 주석이 "회사 · …"인 줄의 값을 false로.
  예) "월요일부터 시작하게 해줘" → [display] week_start = "monday"
  예) "영어로 바꿔줘" → [display] language = "en"
- 이 번들 안에는 설정 템플릿이 없다. 실제로 읽는 파일은 위 경로 하나뿐이다.

2. 처음 설치했을 때
-------------------
앱을 한 번 실행하면 config.toml이 만들어지고 맥에 등록된 계정·캘린더 목록이 채워진다.
그 전에는 파일이 없다(--config-path는 만들어질 경로를 알려준다).
예전 버전의 settings.json이 있으면 처음 실행할 때 옮기고 settings.json.bak으로 남긴다.

3. 고친 뒤 반영
---------------
- 저장하면 실행 중인 앱이 바로 다시 읽는다. 앱을 끄거나 다시 켤 필요 없다.
- 검사: <실행 파일> --check-config  (오류면 내용 출력, 종료 코드 1)
- 잘못된 값이면 앱은 이전 값을 계속 쓰고 화면 맨 아래에 오류를 띄운다. 그동안 앱은 파일을 덮어쓰지 않는다.
- 앱 안 설정 화면(s → 캘린더 선택·표시 → [저장])도 같은 파일을 쓴다. 이때와 앱 시작 때 앱이 파일을
  다시 쓰므로 직접 단 주석은 사라진다(안내 주석과 이름 주석은 다시 만들어진다).
- 반영 확인: 달력에 그 캘린더 일정이 보이는지/사라졌는지, 또는 앱의 s → 캘린더 선택 화면의 체크.

4. 앱이 저절로 기억하는 값
--------------------------
마지막 보기(월간·주간·목록)·마지막으로 쓴 캘린더는 같은 폴더 state.json에 있다. 사용자 설정이
아니라 앱이 쓰는 상태라서, 고치려면 앱을 끈 뒤 고친다(켜져 있으면 보기를 바꿀 때 덮어쓴다).
창 크기(글자 칸)·글꼴 크기는 이 앱(실행기, github.com/zidell/tuidock)이 기억한다:
  defaults read com.zidell.calendar-tui   (cols·rows·font)
다른 앱으로 갈 때와 캘린더를 끌 때 기록되고, 다음 실행 때 새 창에 적용한다.

5. 자격 증명·권한
-----------------
자격 증명은 없다. 계정은 macOS 인터넷 계정이 관리한다.
처음 실행 때 "터미널이 캘린더에 접근"·"Calendar TUI가 Terminal을 제어" 권한을 묻는다.
설정 파일로는 이 권한을 줄 수 없다(시스템 설정 → 개인정보 보호 및 보안).
