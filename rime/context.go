package rime

import (
	"errors"
	"unsafe"

	"github.com/rimeinn/go-rime/abi"
)

// Context is a snapshot of the preedit and candidate menu, owned by Go.
type Context struct {
	Composition       Composition
	Menu              Menu
	CommitTextPreview string
	SelectLabels      []string
}

// Composition contains the preedit string and its byte length and offsets.
type Composition struct {
	Preedit   string
	Length    int
	CursorPos int
	SelStart  int
	SelEnd    int
}

// Candidate is a copy of a native candidate's text and comment.
type Candidate struct {
	Text    string
	Comment string
}

// Menu contains the current page of candidates and its selection metadata.
// PageNo and HighlightedCandidateIndex are zero-based.
type Menu struct {
	PageSize                  int
	PageNo                    int
	IsLastPage                bool
	HighlightedCandidateIndex int
	Candidates                []Candidate
	SelectKeys                string
}

// Context copies the session's current composition and candidate menu. ok
// reports whether librime supplied a context.
func (s *Session) Context() (ctx Context, ok bool, err error) {
	e, err := s.lock()
	if err != nil {
		return Context{}, false, err
	}
	defer e.mu.Unlock()
	if e.api.GetContext == nil {
		return Context{}, false, unsupported("get_context")
	}
	if e.api.FreeContext == nil {
		return Context{}, false, unsupported("free_context")
	}
	var raw abi.Context
	raw.Init()
	if !e.api.GetContext(s.id, &raw).GoBool() {
		return Context{}, false, nil
	}
	defer func() {
		if !e.api.FreeContext(&raw).GoBool() {
			err = errors.Join(err, failed("free_context"))
		}
	}()
	ctx, err = copyContext(&raw)
	return ctx, err == nil, err
}

func copyContext(raw *abi.Context) (Context, error) {
	m := raw.Menu
	if m.NumCandidates < 0 || (m.NumCandidates > 0 && m.Candidates == nil) || m.PageSize < 0 {
		return Context{}, failed("get_context returned invalid menu metadata")
	}
	ctx := Context{
		Composition: Composition{
			Preedit:   abi.GoString(raw.Composition.Preedit),
			Length:    int(raw.Composition.Length),
			CursorPos: int(raw.Composition.CursorPos),
			SelStart:  int(raw.Composition.SelStart),
			SelEnd:    int(raw.Composition.SelEnd),
		},
		Menu: Menu{
			PageSize:                  int(m.PageSize),
			PageNo:                    int(m.PageNo),
			IsLastPage:                m.IsLastPage.GoBool(),
			HighlightedCandidateIndex: int(m.HighlightedCandidateIndex),
			SelectKeys:                abi.GoString(m.SelectKeys),
		},
		CommitTextPreview: abi.GoString(raw.CommitTextPreview),
	}
	if m.NumCandidates > 0 {
		ctx.Menu.Candidates = make([]Candidate, int(m.NumCandidates))
		for i, c := range unsafe.Slice(m.Candidates, int(m.NumCandidates)) {
			ctx.Menu.Candidates[i] = copyCandidate(c)
		}
	}
	if raw.SelectLabels != nil && m.PageSize > 0 {
		ctx.SelectLabels = make([]string, int(m.PageSize))
		for i, label := range unsafe.Slice(raw.SelectLabels, int(m.PageSize)) {
			ctx.SelectLabels[i] = abi.GoString(label)
		}
	}
	return ctx, nil
}

func copyCandidate(raw abi.Candidate) Candidate {
	return Candidate{Text: abi.GoString(raw.Text), Comment: abi.GoString(raw.Comment)}
}
