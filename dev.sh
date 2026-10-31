#!/bin/bash
# 빌드해서 ~/.local/bin/calendar-tui에 설치하고 띄운다. 이미 떠 있으면 새 실행 파일을 감지해 그 창에서 다시 시작한다.
# dev.local.sh(커밋하지 않음)가 있으면 띄우기는 그것에 맡긴다(개발 맥마다 다른 실행 방식).
# 구글 OAuth 클라이언트 JSON(Cloud 콘솔에서 받은 데스크톱 앱 클라이언트, 커밋 금지)이 있으면 릴리스처럼 넣어 빌드한다.
# 원본은 Google Drive `apps/calendar-tui/google_client.json`(GitHub 시크릿은 Actions 밖에서 읽을 수 없어서).
# 찾는 순서: CALENDAR_TUI_CLIENT_JSON → Google Drive → ~/.calendar-tui-google-client.json. 윈도우는 scripts/dev.ps1.
set -e
cd "$(dirname "$0")"
build_only=false
if [ "${1:-}" = --build-only ]; then build_only=true; shift; fi
LDF=""
CJ=""
for f in "$CALENDAR_TUI_CLIENT_JSON" "$HOME/GoogleDrive-Offline/apps/calendar-tui/google_client.json" "$HOME"/Library/CloudStorage/GoogleDrive-*/*/GoogleDrive/apps/calendar-tui/google_client.json \
         "$HOME/.calendar-tui-google-client.json"; do
  if [ -n "$f" ] && [ -f "$f" ]; then CJ="$f"; break; fi
done
if [ -n "$CJ" ]; then
  CID=$(sed -n 's/.*"client_id":"\([^"]*\)".*/\1/p' "$CJ")
  CSEC=$(sed -n 's/.*"client_secret":"\([^"]*\)".*/\1/p' "$CJ")
  LDF="-X main.googleClientID=$CID -X main.googleClientSecret=$CSEC"
fi
mkdir -p dist
staged=$(mktemp "$PWD/dist/.calendar-tui.XXXXXX")
trap 'rm -f "$staged"' EXIT
go build -ldflags "$LDF" -o "$staged" .
if $build_only; then
  mv -f "$staged" dist/calendar-tui
  echo "Built: $PWD/dist/calendar-tui (running app unchanged)"
  exit 0
fi
mv -f "$staged" calendar-tui
BIN="$HOME/.local/bin"
mkdir -p "$BIN"
# 실행 중인 파일을 덮어쓰면 프로세스가 죽을 수 있어 옆에 쓰고 이름을 바꾼다
cp calendar-tui "$BIN/calendar-tui.new"
mv -f "$BIN/calendar-tui.new" "$BIN/calendar-tui"
if [ "$(uname -s)" = Linux ]; then
  if pgrep -u "$(id -u)" -x calendar-tui >/dev/null; then
    echo '실행 중인 창에서 새 빌드로 다시 시작합니다.'
    exit 0
  fi
  exec bash scripts/launch-linux.sh "$BIN/calendar-tui" "$@"
fi
if [ -f dev.local.sh ]; then
  exec bash dev.local.sh "$BIN/calendar-tui"
fi
if ! pgrep -x calendar-tui >/dev/null; then
  if [ "$(uname -s)" = "Darwin" ]; then
  osascript -e "tell application \"Terminal\" to do script \"exec '$BIN/calendar-tui'\"" \
            -e 'tell application "Terminal" to activate' >/dev/null
  fi
fi
