// Command rawecho is the M1 milestone demo: it puts the terminal in raw
// mode, detects and probes terminal capabilities, enables mouse and
// bracketed-paste reporting, and prints every decoded input.Event until
// Ctrl+C. It exercises package term (raw mode, capability probing,
// resize/suspend signal handling) and package input (the byte-stream
// event decoder) together against a real terminal.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/term"
)

const (
	enableMouse  = "\x1b[?1000h\x1b[?1006h"
	disableMouse = "\x1b[?1000l\x1b[?1006l"
	enablePaste  = "\x1b[?2004h"
	disablePaste = "\x1b[?2004l"
	enableFocus  = "\x1b[?1004h"
	disableFocus = "\x1b[?1004l"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "rawecho:", err)
		os.Exit(1)
	}
}

func run() error {
	if !term.IsTerminal(os.Stdin) {
		return errors.New("stdin is not a terminal")
	}

	saved, err := term.MakeRaw(os.Stdin)
	if err != nil {
		return fmt.Errorf("MakeRaw: %w", err)
	}
	defer term.Restore(os.Stdin, saved)

	// Probing must happen after MakeRaw: in canonical (cooked) mode the
	// terminal's reply wouldn't be delivered to us until a newline, since
	// it goes through the same line discipline as typed input.
	caps, leftover := term.Probe(os.Stdin, os.Stdout, 500*time.Millisecond, term.DetectEnv(os.Getenv))

	fmt.Fprintf(os.Stdout, "capabilities: %+v\r\n", caps)
	fmt.Fprint(os.Stdout, "press keys, click/scroll/drag the mouse, or paste; Ctrl+C to quit\r\n\r\n")

	fmt.Fprint(os.Stdout, enableMouse+enablePaste+enableFocus)
	defer fmt.Fprint(os.Stdout, disableMouse+disablePaste+disableFocus)

	watcher := term.NewWatcher()
	defer watcher.Stop()

	// Probe can leave bytes unread (a terminal's reply arriving late, or
	// the user's own early keystrokes) — they must be replayed to the
	// decoder first, or they'd otherwise be silently lost.
	dec := input.NewDecoder(&prefixedReader{
		prefix:        leftover,
		f:             os.Stdin,
		graceDeadline: time.Now().Add(2 * time.Second),
	})
	events := make(chan input.Event)
	errs := make(chan error, 1)
	go func() {
		for {
			ev, err := dec.Decode()
			if err != nil {
				errs <- err
				return
			}
			events <- ev
		}
	}()

	for {
		select {
		case ev := <-events:
			if ke, ok := ev.(input.KeyEvent); ok && ke.Mod&input.ModCtrl != 0 && ke.Rune == 'c' {
				fmt.Fprint(os.Stdout, "\r\nbye\r\n")
				return nil
			}
			fmt.Fprintf(os.Stdout, "%v\r\n", ev)

		case err := <-errs:
			return err

		case <-watcher.Resize():
			size, err := term.GetSize(os.Stdout)
			if err == nil {
				fmt.Fprintf(os.Stdout, "* resize: %+v\r\n", size)
			}

		case <-watcher.Cont():
			if _, err := term.MakeRaw(os.Stdin); err != nil {
				return fmt.Errorf("MakeRaw after resume: %w", err)
			}
			fmt.Fprint(os.Stdout, "* resumed\r\n")
		}
	}
}

// prefixedReader replays prefix before delegating to f — used to hand
// term.Probe's leftover bytes to the decoder that reads os.Stdin next,
// rather than losing them. It also strips any DA1/DECRQM reply that
// arrives within graceDeadline: term.Probe's own timeout is the primary
// defense against such a reply leaking into input, but a slow or
// batching terminal can still reply after Probe has already given up —
// Probe genuinely never sees those bytes, so it has nothing to return
// as leftover for; they'd just arrive on an ordinary later read. See
// term.StripLateReply's doc comment.
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
		// Everything in this read was a stripped reply and nothing
		// else: loop for more rather than returning a zero-byte,
		// nil-error result, which io.Reader disallows here.
	}
}

func (r *prefixedReader) SetReadDeadline(t time.Time) error {
	return r.f.SetReadDeadline(t)
}
