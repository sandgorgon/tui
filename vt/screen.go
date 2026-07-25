package vt

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/internal/wcwidth"
)

// CursorShape is the visual shape a terminal should render the cursor
// as (DECSCUSR, CSI Ps SP q).
type CursorShape uint8

const (
	CursorBlock CursorShape = iota
	CursorUnderline
	CursorBar
)

// savedCursorState is what DECSC (ESC 7) saves and DECRC (ESC 8)
// restores. The primary and alternate screens each keep their own
// slot (see modeAltScreen in modes.go) — switching screens doesn't
// clobber the other screen's saved cursor, matching real terminals.
type savedCursorState struct {
	x, y        int
	style       cell.Style
	autowrap    bool
	originMode  bool
	g0, g1      charset
	glActive    int
	wrapPending bool
}

// Screen is a VT100/xterm-compatible terminal screen: primary and
// alternate cell.Buffers, cursor state, scroll regions, tab stops,
// character-set/graphics-mode state, and window title — the semantic
// model that Parser's Handler interface drives. See docs/DESIGN.md §7.
type Screen struct {
	primary, alt *cell.Buffer
	useAlt       bool
	cols, rows   int

	cx, cy        int
	cursorVisible bool
	cursorShape   CursorShape

	saved, savedAlt savedCursorState

	scrollTop, scrollBottom int

	tabStops []bool

	autowrap      bool
	originMode    bool
	insertMode    bool
	wrapPending   bool
	appCursorKeys bool // DECCKM; a host's key encoder cares about this, Screen itself doesn't act on it

	curStyle cell.Style

	g0, g1   charset
	glActive int

	title string

	sb *scrollback

	mouseMode          MouseMode
	mouseEncoding      MouseEncoding
	bracketedPaste     bool
	focusEvents        bool
	synchronizedOutput bool

	currentHyperlink string

	responses []byte
}

// NewScreen returns a Screen of the given size in the default state:
// primary screen active, cursor visible at (0,0), autowrap on, full-
// screen scroll region, tab stops every 8 columns.
func NewScreen(cols, rows int) *Screen {
	s := &Screen{
		primary:       cell.NewBuffer(cols, rows),
		alt:           cell.NewBuffer(cols, rows),
		cols:          cols,
		rows:          rows,
		cursorVisible: true,
		autowrap:      true,
		scrollBottom:  rows - 1,
		sb:            newScrollback(defaultScrollbackLines),
	}
	s.resetTabStops()
	return s
}

// Buffer returns the currently active screen buffer (primary, or
// alternate if an app has switched to it — see modeAltScreen).
func (s *Screen) Buffer() *cell.Buffer { return s.activeBuffer() }

// Cursor returns the cursor's current position (0-based, in the active
// buffer's coordinate space) and whether it's visible.
func (s *Screen) Cursor() (x, y int, visible bool) { return s.cx, s.cy, s.cursorVisible }

// CursorShape returns the cursor's requested visual shape (DECSCUSR).
func (s *Screen) CursorShape() CursorShape { return s.cursorShape }

// Title returns the most recently set window/tab title (OSC 0/1/2).
func (s *Screen) Title() string { return s.title }

// Size returns the screen's dimensions.
func (s *Screen) Size() (cols, rows int) { return s.cols, s.rows }

// Resize changes the screen's dimensions, preserving as much of the
// existing primary and alternate content as fits, anchored at the
// top-left — matching standard terminal resize behavior of not trying
// to reflow text. The scroll region is clamped to the new size and tab
// stops reset to the default every-8-columns spacing; scrollback is
// untouched.
func (s *Screen) Resize(cols, rows int) {
	if cols == s.cols && rows == s.rows {
		return
	}
	s.primary = resizeBuffer(s.primary, cols, rows)
	s.alt = resizeBuffer(s.alt, cols, rows)
	s.cols, s.rows = cols, rows

	s.resetTabStops()

	if s.scrollBottom >= rows {
		s.scrollBottom = rows - 1
	}
	if s.scrollTop > s.scrollBottom {
		s.scrollTop = 0
	}

	s.clampCursor()
}

func resizeBuffer(old *cell.Buffer, cols, rows int) *cell.Buffer {
	nb := cell.NewBuffer(cols, rows)
	w, h := min(old.Width, cols), min(old.Height, rows)
	for y := range h {
		for x := range w {
			nb.Set(x, y, old.At(x, y))
		}
	}
	return nb
}

