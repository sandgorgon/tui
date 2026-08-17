# Proposal: PageUp/PageDown, buffer-start/end, and word-jump for TextArea

Status: proposal, not accepted. Filed by a consumer (kaze) for
consideration whenever this repo's own maintainer picks it up — not a
commitment to build it, and not built here. Same spirit as the earlier
`text-region-styling.md` proposal this repo already merged (`02a0538`).

## Problem

`widget.TextArea` only moves the cursor one character or one line at a
time — no page-scroll, no jump-to-buffer-start/end, no word-boundary
jump. Confirmed directly: `textAreaWidget.handleKey`
(`widget/textarea.go:259`) has cases for `KeyLeft`/`KeyRight`/`KeyUp`/
`KeyDown`/`KeyHome`/`KeyEnd`/`KeyBackspace`/`KeyDelete`/`KeyEnter`/
`KeyTab`/Ctrl+Z/Ctrl+Y/plain runes — nothing for `KeyPgUp`/`KeyPgDown`,
and no case anywhere checks `ke.Mod&ModCtrl != 0` together with
`KeyLeft`/`KeyRight`/`KeyHome`/`KeyEnd`. For any document longer than a
screenful, or any edit that isn't right next to the cursor, that's a
real gap for a widget whose own doc comment calls it "a real, keystroke-
driven terminal UI" — this consumer (kaze's own `-tui`, a real editor
surface, not a form field) hits it immediately in ordinary use.

**This is not a decoder gap** — `input.Decoder` already produces
everything this needs, confirmed by reading `input/decode.go` directly
rather than assumed:
- `KeyPgUp`/`KeyPgDown` are already named `Key` constants
  (`input/event.go:75`) and already decoded, both via the CSI final-letter
  form and the legacy tilde form (`tildeEvent`, `decode.go:308`, cases
  `5`/`6`).
- Every one of `arrowEvent` (`decode.go:304`, covers Up/Down/Right/Left/
  Home/End) and `tildeEvent` already returns `Mod: modFromParam(ps, 1)`
  — so `Ctrl+Home`, `Ctrl+End`, `Ctrl+Left`, `Ctrl+Right`, and even
  `Ctrl+PageUp`/`Ctrl+PageDown` all already decode with `ModCtrl` set
  correctly today. `textAreaWidget.handleKey` just never looks at that
  modifier for anything but Ctrl+Z/Ctrl+Y.

So the entire gap is confined to `handleKey` and (for PageUp/PageDown
specifically) one small piece of missing retained state — nothing in
the input layer needs to change.

## Proposed additions

All three reuse `editBuffer`'s existing movement primitives
(`widget/editbuffer.go`) — `moveTo(pos, shift)` already handles the
shift-extends-selection-vs-collapses-and-moves logic uniformly, so
every addition below gets Shift-to-select for free by construction, the
same way Home/End already do.

**1. Ctrl+Home / Ctrl+End — jump to buffer start/end.** The simplest of
the three: `case ke.Mod&ModCtrl != 0 && ke.Key == KeyHome:
w.moveTo(0, shift)`, `... KeyEnd: w.moveTo(len(w.buf), shift)`. No new
state needed.

**2. Ctrl+Left / Ctrl+Right — word-boundary jump.** Needs new logic —
there's no existing word-boundary classifier anywhere in the codebase
(confirmed by grep for `isWordChar` and similar, no match). Proposed:
a minimal `isWordChar(r rune) bool` (letter/digit/underscore) plus a
`wordBoundary(buf []rune, from int, delta int) int` scan in that
direction skipping any leading run of non-word chars, then the
following run of word chars (or the reverse for delta<0) — standard
"skip whitespace/punctuation, then skip the word" editor convention.
Result fed through the same `moveTo`.

**3. PageUp / PageDown — cursor by one screenful of lines.** The same
shape as `moveVertical` (`textarea.go:307`, already moves by a delta of
lines, clamping to the target line's length) but with `delta =
±visibleLines` instead of `±1`. The real wrinkle, worth flagging
explicitly rather than glossing over: `moveVertical` has no way to know
how many lines are currently visible — that number (`innerH`) is
computed fresh inside `Paint` (`textarea.go:124`, from the painter's
own `Size()`) and never retained on the widget; only `scrollRow`/
`scrollCol` survive between frames. The natural fix is to retain it
too — a new `lastVisibleLines int` field on `textAreaWidget`, set at
the same point `Paint` already computes `innerH`, read by
`handleKey`'s new `KeyPgUp`/`KeyPgDown` cases (falling back to moving
by 1 if it's still zero, i.e. before the first `Paint`). This is the
one piece of the three that isn't a pure `handleKey` addition.

## Scope

Deliberately narrow, matching what was actually asked for — not
proposing Select-All, word-delete (Ctrl+Backspace/Ctrl+Delete), or any
change to undo/redo, selection rendering, or `TextAreaOptions`'s public
shape. All three additions above are internal to `textAreaWidget` and
change no exported API.
