package tui

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/render"
	"github.com/sandgorgon/tui/term"
)

// App drives a Model: it owns the retained Node tree (see reconcile.go)
// and the frame buffer, and, in Run, the real terminal I/O loop.
// NewApp/Dispatch/Resize/HandleInput/Buffer are pure and I/O-free —
// the core stepping logic, usable from a headless test with no real
// tty by injecting synthetic Msgs/Events and inspecting Buffer() (see
// docs/DESIGN.md §10); Run is a thin wrapper connecting that core to a
// real terminal.
//
// An App is not safe for concurrent use — by design, everything that
// touches Model runs on whichever single goroutine calls Dispatch/
// HandleInput (Run's own event loop, when used), per docs/DESIGN.md
// §3.1's single-threaded-Model guarantee.
type App struct {
	model Model

	root *retained
	buf  *cell.Buffer

	focusables []Widget
	focusKeys  []any // parallel to focusables; see collectFocusablesAndKeys and FocusAware
	focusIdx   int
	rects      map[Widget]cell.Rect // absolute on-screen bounds of each focusable, see hittest.go

	initCmd Cmd
}

// NewApp constructs an App for model sized width x height, and runs
// the first View/reconcile/paint (but not Init's Cmd — see InitCmd).
func NewApp(model Model, width, height int) *App {
	a := &App{model: model, buf: cell.NewBuffer(width, height)}
	a.initCmd = model.Init()
	a.render()
	return a
}

// InitCmd returns the Cmd produced by Model.Init, consuming it — a
// second call returns nil. Run calls this itself; a headless test
// driving an App directly should call it too if it wants Init's Cmd to
// run (e.g. `if cmd := a.InitCmd(); cmd != nil { a.Dispatch(cmd()) }`).
func (a *App) InitCmd() Cmd {
	c := a.initCmd
	a.initCmd = nil
	return c
}

// Buffer returns the App's current frame. It's owned by the App and
// overwritten in place on the next Dispatch/Resize — copy it (e.g. via
// its String method) before mutating the App further if a test needs
// to compare frames.
func (a *App) Buffer() *cell.Buffer { return a.buf }

// Close disposes the App's current Node tree (see dispose.go),
// releasing anything any retained widget in it holds — e.g. a
// widget.Terminal's pty and reader goroutine. Run calls this itself
// on the way out; headless callers driving an App directly should
// call it too once they're done with it.
func (a *App) Close() error {
	disposeTree(a.root)
	return nil
}

// Dispatch feeds msg through Model.Update, re-reconciles the Node tree
// against the resulting View(), repaints, and returns any Cmd Update
// produced (nil if none).
func (a *App) Dispatch(msg Msg) Cmd {
	model, cmd := a.model.Update(msg)
	a.model = model
	a.render()
	return cmd
}

// Resize changes the frame buffer's size and repaints. A Box-based
// View re-runs layout.Split against whatever size it's painted into,
// so this alone is enough to look right — call Dispatch with an
// application-defined Msg too if Update itself needs to react to the
// new size.
func (a *App) Resize(width, height int) {
	a.buf.Resize(width, height)
	a.render()
}

