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
| M12 | Hardening: ~~fuzz corpus expansion~~ (done — see §9), ~~full golden-file coverage across all widgets~~ (done — see §9), ~~benchmarks~~ (done — see §9), ~~docs~~ (done), ~~examples gallery~~ (done — see §9), ~~mouse hit-testing~~ (done — see §9), ~~OSC 52 clipboard copy~~ (done — see §9) |
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
- **macOS: unverified, CI leg removed (2026-07-30).** `pty/pty_darwin.go`
  and `term/term_darwin.go`'s ioctl constants and struct layouts
  (`TIOCPTYGRANT`/`TIOCPTYUNLK`/`TIOCPTYGNAME`, termios field positions,
  `tcflag_t` width — see §6, §3.4) were transcribed from
  `golang.org/x/sys/unix`'s canonical source at M1/M3, not developed
  against a real Mac — this project is developed and maintained entirely
  on Linux, and nobody on the project has Mac hardware. CI briefly ran a
  `macos-latest` matrix leg (added when the repo went public, 2026-07-26)
  and it passed on the first run — build, `go vet`, `test -race`, fuzz
  smoke, on a real GitHub-hosted Mac runner — but it was removed on
  2026-07-30: a green CI checkmark with no one able to diagnose a future
  Mac-specific failure is false confidence, not real coverage. Darwin
  support should be treated as cross-compiles-only/unverified until
  someone with real Mac hardware can test and maintain it. Nobody has
  ever driven this interactively on a Mac (a real terminal session
  attaching a shell/`vim`/`htop` through a `Terminal` widget) either.
  Verification from a contributor with real Mac access closes that
  remaining gap; nothing else is required to trust the ioctl layer
  itself anymore.
