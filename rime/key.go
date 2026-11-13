package rime

// Common librime modifier masks. Pass these to Session.ProcessKey.
const (
	ShiftMask   int32 = 1 << 0
	LockMask    int32 = 1 << 1
	ControlMask int32 = 1 << 2
	AltMask     int32 = 1 << 3
	SuperMask   int32 = 1 << 26
	HyperMask   int32 = 1 << 27
	MetaMask    int32 = 1 << 28
	ReleaseMask int32 = 1 << 30
)

// Common X11 keysyms for non-printable keys. Printable ASCII keysyms have
// the same value as the corresponding Go rune literals.
const (
	KeyBackSpace int32 = 0xff08
	KeyTab       int32 = 0xff09
	KeyReturn    int32 = 0xff0d
	KeyEscape    int32 = 0xff1b
	KeyHome      int32 = 0xff50
	KeyLeft      int32 = 0xff51
	KeyUp        int32 = 0xff52
	KeyRight     int32 = 0xff53
	KeyDown      int32 = 0xff54
	KeyPageUp    int32 = 0xff55
	KeyPageDown  int32 = 0xff56
	KeyEnd       int32 = 0xff57
	KeyDelete    int32 = 0xffff
)
