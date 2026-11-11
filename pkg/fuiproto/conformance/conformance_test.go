package conformance

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	connectorPath := buildFakeConnector(t)

	input := filepath.Join("testdata", "input", "sample.csv")
	if _, err := os.Stat(input); err != nil {
		t.Fatalf("stat input fixture: %v", err)
	}

	Run(t, RunOptions{
		ConnectorPath: connectorPath,
		InputFiles:    []string{input},
		Config: map[string]any{
			"token": "ok",
		},
		BadConfig: map[string]any{
			"token": "",
		},
	})
}

func TestRunOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    RunOptions
		wantErr bool
	}{
		{name: "missing connector path", opts: RunOptions{InputFiles: []string{"a"}}, wantErr: true},
		{name: "missing input files", opts: RunOptions{ConnectorPath: "/bin/echo"}, wantErr: true},
		{name: "valid", opts: RunOptions{ConnectorPath: "/bin/echo", InputFiles: []string{"a"}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunCommandStartError(t *testing.T) {
	_, _, _, err := runCommand("/path/that/does/not/exist", []string{"describe"}, nil)
	if err == nil {
		t.Fatalf("expected start error")
	}
}

func TestRunCommandExitCode(t *testing.T) {
	stdout, stderr, code, err := runCommand("/bin/sh", []string{"-c", "echo out && echo err 1>&2 && exit 7"}, nil)
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}
	if code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}
	if string(stdout) == "" || string(stderr) == "" {
		t.Fatalf("expected stdout and stderr to be captured")
	}
}

func buildFakeConnector(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "fui-connector-fake")
	cmd := exec.Command("go", "build", "-o", binary, "./testdata/fakeconnector")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake connector: %v\n%s", err, string(output))
	}

	return binary
}
