package tui

// Msg is anything fed into Model.Update: a decoded input.Event, the
// result of a Cmd, or an application-defined type. There's no
// interface to implement — any value can be a Msg, and Update
// type-switches on it.
type Msg any

// Cmd is a unit of asynchronous work. The App runs it on its own
// goroutine and feeds the Msg it returns back into the event loop,
// exactly like Elm/bubbletea's command pattern — the mechanism that
// keeps all Model mutation on the single event-loop goroutine even
// though the work producing a Msg (a timer, an I/O call, ...) doesn't
// run there. A nil Cmd (or a Cmd returning a nil Msg) does nothing.
type Cmd func() Msg

// Model is implemented by application state. Update and View must not
// block or do I/O directly — real work belongs in a Cmd returned from
// either, so the event-loop goroutine that calls them never stalls.
type Model interface {
	// Init returns the Cmd (if any) to run once, immediately after the
	// App starts, before the first View.
	Init() Cmd
	// Update handles msg, returning the next Model and an optional Cmd
	// to run as a result.
	Update(msg Msg) (Model, Cmd)
	// View describes the current frame as a Node tree.
	View() Node
}

// FocusAware is an optional interface a Model can implement to learn
// which focusable widget currently holds keyboard focus, identified
// by the Node.Key its Node was given (nil for an unkeyed widget, or
// if nothing is focused, or if focus is currently inside an active
// FocusScope — a Modal/CommandPalette's own scoped body isn't key-
// addressable this way; see collectFocusablesAndKeys). App.render()
// calls SetFocusedKey immediately before View(), so a Model can
// consult it while building the frame.
//
// The key reported is one render call behind the tree View() is about
// to produce — focus order for the *new* tree isn't known until after
// View() runs — but this lag is never visible on screen: Run() only
// flushes to the terminal once per input event, by which point at
// least one further render() has already caught up (see
// App.handleInput, which re-renders after forwarding an event to the
// focused widget).
//
// This exists so a Model can make presentation decisions that depend
// on "is focus currently anywhere within this logical group of
// widgets" — e.g. a pane multiplexer highlighting a pane's title bar
// while focus is on any of that pane's own focusables, not just the
// title bar itself — without tui needing a first-class grouping
// primitive: the Model compares the reported key against its own
// key-naming convention.
type FocusAware interface {
	SetFocusedKey(key any)
}

// QuitMsg, produced by Quit, tells the App to stop its Run loop.
type QuitMsg struct{}

// Quit is a Cmd that ends the App's Run loop. Returning it from Update
// (typically as `return m, tui.Quit()`) is the normal way for an
// application to exit.
func Quit() Cmd {
	return func() Msg { return QuitMsg{} }
}

// ClipboardMsg, produced by CopyToClipboard, tells Run to write Text
// to the system clipboard.
type ClipboardMsg struct{ Text string }

// CopyToClipboard is a Cmd that copies text to the system clipboard
// via OSC 52 (term.WriteClipboard) — the complementary answer to
// mouse reporting breaking the terminal's own native click-drag
// selection (see docs/DESIGN.md §9): an app-initiated copy that
// doesn't depend on it. It's usable from Update (`return m,
// tui.CopyToClipboard(text)`, the same shape as Quit()) or from a
// widget's onEvent callback, which can simply return ClipboardMsg{Text:
// text} directly as its Msg — HandleEvent's existing "wrap whatever
// onEvent returns in a Cmd" plumbing (see Focusable, List, ...) already
// turns that into the same thing.
//
// This Cmd deliberately does not write anything itself: doing so from
// its own goroutine could interleave, byte for byte, with the App's
// own render output on the same stdout, since both would be
// concurrent writers with no shared lock. Instead it returns
// ClipboardMsg, which Run recognizes and writes on its own single
// goroutine — the same one already responsible for all terminal
// output, so there's nothing to interleave with.
func CopyToClipboard(text string) Cmd {
	return func() Msg { return ClipboardMsg{Text: text} }
}

// FocusMsg, produced by SetFocusCmd, tells Run to move focus to Index
// via the App's own SetFocus (see app.go) — the same index space
// FocusIndex/SetFocus already use, and the same silent-no-op behavior
// on an out-of-range Index.
type FocusMsg struct{ Index int }

// SetFocusCmd is a Cmd that moves focus to idx, the missing
// during-Run() counterpart to calling App.SetFocus directly (only
// usable by code holding *App before Run starts). Returning it from
// Update (`return m, tui.SetFocusCmd(idx)`, the same shape as Quit()
// and CopyToClipboard()) lets application logic — not just literal
// Tab/click input — move focus in a live session: a numbered tab-jump
// hotkey, restoring focus after a modal closes, a wizard advancing to
// its next field, and so on.
func SetFocusCmd(idx int) Cmd {
	return func() Msg { return FocusMsg{Index: idx} }
}

// BatchMsg, produced by Batch, tells the App to run each Cmd
// concurrently rather than treating BatchMsg itself as an application
// Msg — Update never sees a BatchMsg.
type BatchMsg []Cmd

// Batch returns a Cmd that runs every non-nil Cmd in cmds concurrently,
// each feeding its own resulting Msg back into the event loop
// independently (there's no combined result to wait for). Useful when
// Update needs to kick off more than one piece of async work in
// response to a single Msg.
func Batch(cmds ...Cmd) Cmd {
	var filtered []Cmd
	for _, c := range cmds {
		if c != nil {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return func() Msg { return BatchMsg(filtered) }
}
