#!/bin/bash
# 릴리스: 유니버설 앱 번들을 만들어 zip으로 GitHub Releases에 올린다.
# 사용: scripts/release.sh v0.1.0
# 설치 스크립트(docs/install.sh)는 releases/latest/download/Calendar-TUI-macos.zip을 받는다.
set -e
cd "$(dirname "$0")/.."
V="$1"
[[ "$V" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "사용: scripts/release.sh v0.1.0"; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "커밋 안 된 변경이 있다"; exit 1; }
VERSION="$V" UNIVERSAL=1 scripts/package.sh
ZIP="dist/Calendar-TUI-macos.zip"
rm -f "$ZIP"
ditto -c -k --keepParent "dist/Calendar TUI.app" "$ZIP"
shasum -a 256 "$ZIP"
git tag "$V"
git push origin "$V"
gh release create "$V" "$ZIP" --title "$V" --notes "macOS(Apple Silicon·Intel) 앱. 설치: \`curl -fsSL https://zidell.github.io/calendar-tui/install.sh | bash\`"
