package linter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/tamnd/gopapy/diag"
)

// Format names the supported diagnostic output formats.
type Format string

const (
	FormatText   Format = "text"
	FormatJSON   Format = "json"
	FormatGithub Format = "github"
	FormatSARIF  Format = "sarif"
)

// ParseFormat converts the CLI string to a Format.
func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case FormatText, FormatJSON, FormatGithub, FormatSARIF:
		return Format(s), nil
	}
	return "", fmt.Errorf("unknown format %q (want one of: text, json, github, sarif)", s)
}

// WriteDiagnostic writes one diagnostic in the requested format.
// FormatSARIF is rejected; use WriteSARIFLog instead.
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
		Line:      d.Pos.Line,
		Column:    d.Pos.Col,
		EndLine:   d.End.Line,
		EndColumn: d.End.Col,
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

func writeGithub(w io.Writer, d diag.Diagnostic) error {
	level := "warning"
	switch d.Severity {
	case diag.SeverityError:
		level = "error"
	case diag.SeverityHint:
		level = "notice"
	}
	col := d.Pos.Col + 1
	msg := sanitizeGithubMsg(d.Msg)
	code := ""
	if d.Code != "" {
		code = d.Code + " "
	}
	_, err := fmt.Fprintf(w, "::%s file=%s,line=%d,col=%d::%s%s\n",
		level, d.Filename, d.Pos.Line, col, code, msg)
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
