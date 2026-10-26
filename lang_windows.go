package main

import "golang.org/x/sys/windows"

// osLang은 윈도우 표시 언어(설정 → 시간 및 언어)의 첫 언어. 예: "ko-KR".
func osLang() string {
	ls, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(ls) == 0 {
		return ""
	}
	return ls[0]
}
