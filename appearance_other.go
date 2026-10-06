//go:build !darwin

package main

func systemDark() (dark, ok bool) { return false, false }
