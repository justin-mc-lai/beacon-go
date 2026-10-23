package core

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// Package fingerprint (P4c; the Go landing of v1.6.3
// beacon-spec-fingerprint-package-auth-paths-v163).
//
// The v1 bug: the spec fingerprint only scanned legacy layer
// directories, so for package-authoritative packages it ignored the
// truth body entirely and spec-drift detection was blind to truth
// edits. The Go kernel has no separate fingerprint surface; what it
// needs is the same guarantee in the surfaces it already has:
//
//   - at freeze/refreeze time the engine records a fingerprint of
//     the package BODY (truth.md + tests.md) in the freeze event;
//   - doctor rebuild recomputes it from the on-disk package and
//     reports a truth_body_drift when the body moved without the
//     change/refreeze flow.
//
// The fingerprint covers package-authoritative paths only (the
// package's own truth.md/tests.md) — there is no legacy layer scan
// to leave stale. The frontmatter `status:` line is normalized
// out: flipping draft->frozen is the freeze action itself, not
// body drift; every other byte counts.

// FingerprintAlg tags the algorithm recorded alongside a
// fingerprint so future algorithm changes stay detectable.
const FingerprintAlg = "pkgfp-v1"

// NormalizeTruthForFingerprint returns the truth bytes with the
// frontmatter status line's value replaced by a fixed marker. Only
// the leading frontmatter block (--- ... ---) is touched; a
// document without frontmatter is returned unchanged.
func NormalizeTruthForFingerprint(raw []byte) []byte {
	text := string(raw)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return raw
	}
	lines := strings.Split(text, "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return raw
	}
	for i := 1; i < end; i++ {
		trimmed := strings.TrimRight(lines[i], "\r")
		if strings.HasPrefix(trimmed, "status:") {
			suffix := ""
			if strings.HasSuffix(lines[i], "\r") {
				suffix = "\r"
			}
			lines[i] = "status: *" + suffix
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// PackageFingerprint hashes a package body: the status-normalized
// truth bytes plus the tests bytes, domain-separated by the
// algorithm tag.
func PackageFingerprint(truthRaw, testsRaw []byte) string {
	h := sha256.New()
	h.Write([]byte(FingerprintAlg))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(truthRaw))
	h.Write([]byte{0})
	h.Write(testsRaw)
	return hex.EncodeToString(h.Sum(nil))
}

// SurfaceFingerprintAlg tags the four-surface package fingerprint
// (S1, AC-137): the refreeze transaction must prove at least one
// requirement surface — truth, tests, tasks or evidence — changed
// content since the last freeze. The recorded PackageFingerprint
// covers only truth+tests, so freeze/refreeze additionally record
// this wider fingerprint; the truth member is status-normalized
// exactly as in PackageFingerprint (the freeze flip itself is not
// a content change).
const SurfaceFingerprintAlg = "pkgsurface-v1"

// PackageSurfaceFingerprint hashes all four requirement surfaces
// of a package (status-normalized truth + tests + tasks +
// evidence), domain-separated by the algorithm tag.
func PackageSurfaceFingerprint(truthRaw, testsRaw, tasksRaw, evidenceRaw []byte) string {
	h := sha256.New()
	h.Write([]byte(SurfaceFingerprintAlg))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(truthRaw))
	h.Write([]byte{0})
	h.Write(testsRaw)
	h.Write([]byte{0})
	h.Write(tasksRaw)
	h.Write([]byte{0})
	h.Write(evidenceRaw)
	return hex.EncodeToString(h.Sum(nil))
}

// RequirementFingerprintAlg tags the per-AC fingerprint map recorded
// at freeze (R1): requirement truth must be self-checking at the
// requirement level, not only at the package level, so a later drift
// report can name WHICH acceptance criterion moved.
const RequirementFingerprintAlg = "reqfp-v1"

var acStartRe = regexp.MustCompile(`\*\*AC-(\d{3})\*\*|^\|\s*AC-(\d{3})\s*\|`)

// RequirementFingerprints returns one stable hash per AC defined in a
// truth document. A bold definition block runs from its line to the
// next AC definition or the next `## ` heading; a table row is one
// line. Multiple occurrences of the same AC are concatenated, so a
// change in any occurrence moves that AC's fingerprint. The map is
// empty when the document defines no ACs.
func RequirementFingerprints(truthRaw []byte) map[string]string {
	text := string(NormalizeTruthForFingerprint(truthRaw))
	lines := strings.Split(text, "\n")
	blocks := map[string][]string{}
	var cur string
	var buf []string
	flush := func() {
		if cur == "" {
			return
		}
		blocks[cur] = append(blocks[cur], strings.Join(buf, "\n"))
		buf = nil
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			cur = ""
			continue
		}
		if m := acStartRe.FindStringSubmatch(line); m != nil {
			flush()
			id := m[1]
			if id == "" {
				id = m[2]
			}
			cur = "AC-" + id
			buf = []string{strings.TrimRight(line, " \t\r")}
			// A table row is complete on its own line.
			if strings.HasPrefix(strings.TrimSpace(line), "|") {
				flush()
				cur = ""
			}
			continue
		}
		if cur != "" {
			buf = append(buf, strings.TrimRight(line, " \t\r"))
		}
	}
	flush()
	out := make(map[string]string, len(blocks))
	for id, parts := range blocks {
		h := sha256.New()
		h.Write([]byte(RequirementFingerprintAlg))
		h.Write([]byte{0})
		h.Write([]byte(id))
		h.Write([]byte{0})
		h.Write([]byte(strings.Join(parts, "\n")))
		out[id] = hex.EncodeToString(h.Sum(nil))
	}
	return out
}
