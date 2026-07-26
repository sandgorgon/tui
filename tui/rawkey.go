package tui

import "github.com/sandgorgon/tui/input"

// RawKeyClaimer is implemented by a widget that wants Tab delivered to
// it directly while focused, rather than intercepted globally for
// Tab/Shift-Tab navigation — e.g. TextArea (to insert a literal tab
// character) or a Terminal hosting a shell (so tab-completion works).
//
// ReleaseKey names the key that, instead of being forwarded to the
// widget, releases the claim and moves focus onward (like an
// unclaimed Tab would have) — the guaranteed way out every such widget
// must provide (trapping keyboard focus with no escape is a real
// accessibility problem, not just an inconvenience — the same
// rationale behind code editors like CodeMirror shipping a "tab moves
// focus" toggle). It's configurable per widget instance rather than
// fixed to one key library-wide: a Terminal hosting a real shell must
// forward Esc to it (vim's insert-mode exit, readline bindings, ...),
// so Esc can't be its release key the way it safely can be for
// TextArea, which has no independent use for Esc.
type RawKeyClaimer interface {
	WantsRawTab() bool
	ReleaseKey() input.KeyEvent
}
