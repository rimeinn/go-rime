//go:build !((windows || darwin || linux) && (amd64 || arm64))

package abi

const platformSupported = false

func defaultLibraryPaths() []string              { return nil }
func openNative(string) (uintptr, error)         { return 0, ErrUnsupported }
func closeNative(uintptr) error                  { return ErrUnsupported }
func registerLibFunc(any, uintptr, string) error { return ErrUnsupported }
func registerFunc(any, uintptr)                  { panic(ErrUnsupported) }

// NewNotificationCallback returns zero on this unsupported platform.
func NewNotificationCallback(NotificationHandler) uintptr { return 0 }
