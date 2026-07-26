package widget

import (
	"strings"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/internal/wcwidth"
	"github.com/sandgorgon/tui/tui"
)

// Paragraph is word-wrapped, styled text. It's a tui.Component (not a
// plain tui.Text) because word-wrapping needs to know the width it's
// been assigned, which isn't known until Paint — Node construction, in
// View(), happens before layout runs (docs/DESIGN.md §3.3) — so
// wrapping can't happen any earlier than that. It has no retained
// state of its own; the wrap is recomputed fresh every Paint.
func Paragraph(text string, style cell.Style) tui.Node {
	return tui.Component(nil, paragraphProps{text: text, style: style}, func() tui.Widget {
		return &paragraphWidget{}
	})
}

type paragraphProps struct {
	text  string
	style cell.Style
}

type paragraphWidget struct {
	paragraphProps
}

func (w *paragraphWidget) Reconcile(props any) bool {
	w.paragraphProps = props.(paragraphProps)
	return true
}

func (w *paragraphWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width <= 0 || height <= 0 {
		return
	}
	for y, line := range wrapText(w.text, width) {
		if y >= height {
			break
		}
		p.Text(0, y, line, w.style)
	}
}

func (w *paragraphWidget) HandleEvent(input.Event) tui.Cmd { return nil }
func (w *paragraphWidget) Focusable() bool                 { return false }
func (w *paragraphWidget) SetFocused(bool)                 {}

// wrapText greedily word-wraps text to fit within width columns
// (measured via wcwidth.RuneWidth, so wide runes count as 2), treating
// each "\n"-separated section of text as its own paragraph — an
// existing newline is always a hard break, never merged into a
// wrapped line — and hard-breaking any single word wider than width
// rather than letting it overflow the line.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return nil
	}
	var lines []string
	for para := range strings.SplitSeq(text, "\n") {
		lines = append(lines, wrapParagraph(para, width)...)
	}
	return lines
}

func wrapParagraph(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	var cur strings.Builder
	curWidth := 0
	flush := func() {
		lines = append(lines, cur.String())
		cur.Reset()
		curWidth = 0
	}

	for _, word := range words {
		wWidth := stringWidth(word)
		for wWidth > width {
			if curWidth > 0 {
				flush()
			}
			var head, rest string
			head, _, rest = breakToWidth(word, width)
			lines = append(lines, head)
			word, wWidth = rest, stringWidth(rest)
		}

		sep := 0
		if curWidth > 0 {
			sep = 1
		}
		if curWidth+sep+wWidth > width {
			flush()
			sep = 0
		}
		if sep == 1 {
			cur.WriteByte(' ')
			curWidth++
		}
		cur.WriteString(word)
		curWidth += wWidth
	}
	if curWidth > 0 || len(lines) == 0 {
		flush()
	}
	return lines
}

func stringWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func runeWidth(r rune) int {
	w := wcwidth.RuneWidth(r)
	if w <= 0 {
		return 1
	}
	return w
}

// breakToWidth splits s at the last rune boundary that keeps head's
// display width <= width, for hard-breaking a single word too wide to
// fit any line at all.
func breakToWidth(s string, width int) (head string, headWidth int, rest string) {
	w := 0
	for i, r := range s {
		rw := runeWidth(r)
		if w+rw > width {
			return s[:i], w, s[i:]
		}
		w += rw
	}
	return s, w, ""
}
