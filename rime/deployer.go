package rime

import "github.com/rimeinn/go-rime/abi"

// StartMaintenance runs the installation check and starts native workspace
// maintenance when needed. Rime returns false when there are no changes to
// deploy or the installation check fails. While maintenance runs, input and
// configuration operations return ErrMaintenance. JoinMaintenance waits for
// the work to finish. Rime reports its result through deploy notifications.
func (e *Engine) StartMaintenance(fullCheck bool) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkLocked(); err != nil {
		return false, err
	}
	if e.api.StartMaintenance == nil {
		return false, unsupported("start_maintenance")
	}
	if e.api.JoinMaintenanceThread == nil {
		return false, unsupported("join_maintenance_thread")
	}
	e.maintenancePending = e.api.StartMaintenance(abi.BoolValue(fullCheck)).GoBool()
	return e.maintenancePending, nil
}

// IsMaintaining reports whether Rime is running a maintenance task.
func (e *Engine) IsMaintaining() (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkOpenLocked(); err != nil {
		return false, err
	}
	if e.api.IsMaintenanceMode == nil {
		return false, unsupported("is_maintenance_mode")
	}
	if e.api.IsMaintenanceMode().GoBool() {
		return true, nil
	}
	if e.maintenancePending {
		e.joinMaintenanceLocked()
	}
	return false, nil
}

// JoinMaintenance waits for the native maintenance task to exit. Rime reports
// the task's result through deploy notifications.
func (e *Engine) JoinMaintenance() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkOpenLocked(); err != nil {
		return err
	}
	if e.api.JoinMaintenanceThread == nil {
		return unsupported("join_maintenance_thread")
	}
	e.joinMaintenanceLocked()
	return nil
}

func (e *Engine) checkMaintenanceLocked() error {
	if !e.maintenancePending {
		return nil
	}
	if e.api.IsMaintenanceMode == nil || e.api.IsMaintenanceMode().GoBool() {
		return ErrMaintenance
	}
	e.joinMaintenanceLocked()
	return nil
}

func (e *Engine) joinMaintenanceLocked() {
	if e.api.JoinMaintenanceThread != nil {
		e.api.JoinMaintenanceThread()
	}
	e.maintenancePending = false
}

// SyncUserData starts Rime's native user data synchronization and reports
// whether Rime started it. Rime destroys all sessions when synchronization is
// requested, even if it fails to start. Create new sessions after maintenance.
func (e *Engine) SyncUserData() (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkLocked(); err != nil {
		return false, err
	}
	if e.api.SyncUserData == nil {
		return false, unsupported("sync_user_data")
	}
	if e.api.JoinMaintenanceThread == nil {
		return false, unsupported("join_maintenance_thread")
	}
	if err := e.initializeDeployerLocked(); err != nil {
		return false, err
	}
	e.maintenancePending = e.api.SyncUserData().GoBool()
	e.invalidateSessionsLocked()
	return e.maintenancePending, nil
}

// Prebuild synchronously builds all schemas in the shared data directory.
func (e *Engine) Prebuild() error {
	return e.deploy("prebuild", func(api *abi.API) deployFunc { return api.Prebuild })
}

// Deploy synchronously updates the workspace and deployed schemas.
func (e *Engine) Deploy() error {
	return e.deploy("deploy", func(api *abi.API) deployFunc { return api.Deploy })
}

// DeploySchema synchronously builds the schema at the given file path.
func (e *Engine) DeploySchema(path string) error {
	p, err := cString(path)
	if err != nil {
		return err
	}
	return e.deploy("deploy_schema", func(api *abi.API) deployFunc {
		if api.DeploySchema == nil {
			return nil
		}
		return func() abi.Bool { return api.DeploySchema(p) }
	})
}

// DeployConfigFile synchronously updates a config file using the given
// version key, such as "config_version".
func (e *Engine) DeployConfigFile(filename, versionKey string) error {
	file, err := cString(filename)
	if err != nil {
		return err
	}
	key, err := cString(versionKey)
	if err != nil {
		return err
	}
	return e.deploy("deploy_config_file", func(api *abi.API) deployFunc {
		if api.DeployConfigFile == nil {
			return nil
		}
		return func() abi.Bool { return api.DeployConfigFile(file, key) }
	})
}

