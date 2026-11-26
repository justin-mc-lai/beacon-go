package intenttree

import "testing"

// Tree node IDs must be namespaced by project version, not by the version
// string alone. Two different projects can share a docs version (e.g. v0.0.1);
// deriving root/migration IDs from the version string alone collided on the
// global tree_nodes primary key.
func TestShortIDNamespacedByVersionID(t *testing.T) {
	const version = "v0.0.1"
	idA := shortID("nroot_", "ver_projectA")
	idB := shortID("nroot_", "ver_projectB")
	if idA == idB {
		t.Fatalf("different versionIDs produced the same root id %s", idA)
	}
	if again := shortID("nroot_", "ver_projectA"); again != idA {
		t.Fatalf("same versionID not idempotent: %s != %s", again, idA)
	}

	for _, prefix := range []string{"nmod_", "nfeat_", "tmig_"} {
		a := shortID(prefix, "ver_projectA", "legacy")
		b := shortID(prefix, "ver_projectB", "legacy")
		if a == b {
			t.Fatalf("%s: different versionIDs produced the same id %s", prefix, a)
		}
	}

	// The old derivation (version string only) must differ from the new one,
	// so pre-fix callers cannot silently keep the colliding shape.
	if legacy := shortID("nroot_", version); legacy == idA || legacy == idB {
		t.Fatalf("version-only root id %s collides with a namespaced id", legacy)
	}
}
