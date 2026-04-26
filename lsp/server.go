package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/tamnd/gopapy/v1/cst"
	"github.com/tamnd/gopapy/v1/diag"
	"github.com/tamnd/gopapy/v1/linter"
)

// ServerVersion is the version string the server reports back in the
// `initialize` response. The CLI overrides it from main.go's `version`
// constant so the LSP-side version stays in lockstep with `gopapy
// version` without an import cycle.
var ServerVersion = "0.0.0"

// Serve runs the language-server loop on r/w until the client sends
// `exit`. Returns nil on a clean shutdown (shutdown + exit, in order)
// and a non-nil error on a forced shutdown (exit without shutdown, or
// a transport-level failure). The CLI uses the error to pick exit
// code 0 vs 1 per LSP spec.
//
// Dispatch is single-goroutine and serial: a request blocks until its
// handler completes, including the lint pass for didOpen/didChange.
// The linter is fast enough on human-scale files that a serial loop
// doesn't lag a typist; if profiling later says otherwise we can
// move lint off the read goroutine without changing the wire format.
func Serve(r io.Reader, w io.Writer) error {
	s := &server{
		rd:        bufio.NewReader(r),
		w:         w,
		docs:      map[string][]byte{},
		cfgCache:  map[string]linter.Config{},
		cfgKnown:  map[string]bool{},
	}
	return s.run()
}

type server struct {
	rd *bufio.Reader
	w  io.Writer
	// writeMu guards writes to w. Dispatch is serial so the only
	// concurrent writer is hypothetical, but a mutex is cheap insurance
	// against future fan-out (e.g. an async lint that publishes when
	// it finishes).
	writeMu sync.Mutex

	// docs holds the latest content of every open document, keyed by
	// URI. Full-sync means each didChange replaces the whole entry; no
	// version tracking, no edit log.
	docs   map[string][]byte
	docsMu sync.Mutex

	// cfgCache memoizes the resolved Config for a workspace directory
	// so we don't re-walk pyproject.toml on every keystroke. Keyed by
	// the document's directory; cfgKnown tracks "we already tried"
	// so we don't repeat a failed lookup.
	cfgCache map[string]linter.Config
	cfgKnown map[string]bool
	cfgMu    sync.Mutex

	shuttingDown bool
}

// run is the read-dispatch loop. EOF on the reader means the client
// hung up without sending exit — treat that as a clean termination
// rather than an error so a Ctrl-C from the user doesn't show up as
// a noisy stack trace.
func (s *server) run() error {
	for {
		body, err := readFrame(s.rd)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("lsp: read: %w", err)
		}
		var msg rawMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			// A malformed frame is fatal: we can't reply (no id) and
			// continuing risks reading a half-aligned next frame.
			return fmt.Errorf("lsp: parse message: %w", err)
		}
		stop, err := s.dispatch(&msg)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}

// dispatch routes one message to its handler. Returns (stop, err):
// stop=true means the loop should exit cleanly (`exit` after a clean
// `shutdown`); err non-nil means the loop should exit with a failure.
func (s *server) dispatch(msg *rawMessage) (bool, error) {
	isRequest := len(msg.ID) > 0

	// Once shutdown has been received, every further request must
	// return InvalidRequest per LSP 3.17 §exit. Notifications are
	// silently ignored except for `exit`.
	if s.shuttingDown && msg.Method != "exit" {
		if isRequest {
			s.respondError(msg.ID, errCodeInvalidRequest, "server is shutting down")
		}
		return false, nil
	}

	switch msg.Method {
	case "initialize":
		return false, s.handleInitialize(msg)
	case "initialized":
		return false, nil
	case "shutdown":
		s.shuttingDown = true
		return false, s.respondResult(msg.ID, json.RawMessage("null"))
	case "exit":
		if s.shuttingDown {
			return true, nil
		}
		return false, errors.New("lsp: exit received without shutdown")
	case "textDocument/didOpen":
		return false, s.handleDidOpen(msg)
	case "textDocument/didChange":
		return false, s.handleDidChange(msg)
	case "textDocument/didClose":
		return false, s.handleDidClose(msg)
	case "textDocument/codeAction":
		return false, s.handleCodeAction(msg)
	default:
		if isRequest {
			s.respondError(msg.ID, errCodeMethodNotFound,
				fmt.Sprintf("method not implemented: %s", msg.Method))
		}
		return false, nil
	}
}

