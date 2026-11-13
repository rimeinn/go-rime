//go:build (windows || darwin || linux) && (amd64 || arm64)

package abi

import "github.com/ebitengine/purego"

const platformSupported = true

func registerFunc(dst any, address uintptr) { purego.RegisterFunc(dst, address) }

func registerLibFunc(dst any, handle uintptr, symbol string) error {
	// Resolve first because RegisterLibFunc would panic for a missing export.
	address, err := lookupNative(handle, symbol)
	if err != nil {
		return err
	}
	purego.RegisterFunc(dst, address)
	return nil
}

// NewNotificationCallback creates a native callback for handler. Purego's
// callback slots are finite and permanent. Reuse a trampoline for subscriptions
// and pass integer tokens as context. A nil handler yields address zero.
func NewNotificationCallback(handler NotificationHandler) uintptr {
	if handler == nil {
		return 0
	}
	// Windows requires a uintptr return value. The void C caller discards it.
	return purego.NewCallback(func(context uintptr, sessionID SessionID, messageType, messageValue *byte) uintptr {
		handler(context, sessionID, messageType, messageValue)
		return 0
	})
}
