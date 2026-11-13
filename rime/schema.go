package rime

import (
	"unsafe"

	"github.com/rimeinn/go-rime/abi"
)

// Schema identifies a configured input schema.
type Schema struct {
	ID   string
	Name string
}

// SchemaList returns the schemas in librime's configured schema list.
func (e *Engine) SchemaList() ([]Schema, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkLocked(); err != nil {
		return nil, err
	}
	if e.api.GetSchemaList == nil {
		return nil, unsupported("get_schema_list")
	}
	if e.api.FreeSchemaList == nil {
		return nil, unsupported("free_schema_list")
	}
	var raw abi.SchemaList
	if !e.api.GetSchemaList(&raw).GoBool() {
		return nil, nil
	}
	defer e.api.FreeSchemaList(&raw)
	if raw.Size > uintptr(^uint(0)>>1) || (raw.Size > 0 && raw.List == nil) {
		return nil, failed("get_schema_list returned invalid array metadata")
	}
	schemas := make([]Schema, int(raw.Size))
	for i, item := range unsafe.Slice(raw.List, int(raw.Size)) {
		schemas[i] = Schema{ID: abi.GoString(item.SchemaID), Name: abi.GoString(item.Name)}
	}
	return schemas, nil
}

// CurrentSchema returns the active schema ID, if librime supplied one.
func (s *Session) CurrentSchema() (string, bool, error) {
	e, err := s.lock()
	if err != nil {
		return "", false, err
	}
	defer e.mu.Unlock()
	if e.api.GetCurrentSchema == nil {
		return "", false, unsupported("get_current_schema")
	}
	return readCStringBuffer(func(buf *byte, size uintptr) bool {
		return e.api.GetCurrentSchema(s.id, buf, size).GoBool()
	})
}

// SelectSchema requests a schema by ID and reports whether librime applied it.
func (s *Session) SelectSchema(id string) (bool, error) {
	return s.stringOperation(id, "select_schema", func(a *abi.API) sessionStringOperation { return a.SelectSchema })
}
