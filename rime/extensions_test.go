package rime

import (
	"errors"
	"slices"
	"testing"
	"unsafe"

	"github.com/rimeinn/go-rime/abi"
)

func TestConfigIterationReleasesNativeIterator(t *testing.T) {
	key, err := abi.CString("@0")
	if err != nil {
		t.Fatal(err)
	}
	path, err := abi.CString("list/@0")
	if err != nil {
		t.Fatal(err)
	}
	var handle byte
	var ended, closed bool
	e := &Engine{
		configs: make(map[*Config]struct{}),
		api: &abi.API{
			ConfigInit: func(c *abi.Config) abi.Bool {
				c.Ptr = unsafe.Pointer(&handle)
				return abi.True
			},
			ConfigClose: func(c *abi.Config) abi.Bool {
				closed = true
				c.Ptr = nil
				return abi.True
			},
			ConfigBeginList: func(it *abi.ConfigIterator, _ *abi.Config, _ *byte) abi.Bool {
				it.Index = -1
				return abi.True
			},
			ConfigNext: func(it *abi.ConfigIterator) abi.Bool {
				it.Index++
				it.Key, it.Path = key, path
				return abi.BoolValue(it.Index == 0)
			},
			ConfigEnd: func(_ *abi.ConfigIterator) {
				ended = true
				*key, *path = 0, 0
			},
		},
	}
	c, err := e.NewConfig()
	if err != nil {
		t.Fatal(err)
	}
	entries, ok, err := c.IterateList("list")
	if !ok || err != nil || !ended {
		t.Fatalf("iteration returned ok=%v, err=%v, native iterator ended=%v", ok, err, ended)
	}
	if err := c.Close(); err != nil || !closed {
		t.Fatalf("Close returned %v, native config closed=%v", err, closed)
	}
	if got, want := slices.Collect(entries), []ConfigEntry{{Key: "@0", Path: "list/@0"}}; !slices.Equal(got, want) {
		t.Fatalf("config entries after close = %+v, want %+v", got, want)
	}
	if _, _, err := c.IterateList("list"); !errors.Is(err, ErrClosed) {
		t.Fatalf("IterateList after Close returned %v", err)
	}
}

func TestMaintenanceGatesNativeOperations(t *testing.T) {
	var maintaining bool
	var joins int
	e := &Engine{api: &abi.API{
		StartMaintenance: func(_ abi.Bool) abi.Bool {
			maintaining = true
			return abi.True
		},
		IsMaintenanceMode: func() abi.Bool { return abi.BoolValue(maintaining) },
		JoinMaintenanceThread: func() {
			maintaining = false
			joins++
		},
	}}
	if started, err := e.StartMaintenance(true); !started || err != nil {
		t.Fatalf("StartMaintenance returned %v, %v", started, err)
	}
	if _, err := e.NewConfig(); !errors.Is(err, ErrMaintenance) {
		t.Fatalf("NewConfig during maintenance returned %v", err)
	}
	if active, err := e.IsMaintaining(); !active || err != nil {
		t.Fatalf("IsMaintaining returned %v, %v", active, err)
	}
	maintaining = false
	if _, err := e.NewConfig(); !errors.Is(err, ErrUnsupported) || joins != 1 {
		t.Fatalf("NewConfig after native maintenance returned %v with %d joins", err, joins)
	}
}

func TestFailedSyncStillInvalidatesSessions(t *testing.T) {
	e := &Engine{
		sessions: make(map[*Session]struct{}),
		api: &abi.API{
			DeployerInitialize:    func(_ *abi.Traits) {},
			JoinMaintenanceThread: func() {},
			SyncUserData:          func() abi.Bool { return abi.False },
		},
	}
	s := &Session{engine: e, id: 1}
	e.sessions[s] = struct{}{}
	if started, err := e.SyncUserData(); started || err != nil {
		t.Fatalf("SyncUserData returned %v, %v", started, err)
	}
	if !s.closed || len(e.sessions) != 0 {
		t.Fatal("session remained live after a sync that failed to start")
	}
}
