//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Carbon -framework CoreFoundation
#include <stdlib.h>
#include <Carbon/Carbon.h>

static char *sourceID(TISInputSourceRef s) {
	if (!s) return NULL;
	CFStringRef id = (CFStringRef)TISGetInputSourceProperty(s, kTISPropertyInputSourceID);
	char buf[256] = {0};
	if (!id || !CFStringGetCString(id, buf, sizeof buf, kCFStringEncodingUTF8)) return NULL;
	return strdup(buf);
}

static char *imeCurrentID(void) {
	TISInputSourceRef s = TISCopyCurrentKeyboardInputSource();
	char *r = sourceID(s);
	if (s) CFRelease(s);
	return r;
}

static char *imeASCIIID(void) {
	TISInputSourceRef s = TISCopyCurrentASCIICapableKeyboardInputSource();
	char *r = sourceID(s);
	if (s) CFRelease(s);
	return r;
}

static void imeSelectID(const char *id) {
	CFStringRef key = CFStringCreateWithCString(NULL, id, kCFStringEncodingUTF8);
	CFDictionaryRef q = CFDictionaryCreate(NULL, (const void **)&kTISPropertyInputSourceID, (const void **)&key, 1,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFArrayRef list = TISCreateInputSourceList(q, false);
	if (list && CFArrayGetCount(list) > 0) TISSelectInputSource((TISInputSourceRef)CFArrayGetValueAtIndex(list, 0));
	if (list) CFRelease(list);
	CFRelease(q);
	CFRelease(key);
}
*/
import "C"

import (
	"runtime"
	"unsafe"
)

// 입력 소스 API는 메인 스레드에서 불러야 한다. Bubble Tea는 Update를 main 고루틴에서 돌리므로 그 스레드를 고정한다.
func init() { runtime.LockOSThread() }

func goStr(p *C.char) string {
	if p == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(p))
	return C.GoString(p)
}

func imeCurrent() string { return goStr(C.imeCurrentID()) }
func imeASCII() string   { return goStr(C.imeASCIIID()) }

func imeSelect(id string) {
	cs := C.CString(id)
	defer C.free(unsafe.Pointer(cs))
	C.imeSelectID(cs)
}
