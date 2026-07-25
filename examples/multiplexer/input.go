package main

import "bytes"

var (
	seqCtrlLeft  = []byte("\x1b[1;5D")
	seqCtrlRight = []byte("\x1b[1;5C")
)

// routeInput forwards data to the focused pane as raw bytes — matching
// examples/ptyshell's byte-passthrough approach — except for a literal
// match on Ctrl+Left/Ctrl+Right, which switch focus instead of being
// forwarded.
//
// This intentionally doesn't use package input's Decoder: that would
// mean re-encoding decoded events back to bytes to forward them, which
// risks fidelity loss (especially for bracketed-paste content or
// sequences the decoder doesn't preserve exactly) for no benefit here.
// A known limitation of this shortcut: if Ctrl+Left/Right's escape
// sequence is split across two separate stdin reads, this literal
// byte-match won't catch it (input.Decoder's ESC-timeout logic exists
// precisely to handle that correctly) — acceptable for this prototype,
// since a real terminal emits the whole sequence in one write in
// practice. The real Terminal widget (M11) should use input.Decoder.
func routeInput(data []byte, panes []*Pane, focused *int) {
	for len(data) > 0 {
		li := bytes.Index(data, seqCtrlLeft)
		ri := bytes.Index(data, seqCtrlRight)

		idx, seqLen, delta := -1, 0, 0
		switch {
		case li >= 0 && (ri < 0 || li < ri):
			idx, seqLen, delta = li, len(seqCtrlLeft), -1
		case ri >= 0:
			idx, seqLen, delta = ri, len(seqCtrlRight), 1
		}

		if idx < 0 {
			forward(data, panes[*focused])
			return
		}
		if idx > 0 {
			forward(data[:idx], panes[*focused])
		}
		*focused = ((*focused+delta)%len(panes) + len(panes)) % len(panes)
		data = data[idx+seqLen:]
	}
}

func forward(b []byte, p *Pane) {
	if len(b) == 0 {
		return
	}
	_, _ = p.pty.Write(b)
}
