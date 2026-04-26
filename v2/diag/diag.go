// Package diag defines the Diagnostic type shared by v2 analyzers
// (symbols2, linter2, and any future type checker). Uses parser2.Pos
// for source positions.
package diag

import (
	"fmt"

	"github.com/tamnd/gopapy/v2/parser2"
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
	Pos      parser2.Pos
	End      parser2.Pos
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
