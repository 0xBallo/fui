package connector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/config"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/pkg/fuiproto"
)

// executablePrefix starts the file name of every external connector.
const executablePrefix = "fui-connector-"

// Names of the built-in connectors.
const (
	builtinFUI = "fui"
	builtinCSV = csvConnector
)

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Registry finds the connectors and implements importing.Connectors: the built-in ones, `fui`
// (a .fui.jsonl file) and `csv` (a profile), then the fui-connector-<name> executables in Dir
// and in the PATH.
type Registry struct {
	// Dir is <config>/connectors, searched before the PATH.
	Dir string
	// Path is the PATH to search; the PATH of the process when empty.
	Path string
	// Timeout is the limit of an external connector run, DefaultTimeout when zero.
	Timeout time.Duration
	// Environ is the environment given to external connectors (without FUI_*); os.Environ when nil.
	Environ []string
	// Profiles are the CSV profiles of the configuration.
	Profiles map[string]config.CSVProfile
	// Currency is the ISO 4217 code used by the csv connector when the request names none.
	Currency string
}

var _ importing.Connectors = Registry{}

func (r Registry) runner() Runner { return Runner{Timeout: r.Timeout, Environ: r.Environ} }

func (r Registry) searchPath() []string {
	path := r.Path
	if path == "" {
		path = os.Getenv("PATH")
	}
	return filepath.SplitList(path)
}

// List implements importing.Connectors.
func (r Registry) List(context.Context) ([]importing.ConnectorInfo, error) {
	out := []importing.ConnectorInfo{
		{Name: builtinCSV, Kind: importing.ConnectorBuiltin}, {Name: builtinFUI, Kind: importing.ConnectorBuiltin},
	}
	seen := map[string]bool{builtinCSV: true, builtinFUI: true}
	for _, dir := range append([]string{r.Dir}, r.searchPath()...) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // a missing directory in the PATH is normal
		}
		for _, e := range entries {
			name, ok := strings.CutPrefix(e.Name(), executablePrefix)
			path := filepath.Join(dir, e.Name())
			if !ok || seen[name] || !validName.MatchString(name) || !isExecutable(path) {
				continue
			}
			seen[name] = true
			out = append(out, importing.ConnectorInfo{Name: name, Kind: importing.ConnectorExternal, Path: path})
		}
	}
	slices.SortFunc(out, func(a, b importing.ConnectorInfo) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func isExecutable(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0o111 != 0
}

// find returns the info of the named connector.
func (r Registry) find(ctx context.Context, name string) (importing.ConnectorInfo, error) {
	all, err := r.List(ctx)
	if err != nil {
		return importing.ConnectorInfo{}, err
	}
	for _, c := range all {
		if c.Name == name {
			return c, nil
		}
	}
	return importing.ConnectorInfo{}, &domain.Error{
		Code: domain.CodeNotFound, Details: map[string]any{"query": name}, Message: fmt.Sprintf("no connector named %q", name),
	}
}

// Describe implements importing.Connectors.
func (r Registry) Describe(ctx context.Context, name string) (fuiproto.DescribeResponse, error) {
	c, err := r.find(ctx, name)
	if err != nil {
		return fuiproto.DescribeResponse{}, err
	}
	switch c.Name {
	case builtinFUI:
		return fuiproto.DescribeResponse{
			Protocol: fuiproto.ProtocolFUI1, Name: builtinFUI, Version: csvVersion, AcceptedFileTypes: []string{".fui.jsonl"},
		}, nil
	case builtinCSV:
		return fuiproto.DescribeResponse{
			Protocol: fuiproto.ProtocolFUI1, Name: builtinCSV, Version: csvVersion, AcceptedFileTypes: []string{".csv"},
			ConfigParams: []fuiproto.ConfigParam{
				{Key: "profile", Type: "string", Required: true, Description: "name of the import.profiles.<name> section"},
				{Key: "account", Type: "string", Description: "account ref and name, default the profile target_account"},
				{Key: "currency", Type: "string", Description: "ISO 4217 code of the account, default the base currency"},
				{Key: "kind", Type: "string", Description: "account kind, default checking"},
			},
		}, nil
	}
	return r.runner().Describe(ctx, c.Path)
}

// Open implements importing.Connectors.
func (r Registry) Open(name string, req fuiproto.FetchRequest) (importing.Source, error) {
	if name == builtinFUI || name == builtinCSV {
		return r.openBuiltin(name, req)
	}
	c, err := r.find(context.Background(), name)
	if err != nil {
		return nil, err
	}
	return r.runner().Source(c.Path, req), nil
}

func (r Registry) openBuiltin(name string, req fuiproto.FetchRequest) (importing.Source, error) {
	if req.File == nil || *req.File == "" {
		return nil, &domain.Error{Code: domain.CodeMissingField, Details: map[string]any{"field": "file"}, Message: name + " connector needs a file"}
	}
	if name == builtinFUI {
		return FileSource{Path: *req.File}, nil
	}
	str := func(key string) string { s, _ := req.Config[key].(string); return s }
	profile, ok := r.Profiles[str("profile")]
	if !ok {
		return nil, &domain.Error{
			Code: domain.CodeNotFound, Details: map[string]any{"query": str("profile")},
			Message: fmt.Sprintf("no csv profile named %q: define import.profiles.%s", str("profile"), str("profile")),
		}
	}
	src := CSVSource{
		Profile: profile, File: *req.File, Account: str("account"), Currency: orDefault(str("currency"), r.Currency),
		Kind: fuiproto.AccountKind(str("kind")),
	}
	if req.Since != nil {
		src.Since = *req.Since
	}
	return src, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
