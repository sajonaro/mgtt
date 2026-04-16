package model

import (
	"fmt"
	"strings"
)

// ProviderRef is a parsed provider reference from a model's `providers:` list.
// Four supported input forms:
//
//	"kubernetes"                           — legacy bare name
//	"mgt-tool/kubernetes"                  — FQN, any version
//	"kubernetes@0.5.0"                     — legacy + version
//	"mgt-tool/kubernetes@>=0.5.0,<1.0.0"  — FQN + constraint
type ProviderRef struct {
	Namespace         string // empty for legacy bare-name refs
	Name              string // required — the short provider name
	VersionConstraint string // empty = any version; may contain SemVer operators (>=, <, ^, etc.)
	LegacyBareName    bool   // true when input had no namespace; triggers validate warning
}

// ParseProviderRef parses a string from a model's `providers:` list into a
// ProviderRef. Handles all four forms.
func ParseProviderRef(s string) (ProviderRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return ProviderRef{}, fmt.Errorf("provider ref must not be empty")
	}

	// 1. Split on "@" to separate name-part from optional version constraint.
	//    Only the first "@" matters; version constraints like ">=0.5.0,<1.0.0"
	//    contain no additional "@" characters.
	var namePart, versionConstraint string
	if idx := strings.Index(s, "@"); idx >= 0 {
		namePart = s[:idx]
		versionConstraint = s[idx+1:]
	} else {
		namePart = s
	}

	// 2. Split name-part on "/" to distinguish bare name from FQN.
	segments := strings.Split(namePart, "/")
	var namespace, name string
	switch len(segments) {
	case 1:
		// Bare name — legacy form (no namespace).
		name = segments[0]
	case 2:
		namespace = segments[0]
		name = segments[1]
	default:
		return ProviderRef{}, fmt.Errorf("provider ref %q: too many path segments (expected at most namespace/name)", s)
	}

	// 3. Validate: namespace (if present) and name must be non-empty and
	//    contain no whitespace.
	if strings.ContainsAny(namespace, " \t\n\r") {
		return ProviderRef{}, fmt.Errorf("provider ref %q: namespace must not contain whitespace", s)
	}
	if namespace == "" && len(segments) == 2 {
		return ProviderRef{}, fmt.Errorf("provider ref %q: namespace must not be empty", s)
	}
	if strings.ContainsAny(name, " \t\n\r") {
		return ProviderRef{}, fmt.Errorf("provider ref %q: name must not contain whitespace", s)
	}
	if name == "" {
		return ProviderRef{}, fmt.Errorf("provider ref %q: name must not be empty", s)
	}

	// 4. Set LegacyBareName when there is no namespace.
	legacyBareName := namespace == ""

	return ProviderRef{
		Namespace:         namespace,
		Name:              name,
		VersionConstraint: versionConstraint,
		LegacyBareName:    legacyBareName,
	}, nil
}
