//go:build (windows || linux || darwin) && (amd64 || arm64)

package rime_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rimeinn/go-rime/rime"
)

func TestLibrimeIntegration(t *testing.T) {
	library := os.Getenv("GO_RIME_TEST_LIBRARY")
	if library == "" {
		t.Skip("set GO_RIME_TEST_LIBRARY to a librime shared library to run the integration test")
	}
	root := t.TempDir()
	shared, user := filepath.Join(root, "shared"), filepath.Join(root, "user")
	must(t, os.CopyFS(shared, os.DirFS("testdata/shared")))
	must(t, os.Mkdir(user, 0o755))

	engine, err := rime.Open(rime.Options{
		LibraryPath:   library,
		SharedDataDir: shared,
		UserDataDir:   user,
		AppName:       "rime.go-rime-test",
		MinLogLevel:   2,
	})
	must(t, err)
	t.Cleanup(func() { must(t, engine.Close()) })
	version, err := engine.Version()
	must(t, err)
	if version == "" {
		t.Fatal("librime returned an empty version")
	}
	t.Logf("librime %s", version)

	events, cancel, err := engine.Subscribe()
	must(t, err)
	t.Cleanup(cancel)
	started, err := engine.StartMaintenance(true)
	must(t, err)
	if !started {
		t.Fatal("full maintenance did not start")
	}
	must(t, engine.JoinMaintenance())
	awaitNotification(t, events, func(event rime.Notification) bool {
		return event.Type == "deploy" && event.Value == "success"
	})
	if _, err := os.Stat(filepath.Join(user, "build", "go_rime_test.schema.yaml")); errors.Is(err, fs.ErrNotExist) {
		t.Fatal("maintenance did not deploy the test schema")
	} else {
		must(t, err)
	}
	must(t, engine.DeployConfigFile("default.yaml", "config_version"))
	userDir, err := engine.UserDataDir()
	must(t, err)
	if filepath.Clean(userDir) != filepath.Clean(user) {
		t.Fatalf("native user directory = %q, want %q", userDir, user)
	}

	t.Run("config", func(t *testing.T) { exerciseConfig(t, engine) })
	t.Run("input", func(t *testing.T) { exerciseInput(t, engine, events) })
	t.Run("lifecycle", func(t *testing.T) { exerciseClose(t, engine) })
	cancel()
	cancel()
	for range events {
	}
}

func exerciseConfig(t *testing.T, engine *rime.Engine) {
	t.Helper()
	cfg, err := engine.OpenConfig("default")
	must(t, err)
	t.Cleanup(func() { must(t, cfg.Close()) })
	value, ok, err := cfg.GetInt("menu/page_size")
	must(t, err)
	if !ok || value != 2 {
		t.Fatalf("default menu/page_size = %d, present = %v", value, ok)
	}

	memory, err := engine.NewConfig()
	must(t, err)
	t.Cleanup(func() { must(t, memory.Close()) })
	must(t, memory.LoadString("test:\n  a: one\n  b: two\n"))
	must(t, memory.SetBool("enabled", true))
	must(t, memory.SetInt("limit", 17))
	must(t, memory.SetDouble("ratio", 1.25))
	must(t, memory.SetString("greeting", "你好"))
	if enabled, ok, err := memory.GetBool("enabled"); err != nil || !ok || !enabled {
		t.Fatalf("enabled = %v, %v, %v", enabled, ok, err)
	}
	if limit, ok, err := memory.GetInt("limit"); err != nil || !ok || limit != 17 {
		t.Fatalf("limit = %d, %v, %v", limit, ok, err)
	}
	if ratio, ok, err := memory.GetDouble("ratio"); err != nil || !ok || ratio != 1.25 {
		t.Fatalf("ratio = %v, %v, %v", ratio, ok, err)
	}
	if greeting, ok, err := memory.GetString("greeting"); err != nil || !ok || greeting != "你好" {
		t.Fatalf("greeting = %q, %v, %v", greeting, ok, err)
	}
	entries, ok, err := memory.IterateMap("test")
	must(t, err)
	if !ok {
		t.Fatal("test map is missing")
	}
	for entry := range entries {
		if entry.Key != "a" && entry.Key != "b" {
			t.Fatalf("unexpected map key %q", entry.Key)
		}
		break
	}
	must(t, memory.Clear("test"))
	must(t, memory.CreateList("items"))
	must(t, memory.SetString("items/@0", "first"))
	item, err := memory.GetItem("items")
	must(t, err)
	t.Cleanup(func() { must(t, item.Close()) })
	size, err := item.ListSize("")
	must(t, err)
	if size != 1 {
		t.Fatalf("list size = %d, want 1", size)
	}
	if value, ok, err := item.GetString("@0"); err != nil || !ok || value != "first" {
		t.Fatalf("first list item = %q, %v, %v", value, ok, err)
	}
	must(t, memory.Close())
	must(t, memory.Close())
}

