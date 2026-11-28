package orchestrator

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
)

// Code2TruthIntakeMulti runs one intake per target area. Each area gets
// its own candidate package and the batch returns a region inventory.
func (e *Engine) Code2TruthIntakeMulti(projectRoot, version, baseSlug string, targets []string, by string) (Code2TruthBatchView, error) {
	if len(targets) == 0 {
		return Code2TruthBatchView{}, core.NewError(core.ReasonCode2TruthTargetMissing, "truth.Code2TruthMulti", "at least one target is required")
	}
	out := Code2TruthBatchView{BaseFeature: baseSlug}
	for _, target := range targets {
		abs, err := filepath.Abs(target)
		if err != nil {
			return Code2TruthBatchView{}, err
		}
		area := filepath.Base(abs)
		if area == "." || area == string(filepath.Separator) || area == "" {
			area = "root"
		}
		areaSlug := code2truthSlug(area)
		if areaSlug == "" {
			areaSlug = "area"
		}
		h := sha256.Sum256([]byte(abs + "#" + fmt.Sprint(len(out.Regions))))
		slug := fmt.Sprintf("%s-%s-%x", baseSlug, areaSlug, h[:4])
		view, err := e.Code2TruthIntake(projectRoot, version, slug, target, by)
		if err != nil {
			if rc, _ := core.ReasonCodeOf(err); rc == core.ReasonCode2TruthClaimsEmpty {
				return Code2TruthBatchView{}, core.NewError(core.ReasonCode2TruthClaimsEmpty, "truth.Code2TruthMulti",
					"area "+area+" ("+target+"): "+err.Error())
			}
			return Code2TruthBatchView{}, err
		}
		out.Areas = append(out.Areas, view)
		out.Regions = append(out.Regions, Code2TruthRegion{
			Area: area, Target: abs, Slug: slug,
			Files: view.Inventory.FileCount, Claims: len(view.Claims),
		})
	}
	return out, nil
}

func code2truthSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
