package abi

import (
	"errors"
	"strings"
	"unsafe"
)

// ErrEmbeddedNUL indicates a Go string cannot be passed as a single C string.
var ErrEmbeddedNUL = errors.New("rime ABI: string contains NUL")

// CString copies s into a NUL-terminated Go allocation. If native code keeps
// the pointer beyond the call, the caller must pin the buffer for that period.
func CString(s string) (*byte, error) {
	if strings.IndexByte(s, 0) >= 0 {
		return nil, ErrEmbeddedNUL
	}
	buf := make([]byte, len(s)+1)
	copy(buf, s)
	return &buf[0], nil
}

// GoString copies a NUL-terminated C string. A nil pointer yields an empty
// string. The caller must ensure the string is readable through the terminator.
func GoString(p *byte) string {
	if p == nil {
		return ""
	}
	length := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), length)) != 0 {
		length++
	}
	return string(unsafe.Slice(p, length))
}

// String copies the borrowed bytes while they are valid. An embedded NUL is
// preserved. A nil pointer yields an empty string.
func (s StringSlice) String() string {
	if s.Str == nil {
		return ""
	}
	return string(unsafe.Slice(s.Str, s.Length))
}
