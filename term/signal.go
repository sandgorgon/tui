package term

import (
	"os"
	"os/signal"
	"syscall"
)

// Watcher delivers coalesced notifications for terminal-related signals:
// resize (SIGWINCH) and resume-after-suspend (SIGCONT). Raw mode should
// be reasserted via MakeRaw after a Cont notification — job control can
// leave termios state altered while the process was stopped.
type Watcher struct {
	sig    chan os.Signal
	resize chan struct{}
	cont   chan struct{}
	done   chan struct{}
}

// NewWatcher starts watching for terminal signals. Call Stop when done.
func NewWatcher() *Watcher {
	w := &Watcher{
		sig:    make(chan os.Signal, 8),
		resize: make(chan struct{}, 1),
		cont:   make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	signal.Notify(w.sig, syscall.SIGWINCH, syscall.SIGCONT)
	go w.run()
	return w
}

func (w *Watcher) run() {
	for {
		select {
		case s, ok := <-w.sig:
			if !ok {
				return
			}
			var target chan struct{}
			switch s {
			case syscall.SIGWINCH:
				target = w.resize
			case syscall.SIGCONT:
				target = w.cont
			default:
				continue
			}
			select {
			case target <- struct{}{}:
			default:
			}
		case <-w.done:
			return
		}
	}
}

// Resize fires (coalesced — a burst of resizes collapses to one pending
// notification) whenever the terminal is resized.
func (w *Watcher) Resize() <-chan struct{} { return w.resize }

// Cont fires whenever the process resumes after having been stopped via
// SIGTSTP; callers should reassert raw mode with MakeRaw at this point.
func (w *Watcher) Cont() <-chan struct{} { return w.cont }

// Stop stops the watcher and releases its signal registration.
func (w *Watcher) Stop() {
	signal.Stop(w.sig)
	close(w.done)
}

// Suspend cooperates with the shell's job control: it restores the
// terminal to cooked (via the given saved State), stops the process
// with SIGTSTP so the shell can take over the terminal, and — once the
// shell resumes the process with SIGCONT — puts the terminal back into
// raw mode, returning the new raw State to use afterward.
func Suspend(f *os.File, cooked *State) (*State, error) {
	if err := Restore(f, cooked); err != nil {
		return nil, err
	}

	signal.Reset(syscall.SIGTSTP)
	_ = syscall.Kill(0, syscall.SIGTSTP)
	// Execution resumes here once the shell sends SIGCONT.

	return MakeRaw(f)
}
