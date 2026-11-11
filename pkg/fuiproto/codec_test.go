package fuiproto

import (
	"bytes"
	"errors"
	"fmt"
	"iter"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDecodeJSONLUnknownFieldsAreIgnored(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"meta","protocol":"fui/1","connector":"x","version":"0.1.0","source":{"file":"a.csv","sha256":"abc"},"x_unknown":"ignored"}`,
		`{"type":"end","transactions":0,"errors":0}`,
	}, "\n") + "\n"

	var records []Record
	for record, err := range DecodeJSONL(strings.NewReader(input)) {
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		records = append(records, record)
	}

	if len(records) != 2 {
		t.Fatalf("decoded records = %d, want 2", len(records))
	}
	if records[0].Type != RecordTypeMeta || records[1].Type != RecordTypeEnd {
		t.Fatalf("decoded types = [%s, %s], want [%s, %s]", records[0].Type, records[1].Type, RecordTypeMeta, RecordTypeEnd)
	}
}

func TestDecodeJSONLRejectsProtocolMajorMismatch(t *testing.T) {
	input := `{"type":"meta","protocol":"fui/2","connector":"x","version":"0.1.0","source":{"file":"a.csv","sha256":"abc"}}` + "\n"

	for _, err := range DecodeJSONL(strings.NewReader(input)) {
		if err == nil {
			t.Fatalf("expected decode error")
		}
		if !strings.Contains(err.Error(), "unsupported protocol major") {
			t.Fatalf("error = %v, want unsupported protocol major", err)
		}
		return
	}
	t.Fatalf("expected one yielded error")
}

func TestDecodeJSONLRejectsLongLines(t *testing.T) {
	longField := strings.Repeat("a", MaxLineBytes+1)
	input := fmt.Sprintf(`{"type":"meta","protocol":"fui/1","connector":"%s","version":"0.1.0","source":{"file":"a.csv","sha256":"abc"}}`+"\n", longField)

	for _, err := range DecodeJSONL(strings.NewReader(input)) {
		if err == nil {
			t.Fatalf("expected decode error")
		}
		if !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("error = %v, want line length error", err)
		}
		return
	}
	t.Fatalf("expected yielded error")
}

func TestEncodeJSONLDeterministicOutput(t *testing.T) {
	records := validStreamRecords()

	first := new(bytes.Buffer)
	second := new(bytes.Buffer)

	if err := EncodeSliceJSONL(first, records); err != nil {
		t.Fatalf("encode first: %v", err)
	}
	if err := EncodeSliceJSONL(second, records); err != nil {
		t.Fatalf("encode second: %v", err)
	}

	if diff := cmp.Diff(first.String(), second.String()); diff != "" {
		t.Fatalf("encoded output differs (-want +got):\n%s", diff)
	}
}

func TestEncodeJSONLFromIterator(t *testing.T) {
	records := validStreamRecords()
	seq := iter.Seq[Record](func(yield func(Record) bool) {
		for _, record := range records {
			if !yield(record) {
				return
			}
		}
	})

	var buf bytes.Buffer
	if err := EncodeJSONL(&buf, seq); err != nil {
		t.Fatalf("encode from iterator: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("expected non-empty output")
	}
}

func TestEncoderWriteErrors(t *testing.T) {
	encoder := NewJSONLEncoder(&failingWriter{})
	err := encoder.Encode(validStreamRecords()[0])
	if err == nil {
		t.Fatalf("expected write error")
	}
	if !strings.Contains(err.Error(), "write JSONL record") {
		t.Fatalf("error = %v, want record write error", err)
	}

	encoder = NewJSONLEncoder(&recordOnlyWriter{})
	err = encoder.Encode(validStreamRecords()[0])
	if err == nil {
		t.Fatalf("expected newline write error")
	}
	if !strings.Contains(err.Error(), "write JSONL newline") {
		t.Fatalf("error = %v, want newline write error", err)
	}
}

type failingWriter struct{}

func (f *failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("boom")
}

type recordOnlyWriter struct {
	writes int
}

func (w *recordOnlyWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes == 1 {
		return len(data), nil
	}
	return 0, errors.New("no newline")
}
