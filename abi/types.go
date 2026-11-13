// Package abi binds the default, integer-Bool librime C ABI. C strings and
// handles returned here carry the ownership and lifetime defined by librime.
// The caller is responsible for serialization and for keeping the library
// loaded until all outstanding calls, callbacks and resources have finished.
package abi

import "unsafe"

// Bool is the C int used by the default librime API for boolean values.
type Bool int32

const (
	False Bool = 0 // False is the default C ABI's False macro.
	True  Bool = 1 // True is the default C ABI's True macro.
)

// BoolValue maps a Go boolean to a librime Bool.
func BoolValue(value bool) Bool {
	if value {
		return True
	}
	return False
}

// GoBool reports whether b is nonzero.
func (b Bool) GoBool() bool { return b != False }

// SessionID maps RimeSessionId, an opaque native-sized integer.
type SessionID = uintptr

// Traits maps RimeTraits. Modules is a pointer to a NULL-terminated array of
// C string pointers. Call Init before passing this struct to librime.
type Traits struct {
	DataSize             int32
	SharedDataDir        *byte
	UserDataDir          *byte
	DistributionName     *byte
	DistributionCodeName *byte
	DistributionVersion  *byte
	AppName              *byte
	Modules              **byte
	MinLogLevel          int32
	LogDir               *byte
	PrebuiltDataDir      *byte
	StagingDir           *byte
}

// Init sets DataSize as RIME_STRUCT_INIT would, leaving other fields intact.
func (v *Traits) Init() { v.DataSize = int32(unsafe.Sizeof(*v) - dataSizeWidth) }

// Composition maps RimeComposition. Its text positions are byte offsets.
type Composition struct {
	Length    int32
	CursorPos int32
	SelStart  int32
	SelEnd    int32
	Preedit   *byte
}

// Candidate maps RimeCandidate.
type Candidate struct {
	Text     *byte
	Comment  *byte
	Reserved unsafe.Pointer
}

// Menu maps RimeMenu. Candidates points to NumCandidates entries.
type Menu struct {
	PageSize                  int32
	PageNo                    int32
	IsLastPage                Bool
	HighlightedCandidateIndex int32
	NumCandidates             int32
	Candidates                *Candidate
	SelectKeys                *byte
}

// Commit maps RimeCommit. A successful GetCommit requires FreeCommit.
type Commit struct {
	DataSize int32
	Text     *byte
}

// Init sets DataSize as RIME_STRUCT_INIT would, leaving other fields intact.
func (v *Commit) Init() { v.DataSize = int32(unsafe.Sizeof(*v) - dataSizeWidth) }

// Context maps RimeContext. A successful GetContext requires FreeContext.
// SelectLabels, when present, points to Menu.PageSize string pointers.
type Context struct {
	DataSize          int32
	Composition       Composition
	Menu              Menu
	CommitTextPreview *byte
	SelectLabels      **byte
}

// Init sets DataSize as RIME_STRUCT_INIT would, leaving other fields intact.
func (v *Context) Init() { v.DataSize = int32(unsafe.Sizeof(*v) - dataSizeWidth) }

// CandidatePreview maps RimeCandidatePreview. A successful GetCandidatePreview
// requires FreeCandidatePreview.
type CandidatePreview struct {
	DataSize            int32
	TextBeforeSelection *byte
	SelectedText        *byte
	TextAfterSelection  *byte
}

// Init sets DataSize as RIME_STRUCT_INIT would, leaving other fields intact.
func (v *CandidatePreview) Init() { v.DataSize = int32(unsafe.Sizeof(*v) - dataSizeWidth) }

// Status maps RimeStatus. A successful GetStatus requires FreeStatus.
type Status struct {
	DataSize      int32
	SchemaID      *byte
	SchemaName    *byte
	IsDisabled    Bool
	IsComposing   Bool
	IsASCIIMode   Bool
	IsFullShape   Bool
	IsSimplified  Bool
	IsTraditional Bool
	IsASCIIPunct  Bool
}

// Init sets DataSize as RIME_STRUCT_INIT would, leaving other fields intact.
func (v *Status) Init() { v.DataSize = int32(unsafe.Sizeof(*v) - dataSizeWidth) }

// CandidateListIterator maps RimeCandidateListIterator. After a successful
// begin, call CandidateListEnd even if iteration stops early.
type CandidateListIterator struct {
	Ptr       unsafe.Pointer
	Index     int32
	Candidate Candidate
}

// Config maps RimeConfig, an opaque configuration handle.
type Config struct {
	Ptr unsafe.Pointer
}

// ConfigIterator maps RimeConfigIterator. After a successful begin, call
// ConfigEnd even if iteration stops early. Key and Path are borrowed.
type ConfigIterator struct {
	List  unsafe.Pointer
	Map   unsafe.Pointer
	Index int32
	Key   *byte
	Path  *byte
}

// SchemaList maps RimeSchemaList. List points to Size entries.
type SchemaList struct {
	Size uintptr
	List *SchemaListItem
}

// SchemaListItem maps RimeSchemaListItem.
type SchemaListItem struct {
	SchemaID *byte
	Name     *byte
	Reserved unsafe.Pointer
}

// StringSlice maps RimeStringSlice, a borrowed, length-delimited byte string.
type StringSlice struct {
	Str    *byte
	Length uintptr
}

// Module maps RimeModule. Initialize, Finalize and GetAPI hold C function
// addresses. Librime retains a registered module and its name indefinitely.
type Module struct {
	DataSize   int32
	ModuleName *byte
	Initialize uintptr
	Finalize   uintptr
	GetAPI     uintptr
}

// Init sets DataSize as RIME_STRUCT_INIT would, leaving other fields intact.
func (v *Module) Init() { v.DataSize = int32(unsafe.Sizeof(*v) - dataSizeWidth) }

// CustomAPI maps the version header shared by custom module API tables.
type CustomAPI struct {
	DataSize int32
}

// Init sets DataSize for the base RimeCustomApi. For an extension, set the
// embedded DataSize using the full extension's size minus sizeof(C int).
func (v *CustomAPI) Init() { v.DataSize = 0 }

// CustomSettings is an opaque levers RimeCustomSettings handle.
type CustomSettings struct{ _ [0]byte }

// SwitcherSettings is an opaque levers RimeSwitcherSettings handle.
type SwitcherSettings struct{ _ [0]byte }

// SchemaInfo is an opaque levers RimeSchemaInfo handle.
type SchemaInfo struct{ _ [0]byte }

// UserDictIterator maps the levers extension's RimeUserDictIterator.
type UserDictIterator struct {
	Ptr unsafe.Pointer
	I   uintptr
}

const dataSizeWidth = unsafe.Sizeof(int32(0))

// StructHasMember reports whether the entire member is within the extent
// declared by a versioned librime structure. Offset and size are in bytes.
func StructHasMember(dataSize int32, offset, size uintptr) bool {
	if dataSize < 0 {
		return false
	}
	total := dataSizeWidth + uintptr(dataSize)
	return offset <= total && size <= total-offset
}
