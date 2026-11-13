package rime

import (
	"fmt"

	"github.com/rimeinn/go-rime/abi"
)

// Session owns a native input session. Its operations serialize with other
// operations on the engine. A Session must not be copied.
type Session struct {
	engine *Engine
	id     abi.SessionID
	closed bool
}

// NewSession creates a new input session owned by the engine.
func (e *Engine) NewSession() (*Session, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkLocked(); err != nil {
		return nil, err
	}
	if e.api.CreateSession == nil {
		return nil, unsupported("create_session")
	}
	if e.api.DestroySession == nil {
		return nil, unsupported("destroy_session")
	}
	id := e.api.CreateSession()
	if id == 0 {
		return nil, failed("create_session")
	}
	s := &Session{engine: e, id: id}
	e.sessions[s] = struct{}{}
	return s, nil
}

// ID returns the session ID used by librime notifications. Native IDs can be
// reused after a session closes.
func (s *Session) ID() uintptr { return s.id }

// Close releases the session. It is safe to call Close more than once.
func (s *Session) Close() error {
	if s == nil || s.engine == nil {
		return nil
	}
	e := s.engine
	e.mu.Lock()
	defer e.mu.Unlock()
	if s.closed || e.closed {
		return nil
	}
	if _, ok := e.sessions[s]; !ok {
		return ErrClosed
	}
	if e.maintenancePending {
		e.joinMaintenanceLocked()
	}
	return s.closeLocked()
}

func (s *Session) closeLocked() error {
	s.closed = true
	delete(s.engine.sessions, s)
	if !s.engine.api.DestroySession(s.id).GoBool() {
		return failed("destroy_session")
	}
	return nil
}

func (s *Session) lock() (*Engine, error) {
	if s == nil || s.engine == nil {
		return nil, ErrClosed
	}
	e := s.engine
	e.mu.Lock()
	if _, ok := e.sessions[s]; !ok || s.closed {
		e.mu.Unlock()
		return nil, ErrClosed
	}
	if err := e.checkLocked(); err != nil {
		e.mu.Unlock()
		return nil, err
	}
	return e, nil
}

// ProcessKey sends an X11 keysym and librime modifier mask. The returned bool
// reports whether librime handled the key.
func (s *Session) ProcessKey(keycode, mask int32) (bool, error) {
	e, err := s.lock()
	if err != nil {
		return false, err
	}
	defer e.mu.Unlock()
	if e.api.ProcessKey == nil {
		return false, unsupported("process_key")
	}
	return e.api.ProcessKey(s.id, keycode, mask).GoBool(), nil
}

// CommitComposition commits the active composition. The result reports whether
// unread text is available to TakeCommit.
func (s *Session) CommitComposition() (bool, error) {
	e, err := s.lock()
	if err != nil {
		return false, err
	}
	defer e.mu.Unlock()
	if e.api.CommitComposition == nil {
		return false, unsupported("commit_composition")
	}
	return e.api.CommitComposition(s.id).GoBool(), nil
}

// ClearComposition aborts the active composition.
func (s *Session) ClearComposition() error {
	e, err := s.lock()
	if err != nil {
		return err
	}
	defer e.mu.Unlock()
	if e.api.ClearComposition == nil {
		return unsupported("clear_composition")
	}
	e.api.ClearComposition(s.id)
	return nil
}

// TakeCommit consumes and copies pending committed text. ok is false if there
// is no unread commit.
func (s *Session) TakeCommit() (text string, ok bool, err error) {
	e, err := s.lock()
	if err != nil {
		return "", false, err
	}
	defer e.mu.Unlock()
	if e.api.GetCommit == nil {
		return "", false, unsupported("get_commit")
	}
	if e.api.FreeCommit == nil {
		return "", false, unsupported("free_commit")
	}
	var raw abi.Commit
	raw.Init()
	if !e.api.GetCommit(s.id, &raw).GoBool() {
		return "", false, nil
	}
	defer func() {
		if !e.api.FreeCommit(&raw).GoBool() {
			err = failed("free_commit")
		}
	}()
	return abi.GoString(raw.Text), true, nil
}

// GetInput returns a copy of the raw input string.
func (s *Session) GetInput() (string, error) {
	e, err := s.lock()
	if err != nil {
		return "", err
	}
	defer e.mu.Unlock()
	if e.api.GetInput == nil {
		return "", unsupported("get_input")
	}
	p := e.api.GetInput(s.id)
	if p == nil {
		return "", failed("get_input")
	}
	return abi.GoString(p), nil
}

// SetInput replaces the raw input string and reports whether librime applied it.
func (s *Session) SetInput(input string) (bool, error) {
	return s.stringOperation(input, "set_input", func(a *abi.API) sessionStringOperation { return a.SetInput })
}

// GetCaretPos returns the caret's byte offset in the raw input string.
func (s *Session) GetCaretPos() (int, error) {
	e, err := s.lock()
	if err != nil {
		return 0, err
	}
	defer e.mu.Unlock()
	if e.api.GetCaretPos == nil {
		return 0, unsupported("get_caret_pos")
	}
	pos := e.api.GetCaretPos(s.id)
	if pos > uintptr(^uint(0)>>1) {
		return 0, failed("get_caret_pos")
	}
	return int(pos), nil
}

// SetCaretPos moves the caret to a byte offset in the raw input string.
// librime clamps offsets past the end to the length of the input.
func (s *Session) SetCaretPos(pos int) error {
	e, err := s.lock()
	if err != nil {
		return err
	}
	defer e.mu.Unlock()
	if pos < 0 {
		return fmt.Errorf("%w: negative caret position", ErrInvalidArgument)
	}
	if e.api.SetCaretPos == nil {
		return unsupported("set_caret_pos")
	}
	e.api.SetCaretPos(s.id, uintptr(pos))
	return nil
}

// SimulateKeySequence sends a sequence in librime notation, such as "ni{space}".
func (s *Session) SimulateKeySequence(sequence string) (bool, error) {
	return s.stringOperation(sequence, "simulate_key_sequence", func(a *abi.API) sessionStringOperation {
		return a.SimulateKeySequence
	})
}

type sessionStringOperation = func(abi.SessionID, *byte) abi.Bool

func (s *Session) stringOperation(value, slot string, get func(*abi.API) sessionStringOperation) (bool, error) {
	e, err := s.lock()
	if err != nil {
		return false, err
	}
	defer e.mu.Unlock()
	call := get(e.api)
	if call == nil {
		return false, unsupported(slot)
	}
	p, err := cString(value)
	if err != nil {
		return false, err
	}
	return call(s.id, p).GoBool(), nil
}
