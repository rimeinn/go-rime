package abi

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

//go:generate go run ./internal/apigen

// ErrUnsupported indicates that a platform or a requested entry is unavailable.
var ErrUnsupported = errors.New("rime ABI: unsupported")

// Library owns a reference to a loaded librime dynamic library. It must not be
// copied. Use Open to construct it and Close after finalizing native state.
type Library struct {
	mu                sync.Mutex
	handle            uintptr
	api               *API
	path              string
	retainedForModule bool
}

// rime_get_api lazily populates its native static table without synchronization.
var openMu sync.Mutex

// Open loads librime and binds the default rime_get_api table. An empty path
// uses platform-specific names. Open leaves setup and initialization to callers.
func Open(path string) (*Library, error) {
	if !platformSupported {
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupported, runtime.GOOS, runtime.GOARCH)
	}
	if strings.IndexByte(path, 0) >= 0 {
		return nil, fmt.Errorf("library path: %w", ErrEmbeddedNUL)
	}
	if path == "" {
		paths := defaultLibraryPaths()
		var failures []error
		for _, candidate := range paths {
			lib, err := open(candidate)
			if err == nil {
				return lib, nil
			}
			failures = append(failures, err)
		}
		return nil, fmt.Errorf("load librime: %w", errors.Join(failures...))
	}
	return open(path)
}

func open(path string) (*Library, error) {
	openMu.Lock()
	defer openMu.Unlock()

	handle, err := openNative(path)
	if err != nil {
		return nil, fmt.Errorf("load %q: %w", path, err)
	}
	var getAPI func() unsafe.Pointer
	if err := registerLibFunc(&getAPI, handle, "rime_get_api"); err != nil {
		return nil, errors.Join(fmt.Errorf("%q: rime_get_api: %w", path, err), closeNative(handle))
	}
	native, err := newTable(getAPI())
	if err != nil {
		return nil, errors.Join(fmt.Errorf("%q: rime_get_api: %w", path, err), closeNative(handle))
	}
	return &Library{handle: handle, path: path, api: bindAPI(native)}, nil
}

// API returns the callable table, or nil if the library has been closed. The
// caller must ensure Close cannot overlap with calls through a returned table.
func (l *Library) API() *API {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.api
}

// Path returns the path or platform library name passed to the loader.
func (l *Library) Path() string { return l.path }

// Close releases the loader reference. The caller must first finish calls and
// callbacks and release all resources backed by the library. Calling Close
// again after a successful Close has no effect. A library with a registered Go
// module stays resident and Close returns ErrUnsupported.
func (l *Library) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.handle == 0 {
		return nil
	}
	if l.retainedForModule {
		return fmt.Errorf("%w: library retained for a registered Go module", ErrUnsupported)
	}
	if err := closeNative(l.handle); err != nil {
		return fmt.Errorf("unload %q: %w", l.path, err)
	}
	l.handle = 0
	l.api = nil
	return nil
}
