package linter

import (
	"encoding/json"
	"io"
	"path/filepath"

	"github.com/tamnd/gopapy/legacy/diag"
)

// ToolInfo identifies the producer of a SARIF run. The linter package
// holds no opinion on the tool's name or version (the CLI knows the
// build version, downstream Go programs may want their own brand), so
// the caller fills this in.
type ToolInfo struct {
	Name           string // e.g. "gopapy"
	Version        string // e.g. "0.1.24"
	InformationURI string // e.g. "https://github.com/tamnd/gopapy"
}

// WriteSARIFLog writes a complete SARIF 2.1.0 log document to w.
// Unlike WriteDiagnostic, this is a whole-batch writer: the caller
// passes the full list of diagnostics for the run and SARIF emits
// once at the end. Streaming isn't an option — `results[]` lives
// inside `runs[0]` inside the top-level object.
//
// rules[] in tool.driver is intentionally omitted. SARIF allows a
// result to reference a ruleId without a corresponding rule entry;
// consumers like GitHub code scanning render fine without the table,
// and skipping it keeps the output stable as new codes ship.
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

// buildSARIFResults converts each diagnostic to a SARIF result.
// Returns a non-nil empty slice when diags is empty so the encoded
// document carries `"results": []` rather than `"results": null` —
// a few consumers reject the latter.
func buildSARIFResults(diags []diag.Diagnostic) []sarifResult {
	out := make([]sarifResult, 0, len(diags))
	for _, d := range diags {
		region := sarifRegion{
			StartLine:   d.Pos.Lineno,
			StartColumn: d.Pos.ColOffset + 1,
		}
		// Include end-line/column only when distinct from start so we
		// don't emit redundant fields.
		if d.End.Lineno != 0 && (d.End.Lineno != d.Pos.Lineno || d.End.ColOffset != d.Pos.ColOffset) {
			region.EndLine = d.End.Lineno
			region.EndColumn = d.End.ColOffset + 1
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

// sarifLevel maps gopapy severities to SARIF result levels. SARIF's
// "none" is for results that aren't issues (e.g. metrics); we don't
// emit that — every gopapy diagnostic is at least a hint.
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

// SARIF 2.1.0 schema slice. Only the fields gopapy actually emits
// are modeled; everything else (notifications, invocations, taxa,
// etc.) is left out so adding a real field can't accidentally
// re-order a JSON key in the golden output.
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
