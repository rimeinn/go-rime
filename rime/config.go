package rime

import (
	"errors"
	"fmt"
	"iter"
	"math"
	"slices"

	"github.com/rimeinn/go-rime/abi"
)

// Config owns a native configuration handle. Close releases the handle. The
// Engine also closes all its configurations when it closes.
type Config struct {
	engine *Engine
	raw    abi.Config
}

// NewConfig initializes an empty in-memory configuration.
func (e *Engine) NewConfig() (*Config, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkLocked(); err != nil {
		return nil, err
	}
	if e.api.ConfigInit == nil {
		return nil, unsupported("config_init")
	}
	return e.createConfigLocked("config_init", e.api.ConfigInit)
}

// OpenConfig opens a named Rime configuration, for example "default".
func (e *Engine) OpenConfig(id string) (*Config, error) {
	return e.openConfig("config_open", id, func(api *abi.API) openConfigFunc { return api.ConfigOpen })
}

// OpenSchemaConfig opens the configuration for a schema ID.
func (e *Engine) OpenSchemaConfig(id string) (*Config, error) {
	return e.openConfig("schema_open", id, func(api *abi.API) openConfigFunc { return api.SchemaOpen })
}

// OpenUserConfig opens a configuration from the user data directory. Rime's
// user config component saves changes when its last reference is released.
func (e *Engine) OpenUserConfig(id string) (*Config, error) {
	return e.openConfig("user_config_open", id, func(api *abi.API) openConfigFunc { return api.UserConfigOpen })
}

type openConfigFunc = func(*byte, *abi.Config) abi.Bool

func (e *Engine) openConfig(slot, id string, open func(*abi.API) openConfigFunc) (*Config, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkLocked(); err != nil {
		return nil, err
	}
	fn := open(e.api)
	if fn == nil {
		return nil, unsupported(slot)
	}
	name, err := cString(id)
	if err != nil {
		return nil, err
	}
	return e.createConfigLocked(slot, func(out *abi.Config) abi.Bool { return fn(name, out) })
}

func (e *Engine) createConfigLocked(slot string, create func(*abi.Config) abi.Bool) (*Config, error) {
	if e.api.ConfigClose == nil {
		return nil, unsupported("config_close")
	}
	c := &Config{engine: e}
	if !create(&c.raw).GoBool() || c.raw.Ptr == nil {
		if c.raw.Ptr != nil {
			e.api.ConfigClose(&c.raw)
		}
		return nil, failed(slot)
	}
	e.configs[c] = struct{}{}
	return c, nil
}

// Close releases the configuration and may wait for pending maintenance.
// Repeated calls to Close are safe.
func (c *Config) Close() error {
	if c == nil || c.engine == nil {
		return nil
	}
	e := c.engine
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, live := e.configs[c]; !live {
		return nil
	}
	if e.maintenancePending {
		e.joinMaintenanceLocked()
	}
	return c.closeLocked()
}

func (c *Config) closeLocked() error {
	if !c.engine.api.ConfigClose(&c.raw).GoBool() {
		return failed("config_close")
	}
	delete(c.engine.configs, c)
	return nil
}

func (e *Engine) closeConfigsLocked() error {
	var err error
	for c := range e.configs {
		err = errors.Join(err, c.closeLocked())
	}
	clear(e.configs)
	return err
}

func (c *Config) lock() (*Engine, error) {
	if c == nil || c.engine == nil {
		return nil, ErrClosed
	}
	e := c.engine
	e.mu.Lock()
	if _, live := e.configs[c]; !live {
		e.mu.Unlock()
		return nil, ErrClosed
	}
	if err := e.checkLocked(); err != nil {
		e.mu.Unlock()
		return nil, err
	}
	return e, nil
}

// LoadString replaces the configuration contents with parsed YAML.
func (c *Config) LoadString(yaml string) error {
	return c.change("config_load_string", yaml, func(api *abi.API) configChange { return api.ConfigLoadString })
}

// GetBool returns the value at key and reports whether Rime could read it as a boolean.
func (c *Config) GetBool(key string) (value bool, ok bool, err error) {
	v, ok, err := configScalar(c, key, "config_get_bool", func(api *abi.API) configRead[abi.Bool] { return api.ConfigGetBool })
	return v.GoBool(), ok, err
}

