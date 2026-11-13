package abi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func cCompiler(t *testing.T) string {
	t.Helper()
	if cc := os.Getenv("CC"); cc != "" {
		path, err := exec.LookPath(cc)
		if err != nil {
			t.Fatalf("CC=%q: %v", cc, err)
		}
		return path
	}
	compilers := []string{"cc", "gcc", "clang"}
	if runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		// An ARM64 Windows machine can have an x64 MinGW compiler on PATH.
		compilers = []string{"clang", "cc", "gcc"}
	}
	for _, cc := range compilers {
		if path, err := exec.LookPath(cc); err == nil {
			return path
		}
	}
	t.Skip("C compiler unavailable (need GCC or Clang)")
	return ""
}

func runCommand(t *testing.T, command string, args ...string) string {
	t.Helper()
	output, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", command, args, err, output)
	}
	return string(output)
}

func TestLayoutWithOfficialHeaders(t *testing.T) {
	include := os.Getenv("RIME_INCLUDE_DIR")
	if include == "" {
		t.Skip("set RIME_INCLUDE_DIR to the directory containing librime C headers")
	}
	cc := cCompiler(t)
	tmp := t.TempDir()
	want := map[string][]uintptr{}
	addLayout(want, "Bool", Bool(0), nil)
	addLayout(want, "RimeSessionId", SessionID(0), nil)
	var traits Traits
	var commit Commit
	var context Context
	var status Status
	var module Module
	var custom CustomAPI
	traits.Init()
	commit.Init()
	context.Init()
	status.Init()
	module.Init()
	custom.Init()
	addLayout(want, "RimeTraits", traits, &traits.DataSize)
	addLayout(want, "RimeComposition", Composition{}, nil)
	addLayout(want, "RimeCandidate", Candidate{}, nil)
	addLayout(want, "RimeMenu", Menu{}, nil)
	addLayout(want, "RimeCommit", commit, &commit.DataSize)
	addLayout(want, "RimeContext", context, &context.DataSize)
	addLayout(want, "RimeStatus", status, &status.DataSize)
	addLayout(want, "RimeCandidateListIterator", CandidateListIterator{}, nil)
	addLayout(want, "RimeConfig", Config{}, nil)
	addLayout(want, "RimeConfigIterator", ConfigIterator{}, nil)
	addLayout(want, "RimeSchemaList", SchemaList{}, nil)
	addLayout(want, "RimeSchemaListItem", SchemaListItem{}, nil)
	addLayout(want, "RimeStringSlice", StringSlice{}, nil)
	addLayout(want, "RimeModule", module, &module.DataSize)
	addLayout(want, "RimeCustomApi", custom, &custom.DataSize)
	addLayout(want, "RimeUserDictIterator", UserDictIterator{}, nil)

	apiHeader := readHeader(t, include, "rime_api.h")
	leversHeader := readHeader(t, include, "rime_levers_api.h")
	var slotProbe strings.Builder
	addTable(t, want, &slotProbe, "RimeApi", apiHeader, reflect.TypeFor[rawAPI](), (&API{}).Slots())
	addTable(t, want, &slotProbe, "RimeLeversApi", leversHeader, reflect.TypeFor[rawLeversAPI](), (&LeversAPI{}).Slots())
	if err := os.WriteFile(filepath.Join(tmp, "layout_slots.h"), []byte(slotProbe.String()), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-std=c11", "-Wall", "-Werror", "-I", include, "-I", tmp}
	if strings.Contains(apiHeader, "RimeCandidatePreview") {
		args = append(args, "-DHAVE_RIME_CANDIDATE_PREVIEW")
		var preview CandidatePreview
		preview.Init()
		addLayout(want, "RimeCandidatePreview", preview, &preview.DataSize)
	}
	bin := filepath.Join(tmp, "layout-probe")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	runCommand(t, cc, append(args, "testdata/layout.c", "-o", bin)...)
	for line := range strings.Lines(runCommand(t, bin)) {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			t.Fatalf("invalid probe output: %q", line)
		}
		name := parts[0]
		expected, ok := want[name]
		if !ok || len(parts)-1 != len(expected) {
			t.Fatalf("unexpected probe output: %q", line)
		}
		for i, field := range parts[1:] {
			native, err := strconv.ParseUint(field, 10, 64)
			if err != nil || native != uint64(expected[i]) {
				t.Errorf("%s[%d]: C %s, Go %d", name, i, field, expected[i])
			}
		}
		delete(want, name)
	}
	for missing := range want {
		t.Errorf("missing C probe result for %s", missing)
	}
}

var initialismBoundary = regexp.MustCompile(`([A-Z])([A-Z][a-z])`)
var wordBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func cFieldName(name string) string {
	name = initialismBoundary.ReplaceAllString(name, "${1}_${2}")
	return strings.ToLower(wordBoundary.ReplaceAllString(name, "${1}_${2}"))
}

func addLayout(want map[string][]uintptr, name string, example any, dataSize *int32) {
	typ := reflect.TypeOf(example)
	want[name] = []uintptr{typ.Size(), uintptr(typ.Align())}
	if dataSize != nil {
		want[name+".init"] = []uintptr{uintptr(*dataSize)}
	}
	if typ.Kind() == reflect.Struct {
		for i := range typ.NumField() {
			field := typ.Field(i)
			want[name+"."+cFieldName(field.Name)] = []uintptr{field.Offset}
		}
	}
}

func readHeader(t *testing.T, include, name string) string {
	t.Helper()
	bytes, err := os.ReadFile(filepath.Join(include, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}

var cComments = regexp.MustCompile(`(?s)/\*.*?\*/|(?m)//[^\n]*`)
var slotName = regexp.MustCompile(`\(\s*\*\s*(\w+)\s*\)\s*\(`)

func addTable(t *testing.T, want map[string][]uintptr, probe *strings.Builder, name, header string, layout reflect.Type, names []string) {
	t.Helper()
	// Discover the native slots independently of Go's generator input. Older
	// public headers have a prefix of the current table.
	header = cComments.ReplaceAllString(header, "")
	end := regexp.MustCompile(`\}\s*(?:RIME_FLAVORED\(\s*` + name + `\s*\)|` + name + `)\s*;`).FindStringIndex(header)
	if end == nil {
		t.Fatalf("no %s struct declaration in header", name)
	}
	start := strings.LastIndexByte(header[:end[0]], '{')
	if start < 0 {
		t.Fatalf("no %s struct body in header", name)
	}
	slots := slotName.FindAllStringSubmatch(header[start:end[0]], -1)
	if len(slots) == 0 || len(slots) > len(names) {
		t.Fatalf("%s has %d C slots and %d Go slots", name, len(slots), len(names))
	}
	for index, slot := range slots {
		if slot[1] != names[index] {
			t.Errorf("%s slot %d: C %s, Go %s", name, index, slot[1], names[index])
		}
		fmt.Fprintf(probe, "FIELD(%s, %s)\n", name, slot[1])
		want[name+"."+slot[1]] = []uintptr{layout.Field(index + 1).Offset}
	}
	last := layout.Field(len(slots))
	size := last.Offset + last.Type.Size()
	want[name] = []uintptr{size, uintptr(layout.Align())}
	want[name+".data_size"] = []uintptr{0}
	want[name+".init"] = []uintptr{size - dataSizeWidth}
}
