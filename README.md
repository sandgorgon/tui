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
  ioctls and cross-compiles cleanly, but has never been run on real
  Apple hardware — this project has only ever been developed on Linux.
  CI (`.github/workflows/ci.yml`) already runs the full suite on
  `macos-latest`, so pushing to a repo with Actions enabled gets it
  exercised on real (GitHub-hosted) Mac hardware automatically; short
  of that, verification from someone with a Mac is genuinely welcome —
  see `pty/pty_darwin.go` and `term/term_darwin.go`.

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
