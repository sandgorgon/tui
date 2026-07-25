// Package input decodes a raw terminal input byte stream into typed
// Events: keys (including CSI-u / kitty keyboard protocol and legacy
// xterm sequences), SGR mouse reports, bracketed paste, and focus
// in/out.
//
// See docs/DESIGN.md §4 for the design.
package input
