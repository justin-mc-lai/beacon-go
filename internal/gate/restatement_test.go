package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const restatementTestBody = "## 1 交付形态\n\n只加解析器。\n\n" +
	"## 2 界面与交互\n\nCLI。\n\n" +
	"## 3 业务流转\n\n## 3.1 首次冻结\n\n强制。\n\n## 3.2 重新冻结\n\n豁免。\n\n" +
	"## 4 边界与不做\n\n不改语义。\n\n" +
	"## 5 歧义与默认值\n\n无。\n"

func signedRestatementDoc(slug, confirmedBy, body, hash string) string {
	if hash == "" {
		sum := sha256.Sum256([]byte(body))
		hash = hex.EncodeToString(sum[:])
	}
	return "---\nslug: " + slug + "\nconfirmed_by: " + confirmedBy +
		"\nconfirmed_at: 2026-11-13T04:39:57Z\nhash: " + hash + "\n---\n" + body
}

// TC-001: the parser reads the four frontmatter keys, hashes the body
// after the closing fence byte for byte, and matches the five section
// headings with subsections counted under their parent.
func TestParseRestatement(t *testing.T) {
	doc := signedRestatementDoc("rest-gate", "justin-mc-lai", restatementTestBody, "")
	r := ParseRestatement(doc)

	if !r.Present {
		t.Fatal("parsed document must be present")
	}
	if r.Slug != "rest-gate" || r.ConfirmedBy != "justin-mc-lai" || r.ConfirmedAt != "2026-11-13T04:39:57Z" {
		t.Fatalf("frontmatter = slug %q by %q at %q", r.Slug, r.ConfirmedBy, r.ConfirmedAt)
	}
	sum := sha256.Sum256([]byte(restatementTestBody))
	want := hex.EncodeToString(sum[:])
	if r.Hash != want || r.BodySHA256 != want {
		t.Fatalf("hash = %q body = %q, want %q", r.Hash, r.BodySHA256, want)
	}
	if !r.Matches {
		t.Fatal("an untouched body must match its recorded hash")
	}
	// The body is exactly the bytes after the closing fence: recompute
	// it through the same accessor and hash it independently.
	body, ok := restatementBody(doc)
	if !ok || body != restatementTestBody {
		t.Fatalf("body extraction = %q (ok=%v)", body, ok)
	}
	if sum2 := sha256.Sum256([]byte(body)); hex.EncodeToString(sum2[:]) != r.BodySHA256 {
		t.Fatal("body hash is not the raw bytes after the fence")
	}
	if got := strings.Join(r.Sections, ","); got != "1,2,3,3.1,3.2,4,5" {
		t.Fatalf("sections = %s, want 1,2,3,3.1,3.2,4,5", got)
	}
	if missing := r.MissingSections(); len(missing) != 0 {
		t.Fatalf("missing sections = %v", missing)
	}
	if !r.HasSection("3") || !r.HasSection("3.1") || !r.HasSection("3.2") {
		t.Fatal("section 3 and its subsections must resolve")
	}
	if r.HasSection("6") || r.HasSection("3.3") {
		t.Fatal("absent sections must not resolve")
	}

	t.Run("subsection satisfies its parent", func(t *testing.T) {
		body := "## 1 a\n## 2 b\n## 3.1 only a subsection\n## 4 d\n## 5 e\n"
		r := ParseRestatement(signedRestatementDoc("sub", "justin", body, ""))
		if !r.HasSection("3") {
			t.Fatal("3.1 must satisfy section 3")
		}
		if missing := r.MissingSections(); len(missing) != 0 {
			t.Fatalf("missing = %v, want none", missing)
		}
		if r.HasSection("3.2") {
			t.Fatal("3.1 must not satisfy the sibling 3.2")
		}
	})

	t.Run("one edited character is stale", func(t *testing.T) {
		edited := strings.Replace(doc, "只加解析器", "只加解析器 ", 1)
		r := ParseRestatement(edited)
		if r.Matches {
			t.Fatal("an edited body must not match the recorded hash")
		}
		if r.BodySHA256 == r.Hash {
			t.Fatalf("body hash %q must differ from recorded %q", r.BodySHA256, r.Hash)
		}
		if missing := r.MissingSections(); len(missing) != 0 {
			t.Fatalf("edit must not be reported as a missing section: %v", missing)
		}
	})

	t.Run("dotted heading numbers", func(t *testing.T) {
		body := "## 1. 交付形态\n## 2. 界面\n## 3. 流转\n## 3.1. 首次冻结\n## 4. 边界\n## 5. 默认值\n"
		r := ParseRestatement(signedRestatementDoc("dotted", "justin", body, ""))
		if missing := r.MissingSections(); len(missing) != 0 {
			t.Fatalf("dotted headings must satisfy the five sections: %v", missing)
		}
		if !r.HasSection("3.1") {
			t.Fatal("dotted subsection must resolve")
		}
	})

	t.Run("unsigned", func(t *testing.T) {
		r := ParseRestatement(signedRestatementDoc("rest-gate", "", restatementTestBody, ""))
		if r.ConfirmedBy != "" {
			t.Fatalf("confirmed_by = %q, want empty", r.ConfirmedBy)
		}
	})

	t.Run("no frontmatter", func(t *testing.T) {
		r := ParseRestatement(restatementTestBody)
		if !r.Present || r.Hash != "" || r.Matches {
			t.Fatalf("frontmatter-less parse = %+v", r)
		}
		sum := sha256.Sum256([]byte(restatementTestBody))
		if r.BodySHA256 != hex.EncodeToString(sum[:]) {
			t.Fatal("a frontmatter-less file still hashes its whole text")
		}
	})
}
