package vt

import (
	"bytes"
	"strconv"
	"unicode/utf8"
)

// Handler receives the semantic actions decoded from a byte stream by
// Parser: printable runes, C0 control execution, and fully-parsed
// ESC/CSI/OSC sequences. Screen implements Handler; Parser itself has
// no terminal semantics at all, only the byte-level parser state
// machine — keeping "how do we recognize a sequence" cleanly separate
// from "what does this sequence mean" (see docs/DESIGN.md §7).
//
// The CSIParams and intermediates/data slices passed to Handler methods
// are only valid for the duration of that call — Parser reuses their
// backing arrays for the next sequence. Implementations that need to
// retain the data must copy it.
type Handler interface {
	Print(r rune)
	Execute(b byte) // a C0 control character
	CSI(private byte, params CSIParams, intermediates []byte, final byte)
	ESC(intermediates []byte, final byte)
	OSC(data []byte)
}

type state uint8

const (
	stateGround state = iota
	stateEscape
	stateEscapeIntermediate
	stateCSI
	stateCSIIgnore
	stateOSC
	stateDCS          // recognized structurally; payload discarded, see docs/DESIGN.md §7
	stateIgnoreString // SOS/PM/APC: recognized structurally, payload discarded
)

// Parser is the DEC ANSI/VT500 parser state machine: it turns a raw
// byte stream into calls on a Handler. It deliberately simplifies a
// few corners of the classic (strict, 8-bit) Paul Williams state
// table where doing so costs nothing for any real, valid input stream:
//
//   - Bytes >= 0x80 are treated as UTF-8 (this parser is UTF-8-native,
//     not 8-bit-C1-control-native — matching how real terminal
//     emulators actually behave today).
//   - CSI's Entry/Param/Intermediate states are merged into one Csi
//     state that routes each byte to a params or intermediates buffer
//     by range, without enforcing the strict ordering rules real
//     programs never violate in practice (a genuinely malformed
//     sequence just gets dispatched with whatever was collected,
//     rather than precisely detected and discarded).
//   - DCS and SOS/PM/APC strings are recognized (so they don't confuse
//     the rest of the parser and the following stream resyncs
//     correctly at their terminator) but never interpreted — no known
//     real-world need covered by this project's scope (bash/zsh/vim/
//     htop/less/tmux) requires acting on their payload.
type Parser struct {
	state state

	params        []byte
	intermediates []byte
	oscBuf        []byte

	utf8Buf []byte

	pendingST bool // saw ESC while collecting a string; waiting to see if '\' follows (confirming ST)
}

// NewParser returns a ready-to-use Parser in the ground state.
func NewParser() *Parser {
	return &Parser{}
}

// Feed decodes data, calling h for each recognized action. Feed may be
// called repeatedly with successive chunks of a stream (e.g. from
// separate pty reads); a sequence — including a UTF-8 rune or an
// escape sequence — split across two calls is handled correctly.
func (p *Parser) Feed(data []byte, h Handler) {
	for _, b := range data {
		p.feedByte(b, h)
	}
}

func (p *Parser) feedByte(b byte, h Handler) {
	if p.state == stateGround {
		p.feedGround(b, h)
		return
	}
	p.feedNonGround(b, h)
}

func (p *Parser) feedGround(b byte, h Handler) {
	switch {
	case b == 0x1B:
		p.flushUTF8Error(h)
		p.enterEscape()
	case b < 0x20:
		p.flushUTF8Error(h)
		h.Execute(b)
	case b == 0x7F:
		p.flushUTF8Error(h)
	case b < 0x80:
		p.flushUTF8Error(h)
		h.Print(rune(b))
	default:
		p.utf8Buf = append(p.utf8Buf, b)
		for utf8.FullRune(p.utf8Buf) {
			r, size := utf8.DecodeRune(p.utf8Buf)
			h.Print(r)
			p.utf8Buf = p.utf8Buf[size:]
		}
	}
}

// flushUTF8Error emits a replacement character for a UTF-8 sequence
// that was interrupted by a control byte or ESC before completing.
func (p *Parser) flushUTF8Error(h Handler) {
	if len(p.utf8Buf) > 0 {
		h.Print(utf8.RuneError)
		p.utf8Buf = p.utf8Buf[:0]
	}
}

func (p *Parser) enterEscape() {
	p.state = stateEscape
	p.params = p.params[:0]
	p.intermediates = p.intermediates[:0]
}

func (p *Parser) feedNonGround(b byte, h Handler) {
	switch p.state {
	case stateEscape:
		p.feedEscape(b, h)
	case stateEscapeIntermediate:
		p.feedEscapeIntermediate(b, h)
	case stateCSI:
		p.feedCSI(b, h)
	case stateCSIIgnore:
		p.feedCSIIgnore(b)
	case stateOSC:
		p.feedString(b, h, stringOSC)
	case stateDCS:
		p.feedString(b, h, stringDCS)
	case stateIgnoreString:
		p.feedString(b, h, stringIgnore)
	}
}

