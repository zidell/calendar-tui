package main

import "testing"

func TestPlatformBackend(t *testing.T) {
	for _, tt := range []struct{ os, want string }{
		{"darwin", "eventkit"},
		{"windows", "google"},
		{"linux", "google"},
	} {
		t.Run(tt.os, func(t *testing.T) {
			if got := platformBackend(tt.os); got != tt.want {
				t.Fatalf("platformBackend(%q) = %q, want %q", tt.os, got, tt.want)
			}
		})
	}
}
