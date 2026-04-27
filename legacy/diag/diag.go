// Package diag defines the Diagnostic type that gopapy analyzers
// (symbols today, the linter and any future type checker tomorrow)
// share for reporting non-fatal semantic problems.
//
// The shape is deliberately compiler-conventional: one Diagnostic is
// one filename:line:col span, a severity, a stable code, and a human
// message. The String() method emits the common
// `filename:line:col: severity[code]: message` form that editors
// already parse for jump-to-source.
package diag

import (
	"encoding/json"
	"fmt"

	"github.com/tamnd/gopapy/ast"
)

// Severity orders diagnostics from most to least urgent. SeverityError
// is the only level that should fail a CLI run; warnings and hints are
// informational.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityHint
)

// String renders a Severity in the lowercase form used by String() and
// the JSONL output.
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

// Diagnostic is one analyzer finding. Pos is the start of the offending
// span; End is the position just past it (both populated since v0.1.6
// gave AST nodes real end positions). Code is a stable identifier of
// the form `S001` so users can filter / suppress by code without the
// message text shifting underneath them.
type Diagnostic struct {
	Filename string
	Pos      ast.Pos
	End      ast.Pos
	Severity Severity
	Code     string
	Msg      string
}

// String formats d as `filename:line:col: severity[code]: message`. The
// filename is omitted when empty so single-file callers that don't
// bother setting it still get a readable line.
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
		prefix, d.Pos.Lineno, d.Pos.ColOffset, d.Severity, code, d.Msg)
}

// MarshalJSON emits a stable wire shape for the --json CLI. Severity
// renders as its lowercase name rather than the integer value so the
// output is human-skimmable too.
func (d Diagnostic) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Filename string  `json:"filename,omitempty"`
		Pos      ast.Pos `json:"pos"`
		End      ast.Pos `json:"end"`
		Severity string  `json:"severity"`
		Code     string  `json:"code,omitempty"`
		Msg      string  `json:"msg"`
	}{
		Filename: d.Filename,
		Pos:      d.Pos,
		End:      d.End,
		Severity: d.Severity.String(),
		Code:     d.Code,
		Msg:      d.Msg,
	})
}
