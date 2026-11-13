package abi

import (
	"errors"
	"fmt"
	"unsafe"
)

// Capability describes the availability of a versioned C API slot.
type Capability uint8

const (
	CapabilityUnknown     Capability = iota // This binding has no slot with that name.
	CapabilityMissing                       // The slot falls outside the reported table size.
	CapabilityNull                          // The slot is in range and librime left it NULL.
	CapabilityUnsupported                   // The platform cannot call this slot.
	CapabilityAvailable                     // The slot is bound and callable.
)

type tableView struct {
	base unsafe.Pointer
	size int32
}

func newTable(base unsafe.Pointer) (tableView, error) {
	if base == nil {
		return tableView{}, errors.New("NULL API table")
	}
	size := *(*int32)(base)
	if size < 0 {
		return tableView{}, fmt.Errorf("invalid data_size %d", size)
	}
	return tableView{base: base, size: size}, nil
}

func (v tableView) bind(offset uintptr, fn any) Capability {
	if !StructHasMember(v.size, offset, unsafe.Sizeof(uintptr(0))) {
		return CapabilityMissing
	}
	address := *(*uintptr)(unsafe.Add(v.base, offset))
	if address == 0 {
		return CapabilityNull
	}
	if !platformSupported {
		return CapabilityUnsupported
	}
	registerFunc(fn, address)
	return CapabilityAvailable
}

// CustomAPI calls the module's get_api callback if that field is available.
// The returned native pointer is borrowed and may be nil.
func (m *Module) CustomAPI() (*CustomAPI, error) {
	if m == nil || !StructHasMember(m.DataSize, unsafe.Offsetof(m.GetAPI), unsafe.Sizeof(m.GetAPI)) || m.GetAPI == 0 || !platformSupported {
		return nil, fmt.Errorf("%w: module get_api", ErrUnsupported)
	}
	var getAPI func() *CustomAPI
	registerFunc(&getAPI, m.GetAPI)
	return getAPI(), nil
}

// Levers binds the integer-Bool custom API published by the levers module.
// Release its schema lists with SchemaListDestroy, its custom settings with
// CustomSettingsDestroy and its user dictionary iterators with
// UserDictIteratorDestroy. SettingsGetConfig borrows the settings' config.
// SwitcherSettings can be cast to *CustomSettings for lifecycle functions.
func (a *API) Levers() (*LeversAPI, error) {
	if a == nil || a.FindModule == nil {
		return nil, fmt.Errorf("%w: find_module", ErrUnsupported)
	}
	name := [...]byte{'l', 'e', 'v', 'e', 'r', 's', 0}
	mod := a.FindModule(&name[0])
	if mod == nil {
		return nil, fmt.Errorf("%w: levers module", ErrUnsupported)
	}
	custom, err := mod.CustomAPI()
	if err != nil {
		return nil, err
	}
	table, err := newTable(unsafe.Pointer(custom))
	if err != nil {
		return nil, fmt.Errorf("levers: %w", err)
	}
	return bindLeversAPI(table), nil
}

// NotificationHandler is the Go signature of a librime notification callback.
// Context is an opaque token. Message strings are borrowed for the duration of
// the callback, which may run on a native thread.
type NotificationHandler func(context uintptr, sessionID SessionID, messageType, messageValue *byte)
