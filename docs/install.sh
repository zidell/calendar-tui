#!/bin/bash
# calendar-tui 설치·업데이트 (macOS)
#   curl -fsSL https://zidell.github.io/calendar-tui/install.sh | bash
# 최신 릴리스의 앱(Calendar TUI.app, Apple Silicon·Intel)을 받아 /Applications에 넣고,
# 캘린더 실행 파일을 ~/Library/Application Support/calendar-tui/calendar에 두고, Dock에 고정한다.
# 다시 실행하면 업데이트된다(설정은 그대로). 시험용 환경변수: APP_DIR, SUPPORT_DIR, NO_DOCK=1, ZIP_URL.
set -euo pipefail

REPO="zidell/calendar-tui"
ZIP_URL="${ZIP_URL:-https://github.com/$REPO/releases/latest/download/Calendar-TUI-macos.zip}"
APP_DIR="${APP_DIR:-/Applications}"
SUPPORT_DIR="${SUPPORT_DIR:-$HOME/Library/Application Support/calendar-tui}"
APP="$APP_DIR/Calendar TUI.app"

say() { printf '\033[1m%s\033[0m\n' "$*"; }
die() { printf 'calendar-tui: %s\n' "$*" >&2; exit 1; }

[ "$(uname -s)" = Darwin ] || die "macOS에서만 설치할 수 있습니다 / macOS only"
major=$(sw_vers -productVersion | cut -d. -f1)
[ "$major" -ge 14 ] || die "macOS 14 이상이 필요합니다 / requires macOS 14+"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "내려받는 중 / Downloading…"
curl -fsSL "$ZIP_URL" -o "$tmp/app.zip" || die "내려받기 실패 / download failed: $ZIP_URL"
ditto -x -k "$tmp/app.zip" "$tmp" || die "압축 풀기 실패 / unzip failed"
[ -d "$tmp/Calendar TUI.app" ] || die "앱이 들어 있지 않습니다 / app not found in archive"
# curl로 받은 파일엔 격리 표시가 붙지 않지만, 혹시 붙어 있으면 지운다
xattr -dr com.apple.quarantine "$tmp/Calendar TUI.app" 2>/dev/null || true

say "설치하는 중 / Installing…"
mkdir -p "$APP_DIR" "$SUPPORT_DIR"
if [ -w "$APP_DIR" ]; then
  rm -rf "$APP" && cp -R "$tmp/Calendar TUI.app" "$APP_DIR/"
else
  echo "$APP_DIR 에 쓰려면 관리자 암호가 필요합니다 / admin password needed for $APP_DIR"
  sudo rm -rf "$APP" && sudo cp -R "$tmp/Calendar TUI.app" "$APP_DIR/"
fi
# 실행 중인 캘린더가 있으면 새 파일을 감지해 그 창에서 다시 시작한다(옆에 쓰고 이름만 바꿈)
cp "$APP/Contents/Resources/calendar" "$SUPPORT_DIR/calendar.new"
mv -f "$SUPPORT_DIR/calendar.new" "$SUPPORT_DIR/calendar"

if [ "${NO_DOCK:-}" != 1 ] && ! defaults read com.apple.dock persistent-apps 2>/dev/null | grep -q "Calendar%20TUI.app"; then
  defaults write com.apple.dock persistent-apps -array-add "<dict><key>tile-data</key><dict><key>file-data</key><dict><key>_CFURLString</key><string>file://$(printf '%s' "$APP/" | sed 's/ /%20/g')</string><key>_CFURLStringType</key><integer>15</integer></dict></dict></dict>"
  killall Dock 2>/dev/null || true
fi

say "설치 완료 / Installed: $("$SUPPORT_DIR/calendar" --version 2>/dev/null || echo calendar-tui)"
cat <<'MSG'

Dock의 "Calendar TUI"를 누르세요. 처음 실행할 때 권한 창 두 개를 허용합니다:
  · Calendar TUI가 Terminal을 제어  · 터미널이 캘린더에 접근
캘린더 계정은 시스템 설정 → 인터넷 계정에 추가해 둔 구글·iCloud 등을 그대로 씁니다.

Click "Calendar TUI" in the Dock. On first launch allow both prompts:
  · Calendar TUI wants to control Terminal  · Terminal wants to access Calendars
It uses the Google/iCloud accounts in System Settings → Internet Accounts.
MSG
