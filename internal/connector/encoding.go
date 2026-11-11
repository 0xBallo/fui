package connector

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// cp1252High maps the bytes 0x80–0x9F of Windows-1252 to Unicode; the rest of the code page
// coincides with Latin-1.
var cp1252High = [32]rune{
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F,
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178,
}

// toUTF8 converts data from the named encoding (UTF-8 when empty) to UTF-8, dropping a BOM.
func toUTF8(data []byte, encoding string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "utf-8", "utf8":
		data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
		if !utf8.Valid(data) {
			return "", fmt.Errorf("input is not valid UTF-8: set the profile encoding (latin-1 or windows-1252)")
		}
		return string(data), nil
	case "latin-1", "latin1", "iso-8859-1":
		return decodeSingleByte(data, false), nil
	case "windows-1252", "cp1252":
		return decodeSingleByte(data, true), nil
	}
	return "", fmt.Errorf("unsupported encoding %q: use utf-8, latin-1 or windows-1252", encoding)
}

func decodeSingleByte(data []byte, cp1252 bool) string {
	var sb strings.Builder
	sb.Grow(len(data))
	for _, b := range data {
		r := rune(b)
		if cp1252 && b >= 0x80 && b <= 0x9F {
			r = cp1252High[b-0x80]
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
