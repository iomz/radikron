package update

import (
	"fmt"
	"strconv"
	"strings"
)

// version is a parsed MAJOR.MINOR.PATCH[-PRERELEASE] release version.
type version struct {
	core       [3]int
	prerelease string
}

// parseVersion accepts release tags with or without a leading "v".
// Build metadata after "+" is ignored, as SemVer requires.
func parseVersion(s string) (version, error) {
	var v version
	rest := strings.TrimPrefix(s, "v")
	rest, _, _ = strings.Cut(rest, "+")
	rest, v.prerelease, _ = strings.Cut(rest, "-")
	parts := strings.Split(rest, ".")
	if len(parts) != len(v.core) {
		return version{}, fmt.Errorf("update: %q is not a MAJOR.MINOR.PATCH version", s)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return version{}, fmt.Errorf("update: %q is not a MAJOR.MINOR.PATCH version", s)
		}
		v.core[i] = n
	}
	return v, nil
}

// compare returns -1, 0, or 1. A prerelease sorts before its release.
// Two prereleases of the same core version compare as plain strings, which
// is enough for the tags this project publishes.
func (v version) compare(o version) int {
	for i := range v.core {
		if v.core[i] != o.core[i] {
			if v.core[i] < o.core[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case v.prerelease == o.prerelease:
		return 0
	case v.prerelease == "":
		return 1
	case o.prerelease == "":
		return -1
	}
	return strings.Compare(v.prerelease, o.prerelease)
}

// IsNewer reports whether latest is a newer version than current.
func IsNewer(current, latest string) (bool, error) {
	cur, err := parseVersion(current)
	if err != nil {
		return false, err
	}
	lat, err := parseVersion(latest)
	if err != nil {
		return false, err
	}
	return lat.compare(cur) > 0, nil
}
