package render

import (
	"bytes"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/term"
)

func TestComputeSpansMergesWithinThreshold(t *testing.T) {
	front := cell.NewBuffer(10, 1)
	back := cell.NewBuffer(10, 1)
	back.Set(1, 0, cell.Cell{Rune: 'a', Width: 1})
	back.Set(4, 0, cell.Cell{Rune: 'b', Width: 1}) // gap of 2 unchanged cells (2,3)

	spans := computeSpans(front, back, 0, 8) // threshold 8 >= gap 2: merge
	if len(spans) != 1 {
		t.Fatalf("spans = %+v, want 1 merged span", spans)
	}
	if spans[0] != (span{1, 5}) {
		t.Errorf("span = %+v, want {1,5}", spans[0])
	}
}

func TestComputeSpansSplitsBeyondThreshold(t *testing.T) {
	front := cell.NewBuffer(20, 1)
	back := cell.NewBuffer(20, 1)
	back.Set(1, 0, cell.Cell{Rune: 'a', Width: 1})
	back.Set(15, 0, cell.Cell{Rune: 'b', Width: 1}) // gap way beyond any reasonable threshold

	spans := computeSpans(front, back, 0, 4)
	if len(spans) != 2 {
		t.Fatalf("spans = %+v, want 2 separate spans", spans)
	}
}

func TestComputeSpansNoChangesReturnsNil(t *testing.T) {
	front := cell.NewBuffer(5, 1)
	back := cell.NewBuffer(5, 1)
	if spans := computeSpans(front, back, 0, 8); spans != nil {
		t.Errorf("spans = %+v, want nil", spans)
	}
}

// nopWriter fails the test if Write is ever called, for asserting
// that Render didn't emit anything.
type failOnWriteWriter struct{ t *testing.T }

func (f failOnWriteWriter) Write(p []byte) (int, error) {
	f.t.Fatalf("unexpected Write call with %q", p)
	return 0, nil
}

func TestRenderNoChangeWritesNothing(t *testing.T) {
	r := NewRenderer(Options{ColorLevel: term.ColorTrueColor})
	back := cell.NewBuffer(5, 1)
	back.Set(0, 0, cell.Cell{Rune: 'x', Width: 1})

	var buf bytes.Buffer
	if err := r.Render(&buf, back, 0, 0, true); err != nil {
		t.Fatal(err)
	}
	buf.Reset()

	// Same content, same cursor: nothing should be written the second time.
	if err := r.Render(failOnWriteWriter{t}, back, 0, 0, true); err != nil {
		t.Fatal(err)
	}
}

func TestRenderFirstFrameFullPaint(t *testing.T) {
	r := NewRenderer(Options{ColorLevel: term.ColorTrueColor})
	back := cell.NewBuffer(3, 1)
	back.Set(0, 0, cell.Cell{Rune: 'a', Width: 1})
	back.Set(1, 0, cell.Cell{Rune: 'b', Width: 1})
	back.Set(2, 0, cell.Cell{Rune: 'c', Width: 1})

	var buf bytes.Buffer
	if err := r.Render(&buf, back, 0, 0, true); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("abc")) {
		t.Errorf("first-frame output = %q, want it to contain \"abc\"", got)
	}
}

func TestRenderOnlyChangedCellUpdates(t *testing.T) {
	r := NewRenderer(Options{ColorLevel: term.ColorTrueColor})
	back := cell.NewBuffer(5, 1)
	for i, ch := range "hello" {
		back.Set(i, 0, cell.Cell{Rune: ch, Width: 1})
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, back, 4, 0, true); err != nil {
		t.Fatal(err)
	}
	buf.Reset()

	back2 := cell.NewBuffer(5, 1)
	for i, ch := range "hallo" { // only column 1 differs: 'e' -> 'a'
		back2.Set(i, 0, cell.Cell{Rune: ch, Width: 1})
	}
	if err := r.Render(&buf, back2, 4, 0, true); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte{'a'}) {
		t.Errorf("output = %q, want it to contain the changed rune 'a'", got)
	}
	// Nothing else on the row changed, so the untouched cells' runes
	// shouldn't appear as fresh output (a weak but meaningful check:
	// the diff shouldn't have repainted the whole row).
	if bytes.Count(buf.Bytes(), []byte{'h'}) > 0 {
		t.Errorf("output = %q, unchanged cell 'h' shouldn't have been re-emitted", got)
	}
}

