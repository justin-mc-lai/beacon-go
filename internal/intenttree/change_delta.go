package intenttree

import buildversion "beacon-go/internal/version"

// bindChangeDelta is shared by all governed change writers. Empty sets are
// explicit; dependency scope is the ancestor closure, not a business graph.
func bindChangeDelta(rec *ChangeRecord, idx Index, action string, added, modified, deprecated, dependencies []string) {
	rec.Action = action
	rec.Added = append([]string{}, added...)
	rec.Modified = append([]string{}, modified...)
	rec.Deprecated = append([]string{}, deprecated...)
	rec.DependencyScope = append([]string{}, dependencies...)
	rec.SourceHash = SourceHash(idx)
	runtime := buildversion.Runtime()
	rec.SourceCommit, rec.BinarySHA = runtime.Commit, runtime.BinarySHA
}
