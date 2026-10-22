// Package skills embeds the progressive-disclosure skill surface into the
// Go binary ("edit as files, ship as binary"). Phase 0 embeds only the
// placeholder README; the six public SKILL.md packages land in Phase P4.
package skills

import _ "embed"

//go:embed README.md
var Readme string
