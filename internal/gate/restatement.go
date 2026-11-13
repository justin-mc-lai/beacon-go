// Requirement-truth restatement gate (restatement-gate): a
// restatement.md sits beside the truth four-piece set and carries the
// author's restatement of the requirement — five fixed sections,
// signed by a human, with the body hash recorded at confirmation time.
//
// The file shape is:
//
//	---
//	slug: <feature slug>
//	confirmed_by: <who confirmed>
//	confirmed_at: <when>
//	hash: <sha256 of the body>
//	---
//	## 1 ...
//	## 2 ...
//	## 3 ...
//	## 4 ...
//	## 5 ...
//
// The body is everything after the closing `---` fence, byte for byte:
// no trimming, no newline normalization. One edited character after
// confirmation makes the file stale. The five sections are matched by
// heading number (`## 1`), subsections like `## 3.2` included.
//
// This file is a pure parser: orchestrator/truth.go decides when a
// missing or invalid restatement blocks a freeze.
package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"sort"
	"strings"

	"beacon-go/internal/core"
)

// RestatementFile is the restatement document's file name inside the
// feature package directory.
const RestatementFile = "restatement.md"

// RestatementRequiredSections is the fixed five-section layout.
var RestatementRequiredSections = []string{"1", "2", "3", "4", "5"}

// Restatement is the parsed restatement document. Present is true when
// the file exists on disk; every other field then describes its
// contents. A missing file (legacy package) is the zero Restatement.
type Restatement struct {
	Present     bool     `json:"present"`
	Path        string   `json:"path,omitempty"`
	Slug        string   `json:"slug,omitempty"`
	ConfirmedBy string   `json:"confirmed_by,omitempty"`
	ConfirmedAt string   `json:"confirmed_at,omitempty"`
	Hash        string   `json:"hash,omitempty"`
	BodySHA256  string   `json:"body_sha256,omitempty"`
	Matches     bool     `json:"matches"`
	Sections    []string `json:"sections,omitempty"`

	sectionIndex map[string]bool
}

var (
	// A trailing dot is accepted (`## 1. 交付形态`) because authors write
	// both forms; the captured number stays the section id either way.
	restatementHeadingRe = regexp.MustCompile(`^##\s+(\d+(?:\.\d+)*)\.?(?:\s|$)`)
	restatementACRowRe   = regexp.MustCompile(`^\|\s*(AC-\d{3})\s*\|`)
	restatesTokenRe      = regexp.MustCompile(`(?i)restates\s*=\s*([0-9]+(?:\.[0-9]+)*)`)
)

// ParseRestatement parses a restatement document.
func ParseRestatement(text string) Restatement {
	r := Restatement{Present: true, sectionIndex: map[string]bool{}}
	if scalars, _, ok := ParseFrontmatter(text); ok {
		r.Slug = scalars["slug"]
		r.ConfirmedBy = strings.TrimSpace(scalars["confirmed_by"])
		r.ConfirmedAt = strings.TrimSpace(scalars["confirmed_at"])
		r.Hash = strings.ToLower(strings.TrimSpace(scalars["hash"]))
	}
	body, ok := restatementBody(text)
	if !ok {
		// No frontmatter fence: the whole document is the body, so a
		// file that lost its frontmatter still reports its real body
		// hash (and reads as unconfirmed, never as matching).
		body = text
	}
	sum := sha256.Sum256([]byte(body))
	r.BodySHA256 = hex.EncodeToString(sum[:])
	r.Matches = r.Hash != "" && r.BodySHA256 == r.Hash
	r.Sections = restatementSections(body)
	for _, s := range r.Sections {
		r.sectionIndex[s] = true
	}
	return r
}

// LoadRestatement reads the restatement document at absPath and labels
// it with relPath. A missing file is not an error: it returns a
// not-present Restatement carrying the path, so the caller can explain
// which file is missing.
func LoadRestatement(absPath, relPath string) (Restatement, error) {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Restatement{Path: relPath, sectionIndex: map[string]bool{}}, nil
		}
		return Restatement{}, err
	}
	r := ParseRestatement(string(raw))
	r.Path = relPath
	return r, nil
}

// restatementBody returns the raw bytes after the closing frontmatter
// fence (the newline that ends the fence belongs to the fence).
func restatementBody(text string) (string, bool) {
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return "", false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return "", false
	}
	rest := text[4+end+4:]
	rest = strings.TrimPrefix(rest, "\r")
	if !strings.HasPrefix(rest, "\n") {
		return "", false
	}
	return rest[1:], true
}

