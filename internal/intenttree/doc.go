package intenttree

import (
	"fmt"
	"strings"

	"beacon-go/internal/core"
)

// RequiredFeatureDocSections are the minimum product-spec headings a
// feature must carry before freeze. Touchpoints do not need a PRD.
var RequiredFeatureDocSections = []string{
	"问题",
	"目标用户",
	"场景",
	"范围",
	"非目标",
	"验收标准",
}

// ValidateFeatureDoc rejects a feature body that is only an AC list. The
// headings are deliberately product-level, not implementation-level.
func ValidateFeatureDoc(doc string) error {
	if strings.TrimSpace(doc) == "" {
		return core.NewError("tree_feature_doc_missing", "intenttree.ValidateFeatureDoc", "feature doc is empty")
	}
	for _, section := range RequiredFeatureDocSections {
		needle := "## " + section
		if !strings.Contains(doc, needle) {
			return core.NewError("tree_feature_doc_missing", "intenttree.ValidateFeatureDoc",
				fmt.Sprintf("feature doc missing section %q", needle))
		}
	}
	return nil
}
