package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
)

func TestStatusBarAlignsSegments(t *testing.T) {
	left := []Segment{{Text: "NORMAL"}}
	center := []Segment{{Text: "msg"}}
	right := []Segment{{Text: "Ln 5"}}

	node := StatusBar(left, center, right, cell.Style{})
	buf := cell.NewBuffer(20, 1)
	paintNode(t, node, buf)

	got := buf.String()
	if !strings.HasPrefix(got, "NORMAL") {
		t.Errorf("Buffer = %q, want to start with left segment", got)
	}
	if !strings.HasSuffix(got, "Ln 5") {
		t.Errorf("Buffer = %q, want to end with right segment", got)
	}
	if !strings.Contains(got, "msg") {
		t.Errorf("Buffer = %q, missing center segment", got)
	}
	// "msg" (width 3) roughly centered in a 20-wide bar: starts near (20-3)/2=8.
	if idx := strings.Index(got, "msg"); idx < 6 || idx > 10 {
		t.Errorf("center segment at column %d, want roughly 8", idx)
	}
}

func TestStatusBarFillsBackground(t *testing.T) {
	bg := cell.Style{Bg: cell.ANSIColor(4)}
	node := StatusBar(nil, nil, nil, bg)
	buf := cell.NewBuffer(5, 1)
	paintNode(t, node, buf)

	for x := range 5 {
		if got := buf.At(x, 0).Style.Bg; got != bg.Bg {
			t.Errorf("At(%d,0).Style.Bg = %+v, want %+v", x, got, bg.Bg)
		}
	}
}

func TestStatusBarSegmentBgIsOverriddenByBarBackground(t *testing.T) {
	bg := cell.Style{Bg: cell.ANSIColor(2)}
	left := []Segment{{Text: "x", Style: cell.Style{Fg: cell.ANSIColor(1), Bg: cell.ANSIColor(9)}}}

	node := StatusBar(left, nil, nil, bg)
	buf := cell.NewBuffer(3, 1)
	paintNode(t, node, buf)

	got := buf.At(0, 0).Style
	if got.Fg != cell.ANSIColor(1) {
		t.Errorf("segment Fg = %+v, want the segment's own Fg preserved", got.Fg)
	}
	if got.Bg != cell.ANSIColor(2) {
		t.Errorf("segment Bg = %+v, want the bar's background, not the segment's own", got.Bg)
	}
}
