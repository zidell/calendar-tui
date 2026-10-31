#!/usr/bin/env bash
# Keep the whole terminal/TUI process tree outside the build agent's cgroup.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
binary="${1:-$HOME/.local/bin/calendar-tui}"
if [[ $# -gt 0 ]]; then shift; fi
systemctl --user show-environment >/dev/null
if [[ ! -x "$binary" ]]; then echo "Missing calendar binary: $binary" >&2; exit 1; fi
if [[ -f "$root/dev.local.sh" ]]; then
  command=(bash "$root/dev.local.sh" "$binary")
elif command -v ghostty >/dev/null; then
  command=("$(command -v ghostty)" --class=dev.zidell.CalendarTui --gtk-single-instance=false --title=Calendar -e "$binary")
elif command -v konsole >/dev/null; then
  command=("$(command -v konsole)" --separate -e "$binary")
else
  echo 'Ghostty or Konsole is required for an independent calendar window.' >&2
  exit 1
fi
unit="calendar-tui-local-$(date +%s)-$$"
systemd-run --user --collect --quiet --unit="$unit" --service-type=exec \
  --property=ExitType=cgroup --working-directory="$root" -- "${command[@]}" "$@"
sleep 1
if ! systemctl --user is-active --quiet "$unit"; then
  echo "Calendar terminal exited; inspect journalctl --user -u $unit" >&2
  exit 1
fi
echo "Calendar runs independently: $unit"
