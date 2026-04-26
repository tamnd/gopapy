package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// session wires a goroutine running Serve to in/out pipes a test can
// drive synchronously. Each test sends frames, reads frames, and tears
// down by sending shutdown+exit (or just closing the reader).
type session struct {
	t          *testing.T
	clientWrt  io.WriteCloser // writes go to server's stdin
	serverOut  *bufio.Reader  // reads pull from server's stdout
	done       chan error
	closeOnce  sync.Once
	closeFuncs []func() error
}

func newSession(t *testing.T) *session {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	s := &session{
		t:         t,
		clientWrt: inW,
		serverOut: bufio.NewReader(outR),
		done:      make(chan error, 1),
		closeFuncs: []func() error{
			inW.Close,
			outR.Close,
		},
	}
	go func() {
		defer close(s.done)
		err := Serve(inR, outW)
		_ = outW.Close()
		s.done <- err
	}()
	t.Cleanup(s.shutdown)
	return s
}

// shutdown closes pipes and drains the server goroutine. Idempotent
// so tests that already sent exit can safely defer it via Cleanup.
func (s *session) shutdown() {
	s.closeOnce.Do(func() {
		for _, f := range s.closeFuncs {
			_ = f()
		}
		select {
		case <-s.done:
		case <-time.After(2 * time.Second):
			s.t.Errorf("server did not exit within 2s")
		}
	})
}

// waitClean expects Serve to return nil within timeout — the
// shutdown+exit path. Failing this means the server hung or returned
// an unexpected error.
func (s *session) waitClean(timeout time.Duration) {
	s.t.Helper()
	select {
	case err := <-s.done:
		if err != nil {
			s.t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(timeout):
		s.t.Fatalf("Serve did not return within %s", timeout)
	}
}

// waitForceExit is the mirror of waitClean: expects Serve to return
// non-nil (exit-without-shutdown).
func (s *session) waitForceExit(timeout time.Duration) {
	s.t.Helper()
	select {
	case err := <-s.done:
		if err == nil {
			s.t.Fatalf("Serve returned nil, wanted error")
		}
	case <-time.After(timeout):
		s.t.Fatalf("Serve did not return within %s", timeout)
	}
}

func (s *session) send(method string, id interface{}, params interface{}) {
	s.t.Helper()
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if id != nil {
		body["id"] = id
	}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	if err != nil {
		s.t.Fatalf("marshal request: %v", err)
	}
	if err := writeFrameTo(s.clientWrt, raw); err != nil {
		s.t.Fatalf("write frame: %v", err)
	}
}

func (s *session) recv() map[string]interface{} {
	s.t.Helper()
	body, err := readFrame(s.serverOut)
	if err != nil {
		s.t.Fatalf("read frame: %v", err)
	}
	var msg map[string]interface{}
	if err := json.Unmarshal(body, &msg); err != nil {
		s.t.Fatalf("decode frame: %v\n%s", err, body)
	}
	return msg
}

// recvNotification reads frames until one matches the wanted method
// name (and is a notification, i.e. has no id). Lets a test skip
// past unrelated notifications without coupling to ordering.
func (s *session) recvNotification(method string) map[string]interface{} {
	s.t.Helper()
	for i := 0; i < 4; i++ {
		msg := s.recv()
		if _, hasID := msg["id"]; hasID {
			continue
		}
		if m, _ := msg["method"].(string); m == method {
			return msg
		}
	}
	s.t.Fatalf("no %s notification after 4 frames", method)
	return nil
}

func writeFrameTo(w io.Writer, body []byte) error {
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func TestInitializeHandshake(t *testing.T) {
	s := newSession(t)
	s.send("initialize", 1, map[string]interface{}{
		"processId":    nil,
		"rootUri":      nil,
		"capabilities": map[string]interface{}{},
	})
	resp := s.recv()
	if got := resp["id"]; toFloat(got) != 1 {
		t.Errorf("response id = %v, want 1", got)
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result missing or not object: %v", resp)
	}
	caps, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("capabilities missing: %v", result)
	}
	sync, ok := caps["textDocumentSync"].(map[string]interface{})
	if !ok {
		t.Fatalf("textDocumentSync missing: %v", caps)
	}
	if sync["openClose"] != true {
		t.Errorf("openClose = %v, want true", sync["openClose"])
	}
	if toFloat(sync["change"]) != 1 {
		t.Errorf("change = %v, want 1", sync["change"])
	}
	info, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("serverInfo missing: %v", result)
	}
	if info["name"] != "gopapy" {
		t.Errorf("serverInfo.name = %v, want gopapy", info["name"])
	}
	// Clean exit so the t.Cleanup doesn't have to time out.
	s.send("shutdown", 2, nil)
	_ = s.recv()
	s.send("exit", nil, nil)
	s.waitClean(2 * time.Second)
}

