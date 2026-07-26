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
	focusIdx   int

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
	a.root = reconcile(a.root, a.model.View())
	a.focusables = collectFocusables(a.root)

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

// HandleInput routes one decoded input.Event. Tab/Shift-Tab move focus
// among the Focusable/List widgets found by collectFocusables and are
// never forwarded any further. Every other event is delivered to the
// Model as a Msg via Dispatch (so application code can implement
// global keys, e.g. quit) and, separately, to whichever widget
// currently holds focus via its HandleEvent — see Focusable and List
// for how their onEvent prop turns that into an application Msg. It
// returns every Cmd produced by either path, for the caller to run.
func (a *App) HandleInput(e input.Event) []Cmd {
	if ke, ok := e.(input.KeyEvent); ok && ke.Key == input.KeyTab {
		a.moveFocus(ke.Mod&input.ModShift == 0)
		return nil
	}

	var cmds []Cmd
	if cmd := a.Dispatch(Msg(e)); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(a.focusables) > 0 {
		if cmd := a.focusables[a.focusIdx].HandleEvent(e); cmd != nil {
			cmds = append(cmds, cmd)
		}
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
	return cmds
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
// package render. It exercises the same pty-free real-terminal setup
// established by examples/rawecho and examples/multiplexer (M1/M6):
// MakeRaw, capability probing, a Watcher for resize/resume, and an
// input.Decoder goroutine fed through prefixedReader so bytes left
// over from Probe aren't lost.
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
			cmds := a.HandleInput(ev)
			redraw()
			for _, c := range cmds {
				runCmd(c)
			}

		case msg := <-msgCh:
			if _, ok := msg.(QuitMsg); ok {
				return nil
			}
			runCmd(a.Dispatch(msg))
			redraw()

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
