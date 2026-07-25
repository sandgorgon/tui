package vt

import "bytes"

// osc dispatches a parsed OSC string's "Ps;Pt" payload: 0/1/2 (window/
// icon title) and 8 (hyperlink) per docs/DESIGN.md §7. Anything else is
// silently ignored.
func (s *Screen) osc(data []byte) {
	ps, pt, _ := bytes.Cut(data, []byte{';'})
	switch string(ps) {
	case "0", "1", "2":
		s.title = string(pt)
	case "8":
		s.oscHyperlink(pt)
	}
}

// oscHyperlink handles OSC 8's payload, "params;URI" (params is
// typically empty or "id=..." and isn't used here). An empty URI
// closes the currently open hyperlink, per the OSC 8 convention.
func (s *Screen) oscHyperlink(pt []byte) {
	_, uri, ok := bytes.Cut(pt, []byte{';'})
	if !ok {
		uri = pt
	}
	s.currentHyperlink = string(uri)
}
