// Package compat runs the grammar fixture oracle tests for each Python minor
// version from 3.8 through 3.14. Each test uses the Python binary from uv
// (or the system PATH) for that minor version.
//
// A test for version 3.X is skipped when:
//   - The Python 3.X binary is not available (uv not installed, or version
//     not yet downloaded).
//   - The fixture has a "# Python 3.Y+" comment on the first line and Y > X.
//
// Version-aware dump is fully implemented. ASTDump respects pyMinor in two ways:
//   - pyMinor <= 12: all empty/None optional fields are printed (showEmpty=true).
//   - pyMinor <= 8: 3.8-specific fields are added (kind=None on Constant,
//     type_comment=None on assignments/functions, Index/ExtSlice subscript wrappers,
//     vararg=None/kwarg=None in arguments, etc.).
//
// Oracle tests for all versions 3.8–3.14 are live and gate CI.
package compat

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const fixtureDir = "tests/grammar"
const oraclePath = "internal/oracle/oracle.py"

// pythonBin returns the path to the python3.X binary for the given minor
// version. It tries uv first (preferred — pinned standalone builds), then
// falls back to `python3.X` on the PATH.
func pythonBin(minor int) (string, bool) {
	// Try uv python find 3.X first.
	uv, err := exec.LookPath("uv")
	if err == nil {
		out, err := exec.Command(uv, "python", "find", "3."+fmt.Sprint(minor)).Output()
		if err == nil {
			p := strings.TrimSpace(string(out))
			if p != "" {
				return p, true
			}
		}
	}
	// Fall back to python3.X on PATH (unix only).
	// On Windows the py launcher (`python.exe`) is not version-specific, so using
	// it as a fallback would silently run the wrong Python version and produce
	// wrong oracle output. Skip the test instead.
	if runtime.GOOS == "windows" {
		return "", false
	}
	name := "python3." + fmt.Sprint(minor)
	p, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return p, true
}

// gopapyBin returns the path to the gopapy binary, building it if needed.
func gopapyBin(t *testing.T) string {
	t.Helper()
	root := rootDir(t)
	name := "gopapy"
	if runtime.GOOS == "windows" {
		name = "gopapy.exe"
	}
	bin := filepath.Join(root, "bin", name)
	if _, err := os.Stat(bin); err == nil {
		return bin
	}
	// Build it.
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/gopapy")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build gopapy: %v\n%s", err, out)
	}
	return bin
}

func rootDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// minVersionFromComment reads the first line of the file and returns the
// minimum Python minor version required, or 0 if there is no such comment.
// It looks for "# Python 3.X+" on the first line.
func minVersionFromComment(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	buf := make([]byte, 256)
	n, _ := f.Read(buf)
	line := string(buf[:n])
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	// Look for "Python 3.X+" pattern.
	_, rest, found := strings.Cut(line, "Python 3.")
	if !found {
		return 0
	}
	minor := 0
	digits := 0
	for _, c := range rest {
		if c >= '0' && c <= '9' {
			minor = minor*10 + int(c-'0')
			digits++
		} else {
			break
		}
	}
	if digits > 0 && strings.HasPrefix(rest[digits:], "+") {
		return minor
	}
	return 0
}

