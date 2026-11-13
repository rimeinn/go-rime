package rime

import (
	"errors"
	"strings"
	"testing"
	"unsafe"

	"github.com/rimeinn/go-rime/abi"
)

func TestContextCopiesAndReleasesNativeData(t *testing.T) {
	text := []byte("你好\x00")
	label := []byte("一\x00")
	candidate := abi.Candidate{Text: &text[0]}
	selectLabel := &label[0]
	var freed bool
	e := &Engine{api: &abi.API{
		GetContext: func(_ abi.SessionID, out *abi.Context) abi.Bool {
			out.Menu = abi.Menu{PageSize: 1, NumCandidates: 1, Candidates: &candidate}
			out.SelectLabels = &selectLabel
			return abi.True
		},
	}, sessions: make(map[*Session]struct{})}
	s := &Session{engine: e, id: 1}
	e.sessions[s] = struct{}{}
	if _, _, err := s.Context(); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Context without native release returned %v", err)
	}
	e.api.FreeContext = func(_ *abi.Context) abi.Bool {
		clear(text)
		clear(label)
		freed = true
		return abi.True
	}
	ctx, ok, err := s.Context()
	if err != nil || !ok || !freed {
		t.Fatalf("Context returned ok=%v, err=%v, freed=%v", ok, err, freed)
	}
	if ctx.Menu.Candidates[0].Text != "你好" || ctx.SelectLabels[0] != "一" {
		t.Fatalf("Context changed during native release: %+v", ctx)
	}
}

func TestReadCStringBufferRetriesTruncation(t *testing.T) {
	// An exact buffer-length string leaves no terminator after C strncpy.
	want := strings.Repeat("a", 256)
	got, ok, err := readCStringBuffer(func(out *byte, size uintptr) bool {
		copy(unsafe.Slice(out, size), want)
		return true
	})
	if err != nil || !ok || got != want {
		t.Fatalf("buffer read returned %q, %v, %v", got, ok, err)
	}
}