func (s *server) handleInitialize(msg *rawMessage) error {
	result := initializeResult{
		Capabilities: serverCapabilities{
			TextDocumentSync: textDocumentSyncOptions{
				OpenClose: true,
				Change:    1, // full sync
			},
			CodeActionProvider: true,
		},
		ServerInfo: serverInfo{
			Name:    "gopapy",
			Version: ServerVersion,
		},
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("lsp: marshal initialize result: %w", err)
	}
	return s.respondResult(msg.ID, body)
}

func (s *server) handleDidOpen(msg *rawMessage) error {
	var p didOpenParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return nil // notifications can't reply; ignore malformed
	}
	s.storeDoc(p.TextDocument.URI, []byte(p.TextDocument.Text))
	return s.lintAndPublish(p.TextDocument.URI)
}

func (s *server) handleDidChange(msg *rawMessage) error {
	var p didChangeParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return nil
	}
	if len(p.ContentChanges) == 0 {
		return nil
	}
	// Full-sync: contentChanges[0].text is the whole new buffer. If a
	// client ever sends multiple entries we still take the last, which
	// is also the most recent state.
	last := p.ContentChanges[len(p.ContentChanges)-1].Text
	s.storeDoc(p.TextDocument.URI, []byte(last))
	return s.lintAndPublish(p.TextDocument.URI)
}

func (s *server) handleDidClose(msg *rawMessage) error {
	var p didCloseParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return nil
	}
	s.dropDoc(p.TextDocument.URI)
	// Publish an empty diagnostic list so the editor clears any
	// existing squiggles from the last lint of this URI.
	return s.publishDiagnostics(p.TextDocument.URI, []lspDiagnostic{})
}

// handleCodeAction is the editor-side counterpart to `gopapy lint
// --fix`. The MVP shape is one CodeAction titled "gopapy: fix all":
// when the linter would change anything in the buffer we offer a
// quick-fix that replaces the whole document with the post-Fix
// unparse. No per-diagnostic targeting yet — the fixer returns a
// rewritten AST, not an edit list, and producing precise (range,
// replacement) pairs is a follow-up batch.
//
// Empty array is the right answer in three cases: the URI isn't open,
// the source can't parse, or the fixer would change nothing. Editors
// treat `[]` as "no actions available" and don't show a lightbulb,
// which is what we want — never offer an action that wouldn't
// actually do something.
func (s *server) handleCodeAction(msg *rawMessage) error {
	emptyResult := json.RawMessage("[]")
	var p codeActionParams
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return s.respondResult(msg.ID, emptyResult)
	}
	if !matchesOnlyFilter(codeActionKindQuickFix, p.Context.Only) {
		return s.respondResult(msg.ID, emptyResult)
	}
	src, ok := s.docFor(p.TextDocument.URI)
	if !ok {
		return s.respondResult(msg.ID, emptyResult)
	}
	cfg := s.configFor(p.TextDocument.URI)
	logical := uriToPath(p.TextDocument.URI)
	if logical == "" {
		logical = p.TextDocument.URI
	}
	cf, perr := cst.Parse(logical, src)
	if perr != nil {
		return s.respondResult(msg.ID, emptyResult)
	}
	_, fixed := linter.FixWithConfig(cf.AST, cfg, logical)
	if len(fixed) == 0 {
		return s.respondResult(msg.ID, emptyResult)
	}
	out := cf.Unparse()
	action := codeAction{
		Title:       "gopapy: fix all",
		Kind:        codeActionKindQuickFix,
		Diagnostics: p.Context.Diagnostics,
		Edit: workspaceEdit{
			Changes: map[string][]textEdit{
				p.TextDocument.URI: {{
					Range: lspRange{
						Start: lspPosition{0, 0},
						End:   documentEndPosition(src),
					},
					NewText: out,
				}},
			},
		},
		IsPreferred: true,
	}
	body, err := json.Marshal([]codeAction{action})
	if err != nil {
		return fmt.Errorf("lsp: marshal code action: %w", err)
	}
	return s.respondResult(msg.ID, body)
}

