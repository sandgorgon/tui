package main

import (
	"os/exec"
	"sync"

	"github.com/sandgorgon/tui/pty"
	"github.com/sandgorgon/tui/term"
	"github.com/sandgorgon/tui/vt"
)

// Rect is a pane's position and size in host terminal coordinates.
type Rect struct {
	X, Y, W, H int
}

// Pane is one pty-attached shell: its vt.Parser/vt.Screen decode the
// child's output into a screen buffer the compositor blits into the
// shared host frame. vt.Screen isn't safe for concurrent use — a
// background goroutine feeds it from the pty while the main loop reads
// it for compositing — so every access goes through mu.
type Pane struct {
	rect Rect

	pty *pty.Pty
	cmd *exec.Cmd

	mu     sync.Mutex
	parser *vt.Parser
	screen *vt.Screen
}

func newPane(rect Rect, shell string) (*Pane, error) {
	cmd := exec.Command(shell)
	p, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	pane := &Pane{
		rect:   rect,
		pty:    p,
		cmd:    cmd,
		parser: vt.NewParser(),
		screen: vt.NewScreen(rect.W, rect.H),
	}
	_ = p.Resize(term.Size{Rows: rect.H, Cols: rect.W})
	return pane, nil
}

// readLoop feeds the pty's output through the parser until the pty
// closes (the child exited). Any DA1/DSR-style responses vt.Screen
// queues in response get written straight back to the child.
func (p *Pane) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := p.pty.Read(buf)
		if n > 0 {
			p.mu.Lock()
			p.parser.Feed(buf[:n], p.screen)
			resp := p.screen.TakeResponses()
			p.mu.Unlock()
			if len(resp) > 0 {
				_, _ = p.pty.Write(resp)
			}
		}
		if err != nil {
			return
		}
	}
}

// snapshot returns the pane's current cursor state under lock; callers
// read cells directly via p.screen.Buffer().At under the same lock (see
// compositeFrame) rather than copying the whole buffer per frame.
func (p *Pane) withScreen(fn func(s *vt.Screen)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fn(p.screen)
}

// resize propagates a new size to both the vt.Screen (so the shell's
// output is interpreted against the right dimensions) and the pty (so
// the shell itself learns its window changed, via SIGWINCH).
func (p *Pane) resize(rect Rect) {
	p.rect = rect
	p.withScreen(func(s *vt.Screen) { s.Resize(rect.W, rect.H) })
	_ = p.pty.Resize(term.Size{Rows: rect.H, Cols: rect.W})
}
