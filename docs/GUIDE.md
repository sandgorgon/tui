# Guide: how apps are built with tui

This is a conceptual walkthrough for someone writing their first app
against this library — what a Model is, how a frame actually gets to
the screen, and how the pieces (`tui`, `widget`, `layout`, `style`,
`cell`, `input`) fit together. For the layered architecture and the
rationale behind each design choice, see [`DESIGN.md`](DESIGN.md); for
a working starting point, see the [Quick Start](../README.md#quick-start)
in the README and `examples/todo`.

## 1. The mental model: Elm-style, one Model per app

Every app implements three methods:

```go
type Model interface {
    Init() Cmd
    Update(msg Msg) (Model, Cmd)
    View() Node
}
```

- **`Init`** runs once, before the first frame is drawn. Return a
  `Cmd` here to kick off something asynchronous up front (loading
  data, starting a timer) — or `nil` if there's nothing to do.
- **`Update`** is the only place your Model's state changes. It's
  called once per incoming `Msg`, and returns the next Model plus an
  optional `Cmd`.
- **`View`** describes what the current Model should look like, as a
  tree of `Node` values. It's called after every `Update` and must be
  a pure function of the Model — no I/O, no mutation of anything
  outside its own return value.

`Model` is a value your app defines; `tui` never subclasses or wraps
it. A `*App` (`tui.NewApp(model, width, height)`) is the thing that
actually owns a `Model` and drives this loop — see §5.

This is the same shape as Elm/`bubbletea`: state lives in one place,
change happens through message-passing, and rendering is a pure
projection of state. If you've used either, the concepts transfer
directly.

## 2. Msg and Cmd: how things happen

```go
type Msg any
type Cmd func() Msg
```

A `Msg` is anything — a decoded keypress, a timer firing, the result
of an HTTP call, an application-defined type like `todosLoadedMsg`.
There's no interface to implement; `Update` just type-switches on it:

```go
func (m *model) Update(msg tui.Msg) (tui.Model, tui.Cmd) {
    switch v := msg.(type) {
    case todosLoadedMsg:
        m.items = v.items
    case input.KeyEvent:
        if v.Rune == 'q' {
            return m, tui.Quit()
        }
    }
    return m, nil
}
```

A `Cmd` is a unit of async work. **`Update` and `View` must never
block or do I/O directly** — if you need to sleep, make a network
call, or read a file, wrap it in a `Cmd` and return it:

```go
func (m *model) Init() tui.Cmd {
    return func() tui.Msg {
        items, err := loadTodos() // this runs on its own goroutine
        if err != nil {
            return loadErrMsg{err}
        }
        return todosLoadedMsg{items}
    }
}
```

The App runs each `Cmd` on its own goroutine and feeds the `Msg` it
returns back into the single event-loop goroutine, which is the only
goroutine that ever touches your Model. That's the whole point of the
`Cmd` indirection: it lets you do arbitrary async work without ever
needing a mutex around your Model. `tui.Batch(cmds...)` runs several
`Cmd`s concurrently when one `Update` needs to kick off more than one
thing. `tui.Quit()` is a `Cmd` that ends the run loop — returning it
is the normal way to exit. `tui.CopyToClipboard(text)` is the same
shape, for writing to the system clipboard via OSC 52.

## 3. Node and View: declarative, not retained

`View() Node` returns a **description** of the UI, not the UI itself.
`Node` is a small, immutable, write-only value — you build one with
`tui.Text`, `tui.Box`, or a widget constructor, return it, and never
read it back:

```go
func (m *model) View() tui.Node {
    return tui.Box(layout.Vertical,
        tui.Child(layout.Length(1), tui.Text("hello", cell.Style{})),
        tui.Child(layout.Fill(1), widget.List(m.items, m.cursor,
            widget.ListOptions{}, listEvent)),
    ).Margin(1)
}
```

Three kinds of `Node`:

- **`tui.Text(s, style)`** — a single line of styled text.
- **`tui.Box(direction, children...)`** — lays out children along
  `layout.Vertical` or `layout.Horizontal` using package `layout`'s
  constraint solver (see §4). `tui.Child(constraint, node)` pairs a
  child with how it should be sized. `.Gap(n)` and `.Margin(n)` add
  spacing.
- **Widget nodes** — anything from package `widget` (`widget.List`,
  `widget.TextInput`, `widget.Table`, ...), or your own via
  `tui.Component`. These wrap a **retained** `Widget` instance — see
  §6, it's the one genuinely novel piece of this library's design.

Every frame, `View()` is called again and its new `Node` tree is
diffed against the previous frame's, matched sibling-by-sibling by
position (or by an explicit `.Key(k)` when a list can reorder — the
same reason React/Elm/bubbletea's list-diffing needs keys). Where a
`Node` still matches the same tree slot, the retained state behind it
(a widget instance, or nothing at all for `Text`/`Box`) survives;
where it doesn't, a fresh one is constructed. You never manage this
yourself — it happens on every `Dispatch`.

## 4. Layout: constraints, not pixels

A `Box`'s children are sized by a one-pass constraint solver
(`package layout`), re-run fresh every frame against whatever `Rect`
the `Box` itself was given — so resizing the terminal window "just
works" with no extra code. The constraints:

| Constraint | Meaning |
|---|---|
| `layout.Length(n)` | exactly `n` cells |
| `layout.Percent(p)` | `p`% of the available space |
| `layout.Ratio(num, den)` | `num/den` of the available space |
| `layout.Min(n)` / `layout.Max(n)` | a floor/ceiling, sharing remaining space with other flexible children |
| `layout.Fill(weight)` | takes a share of whatever's left over, proportional to `weight` among other `Fill` siblings |

`Length`/`Percent`/`Ratio` children are sized first; `Fill`/`Min`/`Max`
children split what's left. This is the same idea as flexbox's
`flex-basis`/`flex-grow`, or `tview`'s `AddItem` proportions, just
computed fresh each frame instead of retained/mutated.

## 5. App: the thing that runs a Model

```go
app := tui.NewApp(model{}, 80, 24)
if err := app.Run(); err != nil { ... }
```

`Run` puts the terminal into raw mode, switches to the alternate
screen, and drives the loop until `Update` returns `tui.Quit()` or
stdin errors out. Each iteration: decode one `input.Event`, feed it in
(see §7), re-run `View`, reconcile, paint the diff to the real
terminal.

`App`'s core stepping logic (`Dispatch`, `Resize`, `HandleInput`,
`Buffer`) is deliberately I/O-free and safe to drive from a test with
no real terminal at all — construct an `App`, feed it synthetic
`Msg`/`input.Event` values, and assert on `Buffer()`. `Run` is a thin
adapter connecting that core to a real tty. This split is why widgets
and whole apps in this repo are unit-tested headlessly rather than
through a pty in most cases (a real pty is still used for true
end-to-end smoke tests — see `examples/*` and the various
`roundtrip_test.go` files).

**An `App` is not safe for concurrent use.** Everything that touches
your `Model` runs on one goroutine (`Run`'s event loop, or whichever
goroutine calls `Dispatch`/`HandleInput` directly in a headless
scenario) — this is what makes it safe for `Update` to mutate the
Model directly without a mutex.

## 6. Retained widgets: where the ephemeral state lives

This is the one deliberately novel piece of the design (see
`DESIGN.md` §3.1 for the full rationale), a hybrid of `bubbletea`
(pure MVU, no retained state at all), `tview` (fully retained/OOP),
and `Ratatui` (immediate mode, nothing retained).

The rule of thumb: **if a piece of state matters to your application
logic, it lives in your Model. If it's purely cosmetic — how the UI
happens to be presenting itself right now — it lives inside the
widget.**

Concretely:

- *Which* todo is selected, *what* text is in a todo, *whether* a
  todo is done — that's business state. Your `Model` owns it, passes
  it into `widget.List`/`widget.TextInput` as props on every frame,
  and reads it back out via the `onEvent` callback.
- A list's scroll offset (so the selected row stays visible), a
  cursor's blink phase, an in-progress undo/redo buffer, a `Table`'s
  column-drag state — that's ephemeral UI state nobody outside the
  widget needs to know about. It lives in the retained `Widget`
  instance the reconciler keeps alive across frames, and there is
  deliberately no getter for it — exposing it would defeat the point.

A widget constructor like `widget.List(items, cursor, opts, onEvent)`
returns a `tui.Node` wrapping a `Widget`:

```go
type Widget interface {
    Reconcile(props any) (changed bool)
    Paint(p *cell.Painter)
    HandleEvent(e input.Event) Cmd
    Focusable() bool
    SetFocused(focused bool)
}
```

You will rarely implement this yourself — package `widget` already
covers text, lists/tables/trees, forms (`TextInput`, `TextArea`,
`Select`, `RadioGroup`, `CheckboxGroup`), overlays (`Modal`,
`CommandPalette`), feedback (`Spinner`, `ProgressBar`, `StatusBar`),
and `Terminal` (a full embedded VT100/xterm emulator wired to a real
pty — enough to run a real shell, `vim`, or `htop` inside a pane). See
`examples/gallery` for all of them exercised in one app, and
`DESIGN.md` §5 for the full catalog.

## 7. Input: one event, two destinations

`App.HandleInput` takes one decoded `input.Event` (`KeyEvent`,
`MouseEvent`, `PasteEvent`, `FocusEvent`) and delivers it two ways
**on the same keypress**:

1. Always, as a `Msg` to `Update` (via `Dispatch`) — so your Model can
   react to global keys (`q` to quit, a hotkey that doesn't belong to
   any one widget) at their original, untranslated coordinates.
2. To whichever widget currently holds keyboard focus, via that
   widget's `HandleEvent` — and, for widgets wrapped with
   `tui.Focusable`, or built with an `onEvent` callback prop (as
   nearly every real `widget.Xxx` is), that callback turns the event
   into an application `Msg` which flows back through `Update` too.

This split matters: a keypress meant only for the focused widget (e.g.
arrow keys moving a list cursor) should be handled *only* in that
widget's `onEvent`, not duplicated in the `input.KeyEvent` case of
`Update` — see the comment in `examples/todo`'s `Update` for a
concrete instance of getting this right.

Tab/Shift-Tab move focus among focusable widgets automatically — you
don't write any focus-traversal code yourself unless a widget needs to
claim raw Tab (e.g. `TextArea` typing a literal tab character; see
`tui.RawKeyClaimer`). Mouse clicks move focus too (click-to-focus),
with coordinates translated to be local to whichever widget was
clicked, so a widget never needs to know its own absolute screen
position.

`TextArea` also handles a standard set of navigation keys beyond plain
arrows: `Ctrl+Left`/`Ctrl+Right` jump by word, `Ctrl+Home`/`Ctrl+End`
jump to the start/end of the buffer, and `PageUp`/`PageDown` move by
one screenful of lines. All of them extend the active selection when
held with `Shift`, the same as the plain arrow keys already do.

## 8. Styling: cell.Style directly, or style.Theme

Every visible thing — `tui.Text`, a widget's rows, borders — is
painted using `cell.Style{Fg, Bg, Attr}` (`cell.Color` values, plus
bold/reverse/underline-style attributes). For a one-off app you can
build these by hand. Package `style` adds a `Theme` on top: a small
set of semantic roles (`Theme.Text()`, `Theme.MutedText()`,
`Theme.BorderStyle()`, `Theme.FocusStyle()`) with `style.DefaultDark`,
`style.DefaultLight`, or `style.DetectAppearance` (reads `$COLORFGBG`)
to pick one — most `widget.Xxx` constructors take a `Theme` (or an
`Options` struct with a `Theme` field) so widgets in the same app look
consistent without threading individual colors through everywhere.

## 9. Putting it together

A typical `View` mixes all of the above — layout for structure,
widgets for interactive pieces, a theme for consistent styling:

```go
func (m *model) View() tui.Node {
    return tui.Box(layout.Vertical,
        tui.Child(layout.Length(1), widget.Tabs(m.pageLabels, m.page, m.theme, tabsEvent)),
        tui.Child(layout.Fill(1), widget.List(m.items, m.cursor,
            widget.ListOptions{Theme: m.theme}, listEvent)),
        tui.Child(layout.Length(1), widget.StatusBar(nil, nil,
            []widget.Segment{{Text: "q: quit"}}, m.theme.MutedText())),
    )
}
```

From here:

- `examples/todo` is the smallest complete example: a Model, focus
  traversal between two widgets, and an async `Init`.
- `examples/gallery` exercises the entire widget catalog, theming, a
  live `Terminal` pane, `Modal`/`CommandPalette`, and mouse support in
  one app — the best place to see real constructor signatures and
  `Options` structs in context.
- `DESIGN.md` is the reference for *why* things are shaped this way,
  the full widget catalog, and the PTY/VT emulator internals if you're
  using `widget.Terminal` or package `vt`/`pty` directly.
