package theme

import (
	"bufio"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
)

// Detector answers whether the terminal's background is dark, used to resolve the "auto" and
// "high-contrast" themes (see "Rilevamento dello sfondo" in the technical spec). Implementations
// query the terminal directly; tests use a fake to simulate a light answer, a dark answer, or no
// answer within the timeout.
type Detector interface {
	// Dark attempts to determine the background color within timeout. ok is false if the
	// terminal didn't answer in time (or at all), in which case dark is meaningless and the
	// caller falls back to COLORFGBG, then to assuming a dark background.
	Dark(timeout time.Duration) (dark, ok bool)
}

// OSCDetector queries the real terminal's background color with the OSC 11 escape sequence,
// writing the query to Out and reading the reply from In. A zero OSCDetector uses os.Stdin and
// os.Stdout.
type OSCDetector struct {
	In  io.Reader
	Out io.Writer
}

// Dark implements Detector by sending "\x1b]11;?\x07" and parsing the terminal's
// "\x1b]11;rgb:RRRR/GGGG/BBBB\x07" (or ST-terminated) reply. It reports ok=false if In/Out aren't
// backed by a real terminal, or if no reply arrives within timeout.
func (d OSCDetector) Dark(timeout time.Duration) (dark, ok bool) {
	in, out := d.In, d.Out
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}

	inFile, isInFile := in.(*os.File)
	outFile, isOutFile := out.(*os.File)
	if !isInFile || !isOutFile || !term.IsTerminal(inFile.Fd()) || !term.IsTerminal(outFile.Fd()) {
		return false, false
	}

	state, err := term.MakeRaw(inFile.Fd())
	if err != nil {
		return false, false
	}
	defer func() { _ = term.Restore(inFile.Fd(), state) }()

	if _, err := out.Write([]byte("\x1b]11;?\x07")); err != nil {
		return false, false
	}

	type reply struct {
		line string
		err  error
	}
	ch := make(chan reply, 1)
	go func() {
		r := bufio.NewReader(in)
		line, err := r.ReadString('\a')
		ch <- reply{line, err}
	}()

	select {
	case rep := <-ch:
		if rep.err != nil {
			return false, false
		}
		return parseOSC11Reply(rep.line)
	case <-time.After(timeout):
		return false, false
	}
}

// parseOSC11Reply extracts the RGB components from an OSC 11 reply and reports whether the
// resulting color is dark (relative luminance below 0.5, matching termenv's own heuristic).
func parseOSC11Reply(reply string) (dark, ok bool) {
	idx := strings.Index(reply, "rgb:")
	if idx < 0 {
		return false, false
	}
	body := strings.TrimRight(reply[idx+len("rgb:"):], "\a\x1b\\")
	parts := strings.Split(body, "/")
	if len(parts) != 3 {
		return false, false
	}
	var rgb [3]float64
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 32)
		if err != nil {
			return false, false
		}
		maxVal := uint64(1)<<(4*len(p)) - 1
		rgb[i] = float64(v) / float64(maxVal)
	}
	return relativeLuminance(rgb[0], rgb[1], rgb[2]) < 0.5, true
}

// relativeLuminance computes the WCAG relative luminance of a color given its sRGB components in
// [0,1].
func relativeLuminance(r, g, b float64) float64 {
	lin := func(c float64) float64 {
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// colorFGBGDark parses the COLORFGBG environment variable (format "fg;bg", e.g. "15;0") and
// reports whether the background index indicates a dark background. Indexes 0-6 and 8 are the
// conventional dark ANSI slots; 7, 9-15 are light, matching the heuristic vim and tmux use.
func colorFGBGDark(v string) (dark, ok bool) {
	parts := strings.Split(v, ";")
	if len(parts) < 2 {
		return false, false
	}
	bg, err := strconv.Atoi(strings.TrimSpace(parts[len(parts)-1]))
	if err != nil {
		return false, false
	}
	switch bg {
	case 7, 15:
		return false, true
	default:
		return true, true
	}
}
