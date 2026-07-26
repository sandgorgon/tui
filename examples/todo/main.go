// Command todo is the M8 milestone demo: a counter plus a todo list,
// built entirely on package tui's App/Model/Update/View loop, proving
// Node, the reconciler, Cmd/Msg plumbing, and Tab/Shift-Tab focus
// traversal end to end.
//
// It exercises three things deliberately: Init returns a Cmd that
// simulates an async load (a real round trip through the Cmd/Msg
// channel, not just synchronous Update calls); the todo List is a
// retained tui.Widget whose scroll offset survives every Model-driven
// re-render, the concrete case docs/DESIGN.md §3.1 argues for; and Tab
// switches keyboard focus between the counter and the list, each
// visibly reacting (a highlighted border on the counter, a "> " marker
// on the list's selection) via tui.Widget.SetFocused.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/tui"
)

func main() {
	app := tui.NewApp(newModel(), 80, 24)
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "todo:", err)
		os.Exit(1)
	}
}

type todoItem struct {
	text string
	done bool
}

// model is deliberately plain application state, per docs/DESIGN.md
// §3.1: count, the items themselves, and which item is selected are
// all business state the app owns. Only the list's scroll position
// (retained inside tui.List's Widget instance) is ephemeral enough to
// live outside this struct.
type model struct {
	loading bool
	count   int
	items   []todoItem
	cursor  int
	nextID  int
}

func newModel() *model {
	return &model{loading: true}
}

type todosLoadedMsg struct{ items []todoItem }

func (m *model) Init() tui.Cmd {
	return func() tui.Msg {
		time.Sleep(400 * time.Millisecond) // stand-in for a real async load
		return todosLoadedMsg{items: []todoItem{
			{text: "wire up Init's Cmd", done: true},
			{text: "build the reconciler", done: true},
			{text: "ship the counter/todo demo", done: false},
		}}
	}
}

// countMsg and todoAction are produced by the focused widget's onEvent
// callback (see View), not by App's global raw-KeyEvent delivery — see
// the comment in Update for why that split matters.
type countMsg struct{ delta int }

type todoAction int

const (
	todoUp todoAction = iota
	todoDown
	todoToggle
	todoAdd
	todoDelete
)

type todoMsg struct{ action todoAction }

func (m *model) Update(msg tui.Msg) (tui.Model, tui.Cmd) {
	switch v := msg.(type) {
	case todosLoadedMsg:
		m.loading = false
		m.items = v.items
		m.nextID = len(v.items)

	case countMsg:
		m.count += v.delta

	case todoMsg:
		m.applyTodoAction(v.action)

	case input.KeyEvent:
		// App delivers every raw key both to Update (here) and to
		// whichever widget currently has focus (see View). This
		// branch is deliberately scoped to *global* keys only —
		// anything focus-scoped (changing the count, moving the todo
		// cursor) comes back as countMsg/todoMsg from the focused
		// widget's onEvent instead, so the same keypress is never
		// handled twice.
		if v.Rune == 'q' || (v.Mod&input.ModCtrl != 0 && v.Rune == 'c') {
			return m, tui.Quit()
		}
	}
	return m, nil
}

func (m *model) applyTodoAction(a todoAction) {
	switch a {
	case todoUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case todoDown:
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case todoToggle:
		if m.cursor < len(m.items) {
			m.items[m.cursor].done = !m.items[m.cursor].done
		}
	case todoAdd:
		m.items = append(m.items, todoItem{text: fmt.Sprintf("new item %d", m.nextID)})
		m.nextID++
		m.cursor = len(m.items) - 1
	case todoDelete:
		if len(m.items) == 0 {
			return
		}
		m.items = append(m.items[:m.cursor], m.items[m.cursor+1:]...)
		if m.cursor >= len(m.items) {
			m.cursor = len(m.items) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
	}
}

func (m *model) View() tui.Node {
	if m.loading {
		return tui.Text("loading...", cell.Style{})
	}

	counter := tui.Focusable("counter",
		tui.Text(fmt.Sprintf(" count: %d   (+/- to change)", m.count), cell.Style{}),
		counterEvent,
	)

	rows := make([]string, len(m.items))
	for i, it := range m.items {
		box := "[ ]"
		if it.done {
			box = "[x]"
		}
		rows[i] = box + " " + it.text
	}
	todos := tui.List(rows, m.cursor, cell.Style{}, cell.Style{Attr: cell.AttrReverse}, todoListEvent)

	help := tui.Text("Tab: switch focus   ^/v: move   enter/space: toggle   a: add   d: delete   q: quit",
		cell.Style{Fg: cell.ANSIColor(8)})

	return tui.Box(layout.Vertical,
		tui.Child(layout.Length(3), counter),
		tui.Child(layout.Fill(1), todos),
		tui.Child(layout.Length(1), help),
	).Margin(1)
}

func counterEvent(e input.Event) tui.Msg {
	ke, ok := e.(input.KeyEvent)
	if !ok {
		return nil
	}
	switch {
	case ke.Rune == '+' || ke.Rune == '=' || ke.Key == input.KeyRight:
		return countMsg{delta: 1}
	case ke.Rune == '-' || ke.Key == input.KeyLeft:
		return countMsg{delta: -1}
	}
	return nil
}

func todoListEvent(e input.Event) tui.Msg {
	ke, ok := e.(input.KeyEvent)
	if !ok {
		return nil
	}
	switch {
	case ke.Key == input.KeyUp:
		return todoMsg{todoUp}
	case ke.Key == input.KeyDown:
		return todoMsg{todoDown}
	case ke.Key == input.KeyEnter || ke.Rune == ' ':
		return todoMsg{todoToggle}
	case ke.Rune == 'a':
		return todoMsg{todoAdd}
	case ke.Rune == 'd':
		return todoMsg{todoDelete}
	}
	return nil
}