func exerciseInput(t *testing.T, engine *rime.Engine, events <-chan rime.Notification) {
	t.Helper()
	const schemaID = "go_rime_test"
	schemas, err := engine.SchemaList()
	must(t, err)
	if len(schemas) != 1 || schemas[0].ID != schemaID {
		t.Fatalf("schemas = %+v", schemas)
	}
	session, err := engine.NewSession()
	must(t, err)
	t.Cleanup(func() { must(t, session.Close()) })
	selected, err := session.SelectSchema(schemaID)
	must(t, err)
	if !selected {
		t.Fatalf("could not select schema %q", schemaID)
	}
	status, ok, err := session.Status()
	must(t, err)
	if !ok || status.SchemaID != schemaID || status.ASCIIMode {
		t.Fatalf("unexpected status %+v, present = %v", status, ok)
	}
	if label, ok, err := session.StateLabel("ascii_mode", false, false); !errors.Is(err, rime.ErrUnsupported) && (err != nil || !ok || label != "中文") {
		t.Fatalf("state label = %q, %v, %v", label, ok, err)
	}
	for _, key := range "ni" {
		handled, err := session.ProcessKey(key, 0)
		must(t, err)
		if !handled {
			t.Fatalf("key %q was not handled", key)
		}
	}
	context, ok, err := session.Context()
	must(t, err)
	if !ok || context.Composition.Preedit != "ni" || len(context.Menu.Candidates) == 0 || context.Menu.Candidates[0].Text != "你好" {
		t.Fatalf("unexpected context %+v, present = %v", context, ok)
	}
	candidates, err := session.Candidates(0, 3)
	must(t, err)
	if len(candidates) != 3 || candidates[0].Text != "你好" || candidates[2].Text != "泥" {
		t.Fatalf("unexpected candidate list %+v", candidates)
	}
	if removed, err := session.DeleteCandidate(9999); !errors.Is(err, rime.ErrUnsupported) && (err != nil || removed) {
		t.Fatalf("out-of-range delete = %v, %v", removed, err)
	}
	if removed, err := session.DeleteCandidateOnCurrentPage(9999); !errors.Is(err, rime.ErrUnsupported) && (err != nil || removed) {
		t.Fatalf("out-of-range page delete = %v, %v", removed, err)
	}
	t.Run("candidate preview", func(t *testing.T) {
		preview, ok, err := session.CandidatePreview()
		if errors.Is(err, rime.ErrUnsupported) {
			t.Skip("native candidate preview slots are unavailable")
		}
		if err != nil || !ok || preview.SelectedText != "你好" {
			t.Fatalf("candidate preview = %+v, %v, %v", preview, ok, err)
		}
	})
	selected, err = session.SelectCandidateOnCurrentPage(0)
	must(t, err)
	if !selected {
		t.Fatal("could not select the first candidate")
	}
	commit, ok, err := session.TakeCommit()
	must(t, err)
	if !ok || commit != "你好" {
		t.Fatalf("commit = %q, present = %v", commit, ok)
	}
	if _, ok, err := session.TakeCommit(); ok || err != nil {
		t.Fatalf("second commit = present %v, error %v", ok, err)
	}
	if context.Menu.Candidates[0].Text != "你好" {
		t.Fatal("context changed after candidate selection")
	}
	must(t, session.SetOption("ascii_mode", true))
	awaitNotification(t, events, func(event rime.Notification) bool {
		return event.SessionID == session.ID() && event.Type == "option" && event.Value == "ascii_mode"
	})
}

func exerciseClose(t *testing.T, engine *rime.Engine) {
	t.Helper()
	session, err := engine.NewSession()
	must(t, err)
	started, err := engine.SyncUserData()
	must(t, err)
	if !started {
		t.Fatal("sync did not start")
	}
	must(t, engine.JoinMaintenance())
	if _, err := session.ProcessKey('n', 0); !errors.Is(err, rime.ErrClosed) {
		t.Fatalf("operation on a session invalidated by sync returned %v", err)
	}
	must(t, session.Close())

	session, err = engine.NewSession()
	must(t, err)
	cfg, err := engine.NewConfig()
	must(t, err)
	var calls sync.WaitGroup
	for range 4 {
		calls.Go(func() {
			for range 50 {
				if _, _, err := session.Status(); err != nil && !errors.Is(err, rime.ErrClosed) {
					t.Errorf("Status during Close: %v", err)
				}
				if _, _, err := cfg.GetString("unused"); err != nil && !errors.Is(err, rime.ErrClosed) {
					t.Errorf("GetString during Close: %v", err)
				}
			}
		})
	}
	err = engine.Close()
	calls.Wait()
	must(t, err)
	must(t, engine.Close())
	must(t, cfg.Close())
	must(t, session.Close())
	if _, err := engine.NewSession(); !errors.Is(err, rime.ErrClosed) {
		t.Fatalf("NewSession after Close returned %v", err)
	}
	if _, err := rime.Open(rime.Options{}); !errors.Is(err, rime.ErrEngineUsed) {
		t.Fatalf("second Open returned %v", err)
	}
}

func awaitNotification(t *testing.T, events <-chan rime.Notification, match func(rime.Notification) bool) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event, open := <-events:
			if !open {
				t.Fatal("notification subscription closed")
			}
			if match(event) {
				return
			}
		case <-timer.C:
			t.Fatal("timed out waiting for a librime notification")
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
