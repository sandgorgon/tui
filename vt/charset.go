package vt

// charset identifies which character set a G-set (G0/G1) is
// designated to. Only ASCII and DEC Special Graphics (line drawing)
// are supported — the common real-world case; broader ISO-2022
// multi-charset switching is out of scope (see docs/DESIGN.md §7).
type charset uint8

const (
	charsetASCII charset = iota
	charsetDECSpecialGraphics
)

// decSpecialGraphics maps the DEC Special Graphics charset's
// printable-ASCII code points to the line-drawing/symbol characters
// they represent when that charset is invoked into GL. This is the
// standard VT100 line-drawing set ("ACS") that curses/ncurses-based
// programs (htop, less, some vim configurations) rely on for box
// borders when not emitting UTF-8 box-drawing characters directly.
var decSpecialGraphics = map[rune]rune{
	'`': '◆', 'a': '▒', 'b': '␉', 'c': '␌', 'd': '␍', 'e': '␊',
	'f': '°', 'g': '±', 'h': '␤', 'i': '␋', 'j': '┘', 'k': '┐',
	'l': '┌', 'm': '└', 'n': '┼', 'o': '⎺', 'p': '⎻', 'q': '─',
	'r': '⎼', 's': '⎽', 't': '├', 'u': '┤', 'v': '┴', 'w': '┬',
	'x': '│', 'y': '≤', 'z': '≥', '{': 'π', '|': '≠', '}': '£', '~': '·',
}
