package abi

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"unsafe"
)

// GoModule describes a module for the default librime ABI. Initialize and
// Finalize, when supplied, run synchronously on the native caller's thread.
// They must return normally and must not reenter librime. CustomSlots are C
// function addresses in the order expected by the consumer's extension table.
type GoModule struct {
	Name        string
	Initialize  func()
	Finalize    func()
	CustomSlots []uintptr
}

type registeredGoModule struct {
	pin    runtime.Pinner
	module Module
}

var goModules struct {
	sync.Mutex
	registered []*registeredGoModule
}

// RegisterGoModule registers a Go-backed module after Setup and before module
// loading. Include its name in Traits.Modules to load it. Serialize this with
// other calls to librime. The module, extension table, callbacks and native
// library stay resident because the module registry has no unregister API.
func (l *Library) RegisterGoModule(definition GoModule) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	api := l.api
	if api == nil || api.RegisterModule == nil || api.FindModule == nil {
		return fmt.Errorf("register Go module: %w", ErrUnsupported)
	}
	if definition.Name == "" {
		return fmt.Errorf("register Go module: empty name")
	}
	name, err := CString(definition.Name)
	if err != nil {
		return fmt.Errorf("register Go module: %w", err)
	}
	wordSize := unsafe.Sizeof(uintptr(0))
	if len(definition.CustomSlots) > int((math.MaxInt32-wordSize)/wordSize) {
		return fmt.Errorf("register Go module: extension table exceeds C data_size")
	}

	goModules.Lock()
	defer goModules.Unlock()
	if api.FindModule(name) != nil {
		return fmt.Errorf("register Go module %q: name already registered", definition.Name)
	}
	registration := &registeredGoModule{}
	registration.module.Init()
	registration.module.ModuleName = name
	registration.pin.Pin(&registration.module)
	registration.pin.Pin(name)
	registered := false
	defer func() {
		if !registered {
			registration.pin.Unpin()
		}
	}()
	if initialize := definition.Initialize; initialize != nil {
		registration.module.Initialize, err = newModuleCallback(func() uintptr {
			initialize()
			return 0 // The native void callback ignores the return register.
		})
		if err != nil {
			return err
		}
	}
	if finalize := definition.Finalize; finalize != nil {
		registration.module.Finalize, err = newModuleCallback(func() uintptr {
			finalize()
			return 0
		})
		if err != nil {
			return err
		}
	}
	if len(definition.CustomSlots) != 0 {
		words := make([]uintptr, 1+len(definition.CustomSlots))
		copy(words[1:], definition.CustomSlots)
		header := (*CustomAPI)(unsafe.Pointer(&words[0]))
		header.DataSize = int32(uintptr(len(words))*wordSize - unsafe.Sizeof(header.DataSize))
		registration.pin.Pin(header)
		address := uintptr(unsafe.Pointer(header))
		registration.module.GetAPI, err = newModuleCallback(func() uintptr { return address })
		if err != nil {
			return err
		}
	}
	if !api.RegisterModule(&registration.module).GoBool() {
		return fmt.Errorf("register Go module %q: librime rejected registration", definition.Name)
	}
	goModules.registered = append(goModules.registered, registration)
	l.retainedForModule = true
	registered = true
	return nil
}
