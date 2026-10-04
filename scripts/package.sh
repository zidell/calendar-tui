#!/bin/bash
# 단독 실행용 앱 번들(dist/Calendar TUI.app)을 만든다. --install이면 /Applications에 설치하고 Dock에 고정한다.
# 앱은 실행기다(scripts/launcher.swift): 캘린더 Terminal 창(제목 "▦ 캘린더")이 떠 있으면 앞으로 가져오고, 없으면 새 Terminal 창에서 실행한다.
# 실제 바이너리는 ~/Library/Application Support/calendar-tui/calendar(번들 밖)를 쓴다. dev.sh가 갈아 끼워도 앱 서명이 그대로라 권한을 다시 묻지 않는다.
# (SwiftTerm 내장 앱은 껍데기만 101MB라 버림 — AGENTS.md "앱 패키징")
set -e
cd "$(dirname "$0")/.."
NAME="Calendar TUI"
APP="dist/$NAME.app"

[ -f assets/icon-1024.png ] || swift scripts/make-icon.swift assets/icon-1024.png
rm -rf "$APP" dist/AppIcon.iconset
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources" dist/AppIcon.iconset

go build -o "$APP/Contents/Resources/calendar" .
# 설치 사용자·에이전트용 안내(설정 파일 위치·형식·적용 방법). github.com/zidell/agent-configuration-accessibility
cp assets/readme.txt "$APP/Contents/Resources/readme.txt"

for s in 16 32 128 256 512; do
  sips -z $s $s assets/icon-1024.png --out "dist/AppIcon.iconset/icon_${s}x${s}.png" >/dev/null
  sips -z $((s * 2)) $((s * 2)) assets/icon-1024.png --out "dist/AppIcon.iconset/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns dist/AppIcon.iconset -o "$APP/Contents/Resources/AppIcon.icns"
rm -rf dist/AppIcon.iconset

cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>Calendar TUI</string>
	<key>CFBundleDisplayName</key><string>Calendar TUI</string>
	<key>CFBundleIdentifier</key><string>com.zidell.calendar-tui</string>
	<key>CFBundleExecutable</key><string>launcher</string>
	<key>CFBundleIconFile</key><string>AppIcon</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>0.1</string>
	<key>CFBundleVersion</key><string>1</string>
	<key>LSMinimumSystemVersion</key><string>14.0</string>
	<key>NSAppleEventsUsageDescription</key><string>캘린더 창을 Terminal에 띄우거나 앞으로 가져오려고 Terminal을 제어합니다.</string>
</dict>
</plist>
PLIST

# 실행기: 캘린더가 떠 있는 동안 같이 살아 Dock 실행 점을 보이고, Dock 클릭·Cmd+Tab 때 캘린더 창을 앞으로 가져온다
swiftc -O -o "$APP/Contents/MacOS/launcher" scripts/launcher.swift
chmod +x "$APP/Contents/MacOS/launcher"

codesign --force --deep -s - "$APP" 2>/dev/null
echo "만듦: $APP"

if [ "$1" = "--install" ]; then
  rm -rf "/Applications/$NAME.app"
  cp -R "$APP" /Applications/
  SUP="$HOME/Library/Application Support/calendar-tui"
  mkdir -p "$SUP"
  cp "$APP/Contents/Resources/calendar" "$SUP/calendar.new" && mv -f "$SUP/calendar.new" "$SUP/calendar"
  echo "설치: /Applications/$NAME.app, $SUP/calendar"
  # Dock에 고정(이미 있으면 그대로). 실행기는 창을 띄우고 바로 끝나서 고정하지 않으면 Dock에 남지 않는다
  if ! defaults read com.apple.dock persistent-apps | grep -q "Calendar%20TUI.app"; then
    defaults write com.apple.dock persistent-apps -array-add "<dict><key>tile-data</key><dict><key>file-data</key><dict><key>_CFURLString</key><string>file:///Applications/Calendar%20TUI.app/</string><key>_CFURLStringType</key><integer>15</integer></dict></dict></dict>"
    killall Dock
    echo "Dock에 고정"
  fi
fi
