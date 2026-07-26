package widget

import (
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/tui"
)

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// SpinnerOptions configures Spinner.
type SpinnerOptions struct {
	Style cell.Style

	// FrameLength is how long each animation frame is shown; <= 0
	// defaults to 80ms.
	FrameLength time.Duration

	// Now is used to derive the current animation frame, for
	// deterministic tests; nil uses time.Now.
	Now func() time.Time
}

// Spinner is a single-cell animated indeterminate-progress indicator
// (the classic braille-dot spinner). Its animation frame is derived
// from elapsed wall-clock time since the widget was first painted —
// the one piece of state it retains, docs/DESIGN.md §3.1's "cursor
// blink phase" example made concrete — rather than from a ticking Cmd/
// Msg counter: this library's App only ever redraws in response to a
// Msg (the change-driven-redraw premise §8 contrasts with M6's
// fixed-rate prototype loop), so a widget that wants to animate
// smoothly has to derive its frame from the clock itself, not from a
// counter something else has to remember to keep incrementing.
// Whether a given app's redraw cadence makes that look smooth while
// otherwise idle is the app's concern, not this widget's — and out of
// scope for M10, which has no live demo requirement.
func Spinner(opts SpinnerOptions) tui.Node {
	return tui.Component(nil, opts, func() tui.Widget {
		return &spinnerWidget{}
	})
}

type spinnerWidget struct {
	opts  SpinnerOptions
	start time.Time
}

func (w *spinnerWidget) Reconcile(props any) bool {
	w.opts = props.(SpinnerOptions)
	if w.start.IsZero() {
		w.start = w.now()()
	}
	return true
}

func (w *spinnerWidget) now() func() time.Time {
	if w.opts.Now != nil {
		return w.opts.Now
	}
	return time.Now
}

func (w *spinnerWidget) frameLength() time.Duration {
	if w.opts.FrameLength > 0 {
		return w.opts.FrameLength
	}
	return 80 * time.Millisecond
}

func (w *spinnerWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width <= 0 || height <= 0 {
		return
	}
	elapsed := w.now()().Sub(w.start)
	idx := int(elapsed/w.frameLength()) % len(spinnerFrames)
	if idx < 0 {
		idx += len(spinnerFrames)
	}
	p.SetCell(0, 0, spinnerFrames[idx], w.opts.Style)
}

func (w *spinnerWidget) HandleEvent(input.Event) tui.Cmd { return nil }
func (w *spinnerWidget) Focusable() bool                 { return false }
func (w *spinnerWidget) SetFocused(bool)                 {}
