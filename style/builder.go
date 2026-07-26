package style

import "github.com/sandgorgon/tui/cell"

// Builder is a small fluent helper for building up a cell.Style,
// meant for widget code that wants e.g.
//
//	style.New(theme.Primary).Bold().Underline(cell.UnderlineCurly).Style()
//
// instead of a cell.Style{} struct literal. Like Node and
// layout.Layout's builder methods, each method returns a modified
// copy, so a Builder can be safely reused as a base for several
// variations.
type Builder struct {
	s cell.Style
}

// New starts a Builder with fg as the foreground color.
func New(fg cell.Color) Builder {
	return Builder{s: cell.Style{Fg: fg}}
}

// From starts a Builder from an existing cell.Style, e.g. to layer a
// widget-specific tweak on top of a Theme's base style.
func From(s cell.Style) Builder {
	return Builder{s: s}
}

func (b Builder) Fg(c cell.Color) Builder { b.s.Fg = c; return b }
func (b Builder) Bg(c cell.Color) Builder { b.s.Bg = c; return b }

func (b Builder) Bold() Builder          { b.s.Attr |= cell.AttrBold; return b }
func (b Builder) Faint() Builder         { b.s.Attr |= cell.AttrFaint; return b }
func (b Builder) Italic() Builder        { b.s.Attr |= cell.AttrItalic; return b }
func (b Builder) Blink() Builder         { b.s.Attr |= cell.AttrBlink; return b }
func (b Builder) Reverse() Builder       { b.s.Attr |= cell.AttrReverse; return b }
func (b Builder) Strikethrough() Builder { b.s.Attr |= cell.AttrStrikethrough; return b }
func (b Builder) Invisible() Builder     { b.s.Attr |= cell.AttrInvisible; return b }

func (b Builder) Underline(u cell.UnderlineStyle) Builder {
	b.s.Underline = u
	return b
}

func (b Builder) UnderlineColor(c cell.Color) Builder {
	b.s.UnderlineColor = c
	return b
}

func (b Builder) Hyperlink(uri string) Builder {
	b.s.Hyperlink = uri
	return b
}

// Style returns the built cell.Style.
func (b Builder) Style() cell.Style {
	return b.s
}
