# tui — a dependency-free Go TUI library with an embedded terminal emulator

`github.com/sandgorgon/tui` · MIT license · Linux + macOS · Go stdlib only

Status: **approved — under active development, starting at M0.**

---

## 1. Goals

1. A complete terminal-UI component library for Go: layout, styling, input, and
   a full widget catalog (text, lists, tables, trees, forms, overlays, data viz).
2. **Zero non-stdlib dependencies.** `go.mod` will have no `require` lines at
   all — not even `golang.org/x/sys` or `golang.org/x/term`. Every syscall,
   width table, and ANSI sequence is implemented directly against the Go
   standard library (`syscall`, `os`, `os/exec`, `unicode/utf8`, `testing`, …).
   This is a real constraint, not aspirational: it's proven achievable because
   libraries like `creack/pty` already do the PTY half of this with zero deps.
3. A **PTY subsystem with a full embedded VT100/xterm-compatible terminal
   emulator** — not just raw pty allocation, but enough of a terminal emulator
   that a real shell, `vim`, `htop`, or `less` can run correctly inside a pane
   of this library (a "terminal-in-a-terminal" widget, the building block for
   things like a multiplexer or an embedded shell panel).
4. An internally coherent, genuinely different architecture from the existing
   Go options (see §3) — not a clone of bubbletea, tview, or tcell, though it
   borrows proven ideas from all three plus Rust's Ratatui and JS's Ink.
5. Testability as a first-class property: everything must be exercisable
   headless, without a real tty, in CI.

### Non-goals (v1)

- **Windows.** ConPTY support is a real, separable feature; it's called out
  as a later phase (§8, M12) rather than baked into the core design now.
- Sixel / kitty graphics protocol image rendering.
- Full terminfo database compatibility (we use a small built-in capability
  table for common terminals instead — see §4.3).
- Screen-reader / accessibility narration mode.
- Bug-for-bug xterm clone behavior in the VT emulator — we target "correct
  enough to run bash/zsh/vim/htop/less/tmux-nested" rather than every obscure
  DEC private mode.

---

## 2. Dependency policy

`go.mod`:

```
module github.com/sandgorgon/tui

go 1.26
```

No `require` block. The only place this gets genuinely hard is the PTY layer
and raw-mode termios control, which normally lean on `golang.org/x/sys` or
`golang.org/x/term`. Those aren't needed: `syscall` already exposes `Syscall`/
`RawSyscall` for `ioctl`, and the handful of platform-specific constants
(`TIOCGWINSZ`, `TIOCSWINSZ`, `TIOCGPTN`, `TCGETS`/`TCSETS`, termios struct
layout) are small, stable, and just need to be hand-declared per `GOOS` in
build-tagged files (`pty_linux.go`, `pty_darwin.go`, `term_linux.go`,
`term_darwin.go`). This is exactly what dependency-free PTY libraries in the
Go ecosystem already do, so it's a known-feasible amount of work, not a
research risk.

Dev-only tooling (linters, `go test -fuzz` corpora, CI config) doesn't count
against this — the constraint is about what ships in the module's import
graph.

---

## 3. Architecture

Layered, bottom-up, each layer only depending on the ones below it:

```
L7  vt        VT100/xterm parser + screen/scrollback model
L6  pty       PTY allocation, process attach, resize, signals   (Linux/macOS)
L5  widget    Box, Text, List, Table, Tree, TextInput, Modal, Terminal, ...
L4  tui       App loop, Model/Update/View, Node tree, reconciler, focus, Cmd
L3  layout    Flex-style constraint solver (Length/Percent/Ratio/Min/Fill)
L2  cell      Cell buffer, styles, wide-rune widths, diff renderer
L1  input     Raw-byte decoder -> typed Event stream (key/mouse/paste/focus)
L0  term      Raw mode, capability probing, SIGWINCH/SIGCONT handling
```

### 3.1 The one genuinely new idea: retained widgets, declarative diffing

The three existing Go/Rust/JS approaches each make a different tradeoff:

- **bubbletea** (pure Elm/MVU): all state — including purely cosmetic state
  like "what's the scroll offset of this list" or "is the cursor blinking
  visible right now" — lives in the app's `Model`. Simple and predictable,
  but it means every widget's internal bookkeeping leaks into your app state
  and your `Update` function.
