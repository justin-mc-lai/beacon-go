package orchestrator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// SDDMigrationOptions never supplies generated commands or inferred dependencies.
// TasksFile is an author-supplied executable DAG, required for checkbox tasks.
type SDDMigrationOptions struct {
	DryRun    bool
	By        string
	TasksFile string
}

// SDDMigrationReport describes filesystem changes only; migration does not sign,
// freeze, register a review, or rewrite the preserved legacy package members.
type SDDMigrationReport struct {
	Feature       string `json:"feature"`
	Contract      string `json:"package_contract"`
	DryRun        bool   `json:"dry_run"`
	AlreadySDD    bool   `json:"already_sdd"`
	TruthPath     string `json:"truth_path"`
	TasksPath     string `json:"tasks_path"`
	CoverageRows  int    `json:"coverage_rows"`
	TasksReplaced bool   `json:"tasks_replaced"`
	BackupDir     string `json:"backup_dir,omitempty"`
	By            string `json:"by"`
	Next          string `json:"next"`
}

func migrationError(detail string) error {
	return core.NewError("truth_migrate_blocked", "truth.MigrateSDD", detail)
}

// MigrateTruthSDD adopts the two-file contract without discarding legacy source
// material. All validation precedes writes; truth is committed last, after task
// staging, and a failed commit restores the original tasks atomically.
func (e *Engine) MigrateTruthSDD(ctx store.FeatureContext, opts SDDMigrationOptions) (SDDMigrationReport, error) {
	report := SDDMigrationReport{Feature: ctx.Feature.Slug, Contract: SDDContract, DryRun: opts.DryRun, By: strings.TrimSpace(opts.By), Next: "fill grounding/risk and applicable SDD sections; obtain independent semantic review and fresh human confirmation before freeze/refreeze"}
	if report.By == "" || strings.ContainsAny(report.By, "\r\n") {
		return report, migrationError("--by must identify the migration author on one line")
	}
	feat, err := e.store.Features().GetByID(ctx.Feature.ID)
	if err != nil {
		return report, err
	}
	ctx.Feature = feat
	pkg := e.packageFiles(ctx)
	report.TruthPath, report.TasksPath = pkg.Truth.Path, pkg.Tasks.Path
	if !pkg.Truth.Present || !pkg.Tasks.Present {
		return report, migrationError("canonical truth and tasks must both exist")
	}
	scalars, _, ok := gate.ParseFrontmatter(pkg.Truth.Text)
	if !ok {
		return report, migrationError("truth requires valid frontmatter")
	}
	if pkg.Contract != "" && pkg.Contract != SDDContract {
		return report, migrationError("unknown package contract " + pkg.Contract)
	}
	pending, err := e.pendingChangeIDs(feat.ID)
	if err != nil {
		return report, err
	}
	if len(pending) > 0 {
		for _, id := range pending {
			if id == "" || filepath.Base(id) != id {
				return report, migrationError("invalid pending change identity")
			}
			rel := filepath.Join(filepath.Dir(feat.TruthPath), "changes", id+".md")
			changePath, pathErr := migrationPath(ctx.Project.RootPath, rel)
			if pathErr != nil {
				return report, pathErr
			}
			raw, readErr := os.ReadFile(changePath)
			if readErr != nil {
				return report, migrationError("pending change document is unavailable: " + id)
			}
			change, _, valid := gate.ParseFrontmatter(string(raw))
			if !valid || change["change_id"] != id || change["slug"] != feat.Slug || change["status"] != "pending" {
				return report, migrationError("pending change document does not bind this package: " + id)
			}
		}
	} else if scalars["status"] == "frozen" || feat.Status == "frozen" || scalars["status"] == "change_pending" || feat.Status == "change_pending" {
		return report, migrationError("frozen/change_pending packages require a genuine open truth change before migration")
	}
	if _, err := e.requireCanonicalBranch(ctx.Project.RootPath, "truth.MigrateSDD"); err != nil {
		return report, err
	}
	truthPath, err := migrationPath(ctx.Project.RootPath, pkg.Truth.Path)
	if err != nil {
		return report, err
	}
	tasksPath, err := migrationPath(ctx.Project.RootPath, pkg.Tasks.Path)
	if err != nil {
		return report, err
	}
	if truthPath == tasksPath {
		return report, migrationError("truth and tasks paths must be distinct")
	}
	for _, member := range []gate.PackageFile{pkg.Tests, pkg.Evidence, pkg.Restatement} {
		if !member.Present || (pkg.Contract == SDDContract && member.Path == pkg.Truth.Path) {
			continue
		}
		memberPath, pathErr := migrationPath(ctx.Project.RootPath, member.Path)
		if pathErr != nil {
			return report, pathErr
		}
		memberInfo, statErr := os.Stat(memberPath)
		if statErr != nil {
			return report, statErr
		}
		for _, writable := range []string{truthPath, tasksPath} {
			info, statErr := os.Stat(writable)
			if statErr != nil {
				return report, statErr
			}
			if os.SameFile(info, memberInfo) {
				return report, migrationError("writable member aliases preserved legacy material")
			}
		}
	}
	newTasks := pkg.Tasks.Text
	if opts.TasksFile != "" {
		raw, readErr := os.ReadFile(opts.TasksFile)
		if readErr != nil {
			return report, readErr
		}
		newTasks = string(raw)
	}
	acs := gate.DefinedACs(pkg.Truth.Text)
	newDAG, err := gate.ParseTaskDAG(newTasks, acs)
	if err != nil {
		return report, migrationError("supply --tasks-file with a real executable DAG; " + err.Error())
	}
	if opts.TasksFile != "" && newTasks != pkg.Tasks.Text {
		oldDAG, oldErr := gate.ParseTaskDAG(pkg.Tasks.Text, acs)
		if oldErr == nil && !reflect.DeepEqual(oldDAG, newDAG) {
			return report, migrationError("supplied DAG conflicts with existing executable tasks")
		}
		if oldErr != nil && strings.Contains(pkg.Tasks.Text, "TASK") && strings.Contains(pkg.Tasks.Text, "|") {
			return report, migrationError("existing executable task table is invalid; resolve its conflict explicitly before migration")
		}
	}
	if pkg.Contract == SDDContract {
		if newTasks != pkg.Tasks.Text {
			return report, migrationError("already SDD; edit tasks through normal authoring/change flow")
		}
		_, rows, coverageErr := migrationCoverage(pkg.Truth.Text)
		if coverageErr != nil {
			return report, coverageErr
		}
		if len(rows) == 0 {
			return report, migrationError("existing SDD coverage matrix is missing")
		}
		report.AlreadySDD, report.CoverageRows = true, len(rows)
		return report, nil
	}
	if !pkg.Tests.Present || pkg.Tests.Path == pkg.Truth.Path {
		return report, migrationError("legacy canonical tests file is missing or aliases truth")
	}
	tables, rows, err := migrationCoverage(pkg.Tests.Text)
	if err != nil {
		return report, err
	}
	report.CoverageRows = len(rows)
	newTruth := pkg.Truth.Text
	existing, existingRows, err := migrationCoverage(pkg.Truth.Text)
	if err != nil {
		return report, err
	}
	if len(existing) > 0 {
		if !reflect.DeepEqual(existingRows, rows) {
			return report, migrationError("truth Coverage Matrix conflicts with legacy tests")
		}
	} else {
		for _, line := range strings.Split(newTruth, "\n") {
			if strings.EqualFold(strings.TrimSpace(line), "## Coverage Matrix") {
				return report, migrationError("existing Coverage Matrix has no safely translatable table")
			}
		}
		newTruth = strings.TrimRight(newTruth, "\n") + "\n\n## Coverage Matrix\n\n" + strings.Join(tables, "\n\n") + "\n"
	}
	if len(rows) == 0 {
		return report, migrationError("legacy tests have no coverage rows")
	}
	// Clear signatures in truth only. Legacy restatement/tests/evidence files stay
	// byte-for-byte intact, but cannot authorize the new SDD contract.
	for _, pair := range [][2]string{
		{"package_contract", SDDContract}, {"package_maturity", "scaffold"},
		{"intent_confirmed", "false"},
		{"sdd_review_by", ""}, {"sdd_review_at", ""}, {"sdd_review_verdict", ""}, {"sdd_review_fingerprint", ""},
		{"confirmed_by", ""}, {"confirmed_at", ""}, {"hash", ""},
		{"intent_confirmed_by", ""}, {"intent_confirmed_at", ""},
	} {
		newTruth, _ = setFrontmatterScalar(newTruth, pair[0], pair[1])
	}
	status := "draft"
	if len(pending) > 0 {
		status = "change_pending"
	}
	newTruth, _ = setFrontmatterScalar(newTruth, "status", status)
	report.TasksReplaced = newTasks != pkg.Tasks.Text
	if opts.DryRun {
		return report, nil
	}
	backupRoot, err := migrationPath(ctx.Project.RootPath, ".beacon/migrations")
	if err != nil {
		return report, err
	}
	if err = os.MkdirAll(backupRoot, 0o755); err != nil {
		return report, err
	}
	backup, err := os.MkdirTemp(backupRoot, "sdd-")
	if err != nil {
		return report, err
	}
	report.BackupDir = backup
	// Keep exact originals and a durable report even if staging or commit fails.
	for _, file := range []struct{ name, text string }{{"truth.md", pkg.Truth.Text}, {"tasks.md", pkg.Tasks.Text}} {
		if err = os.WriteFile(filepath.Join(backup, file.name), []byte(file.text), 0o600); err != nil {
			return report, err
		}
	}
	manifest, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return report, err
	}
	if err = os.WriteFile(filepath.Join(backup, "report.json"), manifest, 0o600); err != nil {
		return report, err
	}
	if err = commitSDDMigration(truthPath, tasksPath, []byte(pkg.Truth.Text), []byte(pkg.Tasks.Text), []byte(newTruth), []byte(newTasks), os.Rename); err != nil {
		return report, err
	}
	return report, nil
}

