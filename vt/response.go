package vt

import "fmt"

// Response generation for the device-attribute and status-report
// queries real CLI apps (vim, htop, less) rely on to detect the
// terminal and behave correctly. Screen has no direct access to a
// writer (see TakeResponses), so these just queue bytes.

// da1 answers a DA1 query (CSI c / CSI 0 c) with a minimal, widely-
// recognized "VT100 with Advanced Video Option" response.
func (s *Screen) da1() {
	s.queueResponse("\x1b[?1;2c")
}

// da2 answers a DA2 query (CSI > c) claiming to be a basic terminal
// (type 0) at firmware version 100.
func (s *Screen) da2() {
	s.queueResponse("\x1b[>0;100;0c")
}

// dsr answers a DSR query (CSI n): 5 ("are you OK?") and 6 (cursor
// position report, CPR).
func (s *Screen) dsr(params CSIParams) {
	switch params.Get(0, 0) {
	case 5:
		s.queueResponse("\x1b[0n")
	case 6:
		s.queueResponse(fmt.Sprintf("\x1b[%d;%dR", s.cy+1, s.cx+1))
	}
}

func (s *Screen) queueResponse(r string) {
	s.responses = append(s.responses, r...)
}
