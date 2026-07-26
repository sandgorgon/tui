package main

import (
	"sort"

	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/tui"
	"github.com/sandgorgon/tui/widget"
)

func (m *model) Update(msg tui.Msg) (tui.Model, tui.Cmd) {
	switch v := msg.(type) {
	case tickMsg:
		m.progress += 0.01
		if m.progress > 1 {
			m.progress = 0
		}
		return m, tickCmd()

	case tabMsg:
		m.applyTab(v)

	case listMsg:
		m.applyList(v)
	case treeMsg:
		m.applyTree(v)
	case tableMsg:
		m.applyTable(v)
	case radioMsg:
		m.radioSelected = clampIndex(m.radioSelected+v.delta, len(m.radioOptions))
	case checkboxMsg:
		m.applyCheckbox(v)
	case selectMsg:
		m.applySelect(v)

	case widget.Command:
		m.status = "command palette: " + v.Label
		m.paletteOpen = false
		if page, ok := v.Data.(int); ok {
			m.page = page
		} else if v.Label == "Reset demo state" {
			m.reset()
		}
	case string:
		if v == "palette-cancel" {
			m.paletteOpen = false
		}
	case modalResultMsg:
		m.modalOpen = false
		if v.confirm {
			m.reset()
			m.status = "modal: confirmed, demo state reset"
		} else {
			m.status = "modal: cancelled"
		}

	case input.KeyEvent:
		// Global keys, delivered to Update in parallel with whatever
		// widget is focused (see docs/DESIGN.md §8's App.HandleInput and
		// the M8 todo example's identical pattern). F-keys and Ctrl+Q are
		// deliberately used instead of bare letters or Ctrl+C: the Forms
		// page has free-text fields (typing "q" must not quit) and the
		// Terminal page hosts a real shell that needs its own Ctrl+C.
		switch {
		case v.Mod&input.ModCtrl != 0 && v.Rune == 'q':
			return m, tui.Quit()
		case v.Key == input.KeyF1:
			m.paletteOpen = !m.paletteOpen
		case v.Key == input.KeyF2:
			m.modalOpen = !m.modalOpen
		case v.Key == input.KeyF3:
			m.status = "copied build status to clipboard (OSC 52)"
			return m, tui.CopyToClipboard("tui gallery: M0-M12 in progress")
		}
	}
	return m, nil
}

func clampIndex(i, n int) int {
	if n <= 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

func (m *model) applyTab(v tabMsg) {
	switch {
	case v.index == -1: // Left
		m.page = (m.page - 1 + pageCount) % pageCount
	case v.index == -2: // Right
		m.page = (m.page + 1) % pageCount
	case v.index >= 100: // absolute, from a digit key
		if idx := v.index - 100; idx < pageCount {
			m.page = idx
		}
	default: // absolute, from a mouse click
		if v.index >= 0 && v.index < pageCount {
			m.page = v.index
		}
	}
}

func (m *model) applyList(v listMsg) {
	if v.toggle {
		if m.listCursor < len(m.listSelected) {
			m.listSelected[m.listCursor] = !m.listSelected[m.listCursor]
		}
		return
	}
	m.listCursor = clampIndex(m.listCursor+v.delta, len(m.listItems))
}

func (m *model) applyTree(v treeMsg) {
	if v.toggle {
		if m.treeCursor < len(m.treeRows) && m.treeRows[m.treeCursor].HasChildren {
			m.treeRows[m.treeCursor].Expanded = !m.treeRows[m.treeCursor].Expanded
		}
		return
	}
	m.treeCursor = clampIndex(m.treeCursor+v.delta, len(m.treeRows))
}

func (m *model) applyTable(v tableMsg) {
	switch {
	case v.sortCol == -2: // 's': cycle to the next column
		m.tableSortCol = (m.tableSortCol + 1) % len(m.tableCols)
		m.tableSortDesc = false
		m.sortTable()
	case v.sortCol >= 0:
		if v.sortCol == m.tableSortCol {
			m.tableSortDesc = !m.tableSortDesc
		} else {
			m.tableSortCol, m.tableSortDesc = v.sortCol, false
		}
		m.sortTable()
	default:
		m.tableCursor = clampIndex(m.tableCursor+v.delta, len(m.tableRows))
	}
}

func (m *model) sortTable() {
	col := m.tableSortCol
	sort.SliceStable(m.tableRows, func(i, j int) bool {
		if m.tableSortDesc {
			return m.tableRows[i][col] > m.tableRows[j][col]
		}
		return m.tableRows[i][col] < m.tableRows[j][col]
	})
}

func (m *model) applyCheckbox(v checkboxMsg) {
	if v.toggle {
		if m.checkboxCursor < len(m.checkboxChecked) {
			m.checkboxChecked[m.checkboxCursor] = !m.checkboxChecked[m.checkboxCursor]
		}
		return
	}
	m.checkboxCursor = clampIndex(m.checkboxCursor+v.delta, len(m.checkboxOptions))
}

func (m *model) applySelect(v selectMsg) {
	switch {
	case v.toggleOpen:
		m.selectOpen = !m.selectOpen
	case v.choose:
		if m.selectOpen {
			m.selectSelected = m.selectCursor
		}
		m.selectOpen = false
	default:
		m.selectCursor = clampIndex(m.selectCursor+v.delta, len(m.selectOptions))
	}
}
