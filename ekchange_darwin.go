//go:build darwin

package main

import "C"

// goStoreChanged는 EventKit 변경 알림(EKEventStoreChangedNotification)을 받으면 Objective-C 쪽에서 부른다.
// 이 파일은 //export 때문에 cgo 머리말에 정의를 둘 수 없어 eventkit_darwin.go와 나눴다.
//
//export goStoreChanged
func goStoreChanged() {
	select {
	case storeChanges <- struct{}{}:
	default: // 이미 알림이 대기 중이면 합친다
	}
}