// Scrollback returns the number of primary-screen lines available in
// scrollback, and the i'th such line (0 = oldest), for a caller that
// wants to render scrollback above the live screen.
func (s *Screen) ScrollbackLen() int               { return s.sb.Len() }
func (s *Screen) ScrollbackLine(i int) []cell.Cell { return s.sb.Line(i) }

// MouseMode, BracketedPaste, and FocusEvents report which input
// reporting modes the application has requested (DECSET), so a host
// (e.g. the eventual Terminal widget) knows whether to translate and
// forward input.Events to the child as the corresponding escape
// sequences, or handle them itself — see docs/DESIGN.md §7's note on
// transparent mode passthrough.
func (s *Screen) MouseMode() MouseMode         { return s.mouseMode }
func (s *Screen) MouseEncoding() MouseEncoding { return s.mouseEncoding }
func (s *Screen) BracketedPaste() bool         { return s.bracketedPaste }
func (s *Screen) FocusEventsEnabled() bool     { return s.focusEvents }
func (s *Screen) SynchronizedOutput() bool     { return s.synchronizedOutput }

// AppCursorKeys reports whether the application has requested
// application cursor-key mode (DECCKM) — relevant to a host encoding
// arrow-key input back to the child (SS3 vs. CSI form); Screen itself
// doesn't act on this, it's informational for that future consumer.
func (s *Screen) AppCursorKeys() bool { return s.appCursorKeys }

// TakeResponses returns and clears any bytes queued for writing back to
// the pty (DA1/DA2/DSR/CPR replies) since the last call. Screen has no
// direct access to a writer by design — that's an I/O concern for the
// caller (see docs/DESIGN.md §7).
func (s *Screen) TakeResponses() []byte {
	r := s.responses
	s.responses = nil
	return r
}

func (s *Screen) activeBuffer() *cell.Buffer {
	if s.useAlt {
		return s.alt
	}
	return s.primary
}

func (s *Screen) resetTabStops() {
	s.tabStops = make([]bool, s.cols)
	for x := 0; x < s.cols; x += 8 {
		s.tabStops[x] = true
	}
}

func (s *Screen) clampCursor() {
	s.cx = clamp(s.cx, 0, s.cols-1)
	s.cy = clamp(s.cy, 0, s.rows-1)
}

func (s *Screen) carriageReturn() {
	s.cx = 0
	s.wrapPending = false
}

// index moves the cursor down one line, scrolling the active region up
// if the cursor is at its bottom margin. What LF/IND/NEL do.
func (s *Screen) index() {
	s.wrapPending = false
	if s.cy == s.scrollBottom {
		s.scrollRegionUp(s.scrollTop, s.scrollBottom, 1, true)
	} else if s.cy < s.rows-1 {
		s.cy++
	}
}

// reverseIndex moves the cursor up one line, scrolling the active
// region down if the cursor is at its top margin. What RI does.
func (s *Screen) reverseIndex() {
	s.wrapPending = false
	if s.cy == s.scrollTop {
		s.scrollRegionDown(s.scrollTop, s.scrollBottom, 1)
	} else if s.cy > 0 {
		s.cy--
	}
}

func (s *Screen) newline() {
	s.index()
	s.cx = 0
}

// moveTo implements cursor positioning (CUP/HVP): row/col are 1-based,
// as CSI sends them. When origin mode (DECOM) is active, row 1 refers
// to the scroll region's top margin, not the physical top of screen,
// and the cursor is confined to the scroll region.
func (s *Screen) moveTo(row, col int) {
	s.wrapPending = false
	top, bottom := 0, s.rows-1
	if s.originMode {
		top, bottom = s.scrollTop, s.scrollBottom
	}
	y := clamp(top+row-1, top, bottom)
	x := clamp(col-1, 0, s.cols-1)
	s.cx, s.cy = x, y
}

