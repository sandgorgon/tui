package term

import (
	"encoding/base64"
	"io"
)

// WriteClipboard writes text to the system clipboard via OSC 52 — an
// app-initiated "copy" that doesn't depend on the terminal's own
// native click-drag selection, which is unavailable whenever an app
// has mouse reporting enabled (the two are mutually exclusive by
// protocol; see docs/DESIGN.md §9). Supported by most modern terminal
// emulators (iTerm2, kitty, Alacritty, WezTerm, Windows Terminal,
// tmux/screen with clipboard passthrough configured, ...); on a
// terminal that doesn't support it, the sequence is simply ignored —
// there's no reliable way to detect support in advance (it isn't
// covered by DA1/DA2 device-attribute responses), so this always
// attempts the write rather than trying to probe first.
//
// The BEL terminator (not ST, "\x1b\\") is used for consistency with
// this project's other OSC sequence (OSC 8 hyperlinks — see
// render.appendHyperlink) — both forms are equally valid per ECMA-48,
// terminals that support OSC 52 accept either.
func WriteClipboard(w io.Writer, text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	seq := make([]byte, 0, len(encoded)+16)
	seq = append(seq, "\x1b]52;c;"...)
	seq = append(seq, encoded...)
	seq = append(seq, '\x07')
	_, err := w.Write(seq)
	return err
}