- **tview** (retained, imperative OOP tree): widgets hold their own state and
  you mutate them directly (`list.AddItem(...)`). Convenient, but there's no
  unidirectional data flow, no diffing, and testing means poking at live
  widget objects.
- **Ratatui** (immediate mode): you rebuild and redraw everything from
  scratch every frame from state you own entirely. No hidden widget state,
  but also no persistence of ephemeral UI state without you managing it.

This library splits the difference: the app author writes Elm-style
`Init/Update/View`, and `View()` returns a lightweight **declarative `Node`
tree** (`Box(...)`, `Text(...)`, `List(...)`) — descriptions of widgets, not
the widgets themselves, similar to a virtual DOM. A **reconciler** diffs the
new `Node` tree against the previous one (matched by position + explicit
`Key()`), and where a node matches a prior one, the *retained* widget
instance survives across frames. That retained instance is where genuinely
ephemeral, non-business state lives: cursor blink phase, scroll offset,
in-progress text-input edit buffer, animation frame counter. The app's
`Model` only holds state that's actually meaningful to the application.

Each widget implements a small interface:

```go
type Widget interface {
    Layout(c layout.Constraints) layout.Size
    Paint(p *cell.Painter)
    HandleEvent(e input.Event) tui.Cmd
    Reconcile(next Node) (changed bool)
}
```

Async work goes through `Cmd` (a func returning a `Msg`, run on its own
goroutine, fed back through a channel into the single event-loop goroutine)
— the same proven pattern as Elm/bubbletea, chosen specifically because it
keeps all user-facing state mutation single-threaded and lock-free.

### 3.2 Rendering: cost-based diffing, not just minimal diffing

The renderer keeps two `cell.Buffer`s (front/back). Each row gets a rolling
hash; unchanged rows are skipped in O(1). For changed rows, instead of naive
"only touch cells that differ," the diff merges nearby small change-spans
before emitting output, because repositioning the cursor (`CSI row;col H`)
costs 6–9 bytes — often more than just re-emitting 2–3 unchanged cells in
between. This span-merge threshold is tuned, not hardcoded, so it can adapt
to link speed characteristics later (e.g. wider merge tolerance over SSH).
Where the terminal advertises support (probed via DA1/DA2, see §4.3),
frames are wrapped in DEC synchronized-update mode (2026) to eliminate
tearing.

### 3.3 Layout: a one-pass flex subset, not a general constraint solver

`layout.Constraint` is one of `Length(n)`, `Percent(p)`, `Ratio(num, den)`,
`Min(n)`, `Max(n)`, `Fill(weight)` — conceptually the same vocabulary
Ratatui uses, implemented independently as a single linear pass over a list
of constraints per split (no iterative/cassowary solver needed for this
subset — deliberately avoiding that complexity since every redraw re-runs
layout).

### 3.4 Terminal capability detection without a terminfo database

No dependency on the system terminfo/termcap database. Detection is layered:

1. Env var sniffing: `$TERM`, `$COLORTERM`, `$TERM_PROGRAM`, `$TMUX`.
2. A small built-in table (~10 entries) for terminals that matter in
   practice: xterm-256color, screen, tmux, alacritty, kitty, iterm, wezterm,
   foot, linux console, vt100 fallback.
3. Active capability probing via DA1/DA2 device-attribute queries and
   cursor-position reports, with a timeout — degrades gracefully to a
   conservative ANSI-16, no-mouse, no-synchronized-update baseline if probing
   is inconclusive (e.g. output is piped, not a tty).

---

## 4. Package-by-package plan

