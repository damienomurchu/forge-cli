package output

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestEscapeText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "printable ASCII", input: `Forge "capture"`, want: `Forge "capture"`},
		{name: "printable Unicode", input: "café 東京 🙂", want: "café 東京 🙂"},
		{name: "replacement rune", input: "valid � rune", want: "valid � rune"},
		{name: "backslash", input: `literal\n newline`, want: `literal\\n newline`},
		{name: "named whitespace controls", input: "line\ncolumn\treturn\r", want: `line\ncolumn\treturn\r`},
		{name: "C0 controls", input: "\x00\x07\x1b", want: `\x00\x07\x1b`},
		{name: "DEL and C1 controls", input: "\x7f\u0085\u009f", want: `\x7f\x85\x9f`},
		{name: "bidirectional controls", input: "a\u061c\u200e\u202e\u2066\u2069b", want: `a\u061c\u200e\u202e\u2066\u2069b`},
		{name: "other non-graphic format", input: "a\u200db", want: `a\u200db`},
		{name: "non-BMP non-graphic", input: "a\U000e0001b", want: `a\U000e0001b`},
		{name: "invalid UTF-8", input: string([]byte{'a', 0xff, 0xc0, 'b'}), want: `a\xff\xc0b`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeText(tt.input); got != tt.want {
				t.Errorf("EscapeText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapeTextRemovesTerminalAndBidirectionalControls(t *testing.T) {
	var unsafe strings.Builder
	for r := rune(0); r <= 0x9f; r++ {
		if unicode.IsControl(r) {
			unsafe.WriteRune(r)
		}
	}
	for _, r := range []rune{
		'\u061c', '\u200e', '\u200f',
		'\u202a', '\u202b', '\u202c', '\u202d', '\u202e',
		'\u2066', '\u2067', '\u2068', '\u2069',
	} {
		unsafe.WriteRune(r)
	}

	got := EscapeText(unsafe.String())
	if !utf8.ValidString(got) {
		t.Fatal("EscapeText() returned invalid UTF-8")
	}
	for _, r := range got {
		if unicode.IsControl(r) {
			t.Errorf("EscapeText() retained control rune U+%04X", r)
		}
		if unicode.Is(unicode.Bidi_Control, r) {
			t.Errorf("EscapeText() retained bidirectional control U+%04X", r)
		}
	}
}
