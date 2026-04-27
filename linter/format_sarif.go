package linter

import (
	"encoding/json"
	"io"
	"path/filepath"

	"github.com/tamnd/gopapy/diag"
)

// ToolInfo identifies the producer of a SARIF run.
type ToolInfo struct {
	Name           string
	Version        string
	InformationURI string
}

// WriteSARIFLog writes a complete SARIF 2.1.0 log document to w.
func WriteSARIFLog(w io.Writer, diags []diag.Diagnostic, tool ToolInfo) error {
	doc := sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           tool.Name,
					Version:        tool.Version,
					InformationURI: tool.InformationURI,
				},
			},
			Results: buildSARIFResults(diags),
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func buildSARIFResults(diags []diag.Diagnostic) []sarifResult {
	out := make([]sarifResult, 0, len(diags))
	for _, d := range diags {
		region := sarifRegion{
			StartLine:   d.Pos.Line,
			StartColumn: d.Pos.Col + 1,
		}
		if d.End.Line != 0 && (d.End.Line != d.Pos.Line || d.End.Col != d.Pos.Col) {
			region.EndLine = d.End.Line
			region.EndColumn = d.End.Col + 1
		}
		out = append(out, sarifResult{
			RuleID:  d.Code,
			Level:   sarifLevel(d.Severity),
			Message: sarifMessage{Text: d.Msg},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{
						URI: filepath.ToSlash(d.Filename),
					},
					Region: region,
				},
			}},
		})
	}
	return out
}

func sarifLevel(s diag.Severity) string {
	switch s {
	case diag.SeverityError:
		return "error"
	case diag.SeverityWarning:
		return "warning"
	case diag.SeverityHint:
		return "note"
	}
	return "warning"
}

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string `json:"name"`
	Version        string `json:"version,omitempty"`
	InformationURI string `json:"informationUri,omitempty"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}
