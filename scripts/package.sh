#!/bin/bash
# 단독 실행용 앱 번들(dist/Calendar TUI.app)을 만든다. --install이면 /Applications에 설치하고 Dock에 고정한다.
# 앱은 tuidock(github.com/zidell/tuidock) 실행기다: 캘린더를 Terminal 창에 띄우고 그 창을 단독 앱처럼 다룬다
# (Dock 아이콘·Cmd+Tab·창 크기 기억·Cmd 단축키). 실행기 소스는 go.mod가 고정한 tuidock 모듈에서 가져온다.
# 실제 바이너리는 ~/Library/Application Support/calendar-tui/calendar(번들 밖)를 먼저 쓴다. dev.sh가 갈아 끼워도 앱 서명이 그대로라 권한을 다시 묻지 않는다.
# 환경변수: VERSION(기본 dev, 번들 버전·--version에 들어감), UNIVERSAL=1이면 Apple Silicon + 인텔 유니버설 빌드(릴리스용).
set -e
cd "$(dirname "$0")/.."
VERSION="${VERSION:-dev}"
LDFLAGS="-X main.version=$VERSION"
NAME="Calendar TUI"
TUIDOCK="$(go list -m -f '{{.Dir}}' github.com/zidell/tuidock)"

[ -f assets/icon-1024.png ] || swift scripts/make-icon.swift assets/icon-1024.png
mkdir -p dist
if [ "$UNIVERSAL" = 1 ]; then
  CGO_ENABLED=1 GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/calendar-arm64 .
  CGO_ENABLED=1 GOARCH=amd64 CC="clang -arch x86_64" go build -ldflags "$LDFLAGS" -o dist/calendar-amd64 .
  lipo -create -output dist/calendar dist/calendar-arm64 dist/calendar-amd64
  rm dist/calendar-arm64 dist/calendar-amd64
  UNI=--universal
else
  go build -ldflags "$LDFLAGS" -o dist/calendar .
fi

# readme.txt: 설치 사용자·에이전트용 안내(설정 파일 위치·형식·적용 방법). github.com/zidell/agent-configuration-accessibility
bash "$TUIDOCK/tuidock" make --name "$NAME" --id com.zidell.calendar-tui --version "$VERSION" \
  --embed dist/calendar --command "~/Library/Application Support/calendar-tui/calendar" \
  --icon assets/icon-1024.png --resource assets/readme.txt --out dist ${UNI:-}
rm dist/calendar

if [ "$1" = "--install" ]; then
  bash "$TUIDOCK/tuidock" install "dist/$NAME.app"
  SUP="$HOME/Library/Application Support/calendar-tui"
  mkdir -p "$SUP"
  cp "dist/$NAME.app/Contents/Resources/calendar" "$SUP/calendar.new" && mv -f "$SUP/calendar.new" "$SUP/calendar"
  echo "설치: $SUP/calendar"
fi
