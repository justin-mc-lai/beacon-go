// Projects registry commands (AC-021): register / list / show over the
// global DB's projects + project_versions tables. Register is idempotent
// on the canonical (resolved) root path.
package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"beacon-go/internal/store"
)

// projectView is the projects JSON schema (AC-065): a project row plus
// its derived project_key, current version string and full versions[].
type projectView struct {
	ID             string                 `json:"id"`
	ProjectKey     string                 `json:"project_key"`
	RootPath       string                 `json:"root_path"`
	Name           string                 `json:"name"`
	RegisteredAt   string                 `json:"registered_at"`
	CurrentVersion string                 `json:"current_version"`
	Versions       []store.ProjectVersion `json:"versions"`
}

func buildProjectView(st *store.Store, p store.Project) (projectView, error) {
	versions, err := st.Projects().ListVersions(p.ID)
	if err != nil {
		return projectView{}, err
	}
	if versions == nil {
		versions = []store.ProjectVersion{}
	}
	view := projectView{
		ID: p.ID, ProjectKey: store.ProjectKey(p.RootPath), RootPath: p.RootPath,
		Name: p.Name, RegisteredAt: p.RegisteredAt, Versions: versions,
	}
	for _, v := range versions {
		if v.IsCurrent {
			view.CurrentVersion = v.Version
		}
	}
	return view, nil
}

func newProjectsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "projects",
		Short: "Machine-level multi-project registry (AC-021)",
	}
	c.AddCommand(newProjectsRegisterCmd(), newProjectsListCmd(), newProjectsShowCmd())
	return c
}

func newProjectsRegisterCmd() *cobra.Command {
	var path, name, version, docsRoot string
	var asJSON bool
	c := &cobra.Command{
		Use:   "register",
		Short: "Register a project root (idempotent); optionally record its current version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			root := store.ResolveRootPath(path)
			p, err := st.Projects().GetByRootPath(root)
			if errors.Is(err, sql.ErrNoRows) {
				regName := name
				if regName == "" {
					regName = filepath.Base(root)
				}
				p, err = st.Projects().Register(store.Project{RootPath: root, Name: regName})
				if err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			// Already registered: return the existing record unchanged
			// (idempotent); only the version bookkeeping below may add.
			if version != "" {
				if err := ensureVersion(st, p, version, docsRoot); err != nil {
					return err
				}
			}
			view, err := buildProjectView(st, p)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "registered %s %s (id %s, current %s)\n",
				view.ProjectKey, view.RootPath, view.ID, view.CurrentVersion)
			return nil
		},
	}
	c.Flags().StringVar(&path, "path", ".", "project root path")
	c.Flags().StringVar(&name, "name", "", "project display name (default: directory base name)")
	c.Flags().StringVar(&version, "version", "", "also record this docs version and mark it current")
	c.Flags().StringVar(&docsRoot, "docs-root", "", "docs root for --version (default docs/beacon/<version>)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

// ensureVersion records (project, version) once and marks it current.
func ensureVersion(st *store.Store, p store.Project, version, docsRoot string) error {
	versions, err := st.Projects().ListVersions(p.ID)
	if err != nil {
		return err
	}
	for _, v := range versions {
		if v.Version == version {
			if !v.IsCurrent {
				return st.Projects().SetCurrent(v.ID)
			}
			return nil
		}
	}
	if docsRoot == "" {
		docsRoot = filepath.Join("docs", "beacon", version)
	}
	_, err = st.Projects().AddVersion(store.ProjectVersion{
		ProjectID: p.ID, Version: version, DocsRoot: docsRoot, IsCurrent: true,
	})
	return err
}

func newProjectsListCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List all registered projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				if asJSON {
					return printJSON(cmd, map[string]any{"projects": []projectView{}})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no projects registered")
				return nil
			}
			defer st.Close()
			projects, err := st.Projects().List()
			if err != nil {
				return err
			}
			views := []projectView{}
			for _, p := range projects {
				view, err := buildProjectView(st, p)
				if err != nil {
					return err
				}
				views = append(views, view)
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"projects": views})
			}
			if len(views) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no projects registered")
				return nil
			}
			for _, v := range views {
				fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-40s current=%s\n", v.ProjectKey, v.RootPath, v.CurrentVersion)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newProjectsShowCmd() *cobra.Command {
	var project string
	var asJSON bool
	c := &cobra.Command{
		Use:   "show",
		Short: "Show one registered project with versions[] and is_current (AC-021)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			root := store.ResolveRootPath(project)
			if !exists {
				return fail(ReasonProjectNotRegistered, "projects show", "no runtime DB yet, project not registered: "+root)
			}
			defer st.Close()
			p, err := st.Projects().GetByRootPath(root)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fail(ReasonProjectNotRegistered, "projects show", "project not registered: "+root)
				}
				return err
			}
			view, err := buildProjectView(st, p)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "project:  %s\nkey:      %s\nroot:     %s\ncurrent:  %s\n",
				view.Name, view.ProjectKey, view.RootPath, view.CurrentVersion)
			for _, v := range view.Versions {
				mark := ""
				if v.IsCurrent {
					mark = " (current)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "version:  %s docs=%s%s\n", v.Version, v.DocsRoot, mark)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
