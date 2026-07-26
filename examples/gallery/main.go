// Command gallery is the M12 "examples gallery" deliverable
// (docs/DESIGN.md §8): a single app exercising the full M9-M11 widget
// catalog end to end — every widget.Xxx constructor, package style
// theming (including appearance detection), and the M12 mouse hit-
// testing/OSC 52 clipboard work — rather than one widget in isolation
// the way the widget package's own tests do.
//
// Per this project's examples philosophy (docs/DESIGN.md, and see the
// comment atop examples/multiplexer/layout.go for the sibling case):
// this is a new example built specifically to showcase the current
// widget catalog, not a retrofit of an older milestone's demo — the
// M8 todo example and M6 multiplexer stay exactly as they were, each
// still proving out only what existed at its own milestone.
package main

import (
	"fmt"
	"os"

	"github.com/sandgorgon/tui/tui"
)

// enableMouse/disableMouse mirror examples/rawecho's exact convention
// (see its enableMouse const): tui.App.Run doesn't enable mouse
// reporting itself, since not every app wants click-to-focus, so an
// app that does has to ask for it.
const (
	enableMouse  = "\x1b[?1000h\x1b[?1006h"
	disableMouse = "\x1b[?1000l\x1b[?1006l"
)

func main() {
	fmt.Fprint(os.Stdout, enableMouse)
	defer fmt.Fprint(os.Stdout, disableMouse)

	app := tui.NewApp(newModel(), 80, 24)
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "gallery:", err)
		os.Exit(1)
	}
}
