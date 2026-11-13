package rime

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/rimeinn/go-rime/abi"
)

var (
	// ErrClosed indicates that the engine or the requested resource is closed.
	ErrClosed = errors.New("rime: resource is closed")
	// ErrUnsupported indicates that the loaded library cannot provide an API.
	ErrUnsupported = errors.New("rime: API unavailable")
	// ErrEngineUsed indicates that an engine has already been opened in this process.
	ErrEngineUsed = errors.New("rime: engine already opened in this process")
	// ErrMaintenance indicates that an asynchronous native task is in progress.
	ErrMaintenance = errors.New("rime: maintenance is in progress")
	// ErrFailed indicates a failure reported by a native operation.
	ErrFailed = errors.New("rime: native operation failed")
	// ErrInvalidArgument indicates a value that cannot be passed to librime.
	ErrInvalidArgument = errors.New("rime: invalid argument")
)

func unsupported(slot string) error { return fmt.Errorf("%w: %s", ErrUnsupported, slot) }

func failed(slot string) error { return fmt.Errorf("%w: %s", ErrFailed, slot) }

func cString(value string) (*byte, error) {
	p, err := abi.CString(value)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return p, nil
}

// Several APIs use strncpy and return success even when truncating. A full
// buffer without a NUL requires another read.
func readCStringBuffer(read func(*byte, uintptr) bool) (string, bool, error) {
	const maxBuffer = 16 << 20
	for size := 256; size <= maxBuffer; size *= 2 {
		buf := make([]byte, size)
		if !read(&buf[0], uintptr(size)) {
			return "", false, nil
		}
		if end := bytes.IndexByte(buf, 0); end >= 0 {
			return string(buf[:end]), true, nil
		}
	}
	return "", false, fmt.Errorf("%w: C string exceeds %d bytes", ErrFailed, maxBuffer-1)
}
