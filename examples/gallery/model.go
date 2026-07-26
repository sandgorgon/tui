package main

import (
	"os"
	"os/exec"
	"time"

	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
	"github.com/sandgorgon/tui/widget"
)

// Page indexes into pageLabels/model.page.
const (
	pageText = iota
	pageData
	pageForms
	pageTerminal
	pageCount
)

var pageLabels = []string{"Text & Feedback", "Lists & Data", "Forms", "Terminal"}

// model is the gallery's single Model, holding every page's business
// state (docs/DESIGN.md §3.1: cursors, selections, checked/expanded
// flags are all business state the app must know to act on, exactly
// like the M8 todo example's cursor/items) plus which page is active
// and a status line every page's onEvent can set for feedback.
type model struct {
	page   int
	status string

	// Page 0: Text & Feedback.
	progress float64 // 0..1, advanced by tick

	// Page 1: Lists & Data.
	listCursor   int
	listSelected []bool
	listItems    []string

	treeCursor    int
	treeNodes     []*treeNode
	treeNodeIndex []*treeNode // parallel to treeRows: row i's source node
	treeRows      []widget.TreeRow

	tableCols     []widget.Column
	tableRows     [][]string
	tableCursor   int
	tableSortCol  int
	tableSortDesc bool

	// Page 2: Forms.
	radioOptions     []string
	radioSelected    int
	checkboxCursor   int
	checkboxChecked  []bool
	checkboxOptions  []string
	selectOptions    []string
	selectSelected   int
	selectCursor     int
	selectOpen       bool
	lastSubmitted    string
	textAreaContents string

	// Overlays.
	paletteOpen bool
	modalOpen   bool

	theme    style.Theme
	shellCmd func() *exec.Cmd
}

// treeNode is the gallery's hierarchical business state for the tree
// demo (docs/DESIGN.md §3.1: expanded flags and the tree's shape are
// business state the app owns, not widget.Tree). rebuildTreeRows
// flattens it into the []widget.TreeRow the widget actually renders,
// skipping the children of any node that isn't expanded.
type treeNode struct {
	label    string
	expanded bool
	children []*treeNode
}

// rebuildTreeRows re-flattens m.treeNodes into m.treeRows, and must be
// called after any change to a node's expanded flag — otherwise a
// collapsed node's children would keep rendering, since widget.Tree
// only draws whatever rows it's given.
func (m *model) rebuildTreeRows() {
	m.treeRows = m.treeRows[:0]
	m.treeNodeIndex = m.treeNodeIndex[:0]
	flattenTree(m.treeNodes, 0, &m.treeRows, &m.treeNodeIndex)
}

func flattenTree(nodes []*treeNode, depth int, rows *[]widget.TreeRow, index *[]*treeNode) {
	for _, n := range nodes {
		*rows = append(*rows, widget.TreeRow{
			Label:       n.label,
			Depth:       depth,
			HasChildren: len(n.children) > 0,
			Expanded:    n.expanded,
		})
		*index = append(*index, n)
		if n.expanded {
			flattenTree(n.children, depth+1, rows, index)
		}
	}
}

func newModel() *model {
	m := &model{
		listItems:       []string{"apple", "banana", "cherry", "date", "elderberry"},
		listSelected:    make([]bool, 5),
		checkboxOptions: []string{"wrap long lines", "show line numbers", "auto-save"},
		checkboxChecked: []bool{true, false, false},
		radioOptions:    []string{"small", "medium", "large"},
		selectOptions:   []string{"tab width 2", "tab width 4", "tab width 8"},
		selectSelected:  1,
		theme:           style.Default(style.DetectAppearance(os.Getenv)),
	}
	m.listSelected[0] = true

	m.treeNodes = []*treeNode{
		{label: "cmd", expanded: true, children: []*treeNode{
			{label: "gallery"},
		}},
		{label: "widget", expanded: true, children: []*treeNode{
			{label: "table.go"},
			{label: "tree.go"},
		}},
		{label: "vt", children: []*treeNode{
			{label: "parser.go"},
			{label: "screen.go"},
			{label: "scrollback.go"},
		}},
	}
	m.rebuildTreeRows()

	m.tableCols = []widget.Column{
		{Title: "Package", Width: 12},
		{Title: "Milestone", Width: 10},
		{Title: "Status", Width: 8},
	}
	m.tableRows = [][]string{
		{"term", "M1", "done"},
		{"cell", "M2", "done"},
		{"vt", "M4", "done"},
		{"render", "M5", "done"},
		{"widget", "M11", "done"},
	}

	m.shellCmd = func() *exec.Cmd {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		return exec.Command(shell)
	}

	return m
}

// reset restores every page's business state to its initial value —
// the command palette's "Reset demo state" entry and the modal's OK
// button both drive this, as a small demo of one Cmd/Msg-free Model
// mutation reachable from two different overlay widgets. Navigation
// (page), theme, and the shell command are deliberately left alone.
func (m *model) reset() {
	fresh := newModel()
	m.progress = 0
	m.listCursor, m.listSelected = 0, fresh.listSelected
	m.treeCursor, m.treeNodes = 0, fresh.treeNodes
	m.rebuildTreeRows()
	m.tableCursor, m.tableSortCol, m.tableSortDesc = 0, 0, false
	m.tableRows = fresh.tableRows
	m.radioSelected = 0
	m.checkboxCursor, m.checkboxChecked = 0, fresh.checkboxChecked
	m.selectCursor, m.selectSelected, m.selectOpen = 0, fresh.selectSelected, false
	m.lastSubmitted, m.textAreaContents = "", ""
}

type tickMsg struct{}

func tickCmd() tui.Cmd {
	return func() tui.Msg {
		time.Sleep(120 * time.Millisecond)
		return tickMsg{}
	}
}

func (m *model) Init() tui.Cmd {
	return tickCmd()
}