// restatementSections returns the numbered headings in document order
// (`1`, `3.2`, ...), de-duplicated.
func restatementSections(body string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSuffix(line, "\r")
		m := restatementHeadingRe.FindStringSubmatch(line)
		if m == nil || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

// HasSection reports whether section no is present. A parent number is
// also satisfied by its subsections: `3` is present when `## 3.1`
// exists, matching the heading-prefix rule. A subsection is only
// satisfied by itself.
func (r Restatement) HasSection(no string) bool {
	no = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(no), "#"))
	if no == "" {
		return false
	}
	if r.sectionIndex[no] {
		return true
	}
	if strings.Contains(no, ".") {
		return false
	}
	prefix := no + "."
	for s := range r.sectionIndex {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// MissingSections lists the required sections that are absent.
func (r Restatement) MissingSections() []string {
	var missing []string
	for _, no := range RestatementRequiredSections {
		if !r.HasSection(no) {
			missing = append(missing, no)
		}
	}
	return missing
}

// ValidateRestatementForFreeze applies the freeze-time contract and
// returns the reason codes in check order (empty means clean). A
// package that has never been frozen must carry a restatement; a
// refreeze keeps the legacy waiver and returns no codes at all.
func ValidateRestatementForFreeze(r Restatement, firstFreeze bool) []string {
	if !r.Present {
		if firstFreeze {
			return []string{core.ReasonRestatementRequired}
		}
		return nil
	}
	var codes []string
	if r.ConfirmedBy == "" {
		codes = append(codes, core.ReasonRestatementUnconfirmed)
	}
	if !r.Matches {
		codes = append(codes, core.ReasonRestatementStale)
	}
	if len(r.MissingSections()) > 0 {
		codes = append(codes, core.ReasonRestatementSectionsMissing)
	}
	return codes
}

// RestatementWarnings names the waived defects for a refreeze (or an
// old package read): the freeze is not blocked, but the state is not
// silent either.
func RestatementWarnings(r Restatement) []string {
	if !r.Present {
		return []string{"restatement: missing (legacy waiver: only a first freeze requires it)"}
	}
	var out []string
	if r.ConfirmedBy == "" {
		out = append(out, "restatement: confirmed_by is empty (waived on refreeze)")
	}
	if !r.Matches {
		out = append(out, "restatement: body hash does not match the recorded hash (waived on refreeze)")
	}
	if missing := r.MissingSections(); len(missing) > 0 {
		out = append(out, "restatement: missing sections "+strings.Join(missing, ", ")+" (waived on refreeze)")
	}
	return out
}

// ParseACRestates reads the Acceptance Criteria table and maps each AC
// id to the section numbers its `restates=` tokens name. Rows outside
// the Acceptance Criteria section (the Intent Coverage table) are not
// collected.
func ParseACRestates(truthText string) map[string][]string {
	out := map[string][]string{}
	inAC := false
	for _, line := range strings.Split(truthText, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "#") {
			level := len(line) - len(strings.TrimLeft(line, "#"))
			title := strings.ToLower(strings.TrimSpace(strings.TrimLeft(line, "#")))
			if level <= 2 {
				inAC = strings.HasPrefix(title, "acceptance criteria")
			}
			continue
		}
		if !inAC {
			continue
		}
		m := restatementACRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if _, ok := out[m[1]]; !ok {
			out[m[1]] = nil
		}
		for _, tok := range restatesTokenRe.FindAllStringSubmatch(line, -1) {
			out[m[1]] = append(out[m[1]], tok[1])
		}
	}
	return out
}

// UntraceableACs lists the Acceptance Criteria rows that carry no
// `restates=` token or whose tokens name no section of the restatement.
// Sorted so the finding order is stable.
func UntraceableACs(truthText string, r Restatement) []string {
	acs := ParseACRestates(truthText)
	ids := make([]string, 0, len(acs))
	for id := range acs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var bad []string
	for _, id := range ids {
		sections := acs[id]
		if len(sections) == 0 {
			bad = append(bad, id)
			continue
		}
		ok := false
		for _, no := range sections {
			if r.HasSection(no) {
				ok = true
				break
			}
		}
		if !ok {
			bad = append(bad, id)
		}
	}
	return bad
}
