package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/0xballo/fui/internal/app/importing"
)

// ConnectorDTO is the --json shape of one discovered connector.
type ConnectorDTO struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Path string `json:"path,omitempty"`
}

func connectorDTO(c importing.ConnectorInfo) ConnectorDTO {
	return ConnectorDTO{Name: c.Name, Kind: string(c.Kind), Path: c.Path}
}

func connectorRows(cs []importing.ConnectorInfo) Rows {
	t := Rows{Header: []string{"NAME", "KIND", "PATH"}}
	for _, c := range cs {
		t.Body = append(t.Body, []string{c.Name, string(c.Kind), c.Path})
	}
	return t
}

// WriteConnectorList renders the connectors discovered by the registry.
func WriteConnectorList(w io.Writer, f Format, cs []importing.ConnectorInfo) error {
	if f == JSON {
		dtos := make([]ConnectorDTO, len(cs))
		for i, c := range cs {
			dtos[i] = connectorDTO(c)
		}
		return WriteJSON(w, dtos)
	}
	if f == CSV {
		return WriteCSV(w, connectorRows(cs))
	}
	return WriteTable(w, connectorRows(cs), false)
}

// ConfigParamDTO is the --json shape of one connector configuration parameter.
type ConfigParamDTO struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

// ConnectorDetailDTO is the --json shape of `connector show`.
type ConnectorDetailDTO struct {
	ConnectorDTO
	Protocol          string           `json:"protocol"`
	Version           string           `json:"version"`
	AcceptedFileTypes []string         `json:"accepted_file_types,omitempty"`
	ConfigParams      []ConfigParamDTO `json:"config_params,omitempty"`
}

func connectorDetailDTO(d importing.ConnectorDetail) ConnectorDetailDTO {
	out := ConnectorDetailDTO{
		ConnectorDTO: connectorDTO(d.Info), Protocol: d.Describe.Protocol, Version: d.Describe.Version,
		AcceptedFileTypes: d.Describe.AcceptedFileTypes,
	}
	for _, p := range d.Describe.ConfigParams {
		out.ConfigParams = append(out.ConfigParams,
			ConfigParamDTO{Key: p.Key, Type: p.Type, Required: p.Required, Description: p.Description})
	}
	return out
}

// WriteConnectorDetail renders the metadata of one connector.
func WriteConnectorDetail(w io.Writer, f Format, d importing.ConnectorDetail) error {
	if f == JSON {
		return WriteJSON(w, connectorDetailDTO(d))
	}
	t := Rows{Header: []string{"FIELD", "VALUE"}}
	t.Body = append(t.Body,
		[]string{"name", d.Info.Name}, []string{"kind", string(d.Info.Kind)},
		[]string{"protocol", d.Describe.Protocol}, []string{"version", d.Describe.Version},
		[]string{"accepted_file_types", strings.Join(d.Describe.AcceptedFileTypes, ", ")},
	)
	for _, p := range d.Describe.ConfigParams {
		t.Body = append(t.Body, []string{"param:" + p.Key, fmt.Sprintf("%s required=%v %s", p.Type, p.Required, p.Description)})
	}
	if f == CSV {
		return WriteCSV(w, t)
	}
	return WriteTable(w, t, false)
}

// RowErrorDTO is the --json shape of one row the source reported or that could not be converted.
type RowErrorDTO struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// TestReportDTO is the --json shape of `connector test`.
type TestReportDTO struct {
	Connector    string        `json:"connector,omitempty"`
	Source       string        `json:"source,omitempty"`
	SourceSHA256 string        `json:"source_sha256,omitempty"`
	Accounts     int           `json:"accounts"`
	Transactions int           `json:"transactions"`
	Balances     int           `json:"balances"`
	Errors       []RowErrorDTO `json:"errors,omitempty"`
	Stderr       string        `json:"stderr,omitempty"`
}

func testReportDTO(rep importing.TestReport, verbose bool) TestReportDTO {
	dto := TestReportDTO{
		Connector: rep.Connector, Source: rep.Source, SourceSHA256: rep.SourceSHA256,
		Accounts: len(rep.Accounts), Transactions: rep.Transactions, Balances: rep.Balances,
	}
	for _, e := range rep.Errors {
		dto.Errors = append(dto.Errors, RowErrorDTO{Line: e.Line, Message: e.Message})
	}
	if verbose {
		dto.Stderr = rep.Stderr
	}
	return dto
}

// WriteTestReport renders the outcome of `connector test`: a fetch and validation with no write.
// verbose includes the connector's stderr.
func WriteTestReport(w io.Writer, f Format, verbose bool, rep importing.TestReport) error {
	if f == JSON {
		return WriteJSON(w, testReportDTO(rep, verbose))
	}
	t := Rows{Header: []string{"FIELD", "VALUE"}}
	t.Body = append(t.Body,
		[]string{"connector", rep.Connector}, []string{"source", rep.Source},
		[]string{"accounts", strconv.Itoa(len(rep.Accounts))}, []string{"transactions", strconv.Itoa(rep.Transactions)},
		[]string{"balances", strconv.Itoa(rep.Balances)}, []string{"errors", strconv.Itoa(len(rep.Errors))},
	)
	var err error
	if f == CSV {
		err = WriteCSV(w, t)
	} else {
		err = WriteTable(w, t, false)
	}
	if err != nil {
		return err
	}
	for _, e := range rep.Errors {
		if _, werr := fmt.Fprintf(w, "line %d: %s\n", e.Line, e.Message); werr != nil {
			return fmt.Errorf("write test report error: %w", werr)
		}
	}
	if verbose && rep.Stderr != "" {
		if _, werr := fmt.Fprintln(w, "stderr:\n"+rep.Stderr); werr != nil {
			return fmt.Errorf("write test report stderr: %w", werr)
		}
	}
	return nil
}
