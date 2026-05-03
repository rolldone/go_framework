package console

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"go_framework/internal/db"
	"go_framework/internal/plugins"
)

const (
	migrationTableSQL = `CREATE TABLE IF NOT EXISTS migrations (
    id BIGSERIAL PRIMARY KEY,
    target TEXT NOT NULL,
    version INTEGER NOT NULL,
    name TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (target, version)
);`

	migrationTargetsSQL = `CREATE TABLE IF NOT EXISTS migration_targets (
    target TEXT PRIMARY KEY,
    dirty BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
)

var (
	migratePluginFlag string
	migrateDBFlag     string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration utilities (core + plugins, tracked in DB)",
	Long: `Manage database migrations for the core application and its plugins.
State is tracked in the migrations/migration_targets tables instead of YAML.`,
}

var migrateMakeCmd = &cobra.Command{
	Use:   "make <name>",
	Short: "Create a new migration pair (up/down)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if migratePluginFlag == "all" {
			return errors.New("use --plugin core or a specific plugin when creating a migration")
		}
		name := sanitizeName(args[0])
		t, err := singleTarget(migratePluginFlag, migrateDBFlag)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(t.Path, 0o755); err != nil {
			return err
		}
		// Use UTC timestamp prefix like Laravel (YYYY_MM_DD_HHMMSS)
		now := time.Now().UTC().Format("2006_01_02_150405")
		base := fmt.Sprintf("%s_%s", now, name)
		upPath := filepath.Join(t.Path, base+".up.sql")
		downPath := filepath.Join(t.Path, base+".down.sql")
		if err := os.WriteFile(upPath, []byte("-- write your UP migration here\n"), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(downPath, []byte("-- write your DOWN migration here\n"), 0o644); err != nil {
			return err
		}
		fmt.Printf("Created migration files:\n%s\n%s\n", upPath, downPath)
		return nil
	},
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply pending migrations (core then plugins)",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets, err := collectTargets(migratePluginFlag, migrateDBFlag)
		if err != nil {
			return err
		}
		for _, t := range targets {
			if err := applyUp(t); err != nil {
				return fmt.Errorf("%s: %w", t.Name, err)
			}
		}
		return nil
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback one migration (plugins last)",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets, err := collectTargets(migratePluginFlag, migrateDBFlag)
		if err != nil {
			return err
		}
		if migratePluginFlag == "all" || migratePluginFlag == "" {
			ok, err := applyDownGlobal(targets)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("all: nothing to rollback")
			}
			return nil
		}
		for i := len(targets) - 1; i >= 0; i-- {
			if err := applyDown(targets[i]); err != nil {
				return fmt.Errorf("%s: %w", targets[i].Name, err)
			}
		}
		return nil
	},
}

var migrateDownAllCmd = &cobra.Command{
	Use:   "down-all",
	Short: "Rollback all migrations (plugins first, then core)",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets, err := collectTargets(migratePluginFlag, migrateDBFlag)
		if err != nil {
			return err
		}
		if migratePluginFlag == "all" || migratePluginFlag == "" {
			return applyDownAllGlobal(targets)
		}
		for i := len(targets) - 1; i >= 0; i-- {
			if err := applyDownAll(targets[i]); err != nil {
				return fmt.Errorf("%s: %w", targets[i].Name, err)
			}
		}
		return nil
	},
}

var migrateListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show migration status per target",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets, err := collectTargets(migratePluginFlag, migrateDBFlag)
		if err != nil {
			return err
		}
		for _, t := range targets {
			dbConn, err := db.GetGormDB()
			if err != nil {
				return err
			}
			if err := ensureMigrationTables(dbConn); err != nil {
				return err
			}
			if err := ensureTargetEntry(dbConn, t.Name); err != nil {
				return err
			}
			current, err := getCurrentVersion(dbConn, t.Name)
			if err != nil {
				return err
			}
			dirty, err := isTargetDirty(dbConn, t.Name)
			if err != nil {
				return err
			}
			files, err := listMigrationFiles(t.Path)
			if err != nil {
				return fmt.Errorf("%s: %w", t.Name, err)
			}
			// Determine applied migration names to compute pending
			rows, err := dbConn.Raw(`SELECT name FROM migrations WHERE target = ? ORDER BY version ASC`, t.Name).Rows()
			if err != nil {
				return err
			}
			defer rows.Close()
			applied := map[string]bool{}
			for rows.Next() {
				var nm string
				if err := rows.Scan(&nm); err != nil {
					return err
				}
				applied[nm] = true
			}
			pending := 0
			for _, f := range files {
				if !applied[f.Name] {
					pending++
				}
			}
			fmt.Printf("%s: current=%d dirty=%v pending=%d\n", t.Name, current, dirty, pending)
		}
		return nil
	},
}

type migrationTarget struct {
	Name string
	Path string
}

func init() {
	migrateCmd.PersistentFlags().StringVar(&migratePluginFlag, "plugin", "core", "target plugin (core, plugin id, or all); defaults to core-only commands")
	migrateCmd.PersistentFlags().StringVar(&migrateDBFlag, "db", "", "override detected database type (postgres, mysql)")
	migrateCmd.AddCommand(migrateMakeCmd, migrateUpCmd, migrateDownCmd, migrateDownAllCmd, migrateListCmd)
	rootCmd.AddCommand(migrateCmd)
}

func singleTarget(target, dbType string) (*migrationTarget, error) {
	targets, err := collectTargets(target, dbType)
	if err != nil {
		return nil, err
	}
	if len(targets) != 1 {
		return nil, fmt.Errorf("expected one target, got %d", len(targets))
	}
	return &targets[0], nil
}

func collectTargets(target, dbType string) ([]migrationTarget, error) {
	if dbType == "" {
		detected, err := detectDBType()
		if err != nil {
			return nil, err
		}
		dbType = detected
	}
	dbType = strings.ToLower(dbType)
	if dbType != "postgres" && dbType != "mysql" {
		return nil, fmt.Errorf("db %s not supported yet", dbType)
	}
	var res []migrationTarget
	wantAll := target == "all" || target == ""

	if wantAll || target == "core" {
		path := filepath.Join("migrations", dbType)
		if exists(path) {
			res = append(res, migrationTarget{Name: "core", Path: path})
		}
	}

	pluginRegistered := false
	for _, p := range plugins.RegisteredPlugins() {
		if target == p.ID() {
			pluginRegistered = true
		}
		if !wantAll && target != p.ID() {
			continue
		}
		path := filepath.Join("plugins", p.ID(), "migrations", dbType)
		if !exists(path) {
			continue
		}
		res = append(res, migrationTarget{Name: p.ID(), Path: path})
	}

	if len(res) == 0 {
		if !pluginRegistered && target != "core" && target != "all" {
			return nil, fmt.Errorf("plugin %q is not registered; add it in cmd/server/main.go or cmd/console/main.go before running migrate", target)
		}
		return nil, fmt.Errorf("no migration paths found for target %q", target)
	}
	ordered, err := orderTargetsDynamically(res)
	if err != nil {
		return nil, err
	}
	res = ordered
	return res, nil
}

func orderTargetsDynamically(targets []migrationTarget) ([]migrationTarget, error) {
	if len(targets) <= 1 {
		return targets, nil
	}

	// Keep core first if present; plugin order is resolved dynamically.
	var coreTargets []migrationTarget
	var pluginTargets []migrationTarget
	for _, t := range targets {
		if t.Name == "core" {
			coreTargets = append(coreTargets, t)
			continue
		}
		pluginTargets = append(pluginTargets, t)
	}
	if len(pluginTargets) <= 1 {
		return append(coreTargets, pluginTargets...), nil
	}

	pluginOrder := map[string]int{}
	for idx, p := range plugins.RegisteredPlugins() {
		pluginOrder[p.ID()] = idx
	}

	owners, err := collectTableOwners(pluginTargets)
	if err != nil {
		return nil, err
	}
	deps, err := collectTargetDependencies(pluginTargets, owners)
	if err != nil {
		return nil, err
	}

	nameToTarget := map[string]migrationTarget{}
	indegree := map[string]int{}
	dependents := map[string][]string{}
	for _, t := range pluginTargets {
		nameToTarget[t.Name] = t
		indegree[t.Name] = len(deps[t.Name])
	}
	for tgt, need := range deps {
		for dep := range need {
			dependents[dep] = append(dependents[dep], tgt)
		}
	}

	var queue []string
	for _, t := range pluginTargets {
		if indegree[t.Name] == 0 {
			queue = append(queue, t.Name)
		}
	}
	sort.Slice(queue, func(i, j int) bool { return lessByRegistration(queue[i], queue[j], pluginOrder) })

	var orderedPlugins []migrationTarget
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		orderedPlugins = append(orderedPlugins, nameToTarget[cur])

		for _, nxt := range dependents[cur] {
			indegree[nxt]--
			if indegree[nxt] == 0 {
				queue = append(queue, nxt)
			}
		}
		sort.Slice(queue, func(i, j int) bool { return lessByRegistration(queue[i], queue[j], pluginOrder) })
	}

	if len(orderedPlugins) != len(pluginTargets) {
		var unresolved []string
		for name, d := range indegree {
			if d > 0 {
				unresolved = append(unresolved, name)
			}
		}
		sort.Strings(unresolved)
		return nil, fmt.Errorf("cannot resolve plugin migration order (cyclic dependencies): %s", strings.Join(unresolved, ", "))
	}

	return append(coreTargets, orderedPlugins...), nil
}

func lessByRegistration(a, b string, pluginOrder map[string]int) bool {
	ai, aok := pluginOrder[a]
	bi, bok := pluginOrder[b]
	if aok && bok && ai != bi {
		return ai < bi
	}
	if aok != bok {
		return aok
	}
	return a < b
}

var (
	createTableRegexp = regexp.MustCompile(`(?i)create\s+table(?:\s+if\s+not\s+exists)?\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	referenceRegexp   = regexp.MustCompile(`(?i)references\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
)

func collectTableOwners(targets []migrationTarget) (map[string]string, error) {
	owners := map[string]string{}
	for _, t := range targets {
		files, err := listMigrationFiles(t.Path)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			content, err := os.ReadFile(f.UpPath)
			if err != nil {
				return nil, err
			}
			for _, m := range createTableRegexp.FindAllStringSubmatch(string(content), -1) {
				if len(m) < 2 {
					continue
				}
				tbl := strings.ToLower(m[1])
				if owner, exists := owners[tbl]; exists && owner != t.Name {
					return nil, fmt.Errorf("table %q defined by multiple plugins: %s and %s", tbl, owner, t.Name)
				}
				owners[tbl] = t.Name
			}
		}
	}
	return owners, nil
}

func collectTargetDependencies(targets []migrationTarget, owners map[string]string) (map[string]map[string]struct{}, error) {
	deps := map[string]map[string]struct{}{}
	for _, t := range targets {
		deps[t.Name] = map[string]struct{}{}
		files, err := listMigrationFiles(t.Path)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			content, err := os.ReadFile(f.UpPath)
			if err != nil {
				return nil, err
			}
			for _, m := range referenceRegexp.FindAllStringSubmatch(string(content), -1) {
				if len(m) < 2 {
					continue
				}
				refTbl := strings.ToLower(m[1])
				owner, ok := owners[refTbl]
				if !ok || owner == t.Name {
					continue
				}
				deps[t.Name][owner] = struct{}{}
			}
		}
	}
	return deps, nil
}

func applyUp(t migrationTarget) error {
	files, err := listMigrationFiles(t.Path)
	if err != nil {
		return err
	}
	dbConn, err := db.GetGormDB()
	if err != nil {
		return err
	}
	if err := ensureMigrationTables(dbConn); err != nil {
		return err
	}
	if err := ensureTargetEntry(dbConn, t.Name); err != nil {
		return err
	}
	dirty, err := isTargetDirty(dbConn, t.Name)
	if err != nil {
		return err
	}
	if dirty {
		return errors.New("state is dirty; resolve manually before continuing")
	}
	current, err := getCurrentVersion(dbConn, t.Name)
	if err != nil {
		return err
	}
	// build set of already applied migration names
	rows, err := dbConn.Raw(`SELECT name FROM migrations WHERE target = ? ORDER BY version ASC`, t.Name).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	applied := map[string]bool{}
	for rows.Next() {
		var nm string
		if err := rows.Scan(&nm); err != nil {
			return err
		}
		applied[nm] = true
	}

	for _, f := range files {
		if applied[f.Name] {
			continue
		}
		if err := setTargetDirty(dbConn, t.Name, true); err != nil {
			return err
		}
		if err := execSQLFile(dbConn, f.UpPath); err != nil {
			_ = setTargetDirty(dbConn, t.Name, true)
			return fmt.Errorf("failed applying %s: %w", filepath.Base(f.UpPath), err)
		}
		nextVersion := current + 1
		if err := insertMigrationRecord(dbConn, t.Name, nextVersion, f.Name); err != nil {
			return err
		}
		if err := setTargetDirty(dbConn, t.Name, false); err != nil {
			return err
		}
		current = nextVersion
		fmt.Printf("%s applied %s\n", t.Name, filepath.Base(f.UpPath))
	}
	return nil
}

func applyDown(t migrationTarget) error {
	dbConn, err := db.GetGormDB()
	if err != nil {
		return err
	}
	if err := ensureMigrationTables(dbConn); err != nil {
		return err
	}
	if err := ensureTargetEntry(dbConn, t.Name); err != nil {
		return err
	}
	dirty, err := isTargetDirty(dbConn, t.Name)
	if err != nil {
		return err
	}
	if dirty {
		return errors.New("state is dirty; resolve manually before continuing")
	}
	rec, err := getLastMigrationRecord(dbConn, t.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Printf("%s: nothing to rollback\n", t.Name)
			return nil
		}
		return err
	}
	files, err := listMigrationFiles(t.Path)
	if err != nil {
		return err
	}
	var targetFile *migrationFile
	for _, f := range files {
		if f.Name == rec.Name {
			targetFile = &f
			break
		}
	}
	if targetFile == nil {
		return fmt.Errorf("down file not found for migration %q; restore the missing migration or run repair", rec.Name)
	}
	if err := setTargetDirty(dbConn, t.Name, true); err != nil {
		return err
	}
	if err := execSQLFile(dbConn, targetFile.DownPath); err != nil {
		_ = setTargetDirty(dbConn, t.Name, true)
		return fmt.Errorf("failed applying %s: %w", filepath.Base(targetFile.DownPath), err)
	}
	if err := deleteMigrationRecord(dbConn, t.Name, rec.Version); err != nil {
		return err
	}
	if err := setTargetDirty(dbConn, t.Name, false); err != nil {
		return err
	}
	current, _ := getCurrentVersion(dbConn, t.Name)
	fmt.Printf("%s: rolled back version %d -> current=%d\n", t.Name, rec.Version, current)
	return nil
}

func applyDownAll(t migrationTarget) error {
	for {
		dbConn, err := db.GetGormDB()
		if err != nil {
			return err
		}
		if err := ensureMigrationTables(dbConn); err != nil {
			return err
		}
		if err := ensureTargetEntry(dbConn, t.Name); err != nil {
			return err
		}
		if _, err := getLastMigrationRecord(dbConn, t.Name); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			return err
		}
		if err := applyDown(t); err != nil {
			return err
		}
	}
	dbConn, err := db.GetGormDB()
	if err != nil {
		return err
	}
	current, err := getCurrentVersion(dbConn, t.Name)
	if err != nil {
		return err
	}
	fmt.Printf("%s: rollback complete; current=%d\n", t.Name, current)
	return nil
}

func applyDownGlobal(targets []migrationTarget) (bool, error) {
	if len(targets) == 0 {
		return false, nil
	}
	targetNames := make([]string, 0, len(targets))
	byName := make(map[string]migrationTarget, len(targets))
	for _, t := range targets {
		targetNames = append(targetNames, t.Name)
		byName[t.Name] = t
	}

	dbConn, err := db.GetGormDB()
	if err != nil {
		return false, err
	}
	if err := ensureMigrationTables(dbConn); err != nil {
		return false, err
	}

	type latestRow struct {
		Target string
	}
	var latest latestRow
	err = dbConn.Raw(`
		SELECT target
		FROM migrations
		WHERE target IN ?
		ORDER BY applied_at DESC, id DESC
		LIMIT 1
	`, targetNames).Scan(&latest).Error
	if err != nil {
		return false, err
	}
	if latest.Target == "" {
		return false, nil
	}

	t, ok := byName[latest.Target]
	if !ok {
		return false, fmt.Errorf("migration target %q not found", latest.Target)
	}
	if err := applyDown(t); err != nil {
		return false, err
	}
	return true, nil
}

func applyDownAllGlobal(targets []migrationTarget) error {
	for {
		rolled, err := applyDownGlobal(targets)
		if err != nil {
			return err
		}
		if !rolled {
			break
		}
	}
	fmt.Println("all: rollback complete")
	return nil
}

type migrationFile struct {
	Number   int
	Name     string
	UpPath   string
	DownPath string
}

var migNumRegexp = regexp.MustCompile(`^([0-9_]+)_.*\.up\.sql$`)

func listMigrationFiles(path string) ([]migrationFile, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []migrationFile{}, nil
		}
		return nil, err
	}
	var files []migrationFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matches := migNumRegexp.FindStringSubmatch(e.Name())
		if len(matches) != 2 {
			continue
		}
		// matches[1] may contain underscores for timestamped names (YYYY_MM_DD_HHMMSS)
		digits := strings.ReplaceAll(matches[1], "_", "")
		n64, _ := strconv.ParseInt(digits, 10, 64)
		n := int(n64)
		base := strings.TrimSuffix(e.Name(), ".up.sql")
		upPath := filepath.Join(path, e.Name())
		downPath := filepath.Join(path, base+".down.sql")
		files = append(files, migrationFile{Number: n, Name: base, UpPath: upPath, DownPath: downPath})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Number < files[j].Number })
	return files, nil
}

func nextMigrationNumber(path string) (int, error) {
	files, err := listMigrationFiles(path)
	if err != nil {
		return 0, err
	}
	max := 0
	for _, f := range files {
		if f.Number > max {
			max = f.Number
		}
	}
	return max + 1, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ensureMigrationTables(dbConn *gorm.DB) error {
	if err := dbConn.Exec(migrationTableSQL).Error; err != nil {
		return err
	}
	return dbConn.Exec(migrationTargetsSQL).Error
}

func ensureTargetEntry(dbConn *gorm.DB, target string) error {
	return dbConn.Exec(`INSERT INTO migration_targets (target) VALUES (?) ON CONFLICT (target) DO UPDATE SET updated_at = NOW()`, target).Error
}

func isTargetDirty(dbConn *gorm.DB, target string) (bool, error) {
	var dirty bool
	row := dbConn.Raw(`SELECT dirty FROM migration_targets WHERE target = ?`, target).Row()
	if err := row.Scan(&dirty); err != nil {
		return false, err
	}
	return dirty, nil
}

func setTargetDirty(dbConn *gorm.DB, target string, dirty bool) error {
	return dbConn.Exec(`UPDATE migration_targets SET dirty = ?, updated_at = NOW() WHERE target = ?`, dirty, target).Error
}

func getCurrentVersion(dbConn *gorm.DB, target string) (int, error) {
	row := dbConn.Raw(`SELECT version FROM migrations WHERE target = ? ORDER BY version DESC LIMIT 1`, target).Row()
	var version int
	if err := row.Scan(&version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return version, nil
}

func getLastMigrationRecord(dbConn *gorm.DB, target string) (*migrationRecord, error) {
	var rec migrationRecord
	row := dbConn.Raw(`SELECT version, name FROM migrations WHERE target = ? ORDER BY version DESC LIMIT 1`, target).Row()
	if err := row.Scan(&rec.Version, &rec.Name); err != nil {
		return nil, err
	}
	return &rec, nil
}

type migrationRecord struct {
	Version int
	Name    string
}

func insertMigrationRecord(dbConn *gorm.DB, target string, version int, name string) error {
	return dbConn.Exec(`INSERT INTO migrations (target, version, name) VALUES (?, ?, ?) ON CONFLICT (target, version) DO UPDATE SET name = EXCLUDED.name, applied_at = NOW()`, target, version, name).Error
}

func deleteMigrationRecord(dbConn *gorm.DB, target string, version int) error {
	return dbConn.Exec(`DELETE FROM migrations WHERE target = ? AND version = ?`, target, version).Error
}

func execSQLFile(gdb *gorm.DB, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sql := strings.TrimSpace(string(content))
	if sql == "" {
		return nil
	}
	return gdb.Exec(sql).Error
}

func detectDBType() (string, error) {
	_ = godotenv.Overload()
	if v := os.Getenv("DB_TYPE"); v != "" {
		return strings.ToLower(v), nil
	}
	// MIGRATE_DB_URL not used; rely on DB_TYPE or DB_PORT
	if os.Getenv("DB_HOST") != "" {
		port := os.Getenv("DB_PORT")
		if port == "3306" || port == "33060" {
			return "mysql", nil
		}
		return "postgres", nil
	}
	return "postgres", nil
}

func sanitizeName(name string) string {
	clean := strings.ToLower(name)
	clean = strings.ReplaceAll(clean, " ", "_")
	clean = strings.ReplaceAll(clean, "-", "_")
	return clean
}
