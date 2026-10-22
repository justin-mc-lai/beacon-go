package core

import "testing"

// ---- Protected path denylist (v1.6.5, global-boundaries §6.1) ----

func TestProtectedPathMatch(t *testing.T) {
	cases := []struct {
		path string
		cat  string
		hit  bool
	}{
		{".env", "environment", true},
		{"config/.env.production", "environment", true},
		{"secrets/api.txt", "secrets", true},
		{"app/credentials/db.json", "secrets", true},
		{"certs/server.pem", "secrets", true},
		{"certs/server.key", "secrets", true},
		{"config/api_key.txt", "secrets", true},
		{"src/auth/login.go", "auth", true},
		{"oauth/callback.go", "auth", true},
		{"sso/saml.go", "auth", true},
		{"payments/charge.go", "payments", true},
		{"web/billing/invoice.go", "payments", true},
		{"stripe/webhook.go", "payments", true},
		{"migrations/001_init.sql", "migrations", true},
		{"db/schema_migrations/x.sql", "migrations", true},
		{"src/main.go", "", false},
		{"docs/beacon/v2.0.0/features/x/truth.md", "", false},
		{"src/authors/list.go", "", false},   // segment must be exact
		{"src/keyboard/input.go", "", false}, // no "_key" fragment
		{"", "", false},
	}
	for _, c := range cases {
		cat, hit := ProtectedPathMatch(c.path)
		if cat != c.cat || hit != c.hit {
			t.Errorf("ProtectedPathMatch(%q) = (%q, %v), want (%q, %v)", c.path, cat, hit, c.cat, c.hit)
		}
	}
	if route := HumanRouteForProtectedPath("secrets", "secrets/api.txt"); route == "" {
		t.Error("human route must never be empty (v1 AC-005)")
	}
}

// ---- Failure-mode catalog (v1.6.5) ----

func TestFailureModeCatalogValidAndConsumable(t *testing.T) {
	if err := ValidateFailureModeCatalog(); err != nil {
		t.Fatalf("catalog must validate (v1 AC-002): %v", err)
	}
	if got := len(FailureModeCatalog()); got != 6 {
		t.Fatalf("catalog modes = %d, want the 6 governed v1 modes", got)
	}
	modes := FailureModesForReasons([]string{ReasonStageExitTruthNotFrozen})
	if len(modes) != 1 || modes[0].ID != "truth-drift" {
		t.Errorf("modes for truth-not-frozen = %+v, want truth-drift only", modes)
	}
	modes = FailureModesForReasons([]string{ReasonProtectedPathDenylist, ReasonMaxRepairLoopsExceeded})
	if len(modes) != 2 {
		t.Errorf("modes = %+v, want protected-path-attempt + infinite-fix-loop", modes)
	}
	if modes := FailureModesForReasons([]string{"some_unrelated_code"}); len(modes) != 0 {
		t.Errorf("unknown codes must match nothing, got %+v", modes)
	}
	if modes := FailureModesForReasons(nil); modes != nil {
		t.Errorf("no codes -> no modes, got %+v", modes)
	}
}

// ---- Package fingerprint (v1.6.3 spec fingerprint landing) ----

func TestPackageFingerprintIgnoresStatusFlipOnly(t *testing.T) {
	draft := []byte("---\nslug: x\nstatus: draft\n---\n# Truth\n\nbody v1\n")
	frozen := []byte("---\nslug: x\nstatus: frozen\n---\n# Truth\n\nbody v1\n")
	tests := []byte("# Tests\n")
	if PackageFingerprint(draft, tests) != PackageFingerprint(frozen, tests) {
		t.Error("the freeze status flip alone must not change the fingerprint")
	}
	edited := []byte("---\nslug: x\nstatus: frozen\n---\n# Truth\n\nbody v2 EDITED\n")
	if PackageFingerprint(edited, tests) == PackageFingerprint(frozen, tests) {
		t.Error("a body edit after freeze must change the fingerprint (that is the drift signal)")
	}
	if PackageFingerprint(frozen, []byte("# Tests CHANGED\n")) == PackageFingerprint(frozen, tests) {
		t.Error("a tests.md edit must change the fingerprint")
	}
	// A status line in the BODY (not frontmatter) is content, not the flip.
	bodyStatus := []byte("---\nslug: x\nstatus: frozen\n---\nstatus: draft\n")
	if PackageFingerprint(bodyStatus, tests) == PackageFingerprint(frozen, tests) {
		t.Error("normalization must only touch the frontmatter block")
	}
}
