package main

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>

// sysDark는 시스템 모양이 다크인지. 동기화 후 읽어서 실행 중 바꾼 값도 바로 보인다(약 25µs, 측정).
static int sysDark(void) {
	CFPreferencesAppSynchronize(kCFPreferencesAnyApplication);
	CFPropertyListRef v = CFPreferencesCopyAppValue(CFSTR("AppleInterfaceStyle"), kCFPreferencesAnyApplication);
	int dark = 0;
	if (v) {
		dark = CFGetTypeID(v) == CFStringGetTypeID() &&
			CFStringCompare((CFStringRef)v, CFSTR("Dark"), kCFCompareCaseInsensitive) == kCFCompareEqualTo;
		CFRelease(v);
	}
	return dark;
}
*/
import "C"

// systemDark는 macOS 시스템 모양(다크·라이트). ok가 false면 알 수 없음.
func systemDark() (dark, ok bool) { return C.sysDark() != 0, true }
