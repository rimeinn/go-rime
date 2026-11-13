package rime

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/rimeinn/go-rime/abi"
)

// Options configures the library loader and the native engine. Unset paths
// use librime defaults. Pass explicit shared and user data directories for
// predictable configuration and deployment.
type Options struct {
	LibraryPath          string
	SharedDataDir        string
	UserDataDir          string
	PrebuiltDataDir      string
	StagingDir           string
	DistributionName     string
	DistributionCodeName string
	DistributionVersion  string

	// AppName enables native logging when set. librime recommends a "rime." prefix.
	AppName string
	// MinLogLevel is the native minimum severity, from 0 (INFO) to 3 (FATAL).
	MinLogLevel int32
	// LogDir leaves the native default in effect when nil. Point to an empty
	// string to request stderr-only logging.
	LogDir *string
	// Modules overrides the native module list when non-nil. An empty,
	// non-nil slice requests no modules.
	Modules []string
}

func makeTraits(opts Options) (*abi.Traits, error) {
	t := new(abi.Traits)
	t.Init()
	t.MinLogLevel = opts.MinLogLevel
	fields := []struct {
		name  string
		dest  **byte
		value string
	}{
		{"SharedDataDir", &t.SharedDataDir, opts.SharedDataDir},
		{"UserDataDir", &t.UserDataDir, opts.UserDataDir},
		{"PrebuiltDataDir", &t.PrebuiltDataDir, opts.PrebuiltDataDir},
		{"StagingDir", &t.StagingDir, opts.StagingDir},
		{"DistributionName", &t.DistributionName, opts.DistributionName},
		{"DistributionCodeName", &t.DistributionCodeName, opts.DistributionCodeName},
		{"DistributionVersion", &t.DistributionVersion, opts.DistributionVersion},
		{"AppName", &t.AppName, opts.AppName},
	}
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		p, err := cString(field.value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field.name, err)
		}
		*field.dest = p
	}
	if opts.LogDir != nil {
		p, err := cString(*opts.LogDir)
		if err != nil {
			return nil, fmt.Errorf("LogDir: %w", err)
		}
		t.LogDir = p
	}
	if opts.Modules != nil {
		modules := make([]*byte, len(opts.Modules)+1)
		for i, name := range opts.Modules {
			p, err := cString(name)
			if err != nil {
				return nil, fmt.Errorf("Modules[%d]: %w", i, err)
			}
			modules[i] = p
		}
		t.Modules = &modules[0]
	}
	return t, nil
}

func pinTraits(t *abi.Traits, moduleCount int, pin *runtime.Pinner) {
	pin.Pin(t)
	for _, p := range []*byte{
		t.SharedDataDir, t.UserDataDir, t.PrebuiltDataDir, t.StagingDir,
		t.DistributionName, t.DistributionCodeName, t.DistributionVersion, t.LogDir,
	} {
		if p != nil {
			pin.Pin(p)
		}
	}
	if t.Modules != nil {
		pin.Pin(t.Modules)
		for _, p := range unsafe.Slice(t.Modules, moduleCount) {
			pin.Pin(p)
		}
	}
}
