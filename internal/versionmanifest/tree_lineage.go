package versionmanifest

// TreeDelta describes one delivery's contract changes, derived from two explicit
// immutable tree baselines. It is not a second requirement authority.
type TreeDelta struct {
	Added           []string `json:"added"`
	Modified        []string `json:"modified"`
	Deprecated      []string `json:"deprecated"`
	DependencyScope []string `json:"dependency_scope"`
}
