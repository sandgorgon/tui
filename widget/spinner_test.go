package widget

import (
	"testing"
	"time"

	"github.com/sandgorgon/tui/cell"
)

func TestSpinnerAdvancesFrameWithClock(t *testing.T) {
	base := time.Unix(0, 0)
	now := base

	node := Spinner(SpinnerOptions{FrameLength: 100 * time.Millisecond, Now: func() time.Time { return now }})
	tr := newTree(t, node)

	buf := cell.NewBuffer(1, 1)
	tr.Paint(cell.NewPainter(buf))
	first := buf.At(0, 0).Rune

	now = base.Add(350 * time.Millisecond) // 3 frames later
	tr.Reconcile(Spinner(SpinnerOptions{FrameLength: 100 * time.Millisecond, Now: func() time.Time { return now }}))
	buf2 := cell.NewBuffer(1, 1)
	tr.Paint(cell.NewPainter(buf2))

	if buf2.At(0, 0).Rune == first {
		t.Error("expected a different spinner glyph after 3 simulated frame-lengths")
	}
	if buf2.At(0, 0).Rune != spinnerFrames[3] {
		t.Errorf("frame = %q, want spinnerFrames[3] = %q", buf2.At(0, 0).Rune, spinnerFrames[3])
	}
}

func TestSpinnerUsesGivenStyle(t *testing.T) {
	style := cell.Style{Fg: cell.ANSIColor(5)}
	node := Spinner(SpinnerOptions{Style: style, Now: func() time.Time { return time.Unix(0, 0) }})
	buf := cell.NewBuffer(1, 1)
	paintNode(t, node, buf)

	if got := buf.At(0, 0).Style; got != style {
		t.Errorf("style = %+v, want %+v", got, style)
	}
}

func TestSpinnerDefaultFrameLength(t *testing.T) {
	node := Spinner(SpinnerOptions{Now: func() time.Time { return time.Unix(0, 0) }})
	buf := cell.NewBuffer(1, 1)
	paintNode(t, node, buf) // just verifying it doesn't panic with FrameLength unset
	if buf.At(0, 0).Rune != spinnerFrames[0] {
		t.Errorf("frame at t=0 = %q, want spinnerFrames[0]", buf.At(0, 0).Rune)
	}
}
