// Package lsp implements a minimum-viable Language Server Protocol
// front end for gopapy. The single entry point is Serve, which runs
// the JSON-RPC 2.0 dispatch loop on stdio (or any io.Reader/Writer
// pair). Every diagnostic that `gopapy lint` emits surfaces as an LSP
// Diagnostic on textDocument/publishDiagnostics, so any LSP-aware
// editor renders gopapy squiggles inline as the buffer changes.
//
// Scope is intentionally narrow — diagnostics-only. Code actions,
// hover, completion, and definition are out; they need machinery
// gopapy doesn't have yet (or want to commit to without a separate
// design pass).
package lsp

import (
	"encoding/json"
)

// jsonrpcVersion is the literal value LSP requires for the `jsonrpc`
// field of every message. Capturing it as a constant keeps the
// payload-builder code from drifting into "2.0" / 2.0 typos.
const jsonrpcVersion = "2.0"

// Standard JSON-RPC error codes used here. The full set (parse error,
// invalid params, etc.) is in the spec; we only emit the two we
// actually return.
const (
	errCodeMethodNotFound = -32601
	errCodeInvalidRequest = -32600
)

// rawMessage is the wire-shape we read off the transport before
// deciding whether a message is a request, response, or notification.
// id is left as a json.RawMessage because LSP allows both numbers and
// strings and we never need to interpret it — it round-trips back to
// the client as-is on the response.
type rawMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// responseMessage is what the server writes back for a request. Either
// Result or Error is set, never both. omitempty on both means the
// encoder picks the right shape without us having to maintain two
// types.
type responseMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      json.RawMessage  `json:"id"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *responseError   `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// notificationMessage is the server-to-client form (no id). LSP
// publishDiagnostics is the only notification we emit.
type notificationMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// initializeResult is the body returned for the `initialize` request.
// Only the subset we actually advertise is modeled; everything else
// is left to the spec's defaults.
type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   serverInfo         `json:"serverInfo"`
}

type serverCapabilities struct {
	TextDocumentSync textDocumentSyncOptions `json:"textDocumentSync"`
}

// textDocumentSyncOptions advertises full-content sync (`change: 1`).
// Incremental sync (`change: 2`) is the more efficient mode but
// requires us to apply per-range edits in order; full sync keeps the
// document store a flat map[uri][]byte and the parser is fast enough
// that resending the buffer per keystroke isn't the bottleneck.
type textDocumentSyncOptions struct {
	OpenClose bool `json:"openClose"`
	Change    int  `json:"change"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// didOpenParams / didChangeParams / didCloseParams cover only the
// fields the server actually reads. languageId/version go unused (we
// lint anything the client opens) and incremental contentChange ranges
// are ignored — full-sync only.
type didOpenParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
}

type didChangeParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

// publishDiagnosticsParams is the body for the
// textDocument/publishDiagnostics notification. The diagnostics slice
// is always non-nil so the encoded JSON carries `[]` rather than
// `null`; the spec allows either but a few clients are picky.
type publishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

// lspDiagnostic is the LSP 3.17 Diagnostic shape, with only the fields
// gopapy populates. severity is an int per spec (1=Error..4=Hint);
// see severityForLSP for the gopapy → LSP mapping.
type lspDiagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity"`
	Code     string   `json:"code,omitempty"`
	Source   string   `json:"source"`
	Message  string   `json:"message"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

// lspPosition uses 0-indexed line and 0-indexed UTF-16 character
// offsets per the LSP spec. gopapy stores 1-indexed lines and
// 0-indexed columns; we subtract 1 from the line and pass the column
// through. Multi-byte handling matches the rest of gopapy: column is a
// byte offset, which is fine for ASCII source and slightly off for
// non-ASCII — fixing that needs a UTF-8→UTF-16 translation pass we
// haven't written yet.
type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
