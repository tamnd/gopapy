// Package diag defines the Diagnostic type shared by v2 analyzers
// (symbols2, linter2, and any future type checker). Uses parser.Pos
// for source positions.
package diag

import (
	"encoding/json"
	"fmt"

	"github.com/tamnd/gopapy/parser"
)

// Severity orders diagnostics from most to least urgent.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityHint
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityHint:
		return "hint"
	}
	return "unknown"
}

// Diagnostic is one analyzer finding.
type Diagnostic struct {
	Filename string
	Pos      parser.Pos
	End      parser.Pos
	Severity Severity
	Code     string
	Msg      string
}

// String formats d as `filename:line:col: severity[code]: message`.
func (d Diagnostic) String() string {
	prefix := ""
	if d.Filename != "" {
		prefix = d.Filename + ":"
	}
	code := ""
	if d.Code != "" {
		code = "[" + d.Code + "]"
	}
	return fmt.Sprintf("%s%d:%d: %s%s: %s",
		prefix, d.Pos.Line, d.Pos.Col, d.Severity, code, d.Msg)
}

// MarshalJSON emits a stable wire shape compatible with v1 diag JSON output.
func (d Diagnostic) MarshalJSON() ([]byte, error) {
	type pos struct {
		Lineno    int `json:"lineno"`
		ColOffset int `json:"col_offset"`
	}
	return json.Marshal(struct {
		Filename string `json:"filename,omitempty"`
		Pos      pos    `json:"pos"`
		End      pos    `json:"end"`
		Severity string `json:"severity"`
		Code     string `json:"code,omitempty"`
		Msg      string `json:"msg"`
	}{
		Filename: d.Filename,
		Pos:      pos{Lineno: d.Pos.Line, ColOffset: d.Pos.Col},
		End:      pos{Lineno: d.End.Line, ColOffset: d.End.Col},
		Severity: d.Severity.String(),
		Code:     d.Code,
		Msg:      d.Msg,
	})
}
