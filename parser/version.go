package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// VersionFeature is a bitmask of grammar and dump features introduced in
// specific Python minor versions. All version knowledge lives here — the
// parser, validator, and ASTDump read this table rather than hard-coding
// comparisons like "if version >= 312".
type VersionFeature uint32

const (
	FeatWalrus          VersionFeature = 1 << iota // 3.8+: := named expression
	FeatPosOnlyParams                               // 3.8+: def f(x, /, y)
	FeatGeneralDeco                                 // 3.9+: arbitrary decorator expressions (PEP 614)
	FeatMatchCase                                   // 3.10+: match/case statement (PEP 634)
	FeatParenWith                                   // 3.10+: with (a, b): (PEP 617)
	FeatExceptStar                                  // 3.11+: except* ExceptionGroup (PEP 654)
	FeatTypeParams                                  // 3.12+: [T] on def/class/type, TypeAlias (PEP 695)
	FeatTypeAlias                                   // 3.12+: type X = ... statement (PEP 695)
	FeatPEP701FString                               // 3.12+: nested/multiline/comment f-strings (PEP 701)
	FeatTypeParamDefaults                           // 3.13+: [T = int] type param defaults (PEP 696)
	FeatTStrings                                    // 3.14+: t"..." template strings (PEP 750)
)

// Has reports whether the feature set includes feat.
func (f VersionFeature) Has(feat VersionFeature) bool { return f&feat != 0 }

// versionFeatures is the single source of truth for which features are
// available in each Python 3.x minor version. Each entry is the cumulative
// set for that version (not a delta).
var versionFeatures = map[int]VersionFeature{
	8: FeatWalrus | FeatPosOnlyParams,
	9: FeatWalrus | FeatPosOnlyParams |
		FeatGeneralDeco,
	10: FeatWalrus | FeatPosOnlyParams |
		FeatGeneralDeco |
		FeatMatchCase | FeatParenWith,
	11: FeatWalrus | FeatPosOnlyParams |
		FeatGeneralDeco |
		FeatMatchCase | FeatParenWith |
		FeatExceptStar,
	12: FeatWalrus | FeatPosOnlyParams |
		FeatGeneralDeco |
		FeatMatchCase | FeatParenWith |
		FeatExceptStar |
		FeatTypeParams | FeatTypeAlias | FeatPEP701FString,
	13: FeatWalrus | FeatPosOnlyParams |
		FeatGeneralDeco |
		FeatMatchCase | FeatParenWith |
		FeatExceptStar |
		FeatTypeParams | FeatTypeAlias | FeatPEP701FString |
		FeatTypeParamDefaults,
	14: FeatWalrus | FeatPosOnlyParams |
		FeatGeneralDeco |
		FeatMatchCase | FeatParenWith |
		FeatExceptStar |
		FeatTypeParams | FeatTypeAlias | FeatPEP701FString |
		FeatTypeParamDefaults |
		FeatTStrings,
}

// LatestMinor is the highest Python 3 minor version gopapy targets.
const LatestMinor = 14

// FeaturesFor returns the cumulative feature set for Python 3.minor.
// For unknown versions above LatestMinor, returns the latest known set.
// For unknown versions below 8, returns an empty set.
func FeaturesFor(minor int) VersionFeature {
	if f, ok := versionFeatures[minor]; ok {
		return f
	}
	if minor > LatestMinor {
		return versionFeatures[LatestMinor]
	}
	return 0
}

// ParseVersion parses a "3.X" or "3.XY" version string and returns the minor
// version number (e.g., "3.10" → 10, "3.8" → 8). Major must be 3.
func ParseVersion(s string) (int, error) {
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid Python version %q: expected format 3.X", s)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major != 3 {
		return 0, fmt.Errorf("invalid Python version %q: major must be 3", s)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return 0, fmt.Errorf("invalid Python version %q: minor must be a non-negative integer", s)
	}
	return minor, nil
}
