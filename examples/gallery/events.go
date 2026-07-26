package main

import (
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/tui"
)

// Every onEvent function below follows the same contract every widget
// in package widget shares (see e.g. widget.List's doc comment):
// receive whatever input.Event the widget is focused for (already
// hit-test-translated by App for a MouseEvent, per the M12 mouse pass
// — see docs/DESIGN.md §9), decide what Msg it means, and return it
// (or nil to ignore); package tui feeds that Msg back through
// Model.Update.

type tabMsg struct{ index int }

func tabsEvent(e input.Event) tui.Msg {
	switch v := e.(type) {
	case input.MouseEvent:
		return tabMsg{index: v.X} // Tabs already translated X to a label index
	case input.KeyEvent:
		switch {
		case v.Key == input.KeyLeft:
			return tabMsg{index: -1} // relative: Update clamps/wraps
		case v.Key == input.KeyRight:
			return tabMsg{index: -2}
		case v.Rune >= '1' && v.Rune <= '9':
			return tabMsg{index: int(v.Rune-'1') + 100} // +100: Update treats >=100 as absolute
		}
	}
	return nil
}

type listMsg struct {
	delta  int
	toggle bool
}

func listEvent(e input.Event) tui.Msg {
	ke, ok := e.(input.KeyEvent)
	if !ok {
		return nil
	}
	switch {
	case ke.Key == input.KeyUp:
		return listMsg{delta: -1}
	case ke.Key == input.KeyDown:
		return listMsg{delta: 1}
	case ke.Key == input.KeyEnter || ke.Rune == ' ':
		return listMsg{toggle: true}
	}
	return nil
}

type treeMsg struct {
	delta  int
	toggle bool
}

func treeEvent(e input.Event) tui.Msg {
	ke, ok := e.(input.KeyEvent)
	if !ok {
		return nil
	}
	switch {
	case ke.Key == input.KeyUp:
		return treeMsg{delta: -1}
	case ke.Key == input.KeyDown:
		return treeMsg{delta: 1}
	case ke.Key == input.KeyEnter || ke.Rune == ' ':
		return treeMsg{toggle: true}
	}
	return nil
}

// tableMsg's sortCol is -1 for "no sort change"; delta is a row-cursor
// move.
type tableMsg struct {
	delta   int
	sortCol int
}

func tableEvent(e input.Event) tui.Msg {
	switch v := e.(type) {
	case input.KeyEvent:
		switch {
		case v.Key == input.KeyUp:
			return tableMsg{sortCol: -1, delta: -1}
		case v.Key == input.KeyDown:
			return tableMsg{sortCol: -1, delta: 1}
		case v.Rune == 's':
			return tableMsg{sortCol: -2} // -2: Update cycles to the next column
		}
	case input.MouseEvent:
		if v.Y == -1 { // header row sentinel, X is the clicked column
			return tableMsg{sortCol: v.X}
		}
	}
	return nil
}

type radioMsg struct{ delta int }

func radioEvent(e input.Event) tui.Msg {
	ke, ok := e.(input.KeyEvent)
	if !ok {
		return nil
	}
	switch ke.Key {
	case input.KeyUp:
		return radioMsg{delta: -1}
	case input.KeyDown:
		return radioMsg{delta: 1}
	}
	return nil
}

type checkboxMsg struct {
	delta  int
	toggle bool
}

func checkboxEvent(e input.Event) tui.Msg {
	ke, ok := e.(input.KeyEvent)
	if !ok {
		return nil
	}
	switch {
	case ke.Key == input.KeyUp:
		return checkboxMsg{delta: -1}
	case ke.Key == input.KeyDown:
		return checkboxMsg{delta: 1}
	case ke.Key == input.KeyEnter || ke.Rune == ' ':
		return checkboxMsg{toggle: true}
	}
	return nil
}

// selectMsg: exactly one of toggleOpen/choose is true, or delta is
// nonzero, per event.
type selectMsg struct {
	delta      int
	toggleOpen bool
	choose     bool
}

func selectEvent(e input.Event) tui.Msg {
	switch v := e.(type) {
	case input.KeyEvent:
		switch v.Key {
		case input.KeyUp:
			return selectMsg{delta: -1}
		case input.KeyDown:
			return selectMsg{delta: 1}
		case input.KeyEnter:
			return selectMsg{choose: true}
		}
	case input.MouseEvent:
		if v.Y == -1 {
			return selectMsg{toggleOpen: true}
		}
		return selectMsg{choose: true}
	}
	return nil
}

// modalButtonEvent is shared by the modal's two Focusable buttons;
// which one is which comes from the confirm bool baked in via a
// closure in pages.go.
func modalButtonEvent(confirm bool) func(input.Event) tui.Msg {
	return func(e input.Event) tui.Msg {
		ke, ok := e.(input.KeyEvent)
		if !ok || ke.Key != input.KeyEnter {
			return nil
		}
		return modalResultMsg{confirm: confirm}
	}
}

type modalResultMsg struct{ confirm bool }
