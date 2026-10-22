package core

import "strings"

// Protected path denylist (P4c; v1.6.5 beacon-protected-path
// -denylist-v165, a cross-version global-boundaries §6.1 principle).
//
// Automatic write surfaces must treat high-risk paths as a
// fail-closed zone: a hit blocks the write with
// reason_code=protected_path_denylist plus an explicit human-route
// recommendation, never a silent allow. Read-only / planner surfaces
// may REPORT the risk (advisory) but an advisory is not permission.
//
// The category list is verbatim from the v1 global-boundaries text:
//
//	environment: .env, .env.*
//	secrets:     secrets/**, credentials/**, *.pem, *.key, *_key*
//	auth:        auth/**, oauth/**, sso/**
//	payments:    payments/**, billing/**, stripe/**
//	migrations:  migrations/**, schema_migrations/**
//
// Matching forms (v1 AC-002): exact file name, extension glob,
// name-fragment glob, and path-segment (any directory component).
// Paths are judged slash-separated and project-relative; absolute
// or backslash forms are normalized first so a caller cannot dodge
// the list with separators.

// ProtectedPathCategories returns the governed category names in
// declaration order.
func ProtectedPathCategories() []string {
	return []string{"environment", "secrets", "auth", "payments", "migrations"}
}

var protectedDirSegments = map[string]string{
	"secrets":           "secrets",
	"credentials":       "secrets",
	"auth":              "auth",
	"oauth":             "auth",
	"sso":               "auth",
	"payments":          "payments",
	"billing":           "payments",
	"stripe":            "payments",
	"migrations":        "migrations",
	"schema_migrations": "migrations",
}

// ProtectedPathMatch reports whether a project-relative path falls
// in the denylist, and in which category. It is a pure matcher: it
// never decides policy, so read surfaces can use it as an advisory
// and write surfaces can use it as the fail-closed guard.
func ProtectedPathMatch(path string) (category string, matched bool) {
	p := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "", false
	}
	segments := strings.Split(p, "/")
	base := segments[len(segments)-1]
	// environment: .env and .env.* (exact name / name prefix glob).
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return "environment", true
	}
	// secrets by extension / fragment globs on the file name.
	if strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") ||
		strings.Contains(base, "_key") {
		return "secrets", true
	}
	// Directory-segment categories (the `dir/**` forms).
	for _, seg := range segments[:len(segments)-1] {
		if cat, ok := protectedDirSegments[seg]; ok {
			return cat, true
		}
	}
	return "", false
}

// HumanRouteForProtectedPath is the explicit human route every
// denylist block must carry (v1 AC-005): the write is not silently
// allowed and not silently dropped — a human decides, scoped.
func HumanRouteForProtectedPath(category, path string) string {
	return "protected path (" + category + "): " + path +
		" — route to a human for a scoped manual edit or an explicit scoped decision; automatic writes stay blocked"
}