// render reconciles the Node tree, recomputes the focus order, keeps
// focus on the same slot if it still exists (clamping otherwise), and
// repaints — the one place all three stay in sync.
func (a *App) render() {
	if fa, ok := a.model.(FocusAware); ok {
		// Necessarily reports the *previous* frame's focus state — the
		// new tree (and its focus order) doesn't exist until View()
		// below returns. See FocusAware's doc comment for why that lag
		// is never visible on screen.
		var key any
		if a.focusIdx >= 0 && a.focusIdx < len(a.focusKeys) {
			key = a.focusKeys[a.focusIdx]
		}
		fa.SetFocusedKey(key)
	}

	a.root = reconcile(a.root, a.model.View())
	a.focusables, a.focusKeys = collectFocusablesAndKeys(a.root)

	a.rects = make(map[Widget]cell.Rect, len(a.focusables))
	collectRects(a.root, cell.Rect{W: a.buf.Width, H: a.buf.Height}, a.rects)

	if len(a.focusables) == 0 {
		a.focusIdx = 0
	} else if a.focusIdx >= len(a.focusables) {
		a.focusIdx = len(a.focusables) - 1
	}
	for i, w := range a.focusables {
		w.SetFocused(i == a.focusIdx)
	}

	// A Node tree isn't guaranteed to paint every cell every frame
	// (margins, gaps, a shorter Text than last frame's, ...), so
	// without this, cells the new tree doesn't touch would keep
	// whatever the previous frame left there. render.Renderer's own
	// diffing is unaffected: it still only emits bytes for cells that
	// actually changed, since that diff is against its own remembered
	// terminal state, not against this Clear.
	a.buf.Clear(cell.Style{})
	a.root.paint(cell.NewPainter(a.buf))

	// A widget that's both the active FocusScope (e.g. an open Modal)
	// and an OverlayPainter gets a second, full-buffer painting pass
	// after everything else — see OverlayPainter's doc comment for why
	// that's necessary (Box's layout.Split gives every child its own
	// non-overlapping Rect; a modal needs to cover its siblings, not
	// sit alongside them).
	if scope := findActiveFocusScope(a.root); scope != nil {
		if overlay, ok := scope.(OverlayPainter); ok {
			overlay.PaintOverlay(cell.NewPainter(a.buf))
		}
	}
}

// HandleInput routes one decoded input.Event. Tab/Shift-Tab normally
// move focus among the Focusable/List widgets found by
// collectFocusables and are never forwarded any further — unless the
// focused widget implements RawKeyClaimer and WantsRawTab, in which
// case Tab goes straight to it instead (e.g. TextArea, so a literal
// tab character can be typed) and only that widget's own declared
// ReleaseKey moves focus onward. A MouseEvent landing inside a
// tracked widget's bounds (see hitTest) moves focus there first
// (click-to-focus) and is forwarded with its coordinates translated
// to be local to that widget, so a widget never needs to know its own
// absolute screen position — the same principle cell.Painter.Clip
// already uses for drawing. A MouseEvent landing on nothing tracked
// while a FocusScope is active (see hitTest's doc comment on why that
// always misses) is checked against the scope's OverlayBounds, if it
// implements that interface: outside the reported bounds, the event is
// withheld from the scope's focused widget entirely (rather than
// delivered with raw absolute coordinates, a real bug for any scope
// body that reacts to MouseEvent) and, if the scope also implements
// OutsideClicker, forwarded to it instead. Every event is also
// delivered to the Model as a Msg via Dispatch, at its original
// (untranslated) coordinates, so application code can implement global
// keys or react to absolute screen position — and, separately, to
// whichever widget currently holds focus via its HandleEvent — see
// Focusable and List for how their onEvent prop turns that into an
// application Msg. It returns every Cmd produced by any of these
// paths, for the caller to run.
func (a *App) HandleInput(e input.Event) []Cmd {
	dispatchCmd, widgetCmd, widgetFirst := a.handleInput(e)
	var cmds []Cmd
	if widgetFirst {
		if widgetCmd != nil {
			cmds = append(cmds, widgetCmd)
		}
		if dispatchCmd != nil {
			cmds = append(cmds, dispatchCmd)
		}
	} else {
		if dispatchCmd != nil {
			cmds = append(cmds, dispatchCmd)
		}
		if widgetCmd != nil {
			cmds = append(cmds, widgetCmd)
		}
	}
	return cmds
}

