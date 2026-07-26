package tui

import "io"

// disposeTree recursively closes every kindWidget node within r whose
// Widget implements io.Closer, then walks into it — called whenever
// the reconciler discards a retained subtree that will never be
// reconciled again (a Box child no longer present in the new frame, or
// a tree slot whose Node kind changed), so a widget managing a real
// resource (a Terminal's pty and reader goroutine, a Viewport's or
// Modal's wrapped tui.Tree) gets a chance to release it.
//
// Implementing io.Closer is entirely optional — most widgets (Text,
// List, Tabs, ...) have nothing to release and are left alone.
func disposeTree(r *retained) {
	if r == nil {
		return
	}
	switch r.kind {
	case kindBox:
		for _, c := range r.children {
			disposeTree(c)
		}
	case kindWidget:
		if closer, ok := r.widget.(io.Closer); ok {
			_ = closer.Close()
		}
	}
}
