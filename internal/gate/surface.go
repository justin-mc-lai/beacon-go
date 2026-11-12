// Requirement-truth surface declaration (qa-surface-e2e): a feature
// package names the surfaces its acceptance criteria live on — api /
// web / desktop / cli — so the qa gate can demand surface-specific
// evidence instead of accepting "some file was registered".
//
// The declaration is read from the truth.md frontmatter:
//
//	surfaces: [api, web]        # inline list (block list also accepted)
//	surface_strict: true        # written by freeze on new packages
//
// and each AC table row may narrow it with a `surface=<value>` token.
// A row without the token inherits the package list. Legacy packages
// (no `surfaces` key at all) keep working: they parse as not declared,
// never strict, and the qa gate downgrades the surface checks to a
// legacy waiver warning.
package gate

import (
	"regexp"
	"sort"
	"strings"

	"beacon-go/internal/core"
)

// QASurfaceValues is the closed surface vocabulary (api | web |
// desktop | cli) in declaration order.
var QASurfaceValues = []string{"api", "web", "desktop", "cli"}

// SurfaceDeclaration is the parsed surface half of a truth package.
type SurfaceDeclaration struct {
	// Declared is true when truth.md carries the `surfaces` key at all.
	// A missing key is the legacy shape (no backfill, no failure).
	Declared bool `json:"surfaces_declared"`
	// Surfaces is the package-level list, normalized to lower case and
	// de-duplicated in declaration order.
	Surfaces []string `json:"surfaces,omitempty"`
	// Strict is the explicit `surface_strict: true` flag. Freeze writes
	// it on new packages that declare surfaces; absent reads as false.
	Strict bool `json:"surface_strict"`
	// StrictDeclared distinguishes `surface_strict: false` from a key
	// the freeze will fill in.
	StrictDeclared bool `json:"surface_strict_declared,omitempty"`
	// ACSurfaces maps AC id -> the surfaces that AC belongs to. Rows
	// without a `surface=` token inherit Surfaces.
	ACSurfaces map[string][]string `json:"ac_surfaces,omitempty"`
}

var (
	surfaceACRowRe = regexp.MustCompile(`^\|\s*(AC-\d{3})\s*\|`)
	surfaceTokenRe = regexp.MustCompile(`(?i)surface\s*=\s*([a-z][a-z_-]*)`)
)

// ValidQASurface reports whether value is a member of the surface
// vocabulary. Empty is never valid (the caller decides whether the
// key is required).
func ValidQASurface(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	for _, s := range QASurfaceValues {
		if s == v {
			return true
		}
	}
	return false
}

// EffectiveStrict is the strictness freeze will apply: an explicit
// `surface_strict: true`, or a declared surface list whose key was
// omitted (a new freeze is strict by default). An explicit false stays
// false — that is the author's legacy waiver.
func (d SurfaceDeclaration) EffectiveStrict() bool {
	if d.Strict {
		return true
	}
	return d.Declared && !d.StrictDeclared && len(d.Surfaces) > 0
}

// frontmatterBlock returns the raw YAML frontmatter body (without the
// --- fences) and whether the document has one.
func frontmatterBlock(text string) (string, bool) {
	if !strings.HasPrefix(text, "---\n") {
		return "", false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return "", false
	}
	return text[4 : 4+end], true
}

