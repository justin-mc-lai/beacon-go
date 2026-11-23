package gate

import (
	"reflect"
	"testing"
)

func TestPackageRequiredMembersAndCoverageAuthority(t *testing.T) {
	for _, tc := range []struct {
		name     string
		contract string
		members  []string
		coverage string
	}{
		{"legacy", "", []string{"truth", "tests", "tasks", "evidence"}, "tests.md"},
		{"sdd", "truth-tasks", []string{"truth", "tasks"}, "truth.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := PackageFiles{
				Contract: tc.contract,
				Truth:    PackageFile{Path: "truth.md", Present: true},
				Tests:    PackageFile{Path: "tests.md", Present: true},
				Tasks:    PackageFile{Path: "tasks.md", Present: true},
				Evidence: PackageFile{Path: "evidence.md", Present: true},
			}
			var names []string
			for _, member := range pkg.RequiredMembers() {
				names = append(names, member.Name)
				if !member.File.Present || member.File.Path != member.Name+".md" {
					t.Fatalf("required member lost its file: %+v", member)
				}
			}
			if !reflect.DeepEqual(names, tc.members) {
				t.Fatalf("members = %v, want %v", names, tc.members)
			}
			if got := pkg.CoverageFile().Path; got != tc.coverage {
				t.Fatalf("coverage authority = %s, want %s", got, tc.coverage)
			}
		})
	}
}

func TestPackageTraceabilityRequired(t *testing.T) {
	for _, tc := range []struct {
		name     string
		contract string
		setting  string
		want     bool
	}{
		{"legacy default", "", "", false},
		{"legacy opt in", "", "traceability_required: true\n", true},
		{"legacy opt out", "", "traceability_required: false\n", false},
		{"sdd default", "truth-tasks", "", true},
		{"sdd explicit true", "truth-tasks", "traceability_required: true\n", true},
		{"sdd cannot bypass conformance", "truth-tasks", "traceability_required: false\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := PackageFiles{Contract: tc.contract, Truth: PackageFile{Text: "---\n" + tc.setting + "---\n# Truth\n"}}
			if got := pkg.TraceabilityRequired(); got != tc.want {
				t.Fatalf("TraceabilityRequired() = %v, want %v", got, tc.want)
			}
		})
	}
}