// GetInt returns the value at key and reports whether Rime could read it as an integer.
func (c *Config) GetInt(key string) (value int, ok bool, err error) {
	v, ok, err := configScalar(c, key, "config_get_int", func(api *abi.API) configRead[int32] { return api.ConfigGetInt })
	return int(v), ok, err
}

// GetDouble returns the value at key and reports whether Rime could read it as a double.
func (c *Config) GetDouble(key string) (value float64, ok bool, err error) {
	return configScalar(c, key, "config_get_double", func(api *abi.API) configRead[float64] { return api.ConfigGetDouble })
}

type configRead[T any] func(*abi.Config, *byte, *T) abi.Bool

func configScalar[T any](c *Config, key, slot string, read func(*abi.API) configRead[T]) (value T, ok bool, err error) {
	e, err := c.lock()
	if err != nil {
		return value, false, err
	}
	defer e.mu.Unlock()
	fn := read(e.api)
	if fn == nil {
		return value, false, unsupported(slot)
	}
	k, err := cString(key)
	if err != nil {
		return value, false, err
	}
	ok = fn(&c.raw, k, &value).GoBool()
	return value, ok, nil
}

// GetString copies the scalar at key. A missing or non-scalar key returns ok=false.
func (c *Config) GetString(key string) (value string, ok bool, err error) {
	e, err := c.lock()
	if err != nil {
		return "", false, err
	}
	defer e.mu.Unlock()
	k, err := cString(key)
	if err != nil {
		return "", false, err
	}
	if e.api.ConfigGetCString != nil {
		p := e.api.ConfigGetCString(&c.raw, k)
		return abi.GoString(p), p != nil, nil
	}
	if e.api.ConfigGetString == nil {
		return "", false, unsupported("config_get_string")
	}
	return readCStringBuffer(func(out *byte, size uintptr) bool {
		return e.api.ConfigGetString(&c.raw, k, out, size).GoBool()
	})
}

// SetBool sets the boolean at key.
func (c *Config) SetBool(key string, value bool) error {
	return setConfigScalar(c, key, "config_set_bool", abi.BoolValue(value), func(api *abi.API) configWrite[abi.Bool] { return api.ConfigSetBool })
}

// SetInt sets the integer at key. The value must fit in a signed 32-bit integer.
func (c *Config) SetInt(key string, value int) error {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return fmt.Errorf("%w: config integer %d is outside the C int range", ErrInvalidArgument, value)
	}
	return setConfigScalar(c, key, "config_set_int", int32(value), func(api *abi.API) configWrite[int32] { return api.ConfigSetInt })
}

// SetDouble sets the double at key using Rime's numeric formatting.
func (c *Config) SetDouble(key string, value float64) error {
	return setConfigScalar(c, key, "config_set_double", value, func(api *abi.API) configWrite[float64] { return api.ConfigSetDouble })
}

// SetString sets the string at key.
func (c *Config) SetString(key, value string) error {
	v, err := cString(value)
	if err != nil {
		return err
	}
	return setConfigScalar(c, key, "config_set_string", v, func(api *abi.API) configWrite[*byte] { return api.ConfigSetString })
}

type configWrite[T any] func(*abi.Config, *byte, T) abi.Bool

func setConfigScalar[T any](c *Config, key, slot string, value T, write func(*abi.API) configWrite[T]) error {
	return c.change(slot, key, func(api *abi.API) configChange {
		fn := write(api)
		if fn == nil {
			return nil
		}
		return func(raw *abi.Config, k *byte) abi.Bool { return fn(raw, k, value) }
	})
}

// GetItem opens a configuration rooted at key. Rime represents missing and
// null keys as an empty configuration. The caller should close the returned handle.
func (c *Config) GetItem(key string) (*Config, error) {
	e, err := c.lock()
	if err != nil {
		return nil, err
	}
	defer e.mu.Unlock()
	if e.api.ConfigGetItem == nil {
		return nil, unsupported("config_get_item")
	}
	k, err := cString(key)
	if err != nil {
		return nil, err
	}
	return e.createConfigLocked("config_get_item", func(out *abi.Config) abi.Bool {
		return e.api.ConfigGetItem(&c.raw, k, out)
	})
}

