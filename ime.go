package main

import "os"

// 한글 입력기는 조합 중인 자모를 붙잡고 터미널에 넘기지 않아, 한글 상태에선 단축키가 앱까지 오지 않는다.
// 그래서 글자를 치는 칸이 아니면 입력 소스를 영문으로 바꾸고, 글자 칸에 들어가면 원래 쓰던 입력 소스로 되돌린다.

// imeUser는 영문으로 바꾸기 전 사용자가 쓰던 입력 소스. 다시 시작할 때도 이어지게 환경변수로 넘긴다.
var imeUser = os.Getenv("CAL_IME")

// imeFor는 글자 입력 중이면 사용자 입력 소스를, 아니면 영문을 고른다.
func imeFor(text bool) {
	cur, ascii := imeCurrent(), imeASCII()
	if cur == "" || ascii == "" {
		return
	}
	if text {
		if imeUser != "" && cur != imeUser {
			logLine("ime → " + imeUser)
			imeSelect(imeUser)
		}
		return
	}
	if cur != ascii {
		logLine("ime " + cur + " → " + ascii)
		imeUser = cur
		imeSelect(ascii)
	}
}

// imeRestore는 종료할 때 사용자가 쓰던 입력 소스로 돌려놓는다.
func imeRestore() {
	if imeUser != "" && imeCurrent() != imeUser {
		imeSelect(imeUser)
	}
}

// imeForceASCII는 영문으로 다시 맞춘다. 이미 영문이어도 한글 → 영문으로 한 번 바꿔 준다.
// 구름은 시스템에 영문(Gureum.system)으로 등록된 채 내부는 한글 모드로 남아 키를 조합하는 때가 있다
// (키 로그: 한글이 들어오는데 현재 입력 소스는 영문으로 보고됨, 2026-10-04). 같은 소스를 다시 고르면 무시하므로 바꿔 준다.
func imeForceASCII() {
	cur, ascii := imeCurrent(), imeASCII()
	if cur == "" || ascii == "" {
		return
	}
	other := imeUser
	if other == "" || other == ascii {
		other = cur
	}
	if other == ascii { // 사용자 입력 소스를 아직 모르면 켜져 있는 영문 아닌 입력 소스(한글 등)를 거쳐 간다
		other = imeOther()
	}
	if other != "" && other != ascii {
		imeSelect(other)
	}
	imeSelect(ascii)
	if other != ascii && other != cur {
		imeUser = other
	}
	logLine("ime force " + other + " → " + ascii)
}
