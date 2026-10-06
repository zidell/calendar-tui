#!/bin/bash
# 부·주 버전 릴리스: 태그를 만들어 푸시한다. 빌드와 업로드는 GitHub Actions(.github/workflows/release.yml)가 한다.
# 패치 버전은 main에 푸시하면 Actions가 알아서 올리므로 이 스크립트가 필요 없다.
# 사용: scripts/release.sh v0.3.0
set -e
cd "$(dirname "$0")/.."
V="$1"
[[ "$V" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "사용: scripts/release.sh v0.1.1"; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "커밋 안 된 변경이 있다"; exit 1; }
git fetch -q origin
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || { echo "main을 먼저 푸시한다(로컬과 origin/main이 다름)"; exit 1; }
git tag "$V"
git push -q origin "$V"
echo "태그 $V 푸시. 빌드·릴리스 진행: https://github.com/zidell/calendar-tui/actions"
