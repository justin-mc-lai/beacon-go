package intenttree

import "path/filepath"

// lockDirForTree keeps runtime locks outside the canonical tree. dir is
// <project>/docs/beacon/<version>/tree, so four parents up is the project.
func lockDirForTree(dir string) string {
	project := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(dir))))
	return filepath.Join(project, ".beacon", "locks")
}

func lockPathForTree(dir string) string {
	return filepath.Join(lockDirForTree(dir), "tree-change.lock")
}
