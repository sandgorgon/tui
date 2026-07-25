package pty

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/sandgorgon/tui/term"
)

// Pty is a pseudo-terminal pair. It embeds the master end (*os.File),
// so callers use it directly as an io.Reader/io.Writer (e.g.
// io.Copy(pty, os.Stdin)); the slave end is not exposed — callers
// interact with the attached child only through the master and the
// process itself.
type Pty struct {
	*os.File
	pid int
}

// Open allocates a new pty master/slave pair, with neither end
// attached to any process yet. Most callers want Start instead; Open
// is exposed for callers that need to manage attachment themselves.
func Open() (master, slave *os.File, err error) {
	return openpty()
}

// Start starts cmd with its stdin, stdout, and stderr attached to a
// freshly allocated pty's slave end, set as its controlling terminal,
// and returns a Pty whose master end is used to communicate with it.
//
// The caller retains ownership of cmd and is responsible for calling
// cmd.Wait() themselves (Start deliberately doesn't wrap or duplicate
// that — two owners calling Wait on the same process is a common
// footgun) and for calling Pty.Close when done with the master.
func Start(cmd *exec.Cmd) (*Pty, error) {
	master, slave, err := openpty()
	if err != nil {
		return nil, err
	}

	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Ctty = 0 // the child's fd 0 (Stdin), i.e. the slave

	if err := cmd.Start(); err != nil {
		master.Close()
		slave.Close()
		return nil, fmt.Errorf("pty: start %s: %w", cmd.Path, err)
	}

	// The child now holds its own reference to the slave (as fds
	// 0/1/2); the parent doesn't need one; keeping it open here would
	// serve no purpose and would leave an extra fd around.
	slave.Close()

	return &Pty{File: master, pid: cmd.Process.Pid}, nil
}

// Resize sets the pty's window size, visible to the child via
// TIOCGWINSZ. The kernel automatically delivers SIGWINCH to the pty's
// foreground process group when the size actually changes (standard
// tty driver behavior — see tty_ioctl(4)), so Resize doesn't need to
// send any signal itself.
func (p *Pty) Resize(size term.Size) error {
	return term.SetSize(p.File, size)
}

// Signal sends sig to the child's entire process group, not just the
// immediate child — matching what a real controlling terminal's line
// discipline does for ISIG-triggered signals. Most signals reach the
// child automatically simply by writing the corresponding raw byte
// (e.g. 0x03 for Ctrl+C, 0x1a for Ctrl+Z) to the Pty: a freshly
// allocated pty slave starts in cooked/ISIG-enabled mode by default,
// independent of whatever mode the host terminal is in, so its own
// line discipline turns that byte into the right signal without any
// help from this package. Signal is an escape hatch for cases that
// aren't naturally byte-driven — e.g. propagating SIGCONT to the child
// after the host process itself resumes from a suspend (see
// term.Suspend), in case the child was independently stopped too.
func (p *Pty) Signal(sig syscall.Signal) error {
	return syscall.Kill(-p.pid, sig)
}

// Close closes the master end. It does not kill the child; a caller
// that wants that should send a signal (or call cmd.Process.Kill)
// first and Wait for it.
func (p *Pty) Close() error {
	return p.File.Close()
}
