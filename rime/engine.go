// Package rime wraps the process-wide librime engine and copies native output
// into Go values.
package rime

import (
	"errors"
	"runtime"
	"sync"

	"github.com/rimeinn/go-rime/abi"
)

var process struct {
	mu      sync.Mutex
	library *abi.Library
	// Keep active native resources rooted even if the caller drops its reference.
	active *Engine
	// Some glog builds retain the original app name for the life of the process.
	appNamePin runtime.Pinner
}

// Engine owns the initialized librime service and serializes native operations.
// An Engine must not be copied. Call Close when finished with it.
type Engine struct {
	mu                 sync.Mutex
	api                *abi.API
	traitsPin          runtime.Pinner
	sessions           map[*Session]struct{}
	configs            map[*Config]struct{}
	notify             *notificationHub
	maintenancePending bool
	closed             bool
}

// Open loads librime, then calls setup and initialize. One successful call is
// allowed per process because librime and its dependencies use global state.
// The caller must coordinate exclusive access with any other native client.
func Open(opts Options) (*Engine, error) {
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.library != nil {
		return nil, ErrEngineUsed
	}

	traits, err := makeTraits(opts)
	if err != nil {
		return nil, err
	}
	library, err := abi.Open(opts.LibraryPath)
	if err != nil {
		if errors.Is(err, abi.ErrUnsupported) {
			return nil, errors.Join(ErrUnsupported, err)
		}
		return nil, err
	}
	api := library.API()
	for _, slot := range []string{"setup", "initialize", "finalize"} {
		if !api.Available(slot) {
			return nil, errors.Join(unsupported(slot), library.Close())
		}
	}
	e := &Engine{
		api:      api,
		sessions: make(map[*Session]struct{}),
		configs:  make(map[*Config]struct{}),
	}
	pinTraits(traits, len(opts.Modules), &e.traitsPin)
	if traits.AppName != nil {
		process.appNamePin.Pin(traits.AppName)
	}
	process.library = library
	process.active = e
	api.Setup(traits)
	e.registerNotifications()
	api.Initialize(traits)
	return e, nil
}

// Close waits for native maintenance, closes child resources, and finalizes
// librime. It is safe to call Close more than once. The library stays resident
// because dependencies may keep process-wide state after finalization.
func (e *Engine) Close() error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.api == nil || e.closed {
		return nil
	}

	// Upstream handler assignment is unsynchronized with notification dispatch.
	// Join the native worker before changing the handler or closing its clients.
	e.joinMaintenanceLocked()
	closeErr := e.closeConfigsLocked()
	for s := range e.sessions {
		closeErr = errors.Join(closeErr, s.closeLocked())
	}
	e.unregisterNotifications()
	e.api.Finalize()
	e.traitsPin.Unpin()
	e.closed = true
	process.mu.Lock()
	process.active = nil
	process.mu.Unlock()
	return closeErr
}

// Version returns the version reported by the native library.
func (e *Engine) Version() (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkOpenLocked(); err != nil {
		return "", err
	}
	if e.api.GetVersion == nil {
		return "", unsupported("get_version")
	}
	return abi.GoString(e.api.GetVersion()), nil
}

// Available reports whether the ABI can call the named C API slot, for example
// "get_candidate_preview".
func (e *Engine) Available(slot string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkOpenLocked(); err != nil {
		return false, err
	}
	return e.api.Available(slot), nil
}

func (e *Engine) checkOpenLocked() error {
	if e.api == nil || e.closed {
		return ErrClosed
	}
	return nil
}

func (e *Engine) checkLocked() error {
	if err := e.checkOpenLocked(); err != nil {
		return err
	}
	return e.checkMaintenanceLocked()
}

func (e *Engine) invalidateSessionsLocked() {
	for s := range e.sessions {
		s.closed = true
	}
	clear(e.sessions)
}