// matchesOnlyFilter implements the LSP §codeAction kind filter: a
// server's action of kind X passes the client's `only: [Y...]` filter
// when some Y is X itself or a dot-prefix of X. Empty filter means
// "no filter, allow everything."
func matchesOnlyFilter(kind string, only []string) bool {
	if len(only) == 0 {
		return true
	}
	for _, f := range only {
		if kind == f || strings.HasPrefix(kind, f+".") {
			return true
		}
	}
	return false
}

// documentEndPosition returns the LSP position of the byte just past
// the last byte of src. Used to build a "replace whole document"
// range without hard-coding a sentinel large number. ASCII-correct;
// non-ASCII columns count bytes, matching the rest of gopapy.
func documentEndPosition(src []byte) lspPosition {
	if len(src) == 0 {
		return lspPosition{Line: 0, Character: 0}
	}
	lines := bytes.Split(src, []byte("\n"))
	last := len(lines) - 1
	return lspPosition{Line: last, Character: len(lines[last])}
}

func (s *server) storeDoc(uri string, body []byte) {
	s.docsMu.Lock()
	defer s.docsMu.Unlock()
	s.docs[uri] = body
}

func (s *server) dropDoc(uri string) {
	s.docsMu.Lock()
	defer s.docsMu.Unlock()
	delete(s.docs, uri)
}

func (s *server) docFor(uri string) ([]byte, bool) {
	s.docsMu.Lock()
	defer s.docsMu.Unlock()
	b, ok := s.docs[uri]
	return b, ok
}

// lintAndPublish runs the linter on the latest content for uri and
// sends a publishDiagnostics notification. A parse failure surfaces as
// a single error-level diagnostic at line 1, col 0 — the buffer is
// unparseable mid-edit constantly and we'd rather show "gopapy is
// alive and saw your change but can't parse this yet" than silently
// hold stale squiggles.
func (s *server) lintAndPublish(uri string) error {
	src, ok := s.docFor(uri)
	if !ok {
		return nil
	}
	cfg := s.configFor(uri)
	logical := uriToPath(uri)
	if logical == "" {
		logical = uri
	}
	diags, err := linter.LintFileWithConfig(logical, src, cfg)
	var out []lspDiagnostic
	if err != nil {
		out = []lspDiagnostic{{
			Range:    lspRange{Start: lspPosition{0, 0}, End: lspPosition{0, 0}},
			Severity: 1, // Error
			Source:   "gopapy",
			Message:  err.Error(),
		}}
	} else {
		out = make([]lspDiagnostic, 0, len(diags))
		for _, d := range diags {
			out = append(out, toLSPDiagnostic(d))
		}
	}
	return s.publishDiagnostics(uri, out)
}

// configFor returns the gopapy lint Config that should govern the
// document at uri. For file:// URIs we walk up looking for
// pyproject.toml; for non-file URIs (untitled buffers, http://...)
// we use a zero Config since there's no filesystem anchor to discover
// from. Results are cached per directory — pyproject.toml lookups
// would otherwise re-scan on every keystroke.
func (s *server) configFor(uri string) linter.Config {
	path := uriToPath(uri)
	if path == "" {
		return linter.Config{}
	}
	dir := filepath.Dir(path)
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	if s.cfgKnown[dir] {
		return s.cfgCache[dir]
	}
	cfg, _, _ := linter.DiscoverConfig(path)
	s.cfgCache[dir] = cfg
	s.cfgKnown[dir] = true
	return cfg
}

