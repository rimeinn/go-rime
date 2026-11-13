package rime

import "github.com/rimeinn/go-rime/abi"

// Status is a snapshot of the active schema and input mode.
type Status struct {
	SchemaID    string
	SchemaName  string
	Disabled    bool
	Composing   bool
	ASCIIMode   bool
	FullShape   bool
	Simplified  bool
	Traditional bool
	ASCIIPunct  bool
}

// Status returns a copy of the native status. ok reports whether librime
// supplied a status.
func (s *Session) Status() (status Status, ok bool, err error) {
	e, err := s.lock()
	if err != nil {
		return Status{}, false, err
	}
	defer e.mu.Unlock()
	if e.api.GetStatus == nil {
		return Status{}, false, unsupported("get_status")
	}
	if e.api.FreeStatus == nil {
		return Status{}, false, unsupported("free_status")
	}
	var raw abi.Status
	raw.Init()
	if !e.api.GetStatus(s.id, &raw).GoBool() {
		return Status{}, false, nil
	}
	defer func() {
		if !e.api.FreeStatus(&raw).GoBool() {
			err = failed("free_status")
		}
	}()
	return Status{
		SchemaID:    abi.GoString(raw.SchemaID),
		SchemaName:  abi.GoString(raw.SchemaName),
		Disabled:    raw.IsDisabled.GoBool(),
		Composing:   raw.IsComposing.GoBool(),
		ASCIIMode:   raw.IsASCIIMode.GoBool(),
		FullShape:   raw.IsFullShape.GoBool(),
		Simplified:  raw.IsSimplified.GoBool(),
		Traditional: raw.IsTraditional.GoBool(),
		ASCIIPunct:  raw.IsASCIIPunct.GoBool(),
	}, true, nil
}
