//go:build (!windows && !linux && !darwin) || (!amd64 && !arm64)

package abi

func newModuleCallback(func() uintptr) (uintptr, error) {
	return 0, ErrUnsupported
}
