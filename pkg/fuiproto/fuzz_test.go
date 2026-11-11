package fuiproto

import (
	"strings"
	"testing"
)

func FuzzDecodeJSONL(f *testing.F) {
	f.Add([]byte(`{"type":"meta","protocol":"fui/1","connector":"x","version":"0.1.0","source":{"file":"a","sha256":"b"}}` + "\n"))
	f.Add([]byte(`{"type":"transaction","account":"a","id":"id","date":"2026-10-01","amount":1,"currency":"EUR","description":"x","status":"booked"}` + "\n"))
	f.Add([]byte("not-json\n"))
	f.Add([]byte(strings.Repeat("a", MaxLineBytes+10)))

	f.Fuzz(func(_ *testing.T, input []byte) {
		for _, err := range DecodeJSONL(strings.NewReader(string(input))) {
			if err != nil {
				return
			}
		}
	})
}
