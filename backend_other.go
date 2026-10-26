//go:build !darwin

package main

import "errors"

func newEventKit() (backend, error) {
	return nil, errors.New(L("EventKit은 macOS에서만 쓸 수 있습니다. config.toml [backend] type = \"google\"로 두세요",
		"EventKit is macOS-only. Set [backend] type = \"google\" in config.toml"))
}
