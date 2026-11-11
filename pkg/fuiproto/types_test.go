package fuiproto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseProtocolMajor(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		want     int
		wantErr  bool
	}{
		{name: "valid", protocol: "fui/1", want: 1},
		{name: "invalid namespace", protocol: "foo/1", wantErr: true},
		{name: "invalid shape", protocol: "fui", wantErr: true},
		{name: "invalid major", protocol: "fui/a", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseProtocolMajor(tc.protocol)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("major = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRecordMarshalJSONErrors(t *testing.T) {
	tests := []struct {
		name    string
		record  Record
		wantSub string
	}{
		{name: "missing meta payload", record: Record{Type: RecordTypeMeta}, wantSub: "meta payload"},
		{name: "missing account payload", record: Record{Type: RecordTypeAccount}, wantSub: "account payload"},
		{name: "missing transaction payload", record: Record{Type: RecordTypeTransaction}, wantSub: "transaction payload"},
		{name: "missing balance payload", record: Record{Type: RecordTypeBalance}, wantSub: "balance payload"},
		{name: "missing schedule payload", record: Record{Type: RecordTypeSchedule}, wantSub: "schedule payload"},
		{name: "missing installment payload", record: Record{Type: RecordTypeInstallment}, wantSub: "installment payload"},
		{name: "missing error payload", record: Record{Type: RecordTypeError}, wantSub: "error payload"},
		{name: "missing end payload", record: Record{Type: RecordTypeEnd}, wantSub: "end payload"},
		{name: "unknown type", record: Record{Type: RecordType("unknown")}, wantSub: "unknown record type"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := json.Marshal(tc.record)
			if err == nil {
				t.Fatalf("expected marshal error")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

func TestRecordUnmarshalJSONErrors(t *testing.T) {
	var record Record

	if err := json.Unmarshal([]byte("{"), &record); err == nil {
		t.Fatalf("expected malformed json error")
	}

	if err := json.Unmarshal([]byte(`{"type":"unknown"}`), &record); err == nil {
		t.Fatalf("expected unknown type error")
	}
}
