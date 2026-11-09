package progressreview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// RevisionLength is the hex length of a SHA256 contract revision.
const RevisionLength = sha256.Size * 2

// NormalizeContractRevision canonicalizes a goal contract revision. A
// revision is a SHA256 hex digest; upper case input is accepted and
// lowered so the same goal always reproduces the same binding.
func NormalizeContractRevision(raw string) (string, error) {
	rev := strings.ToLower(strings.TrimSpace(raw))
	if rev == "" {
		return "", fmt.Errorf("progress review: contract revision is empty")
	}
	if len(rev) != RevisionLength {
		return "", fmt.Errorf("progress review: contract revision must be %d hex characters", RevisionLength)
	}
	if _, err := hex.DecodeString(rev); err != nil {
		return "", fmt.Errorf("progress review: contract revision must be sha256 hex: %w", err)
	}
	return rev, nil
}

// DeriveContractRevision returns the stable SHA256 revision for a goal
// contract body, so a pinned goal reproduces the same revision without
// trusting the caller's formatting.
func DeriveContractRevision(contract string) string {
	sum := sha256.Sum256([]byte(contract))
	return hex.EncodeToString(sum[:])
}
