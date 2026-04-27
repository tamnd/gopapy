package linter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/tamnd/gopapy/diag"
)

// Format names the supported diagnostic output formats. Stable string
// values appear on the CLI as `--format text|json|github`; library
// callers should branch on the typed constants instead.
type Format string

const (
	// FormatText reproduces diag.Diagnostic.String() byte-for-byte:
	// `filename:line:col: severity[CODE]: message`. The default; what
	// humans and grep have always seen.
	FormatText Format = "text"
	// FormatJSON emits one diagnostic per line as a flat JSON object
	// (NDJSON). Field names match what `ruff --output-format json`
	// produces for the overlapping fields, so a tool that consumes
	// ruff's JSON can consume gopapy's with no schema rework.
	FormatJSON Format = "json"
	// FormatGithub emits GitHub Actions workflow command lines so a
	// `gopapy lint` step in a workflow surfaces diagnostics as PR
	// annotations without any glue script.
	FormatGithub Format = "github"
	// FormatSARIF emits a SARIF 2.1.0 log document. Unlike the other
	// formats, SARIF is one whole JSON object — `results[]` lives
	// inside `runs[0]` — so the per-diagnostic WriteDiagnostic path
	// returns an error for it. Use WriteSARIFLog to emit a full run
	// at once, after every diagnostic for the run is collected.
	FormatSARIF Format = "sarif"
)

// ParseFormat converts the CLI string to a Format. Unknown values
// produce an error that lists the accepted choices, so the user sees
// what to type next instead of "unknown format".
func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case FormatText, FormatJSON, FormatGithub, FormatSARIF:
		return Format(s), nil
	}
	return "", fmt.Errorf("unknown format %q (want one of: text, json, github, sarif)", s)
}

// WriteDiagnostic writes one diagnostic in the requested format,
// including the trailing newline. The caller streams diagnostics one
// at a time; no formatter buffers a full list, so a 50k warning run
// stays bounded in memory.
//
// FormatSARIF is rejected here: SARIF wraps results inside a single
// JSON document, so emitting one at a time would produce malformed
// output. Use WriteSARIFLog with the collected slice instead.
func WriteDiagnostic(w io.Writer, d diag.Diagnostic, f Format) error {
	switch f {
	case FormatText, "":
		_, err := fmt.Fprintln(w, d.String())
		return err
	case FormatJSON:
		return writeJSON(w, d)
	case FormatGithub:
		return writeGithub(w, d)
	case FormatSARIF:
		return fmt.Errorf("sarif is a whole-document format; use WriteSARIFLog")
	}
	return fmt.Errorf("unknown format %q", f)
}

// jsonDiag is the on-the-wire shape for FormatJSON. Field names match
// ruff's JSON output (line/column flat) rather than diag.Diagnostic's
// in-process MarshalJSON shape (pos/end nested), because the CLI is
// the integration boundary and ruff is what tooling already speaks.
type jsonDiag struct {
	Filename  string `json:"filename"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
	Severity  string `json:"severity"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
}

func writeJSON(w io.Writer, d diag.Diagnostic) error {
	b, err := json.Marshal(jsonDiag{
		Filename:  d.Filename,
		Line:      d.Pos.Lineno,
		Column:    d.Pos.ColOffset,
		EndLine:   d.End.Lineno,
		EndColumn: d.End.ColOffset,
		Severity:  d.Severity.String(),
		Code:      d.Code,
		Message:   d.Msg,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// writeGithub renders one workflow command line. GitHub's parser
// treats `,` as a property separator and `::` as the message delimiter,
// so messages with those characters need escaping. We collapse them
// to spaces — the alternative (URL-encoding) makes the annotation
// unreadable in the GH UI.
func writeGithub(w io.Writer, d diag.Diagnostic) error {
	level := "warning"
	switch d.Severity {
	case diag.SeverityError:
		level = "error"
	case diag.SeverityHint:
		level = "notice"
	}
	col := d.Pos.ColOffset + 1
	msg := sanitizeGithubMsg(d.Msg)
	code := ""
	if d.Code != "" {
		code = d.Code + " "
	}
	_, err := fmt.Fprintf(w, "::%s file=%s,line=%d,col=%d::%s%s\n",
		level, d.Filename, d.Pos.Lineno, col, code, msg)
	return err
}

func sanitizeGithubMsg(s string) string {
	r := strings.NewReplacer(
		"\n", " ",
		"\r", " ",
		"::", "  ",
	)
	return r.Replace(s)
}