- **Mouse hit-testing — done (M12, first pass).** App now tracks every
  focusable widget's absolute on-screen `Rect` as a byproduct of the
  existing paint walk (`collectRects`, mirroring `paint`'s own
  `layout.Split` geometry rather than needing any change to
  `cell.Painter`, which stays write-only). `App.HandleInput` hit-tests a
  `MouseEvent` against those rects: landing on a non-focused widget moves
  focus there (click-to-focus), and the event forwarded to the target
  widget has its coordinates translated to be local to that widget's
  bounds — a widget never needs to know its own absolute screen position,
  the same principle `cell.Painter.Clip` already uses for drawing.
  `List`/`Tree`/`Select`'s open list/`RadioGroup`/`CheckboxGroup` translate
  a click's Y into an item index; `Table` translates Y to a row and X to a
  column (with Y=-1 as a header-click sentinel, X still the column —
  `Select`'s closed control uses the same Y=-1 convention); `Tabs`
  translates X into a label index by tracking each label's column range
  during `Paint`. `Terminal` needed zero widget-level change: it already
  forwards whatever event it's given straight into `encodeMouse`, and by
  the time it sees a click the coordinates are already local — exactly
  what a real program running inside (vim, tmux, ...) expects.

  **Modal/CommandPalette click-outside-to-close — done (post-M12).**
  `collectRects` still doesn't see into `Modal`/`CommandPalette`'s
  `PaintOverlay` content (drawn outside the normal Box tree at a
  computed centered position), so a click anywhere on screen while one
  is open used to reach whichever of the scope's own `Focusables`
  currently held focus with raw, untranslated absolute coordinates —
  harmless only by accident, since no shipped modal/palette body reacted
  to `MouseEvent`, but a real bug (a body that did would misfire on an
  unrelated click). Fixed with a new pair of optional interfaces in
  `tui`: `OverlayBounds` (a `FocusScope` reports its own last-painted
  absolute `Rect`) and `OutsideClicker` (reacts to a click landing
  outside it). `App.HandleInput` checks `OverlayBounds` whenever a
  `MouseEvent` lands on nothing tracked while a scope is active:
  outside the reported bounds, the event is withheld from the scope's
  focused widget entirely (fixing the misrouting unconditionally) and,
  if the scope also implements `OutsideClicker`, forwarded there
  instead. `centeredOverlay` (shared by `Modal`/`CommandPalette`) now
  returns the `Rect` it computed, not just the clipped `Painter`, so
  each widget can cache it from `PaintOverlay` and serve it back via
  `OverlayBounds`. `CommandPalette` reuses its existing `OnCancel`
  callback for `HandleOutsideClick` (outside-click means the same thing
  as Esc for a picker); `Modal` gained a new `OnOutsideClick` option
  (nil by default — an outside click is simply absorbed rather than
  misrouted, unless the app opts in to using it to close the dialog).

  **Table column drag-to-resize — done (post-M12).** A press (`Button`
  set, `Drag` false) landing in the 1-cell gap after a column (new
  `boundaryAt`, alongside the existing `columnAt`) starts dragging that
  column's width; subsequent `Drag` events move `colWidths[draggingCol]`
  by the X delta since the last event; any `MouseRelease` ends it — new
  `draggingCol`/`dragRow`/`dragLastX` state on `tableWidget`, handled by
  a new `handleColumnDrag` checked first in `HandleEvent`'s `MouseEvent`
  branch. Guards against the coordinate-space gotcha this item was
  flagged with ahead of time: `App.HandleInput` only translates a
  `MouseEvent` to be local to `Table` when the event's absolute position
  still falls inside `Table`'s tracked `Rect` (`hitTest`); if a drag
  gesture's mouse position leaves that `Rect` mid-drag, the event still
  reaches `Table` (it's still focused) but with raw, untranslated
  absolute coordinates, which would silently produce a garbage width
  jump if mixed with the previous event's local `dragLastX`. Defended
  against by remembering the local row the drag started on
  (`dragRow`) and abandoning the drag (not applying that event's delta)
  the moment a continuation event reports a different row — a
  legitimately-continued drag on the same row always reports the same
  local row each time, so a changed row is a reliable, if not perfect,
  signal that trust should end. Keyboard `Shift+Left/Right` resizing is
  unaffected (untouched code path).

  **TextInput/TextArea selection (keyboard and mouse) — done (post-M12),
  completing the last of the three post-M12 mouse follow-ups.** New
  shared state on `editBuffer` (`hasSelection`/`selAnchor`, a collapsed
  selection — anchor == cursor — deliberately not counting as active):
  `selectionRange`/`startSelection`/`clearSelection`, plus
  `moveTo`/`moveHorizontal` encapsulating the "shift extends, plain
  movement drops" convention every navigation key now goes through.
  Built keyboard selection first, as planned, to validate the state
  design before the mouse coordinate math: Shift+Left/Right/Up/Down/
  Home/End extend a selection; plain Left/Right collapse to whichever
  selection edge is in that direction instead of moving one rune from
  the cursor's current position (the standard editor convention); Up/
  Down/Home/End more simply just drop the selection and move normally
  (a deliberate simplification — a widget library, not a full editor).
  Typing a rune, pasting, or inserting a newline/tab now replaces an
  active selection (`insertRune`/`insertString` call the new
  `removeSelectionNoUndo` as part of the same undo step); Backspace/
  Delete delete the whole selection instead of one character
  (`deleteSelection`); undo/redo drop any active selection, matching
  every mainstream editor. Painting (`TextInput`/`TextArea.Paint`) now
  goes through a shared `highlightStyle` helper: reverse video (the
  same convention this codebase already uses for a selected List/Table
  row), for either the active selection or the raw cursor, no separate
  look between them (matches vim visual-mode: no distinct caret glyph
  once a selection is showing). `TextArea`'s multi-line case needed
  care here: a naive per-cell check using the flat buffer index breaks
  down for the padding cells past a short line's end (they'd
  spuriously read as "selected" or not by coincidentally colliding with
  a *different* line's real buffer offsets); fixed by classifying each
  row as either fully swept by the selection (highlight the whole row,
  padding included, so a multi-line selection reads as one continuous
  block) or checking real per-character positions only.
  Click-to-position-cursor and click/drag-to-select reuse the same
  `editBuffer` state: a plain press moves the cursor and drops any
  selection; Shift+press extends the active selection (starting one if
  needed) to the clicked point; while the button stays down, each
  further Drag event (and the final release) starts a selection the
  moment the mouse actually moves and extends it to the new point, so a
  click with no movement never creates a (zero-width, invisible)
  selection. `TextArea.setCursorFromMouse` handles the same drag-
  outside-bounds gotcha `Table`'s column-resize drag needed defending
  against (see that entry above), but resolves it differently and more
  simply: since each mouse event computes an absolute buffer position
  fresh from (X,Y) — clamped to the nearest valid line/column — rather
  than accumulating a delta from the previous event the way Table's
  column width does, one stray untranslated event (raw absolute
  coordinates arriving because the drag left the widget's tracked Rect
  while it's still focused) produces at most one wrong-but-harmless
  cursor placement for that single frame, not Table's compounding-error
  risk — no need to detect and abandon the drag outright.
- **Mouse reporting and native terminal text selection are mutually
  exclusive by protocol** — enabling mouse mode (`\x1b[?1000h\x1b[?1006h`,
  see `examples/rawecho`'s `enableMouse`) is exactly what makes click-to-
  focus etc. possible, but it also means click-drag stops reaching the
  terminal emulator's own selection/copy behavior; the universal escape
  hatch is the terminal's own bypass modifier (Shift-drag, on virtually
  every emulator), not something this library can restore — the same
  tradeoff vim/tmux/htop-with-mouse-on already live with.

  **OSC 52 clipboard copy — done (M12).** `term.WriteClipboard(w,
  text)` base64-encodes text and writes it wrapped in an OSC 52 escape
  sequence (BEL-terminated, matching this project's other OSC sequence
  — OSC 8 hyperlinks, `render.appendHyperlink`); on a terminal that
  doesn't support OSC 52 it's simply ignored, since there's no reliable
  way to probe for support first (unlike DA1/DA2). `tui.ClipboardMsg`/
  `tui.CopyToClipboard(text) Cmd` are the app-facing side — usable from
  `Update` (`return m, tui.CopyToClipboard(text)`, the same shape as
  `Quit()`) or from any widget's `onEvent` callback, which can just
  return `ClipboardMsg{Text: text}` directly as its `Msg` (the existing
  "wrap whatever onEvent returns in a Cmd" plumbing every widget already
  has does the rest). `App.Run` special-cases `ClipboardMsg` the same
  way it already special-cases `QuitMsg`, writing it on Run's own single
  goroutine rather than from the Cmd's — deliberately, since a Cmd
  writing directly to stdout from its own goroutine could interleave,
  byte for byte, with the App's render output on the same fd, and
  there's no shared lock between them. No widget (`Table`/`List`/`Tree`)
  hardcodes a copy keybinding itself — same as every other
  business-decision key (Enter/Space/etc.), which key copies what is
  entirely up to the application's own `onEvent` handling.

  **Fuzz corpus — done (M12).** `go test -fuzz` targets added for the two
  byte-stream state machines §10 always intended to have them:
  `vt.FuzzParser` (feeds arbitrary bytes through `Parser` into a real
  `Screen`, not just the bare `Handler` recorder, so the corpus exercises
  parser + screen semantics together) and `input.FuzzDecode` (feeds
  arbitrary bytes through `Decoder.Decode` over a `net.Pipe`, escape
  timeout cut to 1ms and the decode loop capped at 10k events per input
  so a decoder bug that never advances can't hang the fuzzer). Both
  seeded from the existing hand-written conformance/table-driven
  fixtures. A 30s live-fuzzing pass on each found one real bug on the
  very first seed (the empty-input case): `Decoder.fill`/
  `readByteTimeout` treated a `Read` returning `(0, nil)` — a documented,
  legal `io.Reader` outcome (`net.Pipe` does exactly this for a
  zero-length `Write`) — as EOF/timeout without refilling, so the next
  `readByte` indexed into a stale/empty buffer and panicked. Both loops
  now retry on `(0, nil)` instead of returning, per the `io.Reader`
  contract. No crasher files exist under `testdata/fuzz/` — none of the
  30s runs (either target) found anything beyond that one seed-corpus
  bug, which is fixed in source rather than needing a regression corpus
  entry.

  **Golden-file coverage across all widgets — done (M12).**
  `internal/testutil.Golden(t, name, buf)` compares `buf.String()`'s
  text-grid dump (see `cell.Buffer.String`'s doc comment, which already
  named this exact use case back at M2) against a fixture under
  `testdata/golden/<name>.golden`, with a package-registered `-update`
  flag (`go test -update ./...`) to write/refresh fixtures — this is
  deliberately a different check than the render↔vt round trip: round
  trip proves whatever was painted survives re-encoding, golden files
  protect the specific *content/shape* of what was painted in the first
  place, which a round trip can't catch (a widget silently painting the
  wrong thing still round-trips cleanly, as long as it's well-formed).
  `widget/golden_test.go` covers all 17 `widget.Xxx` constructors, plus
  `Modal`/`CommandPalette` (need a real `tui.App` for the
  `OverlayPainter` path) and `Terminal` (real subprocess, via the same
  `waitFor`-for-stable-output pattern `terminal_test.go` already
  established). Two diffs looked like bugs on first read of the
  generated fixtures and turned out not to be, worth knowing before
  re-generating these: `CommandPalette`'s fixture shows 4 stray
  characters of the host app's background text (`back`) bleeding into
  the palette's own top-left margin — correct, since `tui.Focusable`
  auto-draws a border around whatever's focused (here, the background
  pane) and `centeredOverlay`'s box is narrower than the frame, so
  background content between the two simply isn't covered by either;
  and the palette's placeholder text (`"type a command"`) shows as
  `"ype a command"` — also correct, `paintQueryLine` deliberately
  overwrites column 0 with a blank reverse-video cell to draw the
  empty-input cursor block, which necessarily destroys whatever
  placeholder character was there.

  **Benchmarks — done (M12).** `go test -bench` coverage for the paths
  most likely to matter for interactive responsiveness: `render`'s
  diff/SGR writer (`BenchmarkRenderFullFrame` vs.
  `BenchmarkRenderNoOpDiff` vs. `BenchmarkRenderSmallDiff`, split out
  specifically so a future regression toward full-frame cost on an
  unchanged or lightly-changed frame is visible instead of hiding inside
  one aggregate number), `vt.Parser.Feed` under a realistic mixed CSI/
  SGR/OSC byte stream (not just plain text, since dispatch — not
  `Print` — is where a slow parser would show up), `layout`'s solver
  under a nested mixed-constraint split, and `tui.Tree.Reconcile`/
  `Paint` cold (first mount) vs. warm (steady-state re-reconcile,
  reusing retained widgets) — plus a `widget`-package composite frame
  covering the M10 widget catalog together, the same frame
  `roundtrip_test.go` already used for round-trip coverage.

  **Examples gallery — done (M12).** `examples/gallery`: a single App
  exercising every `widget.Xxx` constructor plus `style` theming
  (including `DetectAppearance`) together, per the examples philosophy
  (a new example built to showcase the current widget catalog, not a
  retrofit of `examples/todo` or `examples/multiplexer`). Four
  `Tabs`-switched pages (text/feedback, lists/data, forms, a live
  `Terminal` running `$SHELL`) plus `CommandPalette`/`Modal` reachable
  globally. Verified end to end via a real pty — not just `go build` —
  using this project's own `pty` package to script keystrokes and read
  the result back through `vt.Parser` (the same pattern the project's
  own integration tests already use), since tmux wasn't available in
  the sandbox this was built in; a driver script is not checked in
  (throwaway, lived under the scratch dir for this session).

  That real end-to-end run — not any unit test — caught two genuine
  library bugs, both fixed at the source rather than routed around in
  the example:

  - **Reconciler panic on an unkeyed widget-type change.** `reconcile`
    (`tui/reconcile.go`) only compared `Node.kind` (`kindText`/
    `kindBox`/`kindWidget`) to decide whether to reuse a retained node
    or mount fresh — but `kindWidget` is one flat tag shared by every
    `widget.Xxx` constructor. Switching gallery's Tabs to a page whose
    content is a different widget type at the same unkeyed tree
    position (page 0's `Paragraph` vs. page 1's `List`, both at the
    same `Box`-child slot) made `reconcile` reuse the old frame's
    `*paragraphWidget` and hand it the new frame's `listProps`,
    panicking on that widget's own `props.(paragraphProps)` type
    assertion — a real, easy-to-hit case (any conditionally-rendered
    branch without an explicit `Node.Key`, not a contrived one) that
    no existing widget/tui test exercised, since none of them swap
    which concrete widget occupies a slot across `Reconcile` calls.
    Fixed by having `retained` remember the `reflect.Type` of the
    props it was last given (`propsType`) and treating a type change
    as a mount-fresh case alongside the existing kind/nil checks — see
    `reconcile`'s doc comment. `tui.TestReconcileWidgetTypeChangeMountsFreshInsteadOfPanicking`
    is the regression test (a `strictWidget` double that actually
    type-asserts its props, unlike `fakeWidget`, which couldn't have
    caught this).
  - **`input.Decoder` crash on a terminal without read-deadline
    support.** `readByteTimeout` treated any `SetReadDeadline` error as
    fatal, propagating it straight out of `Decode` — so on a reader
    that can't support deadlines at all (`os.ErrNoDeadline`, observed
    via the pty allocated by the gallery's own nested test harness; see
    `input.decode_test.go`'s `noDeadlineReader`), a single standalone
    Escape keypress (the one call site that actually needs the
    timeout, to tell a bare Escape from the start of a sequence) killed
    the entire `App` — reproduced identically against the already-
    shipped, unmodified `examples/todo`, confirming this predates the
    gallery and isn't gallery-specific. Fixed to degrade instead of
    crash: an `os.ErrNoDeadline` result from `SetReadDeadline` is now
    treated the same as an immediate timeout (standalone Escape
    reported right away). The trade-off is real and stated in
    `readByteTimeout`'s doc comment: without any way to wait for a
    follow-up byte, multi-byte escape sequences can't be told apart
    from a bare Escape on such a reader and decode as a spurious KeyEsc
    plus separate keystrokes — degraded, but never a crash. Any other
    `SetReadDeadline` failure (e.g. an already-closed reader) still
    propagates as a real error.

- **Per-region styling for `TextArea`/`List` — done.** Filed as
  `docs/proposals/text-region-styling.md` by a downstream consumer
  (kaze, a CRDT-native editor built on this library) wanting to color
  arbitrary byte ranges/rows — syntax highlighting, diagnostics, diff/
  blame views, search-match highlighting all need this, not just one
  consumer's feature. Implemented the proposal's recommended shape
  (general per-position styling) but as caller-supplied *data* rather
  than a callback: `widget.StyleSpan{Start, End int; Style cell.Style}`
  on `TextAreaOptions.Highlights` (buffer rune-offset ranges) and
  `cell.Style` on `ListOptions.RowStyles` (per-item index, List being
  row-granular already needs no sub-row span concept) — matching how
  every other per-frame prop in this codebase (`Selected []bool`,
  `StatusBar`'s `[]Segment`) is data recomputed fresh each frame, not a
  stored closure invoked mid-`Paint`; this also means the widget owns
  the "don't scan the whole document per cell" concern once, rather
  than pushing a hand-rolled interval structure onto every caller.
  `TextArea.Paint` resolves the per-cell style before the existing
  `highlightStyle` selection/cursor overlay runs, so a span's color
  survives being selected exactly like the theme's own colors already
  did (composes via `Attr | AttrReverse`, never replaces `Fg`/`Bg`) —
  confirmed this composed for free with the selection work below
  without changing `highlightStyle` itself. Lookup is a per-row
  `sort.Search` re-seek (spans must be caller-sorted by `Start`,
  non-overlapping) followed by a forward sweep within the row: `idx`
  is only monotonic *within* a row, not row-to-row (a short line
  following a much longer one, with horizontal scroll still parked
  from the long line, can start well before the previous row's last
  `idx`), so a single sweep pointer carried across the whole `Paint`
  call would have been wrong — re-seeking once per visible row instead
  keeps the cost bounded by the viewport, not the document. `List`'s
  cursor-row highlight had to change from a full style *replacement*
  (`rowStyle = selected`) to composing via `Attr |= AttrReverse` the
  same way, or a caller's `RowStyles` color would have been silently
  clobbered on whichever row the cursor sits on — this was a real gap
  in `List`'s existing code, not something the proposal's author could
  have seen without reading `list.go` directly. 4 new tests
  (`widget/textarea_test.go`, `widget/list_test.go`) covering override,
  compose-with-selection, and compose-with-cursor-highlight; `go build/
  vet/gofmt/test -race` clean across the whole repo.

- **Control-rune and wide-rune cursor desync — fixed.** Filed as
  `docs/proposals/control-rune-cursor-desync.md` by a downstream
  consumer (kaze) and confirmed against a real terminal via a raw-byte
  PTY capture, not just read from source: `cell.Painter.SetCell`/`Fill`
  coerced a control rune's (TAB included) *width* to a safe fallback but
  left `Cell.Rune` storing the literal control rune, so `render.emitSpan`
  wrote a real control byte to the terminal — which jumps to the next
  tab stop for TAB, not `+1` column the way the renderer's own `x+=Width`
  bookkeeping assumed, desyncing every byte emitted afterward from where
  the terminal actually put its cursor. Reachable on ordinary use, not
  just crafted input: `TextArea` claims raw Tab specifically to let a
  user type a literal `\t`, and reads it back into `Paint` with no
  expansion. Fixed at the one choke point every widget's drawing already
  goes through — a shared `resolveCell` in `cell/painter.go` substitutes
  a printable space for `Cell.Rune` itself (not just the width) whenever
  `wcwidth.RuneWidth(r) < 0`, leaving the already-documented zero-width-
  combining-mark simplification (`== 0`) untouched. A second, unrelated
  desync was found while evaluating the report: `TextArea` never called
  `wcwidth.RuneWidth` anywhere, so every place it translated a buffer
  rune index to a screen column (`Paint`'s scroll clamp and row loop,
  `setCursorFromMouse`, `moveVertical`/`movePage`'s column preservation)
  assumed one rune is always one column — which a wide rune (CJK, emoji)
  breaks, and which was actively overwriting `SetCell`'s own
  continuation cell mid-frame in `Paint`'s row loop (one `SetCell` call
  per screen column *and* per buffer rune, in lockstep, so the next
  rune's `SetCell` call landed on the same continuation cell the wide
  rune had just written). Added `runeCols`/`visualWidth`/`columnToIndex`
  helpers to `widget/textarea.go` (mirroring `Paragraph`'s existing
  `wcwidth`-aware wrapping) and rewired all four call sites to work in
  visual columns; `Paint`'s row loop now walks buffer runes deriving
  each one's screen column from cumulative width instead of iterating
  screen columns 1:1 with runes, with explicit blank-column handling for
  a wide rune split by the scroll offset or one that would only
  half-fit at the row's right edge (both cases `Painter.SetCell` would
  silently not draw, but the column still needs *some* explicit content
  this frame or a stale cell survives undrawn). 8 new tests across
  `cell`, `render`, and `widget/textarea_test.go`; `go build/vet/test`
  clean across the whole repo.

- **Reconciler: keyed subtree reuse across a reparenting move — done.**
  Filed as [#3](https://github.com/sandgorgon/tui/issues/3) and
  `docs/proposals/reconciler-cross-parent-key-reuse.md`, surfaced
  building 9sh (a pane-splitting terminal multiplexer) on `tui.App`:
  `reconcileChildren`'s key matching only ever searched one parent's
  own previous children, so a leaf keeping its `Node.Key` while moving
  one level deeper (or shallower) — e.g. splitting a pane wraps it in a
  brand-new sibling Box — always mounted a fresh `Widget`, killing a
  live `widget.Terminal`'s pty on every split. Fixed by giving
  `reconcile` (`reconcile.go`) a whole previous-tree key index
  (`reconcileCtx.byKey`, built by `snapshotPrev` before any mutation)
  that `reconcileChildren` falls back to once a key misses locally —
  the per-parent match stays the primary, unchanged path. This forced
  disposal out of the per-parent walk entirely: a retained node left
  unmatched at the parent it used to occupy might still be claimed
  elsewhere in the same pass, so `disposeUnclaimed` now runs once,
  after the whole tree has been walked, closing only what no slot
  anywhere claimed (`reconcileCtx.claimed`) — `disposeTree` itself is
  untouched, still used as-is by `App`/`Tree`/`Focusable`'s own
  `Close()` for full teardown, a separate concern from per-frame
  reconciliation. A claimed key is deleted from the index immediately,
  so a genuine key collision (two `next` slots sharing a key) can't
  alias the same `Widget` instance into two tree positions — the first
  slot visited reuses it, the second mounts fresh. Scoped to one
  top-level `reconcile` call's own tree only: a `widget.Viewport`'s
  hosted `Tree` or a `Focusable`'s wrapped child each still reconcile
  independently, per `reconcile`'s doc comment. 4 new tests in
  `tui/reconcile_test.go` and `tui/dispose_test.go` (reparenting deeper
  and shallower, the collision safety net, and confirming a reparented
  `io.Closer` widget is never disposed); `go build/vet/test -race`
  clean across the whole repo.

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
