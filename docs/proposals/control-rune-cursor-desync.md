# Bug: literal control runes and wide runes desync the renderer's cursor tracking from the real terminal

Status: **accepted and implemented** (not yet committed). Filed by a
consumer (kaze) — confirmed against a real terminal via a raw-byte PTY
capture, not just read from source. A second, related desync in
`TextArea`'s own column math (wide/multi-column runes) was found while
evaluating this report and folded into the same fix, since both are the
same underlying failure mode — code that tracks "cursor position" by
counting one column per rune instead of the rune's real display width —
just in two different layers (`cell`/`render` vs. `widget.TextArea`).

## Bug 1: control runes (e.g. TAB) stored verbatim

### Symptom, as originally reported

kaze's `-tui` editor (built on `widget.TextArea`) leaves stray leftover
characters on screen that don't get cleared as the cursor moves around a
file — reported by a user as general visual corruption during
navigation, no repro steps known at first.

### Root cause

`cell.Painter.SetCell` (`cell/painter.go`) computed
`width := wcwidth.RuneWidth(r)` and fell back to `width = 1` whenever
`width <= 0` — which is exactly what `wcwidth.RuneWidth` returns for any
C0/C1 control rune, TAB (`'\t'`, 0x09) included
(`internal/wcwidth/wcwidth.go:39-40`: `r < 0x20` → `-1`). Critically,
only the **width** was coerced to a safe value — the `Cell.Rune` field
still stored the literal control rune verbatim.

`render.Renderer.emitSpan` (`render/render.go`) later wrote that `Rune`
straight into the output byte stream, substituting a space only for
`rn == 0`. Every other control rune, TAB included, was written to the
terminal exactly as stored — a real control byte, not a printable
glyph. A real terminal receiving a raw `0x09` mid-line doesn't advance
the cursor by one column the way the renderer's own `x += w` bookkeeping
(using the coerced `Width: 1`) assumed — it jumps to the next 8-column
tab stop. Every byte the renderer emitted afterward, believing the
cursor was at its own tracked `x`, actually landed several columns
further right on the real terminal — previously-painted content between
the renderer's believed cursor position and the real one is never
overwritten (leftover garbage), and content that should still be inside
the pane can overrun past it (looks like a broken border).

### Confirmed with a real terminal, not just read from source

A minimal repro: open a Go source file (any `gofmt`-formatted file) in a
`TextArea`-based app, scroll so a tab-indented line is visible. Captured
the actual bytes written to a real PTY:

```
\x1b[5;2H\treturn "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA...
```

— `ESC[5;2H` (move to row 5, col 2), then a **raw 0x09 byte**, then the
rest of the line's text with no compensating cursor move in between.
Reproduces on a fresh, first-time paint of the line — no navigation
required beyond scrolling the line into view.

It's also reachable through ordinary typing, not just loading
tab-indented files: `TextArea` deliberately claims raw Tab
(`WantsRawTab`, `widget/textarea.go`) specifically so a user can type a
literal `\t` (`insertRune('\t')`), and `Paint` read it back verbatim
with no expansion, straight into `SetCell`. `List` was exposed the same
way through `Painter.Text`, for any caller-supplied string containing a
control rune.

### Scope

Any control rune with `wcwidth.RuneWidth(r) < 0` reaching `SetCell` or
`Fill` triggered this — not just TAB. (Zero-width combining marks,
`wcwidth.RuneWidth(r) == 0`, are a separate, already-documented `Painter`
simplification — see the fix below — and were deliberately left alone.)

### Fix implemented

Sanitized at `cell.Painter`'s two choke points every widget's drawing
already goes through — `SetCell` and `Fill` — via a shared
`resolveCell(r rune) (rune, int)` helper: when
`wcwidth.RuneWidth(r) < 0` (a true non-printable control rune), it
substitutes a plain space for `Cell.Rune` itself, not just the tracked
width, so the emitted byte matches the renderer's own column math for
every `Painter` consumer at once. `wcwidth.RuneWidth(r) == 0`
(zero-width combining marks) is unaffected — it keeps the existing
"drawn as its own single-width cell" behavior documented on `Painter`.