// migrationPath rejects aliases and escapes rather than following symlinked
// canonical members or a symlinked backup directory outside the project.
func migrationPath(root, rel string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := resolvePkgPath(root, rel)
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	within, err := filepath.Rel(root, path)
	if err != nil || within == "." || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return "", migrationError("canonical migration paths must be inside project root")
	}
	current := root
	for _, part := range strings.Split(within, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			return "", statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", migrationError("symlinked migration paths are not supported: " + current)
		}
	}
	return path, nil
}

// migrationCoverage preserves complete table bytes, not a re-rendered subset of
// TCRow. Strict shape/row checks prevent ParseCoverageMatrix's permissive parser
// from silently skipping a malformed row or discarding an unknown column.
func migrationCoverage(text string) ([]string, []gate.TCRow, error) {
	var tables []string
	var all []gate.TCRow
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := migrationCells(line)
		isMatrix, hasTC, hasContractColumn := false, false, false
		for _, cell := range cells {
			if strings.EqualFold(cell, "AC ID") {
				isMatrix = true
			}
			if strings.EqualFold(cell, "TC ID") {
				hasTC = true
			}
			if strings.EqualFold(cell, "Command") || strings.EqualFold(cell, "Assertion") {
				hasContractColumn = true
			}
		}
		if isMatrix && !hasTC && hasContractColumn {
			return nil, nil, migrationError("coverage table is missing TC ID")
		}
		if !isMatrix || !hasTC {
			continue
		}
		if len(cells) < 5 || len(cells) > 7 {
			return nil, nil, migrationError("coverage table has unsupported columns")
		}
		for j, want := range []string{"AC ID", "TC ID", "Layer", "Command", "Assertion"} {
			if !strings.EqualFold(cells[j], want) {
				return nil, nil, migrationError("coverage table columns must be AC ID, TC ID, Layer, Command, Assertion")
			}
		}
		seen := map[string]bool{}
		for _, cell := range cells[5:] {
			key := strings.ToLower(cell)
			if (key != "evidence" && key != "required tests") || seen[key] {
				return nil, nil, migrationError("coverage table has unsupported or duplicate columns")
			}
			seen[key] = true
		}
		start, count := i, 0
		for i++; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" {
				continue // The authoritative parser keeps matrices active across blank lines.
			}
			if !strings.HasPrefix(trimmed, "|") {
				break
			}
			row := migrationCells(trimmed)
			newHeader := false
			for _, cell := range row {
				if strings.EqualFold(cell, "AC ID") {
					newHeader = true
				}
			}
			if newHeader {
				break // Let the outer loop validate the next matrix's complete schema.
			}
			if len(row) != len(cells) {
				return nil, nil, migrationError("coverage table row has a different column count")
			}
			separator := true
			for _, cell := range row {
				if cell == "" || strings.Trim(cell, "-:") != "" {
					separator = false
				}
			}
			if separator {
				continue
			}
			if !migrationACID(row[0]) || !sddTCID.MatchString(row[1]) {
				return nil, nil, migrationError("coverage table has an untranslatable AC/TC row")
			}
			count++
		}
		table := strings.Join(lines[start:i], "\n")
		rows, _, found := gate.ParseCoverageMatrix(table)
		if !found || count == 0 || len(rows) != count {
			return nil, nil, migrationError("coverage table cannot be copied without losing rows")
		}
		for j := range rows {
			rows[j].Line = 0
		}
		tables, all = append(tables, table), append(all, rows...)
		i--
	}
	// Compare against the entire authoritative source, not only the table slices
	// selected above, so no row accepted by the consumer can disappear silently.
	original, _, _ := gate.ParseCoverageMatrix(text)
	for i := range original {
		original[i].Line = 0
	}
	if !reflect.DeepEqual(original, all) {
		return nil, nil, migrationError("coverage extraction differs from the complete authoritative matrix")
	}
	copied, _, _ := gate.ParseCoverageMatrix(strings.Join(tables, "\n\n"))
	for i := range copied {
		copied[i].Line = 0
	}
	if !reflect.DeepEqual(original, copied) {
		return nil, nil, migrationError("copied coverage differs from the complete authoritative matrix")
	}
	return tables, all, nil
}

