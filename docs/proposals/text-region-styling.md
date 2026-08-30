# Proposal: per-line / per-region styling for TextArea and List

Status: **partially done.** Filed by a consumer (kaze) for
consideration whenever this repo's own maintainer picks it up — option
2 (general per-position styling) was accepted and implemented as
`TextAreaOptions.Highlights`/`ListOptions.RowStyles` (see
`docs/DESIGN.md` §9, "Per-region styling for `TextArea`/`List`"). A
string-returning variant of option 3 (below) was also implemented,
via [#11](https://github.com/sandgorgon/tui/issues/11) — see
`docs/DESIGN.md` §9, "`TextArea`: initial cursor offset and a
line-number gutter."

## Problem

Every multi-line/multi-row widget in the catalog computes exactly **one
`cell.Style` for its entire content** and varies it only by structural
state (selection, cursor, focus) — never by anything the caller wants to
say about a specific line, byte range, or row.

Confirmed directly in the two widgets where this matters most:

- `widget.TextArea.Paint` (`widget/textarea.go:104`) computes
  `base := w.opts.Theme.Text()` once (line 152) and applies it to every
  cell in the buffer; the only per-cell variation is `highlightStyle`
  toggling for selection/cursor/focus (line 179). There is no hook in
  `TextAreaOptions` for a caller to say "line 12 should be red" or "bytes
  340–412 should be dimmed."
- `widget.List.Paint` (`widget/list.go:62`) has the identical shape:
  `base := w.opts.Theme.Text()` (line 81), one style per row, varied only
  by the cursor-row/selected-row highlight.

The one widget in the catalog with real per-region color is
`widget.StatusBar`'s `Segment{Text string; Style cell.Style}`
(`widget/statusbar.go:13`) — but it's single-line by construction, not
applicable to a multi-line buffer or a scrolling list.

## Why this matters beyond one consumer

This blocks a whole class of use case that comes up in any real editor or
list-heavy TUI, not just one application's specific feature:

- Syntax highlighting (the most obvious one — right now no `tui`-based
  editor can highlight a single keyword or string literal).
- Diagnostic/lint gutters (a red squiggle-equivalent, or a colored line
  marker for "this line has a warning").
- Diff/blame views (green/red line backgrounds, or a colored gutter
  showing "changed"/"unchanged"/"who last touched this").
- Search-match highlighting (coloring just the matched substring within
  a line, not the whole line).
- Any kind of per-item annotation overlay in a `List` (status dots,
  severity coloring, freshness indicators).

## One concrete example, from a real consumer

kaze (a content-addressed, CRDT-native code editor built on this
library) wants to color each declaration in a live-edited source buffer
by two independent axes — an authorship/verification state (four-value
enum) and a "is this cached reference stale" flag — recomputed as the
user types. Today this is entirely infeasible with `TextArea`: there is
no way to say "this byte range is currently green, that one is currently
red, recompute every keystroke." (Full internal design notes, kept in
kaze's own repo since they're specific to its data model, not relevant
here beyond motivating the ask.)

## Candidate API shapes

Three shapes considered, in increasing order of general power and
implementation cost. This is presented as options with a
recommendation, not a mandate — the actual API shape is this library's
own design call, and should fit its existing conventions (compare how
`ListOptions.Selected []bool` is already a caller-supplied per-row
input, which any of these would extend).

1. **Per-line style hook** — `LineStyle func(lineIdx int) cell.Style`
   on `TextAreaOptions` (and the row-equivalent on `ListOptions`).
   Cheapest to implement (one extra style lookup per row in the existing
   `Paint` loop). Covers "this whole line/row is colored uniformly" —
   which covers diagnostics gutters and kaze's declaration-coloring case
   reasonably well if declarations are treated as whole-line spans —
   but can't color a single word or substring within a line.

2. **General per-position style hook** — `StyleAt func(idx int) cell.Style`
   (buffer byte/rune index for `TextArea`, item index for `List`),
   consulted per-cell inside the existing `for col := range innerW`
   loop (`widget/textarea.go:172`) alongside the current
   selection/cursor logic. Most reusable — this is what real syntax
   highlighting needs (coloring a single identifier or string literal,
   not a whole line), so it has value far beyond any one consumer's
   feature. Larger implementation: needs to compose sensibly with the
   existing selection/cursor override logic in `highlightStyle`, and a
   caller recomputing this on every keystroke needs it to stay cheap
   (a range-list or interval lookup, not an O(n) scan per cell, if
   documents get large).

3. **Minimal per-line gutter marker** — `Gutter func(lineIdx int) (rune, cell.Style)`
   painted in a fixed-width column to the left of each line's normal
   text, no change to the line's own coloring at all. Smallest possible
   change (a few cells wide, computed once per visible row, no
   interaction with existing selection/cursor code). Sufficient for a
   binary/enum-state indicator (a colored dot per line) if full-line or
   full-region recoloring isn't wanted.

   **Implemented as a string-returning variant, not this rune-returning
   shape** — see [#11](https://github.com/sandgorgon/tui/issues/11):
   `TextAreaOptions.Gutter func(lineIdx int) (string, cell.Style)`,
   right-aligned in a column sized to the widest string across the
   currently visible rows plus one separator column. #11 pointed out
   this rune-only shape can't render a line *number* ("245" is three
   characters), which is at least as common a use as the single-glyph
   marker case this option was originally scoped for — and the
   single-rune case is still trivially served by a one-character
   string, so the string variant subsumes this one at no extra cost to
   callers who only need a marker.

**Recommendation**: (2), general per-position styling, as the most
reusable investment — it subsumes (1) and enables real syntax
highlighting as a side effect, which is likely valuable to this library
on its own merits. (3) is flagged as the cheap fallback if minimizing
`Paint`-loop complexity matters more than generality right now.

## Scope note

This proposal is about `TextArea` and `List` specifically, since those
are the two widgets where the gap was confirmed directly. Any other
row/buffer-oriented widget added later should be checked against the
same gap before being considered "done."