func TestDidOpenPublishesDiagnostics(t *testing.T) {
	s := newSession(t)
	initOnly(s)
	s.send("textDocument/didOpen", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/x.py",
			"languageId": "python",
			"version":    1,
			"text":       "import os\n",
		},
	})
	pub := s.recvNotification("textDocument/publishDiagnostics")
	params := pub["params"].(map[string]interface{})
	if params["uri"] != "file:///tmp/x.py" {
		t.Errorf("uri = %v", params["uri"])
	}
	diags := params["diagnostics"].([]interface{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
	}
	d := diags[0].(map[string]interface{})
	if d["code"] != "F401" {
		t.Errorf("code = %v, want F401", d["code"])
	}
	if d["source"] != "gopapy" {
		t.Errorf("source = %v, want gopapy", d["source"])
	}
	if toFloat(d["severity"]) != 2 {
		t.Errorf("severity = %v, want 2 (Warning)", d["severity"])
	}
	cleanExit(s)
}

func TestDidChangePublishesUpdated(t *testing.T) {
	s := newSession(t)
	initOnly(s)
	uri := "file:///tmp/y.py"
	s.send("textDocument/didOpen", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": "python",
			"version":    1,
			"text":       "import os\n",
		},
	})
	first := s.recvNotification("textDocument/publishDiagnostics")
	if got := len(first["params"].(map[string]interface{})["diagnostics"].([]interface{})); got != 1 {
		t.Fatalf("first publish diagnostics = %d, want 1", got)
	}
	s.send("textDocument/didChange", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri, "version": 2},
		"contentChanges": []interface{}{
			map[string]interface{}{"text": "import os\nprint(os)\n"},
		},
	})
	second := s.recvNotification("textDocument/publishDiagnostics")
	got := second["params"].(map[string]interface{})["diagnostics"].([]interface{})
	if len(got) != 0 {
		t.Errorf("after change expected 0 diagnostics, got %d: %v", len(got), got)
	}
	cleanExit(s)
}

func TestDidCloseClearsDiagnostics(t *testing.T) {
	s := newSession(t)
	initOnly(s)
	uri := "file:///tmp/z.py"
	s.send("textDocument/didOpen", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "python", "version": 1,
			"text": "import os\n",
		},
	})
	_ = s.recvNotification("textDocument/publishDiagnostics")
	s.send("textDocument/didClose", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
	})
	final := s.recvNotification("textDocument/publishDiagnostics")
	got := final["params"].(map[string]interface{})["diagnostics"].([]interface{})
	if len(got) != 0 {
		t.Errorf("close expected empty diagnostics, got %d", len(got))
	}
	cleanExit(s)
}

func TestShutdownThenExit(t *testing.T) {
	s := newSession(t)
	initOnly(s)
	s.send("shutdown", 99, nil)
	resp := s.recv()
	if _, ok := resp["result"]; !ok {
		t.Errorf("shutdown response missing result: %v", resp)
	}
	if resp["error"] != nil {
		t.Errorf("shutdown should not return error: %v", resp["error"])
	}
	s.send("exit", nil, nil)
	s.waitClean(2 * time.Second)
}