| Package | Responsibility |
|---|---|
| `term` | Raw/cooked mode via termios ioctls, capability probing, SIGWINCH/SIGCONT/SIGTSTP handling, terminal size queries |
| `input` | Byte-stream state machine decoding CSI-u/kitty keyboard protocol, legacy xterm key sequences, SGR mouse (1006), bracketed paste (2004), focus events (1004) into typed `Event`s |
| `cell` | `Color`/`Attr`/`Style` (the core color model — 16/256/truecolor — and text attributes; see note below), `Cell{Rune, Style, Width}`, `Buffer`, `Painter` (clipped sub-rect drawing API), hand-rolled East Asian width + emoji width tables |
| `render` | Front/back buffer diff engine (§3.2), ANSI/SGR output writer (including truecolor→256→16 downsampling — an encoding concern, not a data-model one, so it lives here rather than in `cell`), synchronized-update wrapping |
| `layout` | `Constraint`, `Rect`, horizontal/vertical split solver, nesting, gaps/margins |
| `tui` | `App`, `Model`/`Update`/`View`, `Node` tree + `Key()`, reconciler (§3.1), `Cmd`/`Msg` plumbing, focus-tree traversal (Tab/Shift-Tab), modal focus scoping |
| `style` | A theming layer built on `cell.Color`/`cell.Style` (not a redefinition of them — resolved during M2, since `cell.Cell` needs a concrete `Style` type at L2, three layers below where `style` sits): named/semantic colors, adaptive light/dark palettes, a small builder API that `widget` consumes |
| `widget` | The component catalog — see §5 |
| `pty` | PTY allocation/attach/resize/signals — see §6 |
| `vt` | VT100/xterm parser + screen model — see §7 |
| `internal/wcwidth` | Generated Unicode width range tables (with a documented regeneration script, since we can't depend on `x/text/width`) |
| `internal/testutil` | Headless test harness, golden-buffer comparison helpers |

---

## 5. Widget catalog (L5)

**Structural / layout:** Box (flex container), Grid, SplitPane (resizable,
draggable divider), Stack (z-order overlay host), Viewport (scrollable
clipped region)

**Text:** Label, Paragraph (word-wrapped), RichText (styled spans),
lightweight Markdown renderer

**Input:** TextInput (single-line, undo/redo), TextArea (multi-line),
SearchInput, NumberInput

**Selection / data:** List (single/multi-select, virtualized for large data),
Table (sortable, resizable columns, virtualized rows), Tree
(expand/collapse, lazy-load), Tabs, Select/Dropdown, RadioGroup,
CheckboxGroup

**Feedback:** ProgressBar (determinate/indeterminate), Spinner,
Toast/Notification, StatusBar

**Overlay:** Modal/Dialog, Popover/Tooltip, ContextMenu, CommandPalette
(fuzzy-find, fzf-style)

**Data viz:** Sparkline, BarChart, Gauge

**Misc:** Button, Divider/Border, FilePicker, Paginator, KeyHelp/legend bar

**Terminal:** `Terminal` — wires `pty` (L6) + `vt` (L7) together as a normal
widget: PTY output bytes feed `vt.Parser`, which mutates a `vt.Screen`; the
widget's `Paint()` blits that screen's cells straight into the `Painter`'s
rect; its `HandleEvent()` does the reverse — encodes key/mouse `input.Event`s
back into bytes written to the PTY master. This is what makes a real shell,
editor, or pager runnable inside a pane.

---

## 6. PTY subsystem (L6, Linux + macOS)

Build-tagged per `GOOS` (`pty_linux.go`, `pty_darwin.go`) since the exact
ioctl path differs (`/dev/ptmx` + `TIOCGPTN` on Linux vs. `/dev/ptmx` +
`TIOCPTYGNAME`/`TIOCPTYGRANT`/`TIOCPTYUNLK` on Darwin), all via raw
`syscall.Syscall(SYS_IOCTL, ...)` — no cgo, no `x/sys`.

Capabilities:

- `Open() (pty, tty *os.File, err error)` — allocate a master/slave pair.
- `Start(cmd *exec.Cmd) (*Pty, error)` — attach the slave as the child's
  stdin/stdout/stderr, set `Setsid`/`Setctty` in `SysProcAttr` so the child
  gets a proper controlling terminal, close the slave fd in the parent after
  fork.
- `Resize(rows, cols uint16)` — `TIOCSWINSZ`, plus forwarding host
  `SIGWINCH` to the child's process group.
- Raw/cooked mode toggling on the **host** stdin (termios `cc`/`lflag`
  manipulation), since the outer terminal must be in raw mode for the TUI
  itself while the pty's own termios settings govern the child.
- Explicit signal forwarding (`SIGINT`, `SIGTSTP`, `SIGCONT`) to the child's
  process group — needed because when the *host* terminal is in raw mode,
  the kernel's normal `^C`/`^Z` line-discipline handling for the child
  doesn't kick in the way it would in cooked mode; this has to be done by
  hand and is flagged as a specific risk area in §9.
- `io.Reader`/`io.Writer` on the master fd, `Close()` with full cleanup,
  exit-status propagation.

---

## 7. VT100/xterm terminal emulator (L7)

Classic Paul Williams / DEC VT500 parser state machine (Ground, Escape,
EscapeIntermediate, CsiEntry/Param/Intermediate/Ignore,
OscString, DcsEntry/Param/Passthrough, SosPmApcString) driving a `Screen`
model:

- Primary + alternate screen buffers, scrollback ring buffer.
- Cursor position/visibility/shape, DECSC/DECRC save-restore.
- Full SGR: bold/italic/underline (incl. curly/dashed)/strikethrough/
  reverse/blink, 16/256/24-bit foreground+background.
- Scrolling regions (DECSTBM), tab stops, autowrap (DECAWM), origin mode
  (DECOM), insert/replace mode.
- OSC 0/2 (window/tab title captured as metadata), OSC 8 (hyperlinks
  captured as cell metadata, exposed for the `Terminal` widget to make
  clickable).
- Transparent passthrough of mouse-reporting and bracketed-paste sequences
  so the host app can choose to consume them or forward them to the child.
- Response generation for DA1/DA2/CPR/DSR queries — required for many real
  CLI apps (vim, htop, less) to detect the terminal correctly and behave.

Exposed as `Terminal.Buffer() *cell.Buffer` so it composes directly with L2
— the emulator's output is just another thing that can be painted into a
pane, laid out, and composed with every other widget.

---

## 8. Delivery milestones

PTY/VT are front-loaded, ahead of layout and the widget catalog, because
once the render↔vt round trip (M5) exists it becomes the primary
correctness harness for everything built afterward — feed a `cell.Buffer`
through the renderer to get ANSI bytes, parse those bytes back through
`vt.Parser` into a `Screen`, assert the result equals the source buffer.
That's a much stronger check than hand-written golden text dumps, and it's
reused for the component model (M8) and every widget batch (M10, M11)
rather than being a late add-on.

| # | Deliverable |
|---|---|
| M0 | Repo scaffold: go.mod, LICENSE (MIT), CI (go vet/test/race/short-fuzz on linux+macos matrix), package skeletons |
| M1 | L0 term + L1 input: raw mode, capability probe, input decoder. Demo: raw key/mouse/paste event echo |
| M2 | L2 cell: Buffer, Style, wide-rune width tables — data model only, no renderer yet |
| M3 | L6 pty (Linux+macOS): allocation, attach, resize+SIGWINCH, raw mode, signal forwarding. Demo: standalone raw pty-to-shell CLI |
| M4 | L7 vt: parser + screen model (built on M2's `cell.Buffer`), SGR/color, alt screen, scroll regions, OSC title/hyperlink, DA/DSR responses. Deliverable: headless conformance suite green |
| M5 | render: diff renderer + ANSI/SGR writer. Deliverable: the render↔vt round-trip harness described above — becomes the correctness oracle for every layer from here on |
| M6 | Terminal prototype: M3 pty + M4 vt + M5 render wired into a standalone multi-pane demo running real shells — proves the full loop end-to-end before the component model exists to formalize it as a widget |
| M7 | L3 layout: constraints, split solver, nesting. Demo: static layout |
| M8 | L4 tui: Node, reconciler, App loop, Cmd/Msg, focus tree — verified against the M5 harness. Demo: counter/todo app |
| M9 | style/theme system |
| M10 | Widgets batch 1 — structural/text: Box, Paragraph, List, Viewport, Tabs, StatusBar, ProgressBar, Spinner — golden coverage via the M5 harness |
| M11 | Widgets batch 2 — input/data, plus formalizing M6's prototype as the proper L5 `Terminal` widget: TextInput, TextArea, Table, Tree, Select, Checkbox/Radio, Modal, CommandPalette |
| M12 | Hardening: fuzz corpus expansion, full golden-file coverage across all widgets, benchmarks, docs, examples gallery |
| M13 (explicitly deferred, not in v1) | Windows/ConPTY backend, image protocols, accessibility mode, terminfo db compat |

---

## 9. Risks / open engineering questions

- **Signal handling under raw mode** (SIGTSTP/job control forwarded from a
  raw-mode host into a pty child) is a known-fiddly area across terminal
  emulators generally — needs dedicated integration tests at M3, where the
  pty layer is built, not left until the end.
- **Wide-rune/emoji width tables** will drift as Unicode updates — ship a
  documented regeneration script in `internal/wcwidth` rather than a
  one-time hardcoded table.
- **Capability detection without terminfo** means unusual/rare terminals may
  be misdetected — documented, tested fallback to conservative ANSI-16 mode.
- **VT conformance scope**: explicitly "enough to run bash/zsh/vim/htop/
  less/nested tmux correctly," not a bug-for-bug xterm clone — full DEC
  private-mode coverage is effectively unbounded.
- **Cmd/Msg backpressure**: Cmd goroutines feed a channel into the single
  event loop; needs an explicit bounded-channel + coalescing policy (e.g.
  for rapid resize events) so a slow consumer can't be starved or the
  channel unbounded-grow.

---

## 10. Testing strategy

- **Render↔vt round trip (primary harness, built at M5, used from M6 on):**
  render a `cell.Buffer` to ANSI/SGR bytes, feed those bytes back through
  `vt.Parser` into a `Screen`, assert the resulting screen equals the source
  buffer. Because the parser is an independent decoder of the renderer's own
  encoder, a passing round trip is strong evidence the renderer emits
  well-formed sequences that mean what we intended — this is the main
  correctness check for the component model (M8) and every widget batch
  (M10, M11), not just the render layer itself.
- Table-driven unit tests for the layout solver.
- Golden-buffer tests: render a `Node` tree to a `cell.Buffer`, dump as a
  text grid, diff against a fixture — used where the round-trip harness
  isn't the natural fit (e.g. layout-only assertions with no ANSI output).
- `internal/testutil` headless harness: synthetic `Event` injection and
  buffer inspection with no real tty — this is what makes the whole thing
  CI-friendly.
- VT parser conformance tests against hand-written escape-sequence fixtures
  (since we won't pull in an external corpus), covering the sequences
  `vim`/`htop`/`less`/`tmux` actually emit.
- `go test -fuzz` (stdlib, zero extra deps) on the VT parser and the input
  decoder — both are natural byte-stream state-machine fuzz targets.
- Real-subprocess PTY integration tests (`/bin/echo`, `/bin/cat`, a small
  purpose-built fixture binary) on Linux/macOS CI.
- `go test -race` mandatory in CI for the event loop / Cmd plumbing.
- Allocation-tracking benchmarks for the diff renderer (steady-state target:
  zero allocations per redraw).

---

## 11. Repo layout

```
github.com/sandgorgon/tui/
  go.mod                          module github.com/sandgorgon/tui, go 1.26, no requires
  LICENSE                         MIT
  README.md
  docs/
    DESIGN.md                     (this document, promoted once approved)
  term/
  input/
  cell/
  render/
  layout/
  tui/
  style/
  widget/
    box.go  text.go  list.go  table.go  tree.go  tabs.go
    textinput.go  textarea.go  checkbox.go  radio.go  select.go
    progress.go  spinner.go  modal.go  menu.go  statusbar.go
    sparkline.go  chart.go  filepicker.go  paginator.go
    viewport.go  splitpane.go  commandpalette.go  terminal.go
  pty/
    pty.go  pty_linux.go  pty_darwin.go
  vt/
    parser.go  screen.go  sgr.go  osc.go
  internal/
    wcwidth/
    testutil/
  examples/
    kitchensink/  filemanager/  term-multiplexer/  dashboard/
```

---

## Decisions locked in for this plan

- PTY scope: full embedded VT100/xterm terminal emulator, not just raw
  allocation.
- Platforms: Linux + macOS only for v1; Windows/ConPTY explicitly deferred.
- Module path: `github.com/sandgorgon/tui`.
- License: MIT.
- Go version floor: 1.26.
- Dependency policy: zero non-stdlib imports, no exceptions.
- Milestone order: PTY/VT/render (M1–M6) are front-loaded ahead of layout
  and the widget catalog (M7–M12), specifically so the render↔vt round-trip
  harness (M5) exists early and becomes the correctness oracle used to
  verify rendering for the component model and every subsequent widget
  batch, rather than being bolted on at the end.

## Open for your review

Nothing outstanding — widget list confirmed complete, Go 1.26 confirmed as
the version floor.
