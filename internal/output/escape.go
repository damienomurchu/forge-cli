package output

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const hexadecimalDigits = "0123456789abcdef"

// EscapeText makes untrusted text safe and unambiguous for human terminal
// output while preserving ordinary printable Unicode.
func EscapeText(value string) string {
	var escaped strings.Builder
	escaped.Grow(len(value))

	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		if r == utf8.RuneError && size == 1 {
			writeHexEscape(&escaped, uint32(value[0]), 'x', 2)
			value = value[1:]
			continue
		}
		value = value[size:]

		switch r {
		case '\\':
			escaped.WriteString(`\\`)
		case '\n':
			escaped.WriteString(`\n`)
		case '\r':
			escaped.WriteString(`\r`)
		case '\t':
			escaped.WriteString(`\t`)
		default:
			switch {
			case unicode.Is(unicode.Bidi_Control, r):
				writeRuneEscape(&escaped, r)
			case unicode.IsControl(r) || !unicode.IsGraphic(r):
				writeRuneEscape(&escaped, r)
			default:
				escaped.WriteRune(r)
			}
		}
	}

	return escaped.String()
}

func writeRuneEscape(escaped *strings.Builder, r rune) {
	switch {
	case r <= 0xff:
		writeHexEscape(escaped, uint32(r), 'x', 2)
	case r <= 0xffff:
		writeHexEscape(escaped, uint32(r), 'u', 4)
	default:
		writeHexEscape(escaped, uint32(r), 'U', 8)
	}
}

func writeHexEscape(escaped *strings.Builder, value uint32, prefix byte, width int) {
	escaped.WriteByte('\\')
	escaped.WriteByte(prefix)
	for shift := (width - 1) * 4; shift >= 0; shift -= 4 {
		escaped.WriteByte(hexadecimalDigits[(value>>shift)&0xf])
	}
}
