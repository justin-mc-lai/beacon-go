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

// NormalizeTruthForFingerprint returns the document bytes with
// the frontmatter bookkeeping lines' values replaced by a fixed
// marker: `status:` (the freeze flip itself) and, since R2a,
// `materials_status:` (the kernel's own stale/current marking,
// M7). Both are kernel bookkeeping, not requirement content —
// letting them move the fingerprint would make the kernel's own
// metadata writes masquerade as body drift. Only the leading
// frontmatter block (--- ... ---) is touched; a document without
// frontmatter is returned unchanged.
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
		for _, key := range []string{"status:", "materials_status:"} {
			if strings.HasPrefix(trimmed, key) {
				suffix := ""
				if strings.HasSuffix(lines[i], "\r") {
					suffix = "\r"
				}
				lines[i] = key + " *" + suffix
			}
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// PackageFingerprint hashes a package body: the bookkeeping-
// normalized truth bytes plus the bookkeeping-normalized tests
// bytes, domain-separated by the algorithm tag.
func PackageFingerprint(truthRaw, testsRaw []byte) string {
	h := sha256.New()
	h.Write([]byte(FingerprintAlg))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(truthRaw))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(testsRaw))
	return hex.EncodeToString(h.Sum(nil))
}

// SurfaceFingerprintAlg tags the legacy four-surface package
// fingerprint (S1, AC-137). It stays only so freeze records written
// before the contract/evidence split still compare fresh; new freeze
// records use ContractSurfaceFingerprintAlg.
const SurfaceFingerprintAlg = "pkgsurface-v1"

// ContractSurfaceFingerprintAlg tags the contract-face fingerprint
// (kernel-lake-freeze-closure AC-001): the surfaces whose change
// requires the governed change -> refreeze flow. Evidence is the
// byproduct of implementation and is deliberately outside it.
const ContractSurfaceFingerprintAlg = "pkgcontract-v1"

// PackageSurfaceFingerprint hashes all four requirement surfaces
// of a package (bookkeeping-normalized truth + tests + tasks +
// evidence), domain-separated by the algorithm tag. Legacy: freeze
// records written before the contract split carry this value.
func PackageSurfaceFingerprint(truthRaw, testsRaw, tasksRaw, evidenceRaw []byte) string {
	h := sha256.New()
	h.Write([]byte(SurfaceFingerprintAlg))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(truthRaw))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(testsRaw))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(tasksRaw))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(evidenceRaw))
	return hex.EncodeToString(h.Sum(nil))
}

// PackageContractFingerprint hashes the contract face: truth + tests
// + tasks + restatement. These are the files a change flow governs;
// evidence.md is excluded because implementation appends to it
// (AC-001).
func PackageContractFingerprint(truthRaw, testsRaw, tasksRaw, restatementRaw []byte) string {
	h := sha256.New()
	h.Write([]byte(ContractSurfaceFingerprintAlg))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(truthRaw))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(testsRaw))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(tasksRaw))
	h.Write([]byte{0})
	h.Write(NormalizeTruthForFingerprint(restatementRaw))
	return hex.EncodeToString(h.Sum(nil))
}

// PackageSurfaceDrifted reports whether the live package moved off the
// recorded freeze fingerprint. A legacy pkgsurface-v1 record (four
// surfaces including evidence, before restatement was tracked) still
// compares fresh via its own hash, so upgrading the kernel does not
// fake a drift on packages frozen by an older build.
func PackageSurfaceDrifted(recorded string, truthRaw, testsRaw, tasksRaw, evidenceRaw, restatementRaw []byte) bool {
	if recorded == "" {
		return false
	}
	if recorded == PackageContractFingerprint(truthRaw, testsRaw, tasksRaw, restatementRaw) {
		return false
	}
	return recorded != PackageSurfaceFingerprint(truthRaw, testsRaw, tasksRaw, evidenceRaw)
}

// RequirementFingerprintAlg tags the per-AC fingerprint map recorded
// at freeze (R1): requirement truth must be self-checking at the
// requirement level, not only at the package level, so a later drift
// report can name WHICH acceptance criterion moved.
const RequirementFingerprintAlg = "reqfp-v1"

// AC ids may carry a domain prefix (AC-HTTP-001, AC-S1-003) next to the
// plain AC-001 form. The table / bold forms are otherwise unchanged so
// the existing three-digit contract (and the ids that do not use it)
// keep their current matching behaviour.
var acStartRe = regexp.MustCompile(`\*\*AC-((?:[A-Z0-9]+-)*\d{3})\*\*|^\|\s*AC-((?:[A-Z0-9]+-)*\d{3})\s*\|`)

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
