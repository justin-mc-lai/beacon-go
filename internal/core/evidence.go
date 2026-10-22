// EvidenceChain hash-link semantics (AC-037/038).
//
// The frozen truth fixes the chain's shape (manifest_hash, signature_chain,
// ordered artifact_refs) and the fail-closed code EVIDENCE_CHAIN_BROKEN,
// but does not pin a hash formula. The formula below is the Go track's
// explicit choice, kept deterministic and documented here:
//
//	manifest_hash   = sha256hex(join(artifact_refs, "\n"))
//	signature[0]    = sha256hex(manifest_hash + "|" + artifact_refs[0])
//	signature[i>0]  = sha256hex(signature[i-1] + "|" + artifact_refs[i])
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ComputeManifestHash derives the manifest hash for an ordered ref list.
func ComputeManifestHash(artifactRefs []string) string {
	return sha256hex(strings.Join(artifactRefs, "\n"))
}

// ComputeSignature derives the signature for position i.
func ComputeSignature(prev string, artifactRef string) string {
	return sha256hex(prev + "|" + artifactRef)
}

// NewEvidenceChain builds a well-formed chain over the ordered refs.
func NewEvidenceChain(artifactRefs []string) EvidenceChain {
	chain := EvidenceChain{ArtifactRefs: append([]string(nil), artifactRefs...)}
	chain.ManifestHash = ComputeManifestHash(chain.ArtifactRefs)
	prev := chain.ManifestHash
	for _, ref := range chain.ArtifactRefs {
		sig := ComputeSignature(prev, ref)
		chain.SignatureChain = append(chain.SignatureChain, sig)
		prev = sig
	}
	return chain
}

// Verify checks the hash chain end to end (AC-037/038). An empty chain, a
// length mismatch, a manifest mismatch or any broken link fails closed
// with reason_code=EVIDENCE_CHAIN_BROKEN.
func (c EvidenceChain) Verify() error {
	broken := func(detail string) error {
		return NewError(ReasonEvidenceChainBroken, "EvidenceChain.Verify", detail)
	}
	if len(c.ArtifactRefs) == 0 {
		return broken("empty chain: no artifact refs")
	}
	if c.ManifestHash == "" {
		return broken("empty manifest_hash")
	}
	if len(c.SignatureChain) != len(c.ArtifactRefs) {
		return broken("signature_chain length does not match artifact_refs length")
	}
	if ComputeManifestHash(c.ArtifactRefs) != c.ManifestHash {
		return broken("manifest_hash does not match artifact_refs")
	}
	prev := c.ManifestHash
	for i, ref := range c.ArtifactRefs {
		want := ComputeSignature(prev, ref)
		if c.SignatureChain[i] != want {
			return broken("signature link broken at position " + strconv.Itoa(i))
		}
		prev = c.SignatureChain[i]
	}
	return nil
}
