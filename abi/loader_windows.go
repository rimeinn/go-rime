//go:build windows && (amd64 || arm64)

package abi

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	loadLibrarySearchDLLLoadDir  = 0x00000100
	loadLibrarySearchDefaultDirs = 0x00001000
)

var loadLibraryExW = syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")

func defaultLibraryPaths() []string {
	names := []string{"rime.dll", "librime.dll"}
	paths := append([]string(nil), names...)
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.IsAbs(dir) {
			for _, name := range names {
				paths = append(paths, filepath.Join(dir, name))
			}
		}
	}
	return paths
}

func openNative(path string) (uintptr, error) {
	flags := uintptr(loadLibrarySearchDefaultDirs)
	if filepath.Base(path) != path {
		var err error
		path, err = filepath.Abs(path)
		if err != nil {
			return 0, err
		}
		flags |= loadLibrarySearchDLLLoadDir
	}
	wide, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	handle, _, callErr := loadLibraryExW.Call(uintptr(unsafe.Pointer(wide)), 0, flags)
	runtime.KeepAlive(wide)
	if handle == 0 {
		return 0, callErr
	}
	return handle, nil
}

func lookupNative(handle uintptr, name string) (uintptr, error) {
	return syscall.GetProcAddress(syscall.Handle(handle), name)
}

func closeNative(handle uintptr) error { return syscall.FreeLibrary(syscall.Handle(handle)) }
