package rime

import "github.com/rimeinn/go-rime/abi"

// SetOption updates a named session option.
func (s *Session) SetOption(name string, value bool) error {
	e, err := s.lock()
	if err != nil {
		return err
	}
	defer e.mu.Unlock()
	if e.api.SetOption == nil {
		return unsupported("set_option")
	}
	p, err := cString(name)
	if err != nil {
		return err
	}
	e.api.SetOption(s.id, p, abi.BoolValue(value))
	return nil
}

// GetOption reports the value of a named session option. Native options
// default to false when unset.
func (s *Session) GetOption(name string) (bool, error) {
	e, err := s.lock()
	if err != nil {
		return false, err
	}
	defer e.mu.Unlock()
	if e.api.GetOption == nil {
		return false, unsupported("get_option")
	}
	p, err := cString(name)
	if err != nil {
		return false, err
	}
	return e.api.GetOption(s.id, p).GoBool(), nil
}

// SetProperty updates a named session property.
func (s *Session) SetProperty(name, value string) error {
	e, err := s.lock()
	if err != nil {
		return err
	}
	defer e.mu.Unlock()
	if e.api.SetProperty == nil {
		return unsupported("set_property")
	}
	key, err := cString(name)
	if err != nil {
		return err
	}
	p, err := cString(value)
	if err != nil {
		return err
	}
	e.api.SetProperty(s.id, key, p)
	return nil
}

// GetProperty returns a named property. librime reports unset and empty
// properties as absent.
func (s *Session) GetProperty(name string) (string, bool, error) {
	e, err := s.lock()
	if err != nil {
		return "", false, err
	}
	defer e.mu.Unlock()
	if e.api.GetProperty == nil {
		return "", false, unsupported("get_property")
	}
	key, err := cString(name)
	if err != nil {
		return "", false, err
	}
	return readCStringBuffer(func(buf *byte, size uintptr) bool {
		return e.api.GetProperty(s.id, key, buf, size).GoBool()
	})
}

// StateLabel copies the configured label for an option state. ok is false if
// the schema supplies no label for that state.
func (s *Session) StateLabel(name string, state, abbreviated bool) (string, bool, error) {
	e, err := s.lock()
	if err != nil {
		return "", false, err
	}
	defer e.mu.Unlock()
	p, err := cString(name)
	if err != nil {
		return "", false, err
	}
	if e.api.GetStateLabelAbbreviated != nil {
		label := e.api.GetStateLabelAbbreviated(s.id, p, abi.BoolValue(state), abi.BoolValue(abbreviated))
		return label.String(), label.Str != nil, nil
	}
	if abbreviated {
		return "", false, unsupported("get_state_label_abbreviated")
	}
	if e.api.GetStateLabel == nil {
		return "", false, unsupported("get_state_label")
	}
	label := e.api.GetStateLabel(s.id, p, abi.BoolValue(state))
	return abi.GoString(label), label != nil, nil
}
