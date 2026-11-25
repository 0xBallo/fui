package importing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/domain"
)

type fakeDetector struct {
	sug importing.CSVSuggestion
	err error
	got string
}

func (f *fakeDetector) DetectCSV(_ context.Context, file string) (importing.CSVSuggestion, error) {
	f.got = file
	return f.sug, f.err
}

func TestServiceDetectCSV(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name     string
		detector *fakeDetector
		file     string
		wantErr  error
	}{
		{name: "ok", detector: &fakeDetector{sug: importing.CSVSuggestion{DateFormat: "DD/MM/YYYY"}}, file: "a.csv"},
		{name: "detector error", detector: &fakeDetector{err: boom}, file: "a.csv", wantErr: boom},
		{name: "no file", detector: &fakeDetector{}, wantErr: domain.ErrMissingField},
		{name: "no detector", file: "a.csv", wantErr: domain.ErrMissingField},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := importing.New(nil, nil, nil, nil)
			if tt.detector != nil {
				svc = svc.WithCSVDetector(tt.detector)
			}
			got, err := svc.DetectCSV(context.Background(), tt.file)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if diff := cmp.Diff(tt.detector.sug, got); diff != "" || tt.detector.got != tt.file {
				t.Errorf("got %+v (file %q), diff %s", got, tt.detector.got, diff)
			}
		})
	}
}