// handleInput is HandleInput's implementation, splitting its result
// into dispatchCmd (from Dispatch(Msg(e)), i.e. Update's own reaction
// to the raw event — may legitimately be real asynchronous work, per
// Cmd's documented contract) and widgetCmd (from the focused widget's
// HandleEvent, or an active FocusScope's HandleOutsideClick) instead
// of a single combined slice. widgetFirst reports which came first, so
// HandleInput can reassemble the exact same order it always has.
//
// Run() calls this directly (not HandleInput) so it can resolve
// widgetCmd synchronously via resolveWidgetCmd instead of routing it
// through the async goroutine+channel Cmd machinery — see
// resolveWidgetCmd's doc comment for why that split exists (#18).
// HandleInput itself is unaffected: it still returns widgetCmd
// unresolved, exactly as before.
func (a *App) handleInput(e input.Event) (dispatchCmd Cmd, widgetCmd Cmd, widgetFirst bool) {
	if ke, ok := e.(input.KeyEvent); ok {
		claims, releaseKey := a.rawKeyClaim()
		switch {
		case claims && ke == releaseKey:
			a.moveFocus(true)
			return nil, nil, false
		case !claims && ke.Key == input.KeyTab:
			a.moveFocus(ke.Mod&input.ModShift == 0)
			return nil, nil, false
		}
	}

	widgetEvent := e
	deliverToFocused := true
	if me, ok := e.(input.MouseEvent); ok {
		if idx, local, found := a.hitTest(me); found {
			if idx != a.focusIdx {
				a.focusIdx = idx
				a.render()
			}
			widgetEvent = local
		} else if scope := findActiveFocusScope(a.root); scope != nil {
			if ob, ok := scope.(OverlayBounds); ok {
				if r, ok := ob.OverlayBounds(); ok && !rectContains(r, me.X, me.Y) {
					deliverToFocused = false
					if oc, ok := scope.(OutsideClicker); ok {
						widgetCmd = oc.HandleOutsideClick(me)
						widgetFirst = true
					}
				}
			}
		}
	}

	dispatchCmd = a.Dispatch(Msg(e))
	if deliverToFocused && len(a.focusables) > 0 {
		widgetCmd = a.focusables[a.focusIdx].HandleEvent(widgetEvent)
		// HandleEvent can mutate the widget's own retained state
		// directly (e.g. widget.Viewport's scroll offset) rather than
		// only ever going through a Msg/Update round trip the way
		// List's cursor does — Dispatch's render() above ran before
		// that mutation happened, so without this second render() the
		// change wouldn't be visible until some later, unrelated
		// render. Cheap even when nothing changed: render() re-walks
		// the same Node tree render.Renderer will only emit bytes for
		// cells that actually differ from the terminal's last frame.
		a.render()
	}
	return dispatchCmd, widgetCmd, widgetFirst
}

// resolveWidgetCmd runs c — and, recursively, every sub-Cmd of a
// BatchMsg it produces — synchronously, applying each resulting Msg to
// Update via Dispatch immediately instead of deferring it through
// Run's async goroutine+channel Cmd machinery.
//
// This is safe specifically for widget-sourced Cmds (the focused
// widget's HandleEvent, an OutsideClicker's HandleOutsideClick)
// because every built-in widget's callback-style hook — OnChange,
// OnCursorChange, OnSubmit, OnSelect, and so on — is typed
// func(...) Msg, not Cmd: the widget can only ever repackage a Msg
// value it already computed before HandleEvent returned, so calling it
// here does no blocking work. It is NOT safe for Dispatch's own Cmd
// (Update's reaction to the raw event), which is why that one stays on
// the normal async path in Run() — Cmd's documented contract is
// asynchronous work, and an app is free to kick off real I/O directly
// from a keypress.
//
// Resolving widget Cmds synchronously — rather than handing them to
// runCmd, which sends the result back through a channel a later,
// possibly-reordered select iteration applies — closes a real ordering
// race (#18): a widget's Msg could still be in flight when the *next*
// input event's own synchronous Dispatch call already ran, corrupting
// any caller-side state kept in sync with the widget's live value
// (e.g. restoring cursor position across a tab/pane switch driven by
// TextArea.OnCursorChange).
//
// QuitMsg/ClipboardMsg/FocusMsg are deliberately left unresolved (an
// Immediate-shaped Cmd for the caller to run as before): they're
// one-shot side-effecting commands, not state #18's race can corrupt,
// and actually applying them (writing the clipboard, stopping Run)
// needs machinery Dispatch's I/O-free contract doesn't have.
func (a *App) resolveWidgetCmd(c Cmd) []Cmd {
	if c == nil {
		return nil
	}
	msg := c()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(BatchMsg); ok {
		var out []Cmd
		for _, sub := range batch {
			out = append(out, a.resolveWidgetCmd(sub)...)
		}
		return out
	}
	switch msg.(type) {
	case QuitMsg, ClipboardMsg, FocusMsg:
		return []Cmd{func() Msg { return msg }}
	default:
		if follow := a.Dispatch(msg); follow != nil {
			return []Cmd{follow}
		}
		return nil
	}
}