// SetItem stores value's root item at key. Pass nil to store a null item.
func (c *Config) SetItem(key string, value *Config) error {
	e, err := c.lock()
	if err != nil {
		return err
	}
	defer e.mu.Unlock()
	if e.api.ConfigSetItem == nil {
		return unsupported("config_set_item")
	}
	k, err := cString(key)
	if err != nil {
		return err
	}
	var raw *abi.Config
	if value != nil {
		if _, live := e.configs[value]; !live {
			return fmt.Errorf("%w: item config must be open on the same engine", ErrInvalidArgument)
		}
		raw = &value.raw
	}
	if !e.api.ConfigSetItem(&c.raw, k, raw).GoBool() {
		return failed("config_set_item")
	}
	return nil
}

// Clear stores a null item at key.
func (c *Config) Clear(key string) error {
	return c.change("config_clear", key, func(api *abi.API) configChange { return api.ConfigClear })
}

// CreateMap stores an empty map at key.
func (c *Config) CreateMap(key string) error {
	return c.change("config_create_map", key, func(api *abi.API) configChange { return api.ConfigCreateMap })
}

// CreateList stores an empty list at key.
func (c *Config) CreateList(key string) error {
	return c.change("config_create_list", key, func(api *abi.API) configChange { return api.ConfigCreateList })
}

// UpdateSignature records Rime's deployment signature with the given signer.
func (c *Config) UpdateSignature(signer string) error {
	return c.change("config_update_signature", signer, func(api *abi.API) configChange { return api.ConfigUpdateSignature })
}

type configChange = func(*abi.Config, *byte) abi.Bool

func (c *Config) change(slot, arg string, selectFunc func(*abi.API) configChange) error {
	e, err := c.lock()
	if err != nil {
		return err
	}
	defer e.mu.Unlock()
	fn := selectFunc(e.api)
	if fn == nil {
		return unsupported(slot)
	}
	value, err := cString(arg)
	if err != nil {
		return err
	}
	if !fn(&c.raw, value).GoBool() {
		return failed(slot)
	}
	return nil
}

// ListSize returns the number of entries at key, or zero for a missing or
// non-list key.
func (c *Config) ListSize(key string) (int, error) {
	e, err := c.lock()
	if err != nil {
		return 0, err
	}
	defer e.mu.Unlock()
	if e.api.ConfigListSize == nil {
		return 0, unsupported("config_list_size")
	}
	k, err := cString(key)
	if err != nil {
		return 0, err
	}
	n := e.api.ConfigListSize(&c.raw, k)
	if n > math.MaxInt {
		return 0, failed("config_list_size")
	}
	return int(n), nil
}

// ConfigEntry describes a child of a configuration map or list. Use Path for
// subsequent lookups. List keys have the form "@0", "@1", and so on.
type ConfigEntry struct {
	Index int
	Key   string
	Path  string
}

// IterateMap snapshots the child keys and paths of a map. A missing or
// non-map key returns ok=false. The sequence can be used after Close.
func (c *Config) IterateMap(key string) (entries iter.Seq[ConfigEntry], ok bool, err error) {
	return c.iterate("config_begin_map", key, func(api *abi.API) configBegin { return api.ConfigBeginMap })
}

// IterateList snapshots the child keys and paths of a list. A missing or
// non-list key returns ok=false. The sequence can be used after Close.
func (c *Config) IterateList(key string) (entries iter.Seq[ConfigEntry], ok bool, err error) {
	return c.iterate("config_begin_list", key, func(api *abi.API) configBegin { return api.ConfigBeginList })
}

type configBegin = func(*abi.ConfigIterator, *abi.Config, *byte) abi.Bool

func (c *Config) iterate(slot, key string, selectBegin func(*abi.API) configBegin) (iter.Seq[ConfigEntry], bool, error) {
	e, err := c.lock()
	if err != nil {
		return nil, false, err
	}
	defer e.mu.Unlock()
	begin := selectBegin(e.api)
	switch {
	case begin == nil:
		return nil, false, unsupported(slot)
	case e.api.ConfigNext == nil:
		return nil, false, unsupported("config_next")
	case e.api.ConfigEnd == nil:
		return nil, false, unsupported("config_end")
	}
	k, err := cString(key)
	if err != nil {
		return nil, false, err
	}
	var raw abi.ConfigIterator
	if !begin(&raw, &c.raw, k).GoBool() {
		return nil, false, nil
	}
	defer e.api.ConfigEnd(&raw)
	var entries []ConfigEntry
	for e.api.ConfigNext(&raw).GoBool() {
		entries = append(entries, ConfigEntry{
			Index: int(raw.Index),
			Key:   abi.GoString(raw.Key),
			Path:  abi.GoString(raw.Path),
		})
	}
	return slices.Values(entries), true, nil
}
