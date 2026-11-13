//go:build (windows || linux || darwin) && (amd64 || arm64)

package abi

import (
	"fmt"

	"github.com/ebitengine/purego"
)

func newModuleCallback(fn func() uintptr) (address uintptr, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("allocate Go module callback: %v", failure)
		}
	}()
	return purego.NewCallback(fn), nil
}
