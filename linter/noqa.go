package linter

import (
	"strings"

	"github.com/tamnd/gopapy/v1/cst"
)

// noqaIndex maps a 1-based source line to the suppression scope set
// by a trailing `# noqa` comment on that line. A nil entry on a line
// means no suppression. An entry with allCodes==true suppresses every
// diagnostic on the line; otherwise codes lists the explicit codes.
type noqaIndex struct {
	lines map[int]noqaScope
}

type noqaScope struct {
	allCodes bool
	codes    map[string]bool
}

func (n *noqaIndex) suppresses(line int, code string) bool {
	if n == nil || n.lines == nil {
		return false
	}
	s, ok := n.lines[line]
	if !ok {
		return false
	}
	if s.allCodes {
		return true
	}
	return s.codes[code]
}

// buildNoqaIndex scans every COMMENT/TYPE_COMMENT in the file and
// records `# noqa` suppressions by line. Recognised forms:
//
//	# noqa
//	# noqa: F401
//	# noqa: F401, F811
//
// Whitespace and case in the leading `noqa` keyword are tolerated;
// codes are matched case-insensitively too. The Trivia layer already
// distinguishes leading from trailing — for noqa only trailing
// comments suppress (a comment alone on its line is documentation,
// not a suppression).
func buildNoqaIndex(cf *cst.File) *noqaIndex {
	idx := &noqaIndex{lines: map[int]noqaScope{}}
	t := cf.AttachComments()
	add := func(c cst.Comment) {
		if c.Position != cst.Trailing {
			return
		}
		scope, ok := parseNoqa(c.Text)
		if !ok {
			return
		}
		merged := idx.lines[c.Pos.Line]
		if scope.allCodes {
			merged.allCodes = true
		}
		if merged.codes == nil && len(scope.codes) > 0 {
			merged.codes = map[string]bool{}
		}
		for k := range scope.codes {
			merged.codes[k] = true
		}
		idx.lines[c.Pos.Line] = merged
	}
	for _, cmts := range t.ByNode {
		for _, c := range cmts {
			add(c)
		}
	}
	for _, c := range t.File {
		add(c)
	}
	return idx
}

// parseNoqa parses a single comment text. The leading `#` is part of
// the token value the lexer emits, so we strip it then look for the
// noqa keyword.
func parseNoqa(text string) (noqaScope, bool) {
	body := strings.TrimSpace(strings.TrimPrefix(text, "#"))
	lower := strings.ToLower(body)
	if !strings.HasPrefix(lower, "noqa") {
		return noqaScope{}, false
	}
	rest := strings.TrimSpace(body[len("noqa"):])
	if rest == "" {
		return noqaScope{allCodes: true}, true
	}
	if !strings.HasPrefix(rest, ":") {
		return noqaScope{}, false
	}
	rest = strings.TrimSpace(rest[1:])
	codes := map[string]bool{}
	for _, raw := range strings.Split(rest, ",") {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if code == "" {
			continue
		}
		// Stop at the first space-bounded chunk so trailing prose like
		// `# noqa: F401 keep this for re-export` doesn't get parsed
		// as a code.
		if i := strings.IndexAny(code, " \t"); i >= 0 {
			code = code[:i]
		}
		if code != "" {
			codes[code] = true
		}
	}
	if len(codes) == 0 {
		return noqaScope{allCodes: true}, true
	}
	return noqaScope{codes: codes}, true
}
