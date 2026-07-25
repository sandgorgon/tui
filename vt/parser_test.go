package vt

import (
	"reflect"
	"testing"
	"unicode/utf8"
)

type call struct {
	kind          string
	r             rune
	b             byte
	private       byte
	params        []int
	groups        [][]int
	intermediates []byte
	final         byte
	data          []byte
}

type recorder struct {
	calls []call
}

func (r *recorder) Print(ru rune) { r.calls = append(r.calls, call{kind: "print", r: ru}) }
func (r *recorder) Execute(b byte) {
	r.calls = append(r.calls, call{kind: "execute", b: b})
}
func (r *recorder) CSI(private byte, params CSIParams, intermediates []byte, final byte) {
	r.calls = append(r.calls, call{
		kind: "csi", private: private, params: params.Ints(), groups: params.Groups(),
		intermediates: append([]byte(nil), intermediates...), final: final,
	})
}
func (r *recorder) ESC(intermediates []byte, final byte) {
	r.calls = append(r.calls, call{kind: "esc", intermediates: append([]byte(nil), intermediates...), final: final})
}
func (r *recorder) OSC(data []byte) {
	r.calls = append(r.calls, call{kind: "osc", data: append([]byte(nil), data...)})
}

func feed(chunks ...string) []call {
	p := NewParser()
	rec := &recorder{}
	for _, c := range chunks {
		p.Feed([]byte(c), rec)
	}
	return rec.calls
}

func TestPrintASCII(t *testing.T) {
	got := feed("abc")
	want := []call{{kind: "print", r: 'a'}, {kind: "print", r: 'b'}, {kind: "print", r: 'c'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestExecuteC0(t *testing.T) {
	got := feed("\x07\x08")
	want := []call{{kind: "execute", b: 0x07}, {kind: "execute", b: 0x08}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUTF8SingleFeed(t *testing.T) {
	got := feed("é")
	want := []call{{kind: "print", r: 'é'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUTF8SplitAcrossFeedCalls(t *testing.T) {
	s := "中"
	if len(s) != 3 {
		t.Fatalf("test assumes a 3-byte rune, got %d bytes", len(s))
	}
	got := feed(s[:1], s[1:])
	want := []call{{kind: "print", r: '中'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUTF8InvalidLoneContinuationByte(t *testing.T) {
	got := feed(string([]byte{0x80}))
	want := []call{{kind: "print", r: utf8.RuneError}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUTF8InvalidLeadFollowedByASCII(t *testing.T) {
	got := feed(string([]byte{0xE0, 'A'}))
	want := []call{{kind: "print", r: utf8.RuneError}, {kind: "print", r: 'A'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUTF8InterruptedBySequence(t *testing.T) {
	// 0xE0 starts a 3-byte sequence, then ESC arrives before it
	// completes: the partial sequence should flush as a replacement
	// char, and the ESC sequence should still parse normally.
	got := feed(string([]byte{0xE0}) + "\x1bD")
	want := []call{{kind: "print", r: utf8.RuneError}, {kind: "esc", final: 'D'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCSISimple(t *testing.T) {
	got := feed("\x1b[1;5A")
	want := []call{{kind: "csi", params: []int{1, 5}, groups: [][]int{{1}, {5}}, final: 'A'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCSINoParams(t *testing.T) {
	got := feed("\x1b[A")
	want := []call{{kind: "csi", final: 'A'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCSIPrivateMarker(t *testing.T) {
	got := feed("\x1b[?25h")
	want := []call{{kind: "csi", private: '?', params: []int{25}, groups: [][]int{{25}}, final: 'h'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCSIColonSubparams(t *testing.T) {
	got := feed("\x1b[38:2:255:0:0m")
	want := []call{{
		kind:   "csi",
		params: []int{38}, // Ints() takes only the first sub-value
		groups: [][]int{{38, 2, 255, 0, 0}},
		final:  'm',
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestESCSimple(t *testing.T) {
	got := feed("\x1bD")
	want := []call{{kind: "esc", final: 'D'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestESCWithIntermediate(t *testing.T) {
	got := feed("\x1b(0")
	want := []call{{kind: "esc", intermediates: []byte("("), final: '0'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOSCTerminatedByST(t *testing.T) {
	got := feed("\x1b]0;title\x1b\\")
	want := []call{{kind: "osc", data: []byte("0;title")}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOSCTerminatedByBEL(t *testing.T) {
	got := feed("\x1b]0;title\x07")
	want := []call{{kind: "osc", data: []byte("0;title")}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOSCSplitAcrossFeedCalls(t *testing.T) {
	got := feed("\x1b]0;par", "t2\x07")
	want := []call{{kind: "osc", data: []byte("0;part2")}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDCSRecognizedButIgnoredAndResyncs(t *testing.T) {
	got := feed("\x1bP+q436f6c6f72\x1b\\A")
	want := []call{{kind: "print", r: 'A'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v (DCS payload should produce no calls, and parsing should resync)", got, want)
	}
}

func TestCANAbortsCSISequence(t *testing.T) {
	got := feed("\x1b[1;2\x18A")
	want := []call{{kind: "print", r: 'A'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v (CAN should abort the CSI without dispatching it)", got, want)
	}
}

func TestCSIIgnoreOnInvalidByteThenResyncs(t *testing.T) {
	got := feed("\x1b[1\x7f5A") // DEL mid-params is invalid -> CsiIgnore until a final byte
	if len(got) != 0 {
		t.Errorf("got %+v, want no calls for the malformed sequence", got)
	}

	// Confirm the parser resynced cleanly afterward.
	got2 := feed("\x1b[1\x7f5AX")
	want2 := []call{{kind: "print", r: 'X'}}
	if !reflect.DeepEqual(got2, want2) {
		t.Errorf("got %+v, want %+v", got2, want2)
	}
}

func TestStrayESCDuringOSCAbandonsStringAndReparsesAsNewSequence(t *testing.T) {
	// Mid-OSC-string, an ESC arrives that's NOT followed by '\': the OSC
	// must be abandoned (no dispatch) and the ESC treated as the start
	// of a fresh sequence, correctly parsing the CSI that follows it.
	got := feed("\x1b]0;abc\x1b[1mX")
	want := []call{
		{kind: "csi", params: []int{1}, groups: [][]int{{1}}, final: 'm'},
		{kind: "print", r: 'X'},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestIgnoreStringSOSPMAPCResyncs(t *testing.T) {
	got := feed("\x1b^ignored pm string\x1b\\A") // PM (ESC ^)
	want := []call{{kind: "print", r: 'A'}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
