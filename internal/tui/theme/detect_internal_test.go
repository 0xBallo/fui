package theme

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func TestParseOSC11Reply(t *testing.T) {
	tests := []struct {
		name     string
		reply    string
		wantDark bool
		wantOK   bool
	}{
		{name: "dark reply BEL terminated", reply: "\x1b]11;rgb:0000/0000/0000\a", wantDark: true, wantOK: true},
		{name: "light reply BEL terminated", reply: "\x1b]11;rgb:ffff/ffff/ffff\a", wantDark: false, wantOK: true},
		{name: "ST terminated", reply: "\x1b]11;rgb:1111/1111/1111\x1b\\", wantDark: true, wantOK: true},
		{name: "8-bit channels", reply: "\x1b]11;rgb:ff/ff/ff\a", wantDark: false, wantOK: true},
		{name: "malformed, no rgb prefix", reply: "\x1b]11;garbage\a", wantOK: false},
		{name: "malformed, wrong component count", reply: "\x1b]11;rgb:0000/0000\a", wantOK: false},
		{name: "malformed hex", reply: "\x1b]11;rgb:zzzz/0000/0000\a", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dark, ok := parseOSC11Reply(tt.reply)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && dark != tt.wantDark {
				t.Errorf("dark = %v, want %v", dark, tt.wantDark)
			}
		})
	}
}

func TestColorFGBGDark(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantDark bool
		wantOK   bool
	}{
		{name: "dark background", value: "15;0", wantDark: true, wantOK: true},
		{name: "light background 7", value: "0;7", wantDark: false, wantOK: true},
		{name: "light background 15", value: "0;15", wantDark: false, wantOK: true},
		{name: "dark background other index", value: "0;8", wantDark: true, wantOK: true},
		{name: "missing semicolon", value: "0", wantOK: false},
		{name: "non-numeric", value: "fg;bg", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dark, ok := colorFGBGDark(tt.value)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && dark != tt.wantDark {
				t.Errorf("dark = %v, want %v", dark, tt.wantDark)
			}
		})
	}
}

func TestRelativeLuminance(t *testing.T) {
	if l := relativeLuminance(0, 0, 0); l != 0 {
		t.Errorf("relativeLuminance(black) = %v, want 0", l)
	}
	if l := relativeLuminance(1, 1, 1); l < 0.99 {
		t.Errorf("relativeLuminance(white) = %v, want ~1", l)
	}
}

// TestOSCDetectorNonTerminal checks that OSCDetector gives up immediately (ok=false) when In/Out
// aren't connected to a real terminal, as is the case for every automated test run.
func TestOSCDetectorNonTerminal(t *testing.T) {
	inR, inW := io.Pipe()
	defer func() { _ = inR.Close() }()
	defer func() { _ = inW.Close() }()
	var out bytes.Buffer

	det := OSCDetector{In: inR, Out: &out}
	_, ok := det.Dark(10 * time.Millisecond)
	if ok {
		t.Errorf("Dark() ok = true over a non-terminal pipe, want false")
	}
}
