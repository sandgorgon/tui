# tui

A Go terminal-UI library with a full widget catalog and an embedded
VT100/xterm-compatible terminal emulator, built with **zero non-stdlib
dependencies**.

```
go get github.com/sandgorgon/tui
```

Status: early development (M0 — repo scaffold). No stable API yet.

## What this is

- A complete TUI component library: layout, styling, input, and a full
  widget set (text, lists, tables, trees, forms, overlays, data viz).
- A PTY subsystem with a full embedded VT100/xterm-compatible terminal
  emulator, capable enough to run a real shell, `vim`, `htop`, or `less`
  inside a pane — the building block for things like a terminal
  multiplexer or an embedded shell panel.
- Linux + macOS for v1 (Windows/ConPTY is a deferred, later phase).

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
