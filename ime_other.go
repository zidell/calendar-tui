//go:build !darwin

package main

func imeCurrent() string { return "" }
func imeASCII() string   { return "" }
func imeSelect(string)   {}