func migrationACID(id string) bool {
	if !strings.HasPrefix(id, "AC-") || len(id) < 6 {
		return false
	}
	for _, r := range id[3:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func migrationCells(line string) []string {
	var cells []string
	start, code := 1, false
	for i := 1; i < len(line); i++ {
		if line[i] == '`' {
			code = !code
		}
		if line[i] == '|' && !code {
			cells = append(cells, strings.TrimSpace(line[start:i]))
			start = i + 1
		}
	}
	if start < len(line) {
		cells = append(cells, strings.TrimSpace(line[start:]))
	}
	return cells
}

func commitSDDMigration(truthPath, tasksPath string, oldTruth, oldTasks, truth, tasks []byte, rename func(string, string) error) error {
	stage := func(path string, data []byte) (string, error) {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		f, err := os.CreateTemp(filepath.Dir(path), ".sdd-migrate-")
		if err != nil {
			return "", err
		}
		name := f.Name()
		if err = f.Chmod(info.Mode().Perm()); err == nil {
			_, err = f.Write(data)
		}
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(name)
			return "", err
		}
		return name, nil
	}
	truthStage, err := stage(truthPath, truth)
	if err != nil {
		return err
	}
	defer os.Remove(truthStage)
	taskStage, err := stage(tasksPath, tasks)
	if err != nil {
		return err
	}
	defer os.Remove(taskStage)
	rollback, err := stage(tasksPath, oldTasks)
	if err != nil {
		return err
	}
	defer os.Remove(rollback)
	for _, file := range []struct {
		path string
		old  []byte
	}{{truthPath, oldTruth}, {tasksPath, oldTasks}} {
		raw, readErr := os.ReadFile(file.path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(raw, file.old) {
			return migrationError("package changed during migration; nothing committed")
		}
	}
	if err = rename(taskStage, tasksPath); err != nil {
		return err
	}
	if err = rename(truthStage, truthPath); err != nil {
		if restoreErr := rename(rollback, tasksPath); restoreErr != nil {
			return migrationError(fmt.Sprintf("truth commit failed: %v; task rollback failed: %v; restore exact originals from reported backup directory before continuing", err, restoreErr))
		}
		return err
	}
	return nil
}
