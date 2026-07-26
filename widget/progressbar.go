package widget

import (
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// ProgressBarOptions configures ProgressBar.
type ProgressBarOptions struct {
	Theme         style.Theme
	Indeterminate bool

	// Now is used to derive Indeterminate's animation frame, for
	// deterministic tests; nil uses time.Now.
	Now func() time.Time
}

// ProgressBar is a single-line, non-interactive progress indicator. In
// determinate mode (the default) it fills fraction (clamped to [0,1])
// of its width from theme.Primary. In Indeterminate mode it instead
// animates a moving highlight block whose position is derived from
// elapsed wall-clock time since the widget was first painted — see
// Spinner's doc comment for why time, not a Cmd/Msg tick counter, is
// the right way for a widget in this library to animate on its own.
func ProgressBar(fraction float64, opts ProgressBarOptions) tui.Node {
	return tui.Component(nil, progressBarProps{fraction: fraction, opts: opts}, func() tui.Widget {
		return &progressBarWidget{}
	})
}

type progressBarProps struct {
	fraction float64
	opts     ProgressBarOptions
}

type progressBarWidget struct {
	progressBarProps
	start time.Time
}

func (w *progressBarWidget) Reconcile(props any) bool {
	w.progressBarProps = props.(progressBarProps)
	if w.start.IsZero() {
		w.start = w.now()()
	}
	return true
}

func (w *progressBarWidget) now() func() time.Time {
	if w.opts.Now != nil {
		return w.opts.Now
	}
	return time.Now
}

const progressBarFrameLength = 80 * time.Millisecond

func (w *progressBarWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width <= 0 || height <= 0 {
		return
	}
	filled := cell.Style{Bg: w.opts.Theme.Primary}
	empty := cell.Style{Bg: w.opts.Theme.Border}

	if w.opts.Indeterminate {
		const blockWidth = 3
		period := width + blockWidth
		elapsed := w.now()().Sub(w.start)
		pos := int(elapsed/progressBarFrameLength) % period
		p.Fill(0, 0, width, height, ' ', empty)
		p.Fill(pos-blockWidth, 0, blockWidth, height, ' ', filled)
		return
	}

	frac := min(max(w.fraction, 0), 1)
	filledCols := int(float64(width)*frac + 0.5)
	p.Fill(0, 0, filledCols, height, ' ', filled)
	p.Fill(filledCols, 0, width-filledCols, height, ' ', empty)
}

func (w *progressBarWidget) HandleEvent(input.Event) tui.Cmd { return nil }
func (w *progressBarWidget) Focusable() bool                 { return false }
func (w *progressBarWidget) SetFocused(bool)                 {}