func clamp(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

// moveBy implements relative cursor movement (CUU/CUD/CUF/CUB),
// applying the same origin-mode-aware vertical clamping as moveTo.
func (s *Screen) moveBy(dx, dy int) {
	s.wrapPending = false
	s.cx += dx
	s.cy += dy
	top, bottom := 0, s.rows-1
	if s.originMode {
		top, bottom = s.scrollTop, s.scrollBottom
	}
	s.cy = clamp(s.cy, top, bottom)
	s.cx = clamp(s.cx, 0, s.cols-1)
}

// scrollRegionUp shifts rows (top,bottom] up into [top,bottom), i.e.
// scrolls the region up by n, discarding what falls off the bottom and
// clearing what's revealed at the bottom. On the primary screen, lines
// scrolled off a region whose top boundary is the physical top of
// screen (top==0) are preserved in scrollback if toScrollback is set —
// IL/DL (line insert/delete) pass toScrollback=false since they're
// cursor-relative operations, not "the screen scrolled" in the sense
// that belongs in history.
func (s *Screen) scrollRegionUp(top, bottom, n int, toScrollback bool) {
	buf := s.activeBuffer()
	for range n {
		if toScrollback && !s.useAlt && top == 0 {
			s.sb.push(buf, 0, s.cols)
		}
		for y := top; y < bottom; y++ {
			s.copyRow(buf, y, y+1)
		}
		s.clearRow(buf, bottom)
	}
}

// scrollRegionDown is scrollRegionUp's mirror image: shifts rows
// [top,bottom) down into (top,bottom], clearing what's revealed at top.
func (s *Screen) scrollRegionDown(top, bottom, n int) {
	buf := s.activeBuffer()
	for range n {
		for y := bottom; y > top; y-- {
			s.copyRow(buf, y, y-1)
		}
		s.clearRow(buf, top)
	}
}

func (s *Screen) copyRow(buf *cell.Buffer, dst, src int) {
	for x := 0; x < s.cols; x++ {
		buf.Set(x, dst, buf.At(x, src))
	}
}

// blankCell is what erasing/scrolling reveals: a space in the current
// SGR style — real terminals apply the current (particularly,
// background) attributes to erased regions, which is how programs
// paint colored blank areas.
func (s *Screen) blankCell() cell.Cell {
	return cell.Cell{Rune: ' ', Style: s.curStyle, Width: 1}
}

func (s *Screen) clearRow(buf *cell.Buffer, y int) {
	blank := s.blankCell()
	for x := 0; x < s.cols; x++ {
		buf.Set(x, y, blank)
	}
}

func (s *Screen) eraseInDisplay(mode int) {
	buf := s.activeBuffer()
	switch mode {
	case 0: // cursor to end of screen
		s.eraseInLine(0)
		for y := s.cy + 1; y < s.rows; y++ {
			s.clearRow(buf, y)
		}
	case 1: // start of screen to cursor
		for y := 0; y < s.cy; y++ {
			s.clearRow(buf, y)
		}
		s.eraseInLine(1)
	case 2, 3: // whole screen; 3 also clears scrollback (xterm extension)
		for y := 0; y < s.rows; y++ {
			s.clearRow(buf, y)
		}
		if mode == 3 {
			s.sb = newScrollback(s.sb.max)
		}
	}
}

func (s *Screen) eraseInLine(mode int) {
	buf := s.activeBuffer()
	blank := s.blankCell()
	switch mode {
	case 0: // cursor to end of line
		for x := s.cx; x < s.cols; x++ {
			buf.Set(x, s.cy, blank)
		}
	case 1: // start of line to cursor, inclusive
		for x := 0; x <= s.cx && x < s.cols; x++ {
			buf.Set(x, s.cy, blank)
		}
	case 2: // whole line
		s.clearRow(buf, s.cy)
	}
}

// insertBlanks inserts n blank cells at the cursor, shifting the rest
// of the line right (cells pushed past the right edge are discarded).
// Used by both IRM (insert mode, before printing a char) and ICH.
func (s *Screen) insertBlanks(n int) {
	buf := s.activeBuffer()
	blank := s.blankCell()
	for x := s.cols - 1; x >= s.cx+n; x-- {
		buf.Set(x, s.cy, buf.At(x-n, s.cy))
	}
	for x := s.cx; x < s.cx+n && x < s.cols; x++ {
		buf.Set(x, s.cy, blank)
	}
}

// deleteCells removes n cells at the cursor, shifting the rest of the
// line left and filling the vacated cells at the end with blanks. DCH.
func (s *Screen) deleteCells(n int) {
	buf := s.activeBuffer()
	blank := s.blankCell()
	for x := s.cx; x < s.cols-n; x++ {
		buf.Set(x, s.cy, buf.At(x+n, s.cy))
	}
	start := max(s.cols-n, s.cx)
	for x := start; x < s.cols; x++ {
		buf.Set(x, s.cy, blank)
	}
}

// eraseCellsAtCursor blanks n cells at the cursor in place, without
// shifting anything. ECH.
func (s *Screen) eraseCellsAtCursor(n int) {
	buf := s.activeBuffer()
	blank := s.blankCell()
	for x := s.cx; x < s.cx+n && x < s.cols; x++ {
		buf.Set(x, s.cy, blank)
	}
}

// translateChar applies DEC Special Graphics substitution if the
// currently invoked G-set (GL, via SI/SO) is charsetDECSpecialGraphics.
func (s *Screen) translateChar(r rune) rune {
	active := s.g0
	if s.glActive == 1 {
		active = s.g1
	}
	if active == charsetDECSpecialGraphics {
		if mapped, ok := decSpecialGraphics[r]; ok {
			return mapped
		}
	}
	return r
}

// print writes r at the cursor with the current style, applying
// deferred autowrap: a character that exactly fills the last column
// doesn't wrap immediately — the cursor stays put with a pending-wrap
// flag set, and the actual wrap happens only when the next character
// is printed (or the cursor otherwise moves). This is standard VT100
// behavior and matters a lot in practice: without it, printing a full-
// width line followed by a newline would produce a spurious blank line.
func (s *Screen) print(r rune) {
	r = s.translateChar(r)

	w := wcwidth.RuneWidth(r)
	if w <= 0 {
		w = 1 // combining marks: same documented simplification as cell.Painter (M2)
	}

	if s.wrapPending {
		s.cx = 0
		s.index()
		s.wrapPending = false
	}

	if s.cx+w > s.cols {
		if !s.autowrap {
			return
		}
		s.cx = 0
		s.index()
	}

	buf := s.activeBuffer()
	if s.insertMode {
		s.insertBlanks(w)
	}

	style := s.curStyle
	style.Hyperlink = s.currentHyperlink

	buf.Set(s.cx, s.cy, cell.Cell{Rune: r, Style: style, Width: uint8(w)})
	if w == 2 {
		buf.Set(s.cx+1, s.cy, cell.Cell{Style: style, Width: 0})
	}
	s.cx += w

	if s.cx >= s.cols {
		// wrapPending only means anything when autowrap is on; with it
		// off, the cursor should just stay pinned at the last column
		// and keep overwriting it, never advancing past or wrapping.
		s.wrapPending = s.autowrap
		s.cx = s.cols - 1
	}
}

// tab moves the cursor forward to the next tab stop (HT), or the last
// column if there isn't one.
func (s *Screen) tab() {
	s.wrapPending = false
	for x := s.cx + 1; x < s.cols; x++ {
		if s.tabStops[x] {
			s.cx = x
			return
		}
	}
	s.cx = s.cols - 1
}

// backTab moves the cursor backward to the previous tab stop (CBT), or
// column 0 if there isn't one.
func (s *Screen) backTab() {
	s.wrapPending = false
	for x := s.cx - 1; x >= 0; x-- {
		if s.tabStops[x] {
			s.cx = x
			return
		}
	}
	s.cx = 0
}

func (s *Screen) clearTabStops(mode int) {
	switch mode {
	case 0:
		if s.cx < len(s.tabStops) {
			s.tabStops[s.cx] = false
		}
	case 3:
		for i := range s.tabStops {
			s.tabStops[i] = false
		}
	}
}

// moveToColumn implements CHA: move to the given 1-based column,
// keeping the current row.
func (s *Screen) moveToColumn(col int) {
	s.wrapPending = false
	s.cx = clamp(col-1, 0, s.cols-1)
}

// moveToRow implements VPA: move to the given 1-based row (origin-
// mode-aware, like moveTo), keeping the current column.
func (s *Screen) moveToRow(row int) {
	s.wrapPending = false
	top, bottom := 0, s.rows-1
	if s.originMode {
		top, bottom = s.scrollTop, s.scrollBottom
	}
	s.cy = clamp(top+row-1, top, bottom)
}

func (s *Screen) setCursorShape(n int) {
	switch n {
	case 0, 1, 2:
		s.cursorShape = CursorBlock
	case 3, 4:
		s.cursorShape = CursorUnderline
	case 5, 6:
		s.cursorShape = CursorBar
	}
}

// reset implements RIS (ESC c): a full reset to the screen's initial
// state, as if freshly constructed — including dropping scrollback.
func (s *Screen) reset() {
	*s = *NewScreen(s.cols, s.rows)
}
