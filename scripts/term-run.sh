#!/bin/bash
# Terminal.app 창에서 명령을 실행하고 끝나면 그 창을 닫는다.
# EventKit 권한·입력 소스처럼 Terminal 기준으로 확인해야 하는 실행에 쓴다(종료된 창이 남지 않게).
# 사용: scripts/term-run.sh '명령'   — 명령이 끝날 때까지 기다린다.
osascript - "$1" <<'APPLESCRIPT' >/dev/null
on run argv
	tell application "Terminal"
		set t to do script (item 1 of argv) & "; exit"
		set wid to id of front window
		repeat while busy of t
			delay 0.2
		end repeat
		delay 0.3
		close window id wid
	end tell
end run
APPLESCRIPT
