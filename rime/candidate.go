package rime

import (
	"fmt"
	"math"

	"github.com/rimeinn/go-rime/abi"
)

// Candidates copies up to limit candidates, beginning at the zero-based start
// index in the full candidate list. A session without a menu returns no
// candidates. The returned slice can outlive the session.
func (s *Session) Candidates(start, limit int) ([]Candidate, error) {
	e, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer e.mu.Unlock()
	if start < 0 || start > math.MaxInt32 || limit < 0 || limit-1 > math.MaxInt32-start {
		return nil, fmt.Errorf("%w: candidate range (%d, %d) is outside the C int range", ErrInvalidArgument, start, limit)
	}
	if limit == 0 {
		return nil, nil
	}
	a := e.api
	if a.CandidateListNext == nil {
		return nil, unsupported("candidate_list_next")
	}
	if a.CandidateListEnd == nil {
		return nil, unsupported("candidate_list_end")
	}
	var raw abi.CandidateListIterator
	var started bool
	switch {
	case a.CandidateListBegin != nil && (start == 0 || a.CandidateListFromIndex == nil):
		started = a.CandidateListBegin(s.id, &raw).GoBool()
	case a.CandidateListFromIndex != nil:
		started = a.CandidateListFromIndex(s.id, &raw, int32(start)).GoBool()
	default:
		return nil, unsupported("candidate_list_begin")
	}
	if !started {
		return nil, nil
	}
	defer a.CandidateListEnd(&raw)
	var candidates []Candidate
	for len(candidates) < limit && a.CandidateListNext(&raw).GoBool() {
		if int(raw.Index) >= start {
			candidates = append(candidates, copyCandidate(raw.Candidate))
		}
	}
	return candidates, nil
}

// CandidatePreview is a copy of the composition around the selected candidate.
type CandidatePreview struct {
	TextBeforeSelection string
	SelectedText        string
	TextAfterSelection  string
}

// CandidatePreview copies the composition around the selected candidate. It
// returns ok=false when there is no preview.
func (s *Session) CandidatePreview() (preview CandidatePreview, ok bool, err error) {
	e, err := s.lock()
	if err != nil {
		return preview, false, err
	}
	defer e.mu.Unlock()
	a := e.api
	if a.GetCandidatePreview == nil {
		return preview, false, unsupported("get_candidate_preview")
	}
	if a.FreeCandidatePreview == nil {
		return preview, false, unsupported("free_candidate_preview")
	}
	var raw abi.CandidatePreview
	raw.Init()
	if !a.GetCandidatePreview(s.id, &raw).GoBool() {
		return preview, false, nil
	}
	defer func() {
		if !a.FreeCandidatePreview(&raw).GoBool() {
			preview, ok, err = CandidatePreview{}, false, failed("free_candidate_preview")
		}
	}()
	return CandidatePreview{
		TextBeforeSelection: abi.GoString(raw.TextBeforeSelection),
		SelectedText:        abi.GoString(raw.SelectedText),
		TextAfterSelection:  abi.GoString(raw.TextAfterSelection),
	}, true, nil
}
