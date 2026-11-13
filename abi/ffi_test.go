//go:build (windows || darwin || linux) && (amd64 || arm64)

package abi

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

func buildFFI(t *testing.T, define ...string) string {
	t.Helper()
	flags := []string{"-std=c11", "-shared", "-fPIC"}
	name := "fixture.so"
	switch runtime.GOOS {
	case "windows":
		flags = []string{"-std=c11", "-shared"}
		name = "fixture.dll"
	case "darwin":
		flags[1] = "-dynamiclib"
		name = "fixture.dylib"
	}
	for _, flag := range define {
		flags = append(flags, "-D"+flag)
	}
	path := filepath.Join(t.TempDir(), name)
	runCommand(t, cCompiler(t), append(flags, "testdata/ffi.c", "-o", path)...)
	return path
}

func TestOpenShortAPI(t *testing.T) {
	path := buildFFI(t)
	library, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := library.Close(); err != nil {
			t.Error(err)
		}
	})
	a := library.API()
	if a.SetNotificationHandler == nil || a.Capability("set_notification_handler") != CapabilityAvailable {
		t.Fatal("expected callback registration to be bound")
	}
	if a.Setup != nil || a.Capability("setup") != CapabilityNull {
		t.Fatal("expected a recognized NULL setup slot")
	}
	if a.StartMaintenance != nil || a.Capability("start_maintenance") != CapabilityMissing {
		t.Fatal("partially present pointer was bound")
	}
	if a.GetVersion != nil || a.Capability("get_version") != CapabilityMissing {
		t.Fatal("expected get_version to be outside the table")
	}
	if a.Available("no_such_entry") || a.Capability("no_such_entry") != CapabilityUnknown {
		t.Fatal("unknown entry reported as available")
	}
	if _, err := a.Levers(); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Levers: %v", err)
	}

	var got string
	callback := NewNotificationCallback(func(context uintptr, id SessionID, kind, value *byte) {
		got = fmt.Sprintf("%d %d %s %s", context, id, GoString(kind), GoString(value))
	})
	a.SetNotificationHandler(callback, 42)
	if got != "42 4294967301 type 测试" {
		t.Fatalf("callback received %q", got)
	}
	if err := library.Close(); err != nil {
		t.Fatal(err)
	}
	if library.API() != nil {
		t.Fatal("closed library still exposes its table")
	}
}

func TestNativeStringSliceReturn(t *testing.T) {
	var layout rawAPI
	wordSize := unsafe.Sizeof(uintptr(0))
	index := (unsafe.Offsetof(layout.GetStateLabelAbbreviated) - unsafe.Offsetof(layout.Setup)) / wordSize
	path := buildFFI(t, "FULL_TABLE", fmt.Sprintf("STATE_LABEL_SLOT=%d", index))
	library, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := library.Close(); err != nil {
			t.Error(err)
		}
	})
	a := library.API()
	if a.StartMaintenance == nil || a.GetStateLabelAbbreviated == nil {
		t.Fatal("expected scalar and structure functions to be bound")
	}
	if got := a.StartMaintenance(Bool(-123)); got != Bool(-7) {
		t.Fatalf("C int round trip = %d", got)
	}
	key, err := CString("x")
	if err != nil {
		t.Fatal(err)
	}
	label := a.GetStateLabelAbbreviated(0x100000005, key, True, Bool(-12))
	if got := label.String(); got != "a\x00b" {
		t.Errorf("structure return = %q", got)
	}
}

func TestOpenInvalidLibrary(t *testing.T) {
	for _, tc := range []struct{ define, message string }{
		{"NO_ENTRY_POINT", "rime_get_api"},
		{"NULL_TABLE", "NULL API table"},
		{"NEGATIVE_TABLE", "invalid data_size"},
	} {
		t.Run(tc.define, func(t *testing.T) {
			path := buildFFI(t, tc.define)
			lib, err := Open(path)
			if lib != nil {
				lib.Close()
				t.Fatal("invalid library was accepted")
			}
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("Open: %v, want error containing %q", err, tc.message)
			}
		})
	}
}
