package widget

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/term"
	"github.com/sandgorgon/tui/tui"
)

// fakeStream is a minimal pty.Stream for exercising TerminalOptions.Stream
// without a real pty or child process: r/w is a pipe the test writes
// "child output" into (Read side, what Terminal consumes), written
// records everything Terminal itself writes back (forwarded keystrokes
// and vt.Screen's own terminal-response bytes), and resizes records
// every Resize call so a test can confirm Terminal drives a Stream
// exactly the way it already drives a local Command's *pty.Pty.
type fakeStream struct {
	r *io.PipeReader
	w *io.PipeWriter

	mu      sync.Mutex
	written bytes.Buffer
	resizes []term.Size
	closed  bool
}

func newFakeStream() *fakeStream {
	r, w := io.Pipe()
	return &fakeStream{r: r, w: w}
}

func (f *fakeStream) Read(p []byte) (int, error) { return f.r.Read(p) }

func (f *fakeStream) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.written.Write(p)
}

func (f *fakeStream) Resize(sz term.Size) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resizes = append(f.resizes, sz)
	return nil
}

func (f *fakeStream) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return f.w.Close() // also unblocks a pending Read with io.EOF/io.ErrClosedPipe
}

func (f *fakeStream) Written() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.written.String()
}

func (f *fakeStream) Resizes() []term.Size {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]term.Size(nil), f.resizes...)
}

func (f *fakeStream) Closed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// TestTerminalShowsStreamOutput is TestTerminalShowsCommandOutput's
// Stream-path counterpart: bytes written to a Stream (standing in for
// a remote connection's own output, e.g. attach()'s jobPtyFiles in 9sh)
// render exactly like a local Command's pty output would.
func TestTerminalShowsStreamOutput(t *testing.T) {
	fs := newFakeStream()
	node := Terminal(TerminalOptions{Stream: fs})
	buf := cell.NewBuffer(30, 3)
	var tr tui.Tree
	tr.Reconcile(node)
	tr.Paint(cell.NewPainter(buf)) // establishes size, triggers start() -> Resize

	if _, err := fs.w.Write([]byte("hello-stream")); err != nil {
		t.Fatalf("write to fake stream: %v", err)
	}

	waitFor(t, 2*time.Second, func() { tr.Paint(cell.NewPainter(buf)) }, func() bool {
		return strings.Contains(buf.String(), "hello-stream")
	})

	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if !fs.Closed() {
		t.Error("want the fake stream's own Close to have been called")
	}
}

// TestTerminalHandleEventWritesToStream is
// TestTerminalHandleEventWritesToChild's Stream-path counterpart: a
// keystroke reaches the Stream's Write, the same encoding path a local
// Command's pty master gets.
func TestTerminalHandleEventWritesToStream(t *testing.T) {
	fs := newFakeStream()
	node := Terminal(TerminalOptions{Stream: fs})
	buf := cell.NewBuffer(30, 3)
	var tr tui.Tree
	tr.Reconcile(node)
	tr.Paint(cell.NewPainter(buf)) // establishes size before writing

	widget := tr.Focusables()[0]
	widget.HandleEvent(input.KeyEvent{Rune: 'z'})

	deadline := time.Now().Add(2 * time.Second)
	for !strings.ContainsRune(fs.Written(), 'z') && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.ContainsRune(fs.Written(), 'z') {
		t.Fatalf("stream received %q, want it to contain 'z'", fs.Written())
	}

	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestTerminalStreamResizedOnMount confirms start() calls Resize on an
// opts.Stream exactly like it already does on a local Command's real
// pty — the whole point of a Stream-driven attach() forwarding the
// local terminal's actual size on first mount, not some placeholder.
func TestTerminalStreamResizedOnMount(t *testing.T) {
	fs := newFakeStream()
	node := Terminal(TerminalOptions{Stream: fs})
	buf := cell.NewBuffer(24, 5)
	var tr tui.Tree
	tr.Reconcile(node)
	tr.Paint(cell.NewPainter(buf))

	resizes := fs.Resizes()
	if len(resizes) != 1 || resizes[0] != (term.Size{Cols: 24, Rows: 5}) {
		t.Fatalf("resizes = %v, want exactly one {Cols:24 Rows:5}", resizes)
	}

	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