Regression tests: `cell/painter_test.go`
(`TestPainterSetCellControlRuneSubstitutesPlaceholder`,
`TestPainterFillControlRuneSubstitutesPlaceholder`,
`TestPainterSetCellZeroWidthRuneKeepsRune`) and an end-to-end
`render/render_test.go` case
(`TestRenderControlRuneDoesntDesyncCursor`) going through a real
`cell.Painter` into `Renderer.Render` and checking the emitted bytes
never contain the raw control byte.

## Bug 2: `TextArea`'s column math assumed every rune is 1 screen column

### Root cause

`TextArea` never called `wcwidth.RuneWidth` anywhere — every place it
translated between a buffer rune index and a screen column assumed a
strict 1:1 mapping: `cursorCol := w.cursor - lines[cursorLine].start`
for scroll clamping, `idx := ln.start + w.scrollCol + col` in Paint's
row loop, and the same arithmetic in `setCursorFromMouse` and
`moveVertical`/`movePage`'s column-preservation. The moment a line
contains any width-2 rune (CJK, emoji, other East Asian wide
characters), this breaks two ways:

1. Every column after the wide rune is off by however many wide runes
   preceded it, since `idx` and the screen column silently diverge —
   miscomputed scroll clamping, mouse hit-testing, and vertical-move
   column preservation.
2. It corrupted `cell.Painter.SetCell`'s own continuation-cell
   invariant: `SetCell` writes a wide rune's continuation cell at
   `x+1`, but Paint's next loop iteration (one iteration per screen
   column *and* per buffer rune, in lockstep) called `SetCell` again at
   that same `x+1` for the *next* buffer rune — overwriting the
   continuation cell immediately, in the same frame that wrote it.

Unlike Bug 1, this doesn't touch a real terminal's own quirky control-
sequence handling — it's a self-inflicted desync purely within
`TextArea`'s own bookkeeping and its calls into `Painter`.

`widget.Paragraph` was already correct here (`wcwidth.RuneWidth`-aware
wrapping); `List`/`Table` don't do `TextArea`'s bidirectional
index↔column translation for an editable cursor, so they weren't
exposed to the same corruption, only to (smaller) alignment issues.

### Fix implemented

Added `runeCols`, `visualWidth`, and `columnToIndex` helpers to
`widget/textarea.go` (rune-width-aware column math, mirroring
`Paragraph`'s existing use of `wcwidth`), and rewired every place that
used to conflate buffer rune-index with screen column:

- Paint's scroll-clamp (`cursorCol`) now sums visual width from line
  start to the cursor, not a raw rune count.
- Paint's row-render loop now walks buffer runes and derives each
  one's screen column from cumulative width, instead of iterating
  screen columns 1:1 with buffer runes — including a case for a wide
  rune split by the scroll offset (renders the leftover column(s)
  blank rather than half a glyph) and a wide rune that would only
  half-fit at the row's right edge (renders blank there too, since
  `Painter.SetCell` won't draw it at all but the column still needs
  explicit content this frame, or a stale cell survives undrawn).
- `setCursorFromMouse` now resolves a click's screen column back to a
  buffer index via `columnToIndex` instead of raw pointer arithmetic.
- `moveVertical`/`movePage` now preserve the cursor's *visual* column
  across lines, not its buffer rune-count column, so Up/Down keeps the
  cursor under the same on-screen position even when one of the two
  lines has a wide rune the other doesn't.

Regression tests in `widget/textarea_test.go`:
`TestTextAreaPaintWideRuneNoCorruption` (continuation cell survives the
frame intact), `TestTextAreaClickAccountsForWideRuneColumns` (mouse
hit-testing lands past a wide rune's 2 columns, not 1),
`TestTextAreaMoveVerticalPreservesVisualColumnAcrossWideRunes` (Down
lands at the same visual column, not the same rune count, across a
line with a wide rune).
