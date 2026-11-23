package gate

import "strings"

// NamedPackageFile associates a required package member with its contract name.
type NamedPackageFile struct {
	Name string
	File PackageFile
}

// RequiredMembers returns the files whose absence makes the package incomplete.
func (p PackageFiles) RequiredMembers() []NamedPackageFile {
	if p.Contract == "truth-tasks" {
		return []NamedPackageFile{{Name: "truth", File: p.Truth}, {Name: "tasks", File: p.Tasks}}
	}
	return []NamedPackageFile{
		{Name: "truth", File: p.Truth},
		{Name: "tests", File: p.Tests},
		{Name: "tasks", File: p.Tasks},
		{Name: "evidence", File: p.Evidence},
	}
}

// CoverageFile returns the contract's authoritative AC/TC matrix document.
func (p PackageFiles) CoverageFile() PackageFile {
	if p.Contract == "truth-tasks" {
		return p.Truth
	}
	return p.Tests
}

// TraceabilityRequired keeps SDD conformance fail-closed even for an invalid
// explicit opt-out; SDD validation rejects that declaration separately.
// Legacy packages retain their opt-in behavior.
func (p PackageFiles) TraceabilityRequired() bool {
	if p.Contract == "truth-tasks" {
		return true
	}
	scalars, _, ok := ParseFrontmatter(p.Truth.Text)
	return ok && strings.EqualFold(strings.TrimSpace(scalars["traceability_required"]), "true")
}