// runOracleTest runs the grammar fixture oracle test for the given Python minor
// version. It uses `gopapy dump --py 3.X` and compares against `python3.X oracle.py`.
func runOracleTest(t *testing.T, pyMinor int) {
	t.Helper()

	pyBin, ok := pythonBin(pyMinor)
	if !ok {
		t.Skipf("Python 3.%d not available (install with: uv python install 3.%d)", pyMinor, pyMinor)
	}

	gopapy := gopapyBin(t)
	root := rootDir(t)
	fixtures, err := filepath.Glob(filepath.Join(root, fixtureDir, "*.py"))
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no fixtures found in %s", filepath.Join(root, fixtureDir))
	}

	oracle := filepath.Join(root, oraclePath)
	pass, skip, fail := 0, 0, 0

	for _, fix := range fixtures {
		name := filepath.Base(fix)

		// Skip if fixture requires a newer Python.
		minMinor := minVersionFromComment(fix)
		if minMinor > pyMinor {
			skip++
			continue
		}

		// Get oracle output.
		oracleCmd := exec.Command(pyBin, oracle, fix)
		wantOut, err := oracleCmd.Output()
		if err != nil {
			t.Errorf("oracle error for %s: %v", name, err)
			fail++
			continue
		}
		want := strings.TrimRight(string(wantOut), "\r\n")

		// Get gopapy dump output.
		gopapyArgs := []string{"dump", "--py", fmt.Sprintf("3.%d", pyMinor), fix}
		gopapyCmd := exec.Command(gopapy, gopapyArgs...)
		gotOut, err := gopapyCmd.Output()
		if err != nil {
			t.Errorf("gopapy dump error for %s: %v", name, err)
			fail++
			continue
		}
		got := strings.TrimRight(string(gotOut), "\r\n")

		if got != want {
			t.Errorf("mismatch for %s:\nwant: %s\n got: %s\ndiff:\n%s",
				name, want, got, diffLines(want, got))
			fail++
		} else {
			pass++
		}
	}

	t.Logf("Python 3.%d: %d passed, %d skipped, %d failed", pyMinor, pass, skip, fail)
}

// diffLines returns a simple line-by-line diff of two strings.
func diffLines(want, got string) string {
	wLines := strings.Split(want, "\n")
	gLines := strings.Split(got, "\n")
	var b strings.Builder
	max := len(wLines)
	if len(gLines) > max {
		max = len(gLines)
	}
	for i := 0; i < max; i++ {
		w, g := "", ""
		if i < len(wLines) {
			w = wLines[i]
		}
		if i < len(gLines) {
			g = gLines[i]
		}
		if w != g {
			fmt.Fprintf(&b, "line %d:\n  want: %s\n   got: %s\n", i+1, w, g)
		}
	}
	return b.String()
}

// TestOracle_Py314 verifies that `gopapy dump --py 3.14` produces output
// byte-identical to Python 3.14's ast.dump() for all fixtures.
func TestOracle_Py314(t *testing.T) {
	runOracleTest(t, 14)
}

// TestOracle_Py313 verifies that `gopapy dump --py 3.13` produces output
// byte-identical to Python 3.13's ast.dump() for all compatible fixtures.
func TestOracle_Py313(t *testing.T) {
	runOracleTest(t, 13)
}

// TestOracle_Py312 verifies that `gopapy dump --py 3.12` produces output
// byte-identical to Python 3.12's ast.dump() for all compatible fixtures.
// Python 3.12 always prints empty list/None fields (showEmpty behavior).
func TestOracle_Py312(t *testing.T) {
	runOracleTest(t, 12)
}

// TestOracle_Py311 verifies that `gopapy dump --py 3.11` produces output
// byte-identical to Python 3.11's ast.dump() for all compatible fixtures.
// Python 3.11 uses showEmpty format; no type_params field (added in 3.12).
func TestOracle_Py311(t *testing.T) {
	runOracleTest(t, 11)
}

// TestOracle_Py310 verifies that `gopapy dump --py 3.10` produces output
// byte-identical to Python 3.10's ast.dump() for all compatible fixtures.
// Python 3.10 uses showEmpty format; adds match/case statement (PEP 634).
func TestOracle_Py310(t *testing.T) {
	runOracleTest(t, 10)
}

// TestOracle_Py39 verifies that `gopapy dump --py 3.9` produces output
// byte-identical to Python 3.9's ast.dump() for all compatible fixtures.
// Python 3.9 uses showEmpty format; no match/case, no except*, no type_params.
func TestOracle_Py39(t *testing.T) {
	runOracleTest(t, 9)
}

// TestOracle_Py38 verifies that `gopapy dump --py 3.8` produces output
// byte-identical to Python 3.8's ast.dump() for all compatible fixtures.
// Python 3.8 uses showEmpty format; has Index/ExtSlice wrappers for subscripts,
// kind=None on Constant, type_comment=None on several nodes.
func TestOracle_Py38(t *testing.T) {
	runOracleTest(t, 8)
}
