package connector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// maxFileBytes bounds the input files read in memory: connector input is not trusted.
const maxFileBytes = 64 << 20

// FileSource reads a fui/1 stream from a .fui.jsonl file. When the meta record names no
// source, it is filled with the file name and its sha256, so that importing the same file twice
// is recognised.
type FileSource struct {
	Path string
}

var _ importing.Source = FileSource{}

// Records implements importing.Source.
func (f FileSource) Records(ctx context.Context) iter.Seq2[fuiproto.Record, error] {
	return func(yield func(fuiproto.Record, error) bool) {
		sum, err := fileSHA256(f.Path)
		if err != nil {
			yield(fuiproto.Record{}, err)
			return
		}
		fh, err := os.Open(f.Path)
		if err != nil {
			yield(fuiproto.Record{}, fmt.Errorf("open %s: %w", f.Path, err))
			return
		}
		defer fh.Close() //nolint:errcheck // read-only file
		for rec, err := range fuiproto.DecodeJSONL(fh) {
			if err == nil {
				if cerr := ctx.Err(); cerr != nil {
					yield(fuiproto.Record{}, cerr)
					return
				}
				if rec.Meta != nil && rec.Meta.Source == (fuiproto.SourceMeta{}) {
					rec.Meta.Source = fuiproto.SourceMeta{File: filepath.Base(f.Path), SHA256: sum}
				}
			}
			if !yield(rec, err) {
				return
			}
		}
	}
}

func fileSHA256(path string) (string, error) {
	fh, err := os.Open(path) //nolint:gosec // the path is chosen by the user on the command line
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer fh.Close() //nolint:errcheck // read-only file
	h := sha256.New()
	if _, err := io.Copy(h, fh); err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// readCapped reads a whole file, failing when it exceeds maxFileBytes.
func readCapped(path string) ([]byte, error) {
	fh, err := os.Open(path) //nolint:gosec // the path is chosen by the user on the command line
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer fh.Close() //nolint:errcheck // read-only file
	data, err := io.ReadAll(io.LimitReader(fh, maxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) > maxFileBytes {
		return nil, fmt.Errorf("%s is larger than %d MiB", path, maxFileBytes>>20)
	}
	return data, nil
}