func (p *Parser) feedEscape(b byte, h Handler) {
	switch {
	case b < 0x20:
		h.Execute(b)
	case b == '[':
		p.state = stateCSI
		p.params = p.params[:0]
		p.intermediates = p.intermediates[:0]
	case b == ']':
		p.state = stateOSC
		p.oscBuf = p.oscBuf[:0]
	case b == 'P':
		p.state = stateDCS
	case b == 'X' || b == '^' || b == '_':
		p.state = stateIgnoreString
	case b >= 0x20 && b <= 0x2F:
		p.intermediates = append(p.intermediates, b)
		p.state = stateEscapeIntermediate
	case b >= 0x30 && b <= 0x7E:
		h.ESC(p.intermediates, b)
		p.state = stateGround
	default:
		p.state = stateGround
	}
}

func (p *Parser) feedEscapeIntermediate(b byte, h Handler) {
	switch {
	case b < 0x20:
		h.Execute(b)
	case b >= 0x20 && b <= 0x2F:
		p.intermediates = append(p.intermediates, b)
	case b >= 0x30 && b <= 0x7E:
		h.ESC(p.intermediates, b)
		p.state = stateGround
	default:
		p.state = stateGround
	}
}

func (p *Parser) feedCSI(b byte, h Handler) {
	switch {
	case b == 0x1B:
		p.enterEscape()
	case b == 0x18 || b == 0x1A:
		p.state = stateGround
	case b < 0x20:
		h.Execute(b)
	case b >= 0x30 && b <= 0x3F:
		p.params = append(p.params, b)
	case b >= 0x20 && b <= 0x2F:
		p.intermediates = append(p.intermediates, b)
	case b >= 0x40 && b <= 0x7E:
		p.dispatchCSI(h, b)
		p.state = stateGround
	default:
		p.state = stateCSIIgnore
	}
}

func (p *Parser) feedCSIIgnore(b byte) {
	switch {
	case b == 0x18 || b == 0x1A || b == 0x1B:
		p.state = stateGround
	case b >= 0x40 && b <= 0x7E:
		p.state = stateGround
	}
}

func (p *Parser) dispatchCSI(h Handler, final byte) {
	private := byte(0)
	raw := p.params
	if len(raw) > 0 && isPrivateMarker(raw[0]) {
		private = raw[0]
		raw = raw[1:]
	}
	h.CSI(private, CSIParams{raw: raw}, p.intermediates, final)
}

func isPrivateMarker(b byte) bool {
	return b == '?' || b == '<' || b == '=' || b == '>'
}

type stringKind uint8

const (
	stringOSC stringKind = iota
	stringDCS
	stringIgnore
)

// feedString collects bytes of an OSC/DCS/SOS-PM-APC string until its
// terminator: ST (ESC \), or BEL (a common xterm-originated extension
// this parser accepts as an alternate terminator for all string kinds,
// not just OSC, for leniency).
func (p *Parser) feedString(b byte, h Handler, kind stringKind) {
	if p.pendingST {
		p.pendingST = false
		if b == '\\' {
			p.terminateString(h, kind)
			p.state = stateGround
			return
		}
		// Not a valid ST after all: the ESC we saw starts a fresh
		// sequence instead, abandoning this string without dispatch.
		p.state = stateGround
		p.enterEscape()
		p.feedEscape(b, h)
		return
	}

	switch b {
	case 0x1B:
		p.pendingST = true
	case 0x07:
		p.terminateString(h, kind)
		p.state = stateGround
	case 0x18, 0x1A:
		p.state = stateGround
	default:
		if kind == stringOSC {
			p.oscBuf = append(p.oscBuf, b)
		}
	}
}

func (p *Parser) terminateString(h Handler, kind stringKind) {
	if kind == stringOSC {
		h.OSC(p.oscBuf)
	}
}

// CSIParams is a parsed CSI parameter list: semicolon-separated
// fields, each optionally colon-subdivided (used by SGR's extended
// color forms, e.g. "38:2:255:0:0"). Most CSI sequences never use
// colon sub-parameters; Ints ignores them (taking each field's first
// sub-value) for the common case, while Groups exposes the full
// structure for SGR's color parsing. The zero value is an empty
// parameter list.
type CSIParams struct {
	raw []byte
}

// Groups returns the full colon-subdivided parameter structure, or nil
// if no parameters were present at all.
func (p CSIParams) Groups() [][]int {
	if len(p.raw) == 0 {
		return nil
	}
	fields := bytes.Split(p.raw, []byte{';'})
	groups := make([][]int, len(fields))
	for i, field := range fields {
		subs := bytes.Split(field, []byte{':'})
		group := make([]int, len(subs))
		for j, sub := range subs {
			n, _ := strconv.Atoi(string(sub)) // empty/invalid -> 0, the ECMA-48 "omitted parameter" convention
			group[j] = n
		}
		groups[i] = group
	}
	return groups
}

// Ints returns each field's first value (ignoring any colon
// sub-values), or nil if no parameters were present at all.
func (p CSIParams) Ints() []int {
	groups := p.Groups()
	if groups == nil {
		return nil
	}
	out := make([]int, len(groups))
	for i, g := range groups {
		out[i] = g[0]
	}
	return out
}

// Len returns the number of semicolon-separated fields.
func (p CSIParams) Len() int { return len(p.Groups()) }

// Get returns the i'th field's first value, or def if there aren't
// that many fields (including the case of no parameters at all).
func (p CSIParams) Get(i, def int) int {
	ints := p.Ints()
	if i < len(ints) {
		return ints[i]
	}
	return def
}