// rectContains reports whether (x,y) falls within r.
func rectContains(r cell.Rect, x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// hitTest reports the focusables index and translated (local-to-
// widget) coordinates for a MouseEvent landing on a tracked focusable
// widget's bounds, or found=false if it lands on nothing tracked —
// the background, a non-focusable widget (its Rect was never
// recorded), or, while a FocusScope is active, anything at all (see
// collectRects's doc comment on why that degrades safely rather than
// needing an explicit check here).
func (a *App) hitTest(me input.MouseEvent) (idx int, local input.MouseEvent, found bool) {
	for i, w := range a.focusables {
		r, ok := a.rects[w]
		if !ok || me.X < r.X || me.X >= r.X+r.W || me.Y < r.Y || me.Y >= r.Y+r.H {
			continue
		}
		local = me
		local.X, local.Y = me.X-r.X, me.Y-r.Y
		return i, local, true
	}
	return 0, input.MouseEvent{}, false
}

// rawKeyClaim reports whether the currently focused widget implements
// RawKeyClaimer and wants raw Tab, and if so its declared release key.
func (a *App) rawKeyClaim() (claims bool, releaseKey input.KeyEvent) {
	if len(a.focusables) == 0 {
		return false, input.KeyEvent{}
	}
	rc, ok := a.focusables[a.focusIdx].(RawKeyClaimer)
	if !ok || !rc.WantsRawTab() {
		return false, input.KeyEvent{}
	}
	return true, rc.ReleaseKey()
}

// FocusIndex returns the index into the current focus order (document
// order over the tree View produced for the last render) of the
// currently focused widget, or 0 if there are no focusables.
func (a *App) FocusIndex() int { return a.focusIdx }

// SetFocus moves focus directly to the focusable at idx, the same
// index space FocusIndex reports and the same order the ordinary Tab/
// Shift+Tab traversal (moveFocus) walks. It reports whether idx was in
// range; out of range (including when there are no focusables at all)
// is a no-op returning false, matching moveFocus's own defensive
// len(a.focusables) == 0 check rather than panicking.
//
// The reassignment is unconditional, mirroring HandleInput's mouse-
// click branch: a widget mid-raw-key-claim (RawKeyClaimer, see
// rawkey.go) is not given any chance to react before losing focus,
// exactly as clicking away from it already doesn't.
func (a *App) SetFocus(idx int) bool {
	if idx < 0 || idx >= len(a.focusables) {
		return false
	}
	a.focusIdx = idx
	a.render()
	return true
}

func (a *App) moveFocus(forward bool) {
	if len(a.focusables) == 0 {
		return
	}
	n := len(a.focusables)
	if forward {
		a.focusIdx = (a.focusIdx + 1) % n
	} else {
		a.focusIdx = (a.focusIdx - 1 + n) % n
	}
	a.render()
}

// Run puts the terminal into raw mode and the alternate screen, then
// drives the App until Update produces a Cmd that yields QuitMsg (see
// Quit) or stdin returns an error, rendering each frame's diff via
// package render. A ClipboardMsg (see CopyToClipboard) is handled here
// too — written directly rather than passed to Dispatch, since Run's
// own goroutine is the only place it's safe to write it without
// interleaving with render output on the same stdout. Likewise a
// FocusMsg (see SetFocusCmd) is applied here via SetFocus rather than
// passed to Dispatch, since SetFocus touches App's own private focus
// state the same way ClipboardMsg's write touches stdout. It exercises the
// same pty-free real-terminal setup established by examples/rawecho
// and examples/multiplexer (M1/M6): MakeRaw, capability probing, a
// Watcher for resize/resume, and an input.Decoder goroutine fed
// through prefixedReader so bytes left over from Probe aren't lost.
func (a *App) Run() error {
	if !term.IsTerminal(os.Stdin) {
		return errors.New("stdin is not a terminal")
	}

	saved, err := term.MakeRaw(os.Stdin)
	if err != nil {
		return fmt.Errorf("MakeRaw: %w", err)
	}
	defer term.Restore(os.Stdin, saved)
	defer a.Close()

	os.Stdout.WriteString("\x1b[?1049h")
	defer os.Stdout.WriteString("\x1b[?1049l")

	caps, leftover := term.Probe(os.Stdin, os.Stdout, 500*time.Millisecond, term.DetectEnv(os.Getenv))
	renderer := render.NewRenderer(render.Options{
		ColorLevel:         caps.ColorLevel,
		SynchronizedOutput: caps.SynchronizedOutput,
	})

	if size, err := term.GetSize(os.Stdout); err == nil {
		a.Resize(size.Cols, size.Rows)
	}

	dec := input.NewDecoder(&prefixedReader{
		prefix:        leftover,
		f:             os.Stdin,
		graceDeadline: time.Now().Add(2 * time.Second),
	})
	events := make(chan input.Event)
	decodeErrs := make(chan error, 1)
	go func() {
		for {
			ev, err := dec.Decode()
			if err != nil {
				decodeErrs <- err
				return
			}
			events <- ev
		}
	}()

	// msgCh is the bounded Cmd/Msg channel §9 of docs/DESIGN.md calls
	// for: each Cmd runs on its own goroutine and blocks sending its
	// result here, so a slow event loop applies backpressure to Cmd
	// producers instead of the channel growing without bound. Resize
	// coalescing is handled upstream by term.Watcher itself (a
	// buffered-1, non-blocking-send channel — see term/signal.go), not
	// here; it doesn't need this generic path at all.
	msgCh := make(chan Msg, 256)
	var runCmd func(Cmd)
	runCmd = func(c Cmd) {
		if c == nil {
			return
		}
		go func() {
			msg := c()
			if msg == nil {
				return
			}
			if batch, ok := msg.(BatchMsg); ok {
				for _, sub := range batch {
					runCmd(sub)
				}
				return
			}
			msgCh <- msg
		}()
	}

	watcher := term.NewWatcher()
	defer watcher.Stop()

	runCmd(a.InitCmd())

	redraw := func() {
		_ = renderer.Render(os.Stdout, a.buf, 0, 0, false)
	}
	redraw()

	for {
		select {
		case ev := <-events:
			dispatchCmd, widgetCmd, _ := a.handleInput(ev)
			// widgetCmd is resolved synchronously, before redraw(),
			// rather than run through runCmd like dispatchCmd — see
			// resolveWidgetCmd's doc comment (#18).
			followups := a.resolveWidgetCmd(widgetCmd)
			redraw()
			runCmd(dispatchCmd)
			for _, c := range followups {
				runCmd(c)
			}

		case msg := <-msgCh:
			switch m := msg.(type) {
			case QuitMsg:
				return nil
			case ClipboardMsg:
				// Written here, on Run's single goroutine, rather than
				// from whatever Cmd produced this Msg — see
				// CopyToClipboard's doc comment on why that matters.
				_ = term.WriteClipboard(os.Stdout, m.Text)
			case FocusMsg:
				// SetFocus itself only updates a.buf (via render()); it
				// doesn't flush to the terminal, so this needs the same
				// explicit redraw() the "ev := <-events" case above
				// takes after HandleInput.
				a.SetFocus(m.Index)
				redraw()
			default:
				runCmd(a.Dispatch(msg))
				redraw()
			}

		case <-watcher.Resize():
			if size, err := term.GetSize(os.Stdout); err == nil {
				a.Resize(size.Cols, size.Rows)
				redraw()
			}

		case <-watcher.Cont():
			if _, err := term.MakeRaw(os.Stdin); err == nil {
				redraw()
			}

		case err := <-decodeErrs:
			return err
		}
	}
}

// prefixedReader replays prefix (term.Probe's leftover bytes) before
// delegating to f, stripping any late DA1/DECRQM reply within
// graceDeadline — see examples/rawecho's identical helper for the full
// rationale (docs/DESIGN.md's M1 post-mortem in project memory).
type prefixedReader struct {
	prefix        []byte
	f             *os.File
	graceDeadline time.Time
}

func (r *prefixedReader) Read(p []byte) (int, error) {
	if len(r.prefix) > 0 {
		n := copy(p, r.prefix)
		r.prefix = r.prefix[n:]
		return n, nil
	}
	for {
		n, err := r.f.Read(p)
		if n > 0 && time.Now().Before(r.graceDeadline) {
			n = copy(p, term.StripLateReply(p[:n]))
		}
		if n > 0 || err != nil {
			return n, err
		}
	}
}

func (r *prefixedReader) SetReadDeadline(t time.Time) error {
	return r.f.SetReadDeadline(t)
}
