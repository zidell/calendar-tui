#!/bin/bash
# calendar-tui 설치·업데이트 (macOS)
#   curl -fsSL https://zidell.github.io/calendar-tui/install.sh | bash
# 최신 릴리스의 실행 파일(Apple Silicon·Intel)을 받아 ~/.local/bin/calendar-tui에 둔다.
# 다시 실행하면 업데이트된다(설정은 그대로). 시험용 환경변수: BIN_DIR, TAR_URL.
set -euo pipefail

REPO="zidell/calendar-tui"
TAR_URL="${TAR_URL:-https://github.com/$REPO/releases/latest/download/calendar-tui-macos.tar.gz}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

say() { printf '\033[1m%s\033[0m\n' "$*"; }
die() { printf 'calendar-tui: %s\n' "$*" >&2; exit 1; }

[ "$(uname -s)" = Darwin ] || die "macOS에서만 설치할 수 있습니다 / macOS only"
major=$(sw_vers -productVersion | cut -d. -f1)
[ "$major" -ge 14 ] || die "macOS 14 이상이 필요합니다 / requires macOS 14+"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "내려받는 중 / Downloading…"
curl -fsSL "$TAR_URL" -o "$tmp/calendar-tui.tar.gz" || die "내려받기 실패 / download failed: $TAR_URL"
tar -xzf "$tmp/calendar-tui.tar.gz" -C "$tmp" || die "압축 풀기 실패 / extract failed"
[ -f "$tmp/calendar-tui" ] || die "실행 파일이 들어 있지 않습니다 / binary not found in archive"
# curl로 받은 파일엔 격리 표시가 붙지 않지만, 혹시 붙어 있으면 지운다
xattr -d com.apple.quarantine "$tmp/calendar-tui" 2>/dev/null || true

mkdir -p "$BIN_DIR"
# 실행 중인 calendar-tui가 있으면 새 파일을 감지해 그 창에서 다시 시작한다(옆에 쓰고 이름만 바꿈)
cp "$tmp/calendar-tui" "$BIN_DIR/calendar-tui.new"
chmod +x "$BIN_DIR/calendar-tui.new"
mv -f "$BIN_DIR/calendar-tui.new" "$BIN_DIR/calendar-tui"

say "설치 완료 / Installed: $("$BIN_DIR/calendar-tui" --version 2>/dev/null || echo calendar-tui) → $BIN_DIR/calendar-tui"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) cat <<MSG

$BIN_DIR 이 PATH에 없습니다. ~/.zshrc에 추가하세요 / Add it to your PATH in ~/.zshrc:
  export PATH="$BIN_DIR:\$PATH"
MSG
  ;;
esac
cat <<'MSG'

터미널에서 calendar-tui 를 실행하세요. 처음 실행할 때 "터미널이 캘린더에 접근" 권한 창을 허용합니다.
캘린더 계정은 시스템 설정 → 인터넷 계정에 추가해 둔 구글·iCloud 등을 그대로 씁니다.

Run calendar-tui in your terminal. On first launch allow "Terminal wants to access Calendars".
It uses the Google/iCloud accounts in System Settings → Internet Accounts.
MSG
