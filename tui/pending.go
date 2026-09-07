package tui

// PendingMsgSource is an optional interface a retained Widget can
// implement to report a Msg it produced asynchronously, outside any
// HandleEvent call — the case a background goroutine notices something
// (e.g. widget.Terminal's child process exiting) with no channel of
// its own into the Cmd/Msg loop, since only Model.Update/
// Widget.HandleEvent can originate a Cmd.
//
// App.Dispatch calls TakePendingMsg on every widget in the tree after
// every render — the one funnel both a real input event's own raw
// Dispatch(Msg(e)) call and a host's periodic redraw Cmd (already
// needed by any widget with this kind of background state, so it
// exists in every app that would ever need this) already pass through.
// That means the Msg surfaces on whatever Dispatch happens to run
// next, rather than needing a real input event to notice it — the
// widget itself never has to spend an incoming event on detection, so
// nothing it would otherwise have done with that event is lost.
//
// TakePendingMsg must consume/clear whatever it reports: it's called
// unconditionally on every Dispatch, so returning the same Msg twice
// would apply it twice. Return nil whenever there's nothing new.
type PendingMsgSource interface {
	TakePendingMsg() Msg
}

// collectPendingMsgs walks r for widgets implementing PendingMsgSource
// and returns whatever non-nil Msgs they report, consuming each — the
// same shape of tree walk as disposeTree, just checking for a
// different optional interface. Like disposeTree (and unlike the
// focus-order walk), it doesn't descend into a widget's own private
// Tree (a FocusScope's scoped body, e.g. Modal/CommandPalette) — a
// widget hosting a PendingMsgSource inside one of those would need to
// forward it itself, the same limitation collectFocusablesAndKeys
// already documents for scoped content.
func collectPendingMsgs(r *retained) []Msg {
	if r == nil {
		return nil
	}
	var out []Msg
	switch r.kind {
	case kindBox:
		for _, c := range r.children {
			out = append(out, collectPendingMsgs(c)...)
		}
	case kindWidget:
		if src, ok := r.widget.(PendingMsgSource); ok {
			if msg := src.TakePendingMsg(); msg != nil {
				out = append(out, msg)
			}
		}
	}
	return out
}
