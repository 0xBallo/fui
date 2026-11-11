package fuiproto

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
)

// DecodeJSONL decodes a UTF-8 JSONL stream into record/error pairs.
func DecodeJSONL(r io.Reader) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), MaxLineBytes)

		line := 0
		for scanner.Scan() {
			line++

			text := scanner.Text()
			if strings.TrimSpace(text) == "" {
				yieldErr(yield, line, fmt.Errorf("empty JSONL line"))
				return
			}

			var record Record
			if err := json.Unmarshal([]byte(text), &record); err != nil {
				yieldErr(yield, line, fmt.Errorf("decode record: %w", err))
				return
			}

			if record.Type == RecordTypeMeta {
				if err := validateMetaProtocol(record.Meta); err != nil {
					yieldErr(yield, line, err)
					return
				}
			}

			if !yield(record, nil) {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				yieldErr(yield, line+1, fmt.Errorf("record exceeds %d bytes", MaxLineBytes))
				return
			}
			yieldErr(yield, line+1, fmt.Errorf("scan JSONL stream: %w", err))
		}
	}
}

// JSONLEncoder writes deterministic JSONL records.
type JSONLEncoder struct {
	w io.Writer
}

// NewJSONLEncoder creates a new JSONL encoder.
func NewJSONLEncoder(w io.Writer) *JSONLEncoder {
	return &JSONLEncoder{w: w}
}

// Encode writes one JSON line for the given record.
func (e *JSONLEncoder) Encode(record Record) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	if _, err := e.w.Write(data); err != nil {
		return fmt.Errorf("write JSONL record: %w", err)
	}
	if _, err := e.w.Write([]byte{'\n'}); err != nil {
		return fmt.Errorf("write JSONL newline: %w", err)
	}

	return nil
}

// EncodeJSONL writes all records from the provided iterator.
func EncodeJSONL(w io.Writer, records iter.Seq[Record]) error {
	enc := NewJSONLEncoder(w)
	for record := range records {
		if err := enc.Encode(record); err != nil {
			return err
		}
	}

	return nil
}

// EncodeSliceJSONL writes a slice of records as JSONL.
func EncodeSliceJSONL(w io.Writer, records []Record) error {
	enc := NewJSONLEncoder(w)
	for _, record := range records {
		if err := enc.Encode(record); err != nil {
			return err
		}
	}
	return nil
}

func yieldErr(yield func(Record, error) bool, line int, err error) {
	yield(Record{}, fmt.Errorf("line %d: %w", line, err))
}

func validateMetaProtocol(meta *MetaRecord) error {
	if meta == nil {
		return fmt.Errorf("meta payload is required")
	}

	major, err := ParseProtocolMajor(meta.Protocol)
	if err != nil {
		return fmt.Errorf("invalid protocol: %w", err)
	}
	if major != 1 {
		return fmt.Errorf("unsupported protocol major %d", major)
	}

	return nil
}
