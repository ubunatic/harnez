package tools

// Modifier bitmask representation.
// Bit 0: Left Ctrl   (1 << 0 = 0x01)
// Bit 1: Right Ctrl  (1 << 1 = 0x02)
// Bit 2: Left Alt    (1 << 2 = 0x04)
// Bit 3: Right Alt   (1 << 3 = 0x08)
// Bit 4: Left Super  (1 << 4 = 0x10)
// Bit 5: Right Super (1 << 5 = 0x20)
// Bit 6: Left Shift  (1 << 6 = 0x40)
// Bit 7: Right Shift (1 << 7 = 0x80)
type ModifierMask uint8

const (
	ModLeftCtrl   ModifierMask = 1 << 0 // 0x01
	ModRightCtrl  ModifierMask = 1 << 1 // 0x02
	ModLeftAlt    ModifierMask = 1 << 2 // 0x04
	ModRightAlt   ModifierMask = 1 << 3 // 0x08
	ModLeftSuper  ModifierMask = 1 << 4 // 0x10
	ModRightSuper ModifierMask = 1 << 5 // 0x20
	ModLeftShift  ModifierMask = 1 << 6 // 0x40
	ModRightShift ModifierMask = 1 << 7 // 0x80

	ModCtrl  ModifierMask = ModLeftCtrl | ModRightCtrl
	ModAlt   ModifierMask = ModLeftAlt | ModRightAlt
	ModSuper ModifierMask = ModLeftSuper | ModRightSuper
	ModShift ModifierMask = ModLeftShift | ModRightShift
	ModAny   ModifierMask = 0xFF
)

// Linux evdev keycodes from <linux/input-event-codes.h>.
const (
	evKey = 0x01

	keyLeftCtrl   = 29
	keyRightCtrl  = 97
	keyLeftShift  = 42
	keyRightShift = 54
	keyLeftAlt    = 56
	keyRightAlt   = 100
	keyLeftMeta   = 125 // Super / Windows key
	keyRightMeta  = 126
	keyCapsLock   = 58

	keyMax = 0x2ff // 767
)

// ModifierKeyDef maps an evdev keycode to a modifier bitmask and human-readable name.
type ModifierKeyDef struct {
	Code int
	Mask ModifierMask
	Name string
}

// WatchedModifierKeys defines all physical modifiers tracked by the daemon.
// CapsLock is intentionally excluded from the bitmask as it is a toggle lock rather than a held modifier.
// The 8 primary modifiers map directly to the 8 bits of a single byte.
var WatchedModifierKeys = []ModifierKeyDef{
	{Code: keyLeftCtrl, Mask: ModLeftCtrl, Name: "LeftCtrl"},
	{Code: keyRightCtrl, Mask: ModRightCtrl, Name: "RightCtrl"},
	{Code: keyLeftAlt, Mask: ModLeftAlt, Name: "LeftAlt"},
	{Code: keyRightAlt, Mask: ModRightAlt, Name: "RightAlt"},
	{Code: keyLeftMeta, Mask: ModLeftSuper, Name: "LeftSuper"},
	{Code: keyRightMeta, Mask: ModRightSuper, Name: "RightSuper"},
	{Code: keyLeftShift, Mask: ModLeftShift, Name: "LeftShift"},
	{Code: keyRightShift, Mask: ModRightShift, Name: "RightShift"},
}

// KeyCodeToModifierMask returns the ModifierMask bit corresponding to an evdev keycode, or 0 if not a modifier.
func KeyCodeToModifierMask(code int) ModifierMask {
	for _, def := range WatchedModifierKeys {
		if def.Code == code {
			return def.Mask
		}
	}
	return 0
}

// MaskNames returns a list of human-readable modifier names active in the mask.
func (m ModifierMask) Names() []string {
	var names []string
	for _, def := range WatchedModifierKeys {
		if m&def.Mask != 0 {
			names = append(names, def.Name)
		}
	}
	return names
}

// ShortNames returns concise, deduplicated modifier names (e.g. "Ctrl", "Super", "Alt", "Shift").
func (m ModifierMask) ShortNames() []string {
	var names []string
	if m&ModCtrl != 0 {
		names = append(names, "Ctrl")
	}
	if m&ModSuper != 0 {
		names = append(names, "Super")
	}
	if m&ModAlt != 0 {
		names = append(names, "Alt")
	}
	if m&ModShift != 0 {
		names = append(names, "Shift")
	}
	return names
}

// AnyActive returns true if any modifier bit is set.
func (m ModifierMask) AnyActive() bool {
	return m != 0
}

// DefaultModifierStatePath returns the standard world-readable runtime state file path.
const DefaultModifierStatePath = "/run/harnez/modifiers"