func TestRenderSynchronizedOutputWrapsFrame(t *testing.T) {
	r := NewRenderer(Options{ColorLevel: term.ColorTrueColor, SynchronizedOutput: true})
	back := cell.NewBuffer(3, 1)
	back.Set(0, 0, cell.Cell{Rune: 'x', Width: 1})

	var buf bytes.Buffer
	if err := r.Render(&buf, back, 0, 0, true); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	if !bytes.HasPrefix(got, []byte("\x1b[?2026h")) {
		t.Errorf("output = %q, want it to start with the sync-begin sequence", got)
	}
	if !bytes.HasSuffix(got, []byte("\x1b[?2026l")) {
		t.Errorf("output = %q, want it to end with the sync-end sequence", got)
	}
}

func TestRenderCursorVisibilityToggle(t *testing.T) {
	r := NewRenderer(Options{ColorLevel: term.ColorTrueColor})
	back := cell.NewBuffer(3, 1)

	var buf bytes.Buffer
	if err := r.Render(&buf, back, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("\x1b[?25l")) {
		t.Errorf("output = %q, want a hide-cursor sequence", buf.Bytes())
	}
	buf.Reset()

	if err := r.Render(&buf, back, 0, 0, true); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("\x1b[?25h")) {
		t.Errorf("output = %q, want a show-cursor sequence", buf.Bytes())
	}
}

func TestRenderControlRuneDoesntDesyncCursor(t *testing.T) {
	// A control rune (TAB here) painted via cell.Painter must never
	// reach the terminal as a raw control byte: a real terminal jumps a
	// literal TAB to the next 8-column tab stop rather than advancing
	// one column, which would desync the renderer's own x+=Width cursor
	// bookkeeping from where the terminal actually ends up. SetCell is
	// responsible for substituting a printable placeholder before the
	// cell ever reaches the renderer — this exercises that whole path,
	// not just emitSpan in isolation.
	back := cell.NewBuffer(4, 1)
	p := cell.NewPainter(back)
	p.SetCell(0, 0, 'a', cell.Style{})
	p.SetCell(1, 0, '\t', cell.Style{})
	p.SetCell(2, 0, 'b', cell.Style{})

	r := NewRenderer(Options{ColorLevel: term.ColorTrueColor})
	var buf bytes.Buffer
	if err := r.Render(&buf, back, 0, 0, true); err != nil {
		t.Fatal(err)
	}
	if bytes.ContainsRune(buf.Bytes(), '\t') {
		t.Errorf("output = %q, must not contain a raw TAB byte", buf.Bytes())
	}
	if !bytes.Contains(buf.Bytes(), []byte("a b")) {
		t.Errorf("output = %q, want it to contain \"a b\" (TAB substituted with a single space)", buf.Bytes())
	}
}

func TestRenderWideRuneSpan(t *testing.T) {
	r := NewRenderer(Options{ColorLevel: term.ColorTrueColor})
	back := cell.NewBuffer(4, 1)
	back.Set(0, 0, cell.Cell{Rune: '中', Width: 2})
	back.Set(1, 0, cell.Cell{Width: 0})
	back.Set(2, 0, cell.Cell{Rune: 'b', Width: 1})

	var buf bytes.Buffer
	if err := r.Render(&buf, back, 0, 0, true); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("中b")) {
		t.Errorf("output = %q, want it to contain \"中b\" with no gap for the continuation cell", buf.Bytes())
	}
}
