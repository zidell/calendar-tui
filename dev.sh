#!/bin/bash
# 빌드해서 ~/.local/bin/calendar-tui에 설치하고 띄운다. 이미 떠 있으면 새 실행 파일을 감지해 그 창에서 다시 시작한다.
# dev.local.sh(커밋하지 않음)가 있으면 띄우기는 그것에 맡긴다(개발 맥마다 다른 실행 방식).
set -e
cd "$(dirname "$0")"
go build -o calendar-tui .
BIN="$HOME/.local/bin"
mkdir -p "$BIN"
# 실행 중인 파일을 덮어쓰면 프로세스가 죽을 수 있어 옆에 쓰고 이름을 바꾼다
cp calendar-tui "$BIN/calendar-tui.new"
mv -f "$BIN/calendar-tui.new" "$BIN/calendar-tui"
if [ -f dev.local.sh ]; then
  exec bash dev.local.sh "$BIN/calendar-tui"
fi
if ! pgrep -x calendar-tui >/dev/null; then
  osascript -e "tell application \"Terminal\" to do script \"exec '$BIN/calendar-tui'\"" \
            -e 'tell application "Terminal" to activate' >/dev/null
fi
