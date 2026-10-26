//go:build !windows

package main

import (
	"os/exec"
	"strings"
)

// osLang은 macOS 언어 설정(AppleLanguages)의 첫 언어. 리눅스 등에선 "".
func osLang() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		return ""
	}
	s := strings.Trim(strings.TrimSpace(string(out)), "()\n ")
	first, _, _ := strings.Cut(s, ",")
	return strings.Trim(strings.TrimSpace(first), `"`)
}
