package widget

import (
	"strings"
	"testing"
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/style"
)

func TestProgressBarDeterminateFillsFraction(t *testing.T) {
	theme := style.DefaultDark()
	node := ProgressBar(0.5, ProgressBarOptions{Theme: theme})
	buf := cell.NewBuffer(10, 1)
	paintNode(t, node, buf)

	for x := range 5 {
		if got := buf.At(x, 0).Style.Bg; got != theme.Primary {
			t.Errorf("At(%d,0).Bg = %+v, want theme.Primary (filled half)", x, got)
		}
	}
	for x := 5; x < 10; x++ {
		if got := buf.At(x, 0).Style.Bg; got != theme.Border {
			t.Errorf("At(%d,0).Bg = %+v, want theme.Border (empty half)", x, got)
		}
	}
}

func TestProgressBarClampsFraction(t *testing.T) {
	theme := style.DefaultDark()

	over := ProgressBar(2.0, ProgressBarOptions{Theme: theme})
	buf := cell.NewBuffer(4, 1)
	paintNode(t, over, buf)
	for x := range 4 {
		if got := buf.At(x, 0).Style.Bg; got != theme.Primary {
			t.Errorf("fraction>1: At(%d,0).Bg = %+v, want fully filled", x, got)
		}
	}

	under := ProgressBar(-1.0, ProgressBarOptions{Theme: theme})
	buf2 := cell.NewBuffer(4, 1)
	paintNode(t, under, buf2)
	for x := range 4 {
		if got := buf2.At(x, 0).Style.Bg; got != theme.Border {
			t.Errorf("fraction<0: At(%d,0).Bg = %+v, want fully empty", x, got)
		}
	}
}

func TestProgressBarIndeterminateAnimatesWithClock(t *testing.T) {
	theme := style.DefaultDark()
	base := time.Unix(0, 0)
	now := base

	node := ProgressBar(0, ProgressBarOptions{
		Theme: theme, Indeterminate: true,
		Now: func() time.Time { return now },
	})

	tr := newTree(t, node)
	buf := cell.NewBuffer(10, 1)
	tr.Paint(cell.NewPainter(buf))
	first := filledColumns(buf, theme)

	now = base.Add(2 * time.Second)
	tr.Reconcile(ProgressBar(0, ProgressBarOptions{
		Theme: theme, Indeterminate: true,
		Now: func() time.Time { return now },
	}))
	buf2 := cell.NewBuffer(10, 1)
	tr.Paint(cell.NewPainter(buf2))

	// buf.String() only dumps runes (see its doc comment), and every
	// column is drawn with the same ' ' rune regardless of fill state
	// — the animation is visible only in each cell's background color,
	// so that's what has to be compared here.
	if filledColumns(buf2, theme) == first {
		t.Error("indeterminate bar's filled column(s) should move after 2s of simulated elapsed time")
	}
}

// filledColumns marks each column '#' if it's painted with theme's
// filled (Primary) background, '.' if it's the empty (Border) one.
func filledColumns(buf *cell.Buffer, theme style.Theme) string {
	var sb strings.Builder
	for x := range buf.Width {
		if buf.At(x, 0).Style.Bg == theme.Primary {
			sb.WriteByte('#')
		} else {
			sb.WriteByte('.')
		}
	}
	return sb.String()
}