// RunTask synchronously runs a registered Rime deployment task by name.
func (e *Engine) RunTask(name string) error {
	n, err := cString(name)
	if err != nil {
		return err
	}
	return e.deploy("run_task", func(api *abi.API) deployFunc {
		if api.RunTask == nil {
			return nil
		}
		return func() abi.Bool { return api.RunTask(n) }
	})
}

type deployFunc = func() abi.Bool

func (e *Engine) deploy(slot string, selectFunc func(*abi.API) deployFunc) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkLocked(); err != nil {
		return err
	}
	fn := selectFunc(e.api)
	if fn == nil {
		return unsupported(slot)
	}
	if err := e.initializeDeployerLocked(); err != nil {
		return err
	}
	if !fn().GoBool() {
		return failed(slot)
	}
	return nil
}

func (e *Engine) initializeDeployerLocked() error {
	if e.api.DeployerInitialize == nil {
		return unsupported("deployer_initialize")
	}
	// Deployment tasks are registered by the deployer modules. Passing nil
	// traits loads those modules while preserving the existing directories.
	// Upstream skips modules that have already been loaded.
	e.api.DeployerInitialize(nil)
	return nil
}

// SharedDataDir returns the effective shared data directory.
func (e *Engine) SharedDataDir() (string, error) {
	return e.deployerPath("get_shared_data_dir", func(api *abi.API) (func() *byte, pathReader) {
		return api.GetSharedDataDir, api.GetSharedDataDirS
	})
}

// UserDataDir returns the effective user data directory.
func (e *Engine) UserDataDir() (string, error) {
	return e.deployerPath("get_user_data_dir", func(api *abi.API) (func() *byte, pathReader) {
		return api.GetUserDataDir, api.GetUserDataDirS
	})
}

// PrebuiltDataDir returns the directory containing prebuilt data.
func (e *Engine) PrebuiltDataDir() (string, error) {
	return e.deployerPath("get_prebuilt_data_dir", func(api *abi.API) (func() *byte, pathReader) {
		return api.GetPrebuiltDataDir, api.GetPrebuiltDataDirS
	})
}

// StagingDir returns the directory containing data built by Rime.
func (e *Engine) StagingDir() (string, error) {
	return e.deployerPath("get_staging_dir", func(api *abi.API) (func() *byte, pathReader) {
		return api.GetStagingDir, api.GetStagingDirS
	})
}

// SyncDir returns the effective root directory for user data synchronization.
func (e *Engine) SyncDir() (string, error) {
	return e.deployerPath("get_sync_dir", func(api *abi.API) (func() *byte, pathReader) {
		return api.GetSyncDir, api.GetSyncDirS
	})
}

// UserDataSyncDir returns the synchronization directory for this installation.
func (e *Engine) UserDataSyncDir() (string, error) {
	return e.deployerPath("get_user_data_sync_dir", func(api *abi.API) (func() *byte, pathReader) {
		return nil, api.GetUserDataSyncDir
	})
}

// UserID returns Rime's installation ID.
func (e *Engine) UserID() (string, error) {
	return e.deployerPath("get_user_id", func(api *abi.API) (func() *byte, pathReader) {
		return api.GetUserID, nil
	})
}

type pathReader = func(*byte, uintptr)

func (e *Engine) deployerPath(slot string, selectReader func(*abi.API) (func() *byte, pathReader)) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkLocked(); err != nil {
		return "", err
	}
	legacy, read := selectReader(e.api)
	if read == nil {
		if legacy == nil {
			return "", unsupported(slot)
		}
		p := legacy()
		if p == nil {
			return "", failed(slot)
		}
		return abi.GoString(p), nil
	}
	value, _, err := readCStringBuffer(func(out *byte, size uintptr) bool {
		read(out, size)
		return true
	})
	return value, err
}
