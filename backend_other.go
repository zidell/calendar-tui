//go:build !darwin

package main

import "errors"

func newEventKit() (backend, error) {
	return nil, errors.New("EventKit은 macOS에서만 쓸 수 있습니다")
}
