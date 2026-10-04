#!/bin/bash
# 빌드하고 앱을 띄운다. 이미 떠 있으면 앱이 새 실행 파일을 감지해 그 창에서 다시 시작한다.
# 앱(Calendar TUI.app)이 설치돼 있으면 앱이 쓰는 바이너리(~/Library/Application Support/calendar-tui/calendar)를 바꾼다.
# 앱 번들은 건드리지 않는다(서명이 바뀌면 Terminal 제어 권한을 다시 묻는다).
set -e
cd "$(dirname "$0")"
go build -o calendar .
APP="/Applications/Calendar TUI.app"
SUP="$HOME/Library/Application Support/calendar-tui"
if [ -d "$APP" ]; then
  mkdir -p "$SUP"
  # 실행 중인 파일을 덮어쓰면 프로세스가 죽을 수 있어 옆에 쓰고 이름을 바꾼다
  cp calendar "$SUP/calendar.new"
  mv -f "$SUP/calendar.new" "$SUP/calendar"
  pgrep -x calendar >/dev/null || open "$APP"
  exit 0
fi
if ! pgrep -x calendar >/dev/null; then
  osascript -e "tell application \"Terminal\" to do script \"cd '$PWD' && exec ./calendar\"" \
            -e 'tell application "Terminal" to activate' >/dev/null
fi
