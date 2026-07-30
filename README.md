# tui

A Go terminal-UI library with a full widget catalog and an embedded
VT100/xterm-compatible terminal emulator, built with **zero non-stdlib
dependencies**.

```
go get github.com/sandgorgon/tui
```

Status: pre-1.0, M0-M12 shipped (full widget catalog, PTY subsystem,
embedded terminal emulator, theming, fuzzing/golden tests/benchmarks/
examples gallery). No stable API yet — see
[`docs/DESIGN.md`](docs/DESIGN.md) §8 for the milestone table.

## What this is

- A complete TUI component library: layout, styling, input, and a full
  widget set (text, lists, tables, trees, forms, overlays, data viz).
  See `examples/gallery` for all of it running together in one app.
- A PTY subsystem with a full embedded VT100/xterm-compatible terminal
  emulator, capable enough to run a real shell, `vim`, `htop`, or `less`
  inside a pane — the building block for things like a terminal
  multiplexer or an embedded shell panel.
- Linux + macOS for v1 (Windows/ConPTY is a deferred, later phase).
  **macOS status:** implemented against Darwin's documented termios/pty
  ioctls and cross-compiles cleanly, but this project is developed and
  maintained entirely on Linux — no one on the project has Mac hardware
  to test or debug against. CI no longer runs a `macos-latest` leg (it
  briefly did and passed, but a CI-only pass with no one able to act on
  a Mac-specific failure isn't worth the false confidence). Treat Darwin
  support as unverified; see `pty/pty_darwin.go` and `term/term_darwin.go`
  if you hit anything Mac-specific, and contributions/reports from
  someone with real Mac hardware are welcome.

## Quick start

```go
package main

import (
	"fmt"
	"os"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/tui"
)

type model struct{}

func (m model) Init() tui.Cmd { return nil }

func (m model) Update(msg tui.Msg) (tui.Model, tui.Cmd) {
	if ke, ok := msg.(input.KeyEvent); ok && ke.Rune == 'q' {
		return m, tui.Quit()
	}
	return m, nil
}

func (m model) View() tui.Node {
	return tui.Text("hello, tui — press q to quit", cell.Style{})
}

func main() {
	if err := tui.NewApp(model{}, 80, 24).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

`Run` puts the terminal into raw mode and drives the app until `Update`
returns a `Cmd` that yields `tui.Quit()`. See `examples/todo` for a
complete Model with focus traversal and a retained `List` widget, and
`examples/gallery` for the full widget catalog exercised in one app.

New to the library? [`docs/GUIDE.md`](docs/GUIDE.md) walks through the
concepts above in more depth — Model/Update/View, Msg/Cmd, layout
constraints, retained widgets, and how input routing works.

## Design

The full architecture — layering, the component model, the PTY and VT
emulator design, the widget catalog, and the delivery roadmap — is in
[`docs/DESIGN.md`](docs/DESIGN.md). Two decisions worth calling out up
front:

- **Zero dependencies.** `go.mod` has no `require` lines at all, not even
  `golang.org/x/sys` or `golang.org/x/term`. Every syscall, ANSI sequence,
  and width table is implemented directly against the Go standard library.
- **Retained widgets, declarative diffing.** App code is written Elm-style
  (`Init`/`Update`/`View`), but `View()` returns a declarative node tree
  that's diffed and reconciled against retained widget instances — so
  ephemeral UI state (scroll position, cursor blink, in-progress edits)
  lives in the widgets, not in your application state. See §3.1 of the
  design doc for the full rationale.

## License

MIT — see [`LICENSE`](LICENSE).