// ParseSurfaceDeclaration reads the surface declaration out of a
// truth document. Both the inline (`[api, web]`) and block (`- api`)
// list forms are accepted, as is a bare comma list.
func ParseSurfaceDeclaration(text string) SurfaceDeclaration {
	d := SurfaceDeclaration{ACSurfaces: map[string][]string{}}
	fm, ok := frontmatterBlock(text)
	if !ok {
		return d
	}
	lines := strings.Split(fm, "\n")
	inBlockList := false
	for _, line := range lines {
		if inBlockList {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") {
				d.Surfaces = appendSurfaceValue(d.Surfaces, strings.TrimSpace(trimmed[2:]))
				continue
			}
			if trimmed == "" {
				continue
			}
			inBlockList = false
		}
		if !strings.HasPrefix(line, " ") {
			key, val, found := strings.Cut(line, ":")
			if !found {
				continue
			}
			switch strings.TrimSpace(key) {
			case "surfaces":
				d.Declared = true
				raw := strings.TrimSpace(val)
				switch {
				case raw == "", raw == "[]":
					inBlockList = raw == ""
				default:
					raw = strings.Trim(raw, "[]")
					for _, part := range strings.Split(raw, ",") {
						d.Surfaces = appendSurfaceValue(d.Surfaces, part)
					}
				}
			case "surface_strict":
				d.StrictDeclared = true
				d.Strict = strings.EqualFold(strings.Trim(strings.TrimSpace(val), `"'`), "true")
			}
		}
	}
	for _, line := range strings.Split(text, "\n") {
		m := surfaceACRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ac := m[1]
		explicit := []string{}
		for _, tok := range surfaceTokenRe.FindAllStringSubmatch(line, -1) {
			explicit = appendSurfaceValue(explicit, tok[1])
		}
		if len(explicit) > 0 {
			d.ACSurfaces[ac] = explicit
		} else {
			d.ACSurfaces[ac] = append([]string{}, d.Surfaces...)
		}
	}
	return d
}

func appendSurfaceValue(list []string, raw string) []string {
	v := strings.ToLower(strings.TrimSpace(strings.Trim(raw, `"'`)))
	if v == "" {
		return list
	}
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}

// ValidateSurfaceDeclaration applies the freeze rules and returns the
// reason codes in check order (empty means clean). A missing key is
// the legacy shape and validates cleanly; an explicitly empty list is
// a refusal.
func ValidateSurfaceDeclaration(d SurfaceDeclaration) []string {
	if !d.Declared {
		return nil
	}
	return validateSurfaceDeclarationDeclared(d)
}

// ValidateSurfaceDeclarationForFreeze is the freeze-time rule with the
// draft/legacy distinction. A FIRST freeze (a package that has never
// been frozen) must declare its surfaces — a fresh draft that omits the
// key would otherwise buy the legacy waiver forever and downgrade both
// qa evidence gates to warnings. A REFREEZE of a package that was frozen
// before keeps the legacy shape valid: its missing key is history, not
// a new omission.
func ValidateSurfaceDeclarationForFreeze(d SurfaceDeclaration, firstFreeze bool) []string {
	if !d.Declared {
		if firstFreeze {
			return []string{core.ReasonTruthSurfaceRequired}
		}
		return nil
	}
	return validateSurfaceDeclarationDeclared(d)
}

func validateSurfaceDeclarationDeclared(d SurfaceDeclaration) []string {
	if len(d.Surfaces) == 0 {
		return []string{core.ReasonTruthSurfaceRequired}
	}
	for _, s := range d.Surfaces {
		if !ValidQASurface(s) {
			return []string{core.ReasonTruthSurfaceInvalid}
		}
	}
	if !d.Strict {
		return nil
	}
	for _, ac := range sortedACIDs(d.ACSurfaces) {
		for _, s := range d.ACSurfaces[ac] {
			if !ValidQASurface(s) || !containsSurface(d.Surfaces, s) {
				return []string{core.ReasonTruthACSurfaceUndeclared}
			}
		}
	}
	return nil
}

func containsSurface(list []string, value string) bool {
	for _, s := range list {
		if s == value {
			return true
		}
	}
	return false
}

func sortedACIDs(m map[string][]string) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// SurfaceEvidenceKind is the artifact kind that proves one surface:
// api and cli ride on an e2e report, web on a browser trace, desktop
// on an accessibility tree.
func SurfaceEvidenceKind(surface string) string {
	switch strings.ToLower(strings.TrimSpace(surface)) {
	case "api", "cli":
		return "e2e_report"
	case "web":
		return "browser_trace"
	case "desktop":
		return "a11y_tree"
	}
	return ""
}

// SurfaceEvidenceReason is the per-surface failure code.
func SurfaceEvidenceReason(surface string) string {
	return "qa_surface_evidence_miss:" + strings.ToLower(strings.TrimSpace(surface))
}
