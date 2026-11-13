package rime

import (
	"fmt"
	"math"

	"github.com/rimeinn/go-rime/abi"
)

// SelectCandidate selects a zero-based index in the full candidate list.
func (s *Session) SelectCandidate(index int) (bool, error) {
	return s.candidateAction(index, "select_candidate", func(a *abi.API) candidateOperation { return a.SelectCandidate })
}

// SelectCandidateOnCurrentPage selects a zero-based index on the current page.
func (s *Session) SelectCandidateOnCurrentPage(index int) (bool, error) {
	return s.candidateAction(index, "select_candidate_on_current_page", func(a *abi.API) candidateOperation {
		return a.SelectCandidateOnCurrentPage
	})
}

// HighlightCandidate moves the highlight to a zero-based index in the full
// candidate list. librime reports whether the highlight changed.
func (s *Session) HighlightCandidate(index int) (bool, error) {
	return s.candidateAction(index, "highlight_candidate", func(a *abi.API) candidateOperation { return a.HighlightCandidate })
}

// HighlightCandidateOnCurrentPage moves the highlight to a zero-based index
// on the current page. librime reports whether the highlight changed.
func (s *Session) HighlightCandidateOnCurrentPage(index int) (bool, error) {
	return s.candidateAction(index, "highlight_candidate_on_current_page", func(a *abi.API) candidateOperation {
		return a.HighlightCandidateOnCurrentPage
	})
}

// DeleteCandidate requests deletion of the candidate at the given zero-based
// index. The bool reports whether librime handled the request.
func (s *Session) DeleteCandidate(index int) (bool, error) {
	e, err := s.lock()
	if err != nil {
		return false, err
	}
	defer e.mu.Unlock()
	if index < 0 || index > math.MaxInt32 {
		return false, fmt.Errorf("%w: invalid candidate index %d", ErrInvalidArgument, index)
	}
	a := e.api
	if a.DeleteCandidate == nil {
		return false, unsupported("delete_candidate")
	}
	if a.CandidateListFromIndex == nil || a.CandidateListNext == nil || a.CandidateListEnd == nil {
		return false, unsupported("candidate_list_from_index/next/end required for delete_candidate")
	}
	// Debug builds of librime dereference the indexed candidate during delete.
	// Verify the index while the same menu remains current.
	var iter abi.CandidateListIterator
	if !a.CandidateListFromIndex(s.id, &iter, int32(index)).GoBool() {
		return false, nil
	}
	exists := a.CandidateListNext(&iter).GoBool()
	a.CandidateListEnd(&iter)
	if !exists {
		return false, nil
	}
	return a.DeleteCandidate(s.id, uintptr(index)).GoBool(), nil
}

// DeleteCandidateOnCurrentPage requests deletion by zero-based page index.
// The bool reports whether librime handled the request.
func (s *Session) DeleteCandidateOnCurrentPage(index int) (bool, error) {
	e, err := s.lock()
	if err != nil {
		return false, err
	}
	defer e.mu.Unlock()
	if index < 0 {
		return false, fmt.Errorf("%w: negative candidate index", ErrInvalidArgument)
	}
	a := e.api
	if a.DeleteCandidateOnCurrentPage == nil {
		return false, unsupported("delete_candidate_on_current_page")
	}
	if a.GetContext == nil || a.FreeContext == nil {
		return false, unsupported("get_context/free_context required for delete_candidate_on_current_page")
	}
	var ctx abi.Context
	ctx.Init()
	if !a.GetContext(s.id, &ctx).GoBool() {
		return false, nil
	}
	exists := index < int(ctx.Menu.NumCandidates)
	if !a.FreeContext(&ctx).GoBool() {
		return false, failed("free_context")
	}
	if !exists {
		return false, nil
	}
	return a.DeleteCandidateOnCurrentPage(s.id, uintptr(index)).GoBool(), nil
}

// ChangePage asks librime to move the highlight to an adjacent candidate page.
func (s *Session) ChangePage(backward bool) (bool, error) {
	e, err := s.lock()
	if err != nil {
		return false, err
	}
	defer e.mu.Unlock()
	if e.api.ChangePage == nil {
		return false, unsupported("change_page")
	}
	return e.api.ChangePage(s.id, abi.BoolValue(backward)).GoBool(), nil
}

type candidateOperation = func(abi.SessionID, uintptr) abi.Bool

func (s *Session) candidateAction(index int, slot string, get func(*abi.API) candidateOperation) (bool, error) {
	e, err := s.lock()
	if err != nil {
		return false, err
	}
	defer e.mu.Unlock()
	if index < 0 {
		return false, fmt.Errorf("%w: negative candidate index", ErrInvalidArgument)
	}
	call := get(e.api)
	if call == nil {
		return false, unsupported(slot)
	}
	return call(s.id, uintptr(index)).GoBool(), nil
}