func TestExitWithoutShutdownIsError(t *testing.T) {
	s := newSession(t)
	initOnly(s)
	s.send("exit", nil, nil)
	s.waitForceExit(2 * time.Second)
}

func TestUnknownMethodReturnsError(t *testing.T) {
	s := newSession(t)
	initOnly(s)
	s.send("workspace/configuration", 7, map[string]interface{}{})
	resp := s.recv()
	if toFloat(resp["id"]) != 7 {
		t.Errorf("response id = %v, want 7", resp["id"])
	}
	errBody, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error response, got: %v", resp)
	}
	if toFloat(errBody["code"]) != float64(errCodeMethodNotFound) {
		t.Errorf("error code = %v, want %d", errBody["code"], errCodeMethodNotFound)
	}
	cleanExit(s)
}

func TestParseFailureSurfacesAsDiagnostic(t *testing.T) {
	s := newSession(t)
	initOnly(s)
	s.send("textDocument/didOpen", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/bad.py", "languageId": "python", "version": 1,
			// `1 +` is unparseable; the parser refuses it before any
			// AST is built so LintFileWithConfig returns the error.
			"text": "1 +\n",
		},
	})
	pub := s.recvNotification("textDocument/publishDiagnostics")
	diags := pub["params"].(map[string]interface{})["diagnostics"].([]interface{})
	if len(diags) != 1 {
		t.Fatalf("expected 1 parse-failure diagnostic, got %d", len(diags))
	}
	d := diags[0].(map[string]interface{})
	if toFloat(d["severity"]) != 1 {
		t.Errorf("parse failure severity = %v, want 1 (Error)", d["severity"])
	}
	rng := d["range"].(map[string]interface{})
	start := rng["start"].(map[string]interface{})
	if toFloat(start["line"]) != 0 || toFloat(start["character"]) != 0 {
		t.Errorf("parse failure range start = %v, want {0,0}", start)
	}
	cleanExit(s)
}

func TestNoqaSuppressionApplies(t *testing.T) {
	s := newSession(t)
	initOnly(s)
	s.send("textDocument/didOpen", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/noqa.py", "languageId": "python", "version": 1,
			"text": "import os  # noqa: F401\n",
		},
	})
	pub := s.recvNotification("textDocument/publishDiagnostics")
	diags := pub["params"].(map[string]interface{})["diagnostics"].([]interface{})
	if len(diags) != 0 {
		t.Errorf("# noqa: F401 should suppress, got %d diagnostics: %v", len(diags), diags)
	}
	cleanExit(s)
}

func TestUriToPath(t *testing.T) {
	cases := []struct{ uri, want string }{
		{"file:///tmp/x.py", "/tmp/x.py"},
		{"file:///Users/a%20b/x.py", "/Users/a b/x.py"},
		{"untitled:Untitled-1", ""},
		{"http://example.com/x.py", ""},
	}
	for _, tc := range cases {
		if got := uriToPath(tc.uri); got != tc.want {
			t.Errorf("uriToPath(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}

func TestReadFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := []byte(`{"jsonrpc":"2.0","method":"hi"}`)
	if err := writeFrameTo(&buf, want); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readFrame(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("round-trip body mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestReadFrameRejectsMissingContentLength(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("Content-Type: x\r\n\r\n"))
	_, err := readFrame(r)
	if err == nil {
		t.Fatalf("expected error for missing Content-Length")
	}
}

// initOnly drives the initialize handshake and discards the response,
// for tests that only care about the post-handshake methods.
func initOnly(s *session) {
	s.t.Helper()
	s.send("initialize", 1, map[string]interface{}{})
	_ = s.recv()
	s.send("initialized", nil, map[string]interface{}{})
}

// cleanExit sends shutdown+exit and asserts Serve returned nil.
func cleanExit(s *session) {
	s.t.Helper()
	id := strconv.Itoa(99)
	s.send("shutdown", id, nil)
	_ = s.recv()
	s.send("exit", nil, nil)
	s.waitClean(2 * time.Second)
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return -1
}