func (s *server) publishDiagnostics(uri string, diags []lspDiagnostic) error {
	if diags == nil {
		diags = []lspDiagnostic{}
	}
	return s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diags,
	})
}

func (s *server) notify(method string, params interface{}) error {
	body, err := json.Marshal(notificationMessage{
		JSONRPC: jsonrpcVersion,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("lsp: marshal notification %s: %w", method, err)
	}
	return s.writeFrame(body)
}

func (s *server) respondResult(id, result json.RawMessage) error {
	body, err := json.Marshal(responseMessage{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Result:  result,
	})
	if err != nil {
		return fmt.Errorf("lsp: marshal response: %w", err)
	}
	return s.writeFrame(body)
}

// respondError builds and sends a JSON-RPC error response. The return
// value is intentionally swallowed: the dispatcher only calls this on
// the error path where we'd already have nothing actionable to do
// with a write failure.
func (s *server) respondError(id json.RawMessage, code int, message string) {
	body, err := json.Marshal(responseMessage{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error:   &responseError{Code: code, Message: message},
	})
	if err != nil {
		return
	}
	_ = s.writeFrame(body)
}

func (s *server) writeFrame(body []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := fmt.Fprintf(s.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err := s.w.Write(body)
	return err
}

// readFrame parses one LSP message off rd: header lines terminated by
// CRLF, an empty CRLF line, then exactly Content-Length bytes of
// body. Other headers (Content-Type) are read and ignored — we don't
// negotiate encodings.
func readFrame(rd *bufio.Reader) ([]byte, error) {
	var contentLength int
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if k, v, ok := splitHeader(line); ok && strings.EqualFold(k, "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, fmt.Errorf("lsp: bad Content-Length %q: %w", v, err)
			}
			contentLength = n
		}
	}
	if contentLength <= 0 {
		return nil, fmt.Errorf("lsp: missing or zero Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(rd, body); err != nil {
		return nil, err
	}
	return body, nil
}

func splitHeader(line string) (key, value string, ok bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", false
	}
	return line[:i], line[i+1:], true
}

// uriToPath converts a file:// URI to a local filesystem path. Returns
// "" for any non-file URI (untitled, http, etc.) so the caller knows
// to skip filesystem-dependent work like pyproject.toml discovery.
func uriToPath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return ""
	}
	rest := strings.TrimPrefix(uri, "file://")
	// file://host/path is rare in editors but valid; strip the
	// authority. file:///path leaves rest = "/path".
	if i := strings.IndexByte(rest, '/'); i > 0 {
		rest = rest[i:]
	}
	decoded, err := url.PathUnescape(rest)
	if err != nil {
		return ""
	}
	return decoded
}

// toLSPDiagnostic translates one gopapy diag.Diagnostic into the LSP
// shape. End is filled in from gopapy's End span when present, else
// it collapses to start (LSP requires end >= start).
func toLSPDiagnostic(d diag.Diagnostic) lspDiagnostic {
	start := lspPosition{
		Line:      d.Pos.Lineno - 1,
		Character: d.Pos.ColOffset,
	}
	end := start
	if d.End.Lineno != 0 {
		end = lspPosition{
			Line:      d.End.Lineno - 1,
			Character: d.End.ColOffset,
		}
	}
	return lspDiagnostic{
		Range:    lspRange{Start: start, End: end},
		Severity: severityForLSP(d.Severity),
		Code:     d.Code,
		Source:   "gopapy",
		Message:  d.Msg,
	}
}

// severityForLSP maps gopapy's three-level severity into LSP's
// four-level scale. SeverityHint becomes LSP Hint (4), not Information
// (3), because gopapy hints are "minor lint nudge" — the same role
// editors give Hint diagnostics.
func severityForLSP(s diag.Severity) int {
	switch s {
	case diag.SeverityError:
		return 1
	case diag.SeverityWarning:
		return 2
	case diag.SeverityHint:
		return 4
	}
	return 2
}
