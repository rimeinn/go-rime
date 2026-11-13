//go:build (windows || linux || darwin) && (amd64 || arm64)

package abi

import (
	"errors"
	"os"
	"runtime"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
)

func TestGoModuleIntegration(t *testing.T) {
	path := os.Getenv("GO_RIME_TEST_LIBRARY")
	if path == "" {
		t.Skip("set GO_RIME_TEST_LIBRARY to test module registration with librime")
	}
	library, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	api := library.API()
	if api.Setup == nil || api.Initialize == nil || api.Finalize == nil {
		t.Fatal("library does not support setup, initialize and finalize")
	}
	api.Setup(nil)
	t.Run("levers", func(t *testing.T) {
		levers, err := api.Levers()
		if errors.Is(err, ErrUnsupported) {
			t.Skip("native levers module is unavailable")
		}
		if err != nil {
			t.Fatal(err)
		}
		if levers.CustomSettingsInit == nil || levers.CustomSettingsDestroy == nil {
			t.Fatal("levers settings lifecycle slots are unavailable")
		}
		id, err := CString("go_rime_test")
		if err != nil {
			t.Fatal(err)
		}
		settings := levers.CustomSettingsInit(id, id)
		if settings == nil {
			t.Fatal("levers returned no settings")
		}
		levers.CustomSettingsDestroy(settings)
	})
	value, err := newModuleCallback(func() uintptr { return 42 })
	if err != nil {
		t.Fatal(err)
	}
	var initialized, finalized int
	const name = "go_rime_test_module"
	definition := GoModule{
		Name:        name,
		Initialize:  func() { initialized++ },
		Finalize:    func() { finalized++ },
		CustomSlots: []uintptr{value},
	}
	if err := library.RegisterGoModule(definition); err != nil {
		t.Fatal(err)
	}
	if err := library.RegisterGoModule(definition); err == nil {
		t.Fatal("duplicate module registration succeeded")
	}
	moduleName, err := CString(name)
	if err != nil {
		t.Fatal(err)
	}
	var traits Traits
	traits.Init()
	modules := []*byte{moduleName, nil}
	traits.Modules = &modules[0]
	var pin runtime.Pinner
	pin.Pin(&traits)
	pin.Pin(traits.Modules)
	pin.Pin(moduleName)
	defer pin.Unpin()
	api.Initialize(&traits)
	t.Cleanup(func() {
		api.Finalize()
		if finalized != 1 {
			t.Errorf("module finalized %d times", finalized)
		}
	})
	if initialized != 1 {
		t.Fatalf("module initialized %d times", initialized)
	}
	runtime.GC()
	module := api.FindModule(moduleName)
	if module == nil || GoString(module.ModuleName) != name {
		t.Fatal("cannot find registered module after GC")
	}
	custom, err := module.CustomAPI()
	if err != nil {
		t.Fatal(err)
	}
	wordSize := unsafe.Sizeof(uintptr(0))
	if custom == nil || !StructHasMember(custom.DataSize, wordSize, wordSize) {
		t.Fatal("missing extension table slot")
	}
	var read func() uintptr
	purego.RegisterFunc(&read, *(*uintptr)(unsafe.Add(unsafe.Pointer(custom), wordSize)))
	if result := read(); result != 42 {
		t.Fatalf("extension callback returned %d", result)
	}
	if err := library.Close(); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Close on retained library returned %v", err)
	}
}
