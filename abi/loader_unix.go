//go:build (darwin || linux) && (amd64 || arm64)

package abi

import (
	"runtime"

	"github.com/ebitengine/purego"
)

func defaultLibraryPaths() []string {
	if runtime.GOOS == "darwin" {
		return []string{"librime.1.dylib", "librime.dylib"}
	}
	return []string{"librime.so.1", "librime.so"}
}

func openNative(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
}

func lookupNative(handle uintptr, name string) (uintptr, error) {
	return purego.Dlsym(handle, name)
}

func closeNative(handle uintptr) error { return purego.Dlclose(handle) }
