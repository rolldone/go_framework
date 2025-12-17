package console

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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
		nextNum, err := nextMigrationNumber(t.Path)
		if err != nil {
			return err
		}
		base := fmt.Sprintf("%06d_%s", nextNum, name)
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
			pending := 0
			for _, f := range files {
				if f.Number > current {
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
	return res, nil
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
	for _, f := range files {
		if f.Number <= current {
			continue
		}
		if err := setTargetDirty(dbConn, t.Name, true); err != nil {
			return err
		}
		if err := execSQLFile(dbConn, f.UpPath); err != nil {
			_ = setTargetDirty(dbConn, t.Name, true)
			return fmt.Errorf("failed applying %s: %w", filepath.Base(f.UpPath), err)
		}
		if err := insertMigrationRecord(dbConn, t.Name, f.Number, f.Name); err != nil {
			return err
		}
		if err := setTargetDirty(dbConn, t.Name, false); err != nil {
			return err
		}
		current = f.Number
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
		if f.Number == rec.Version {
			targetFile = &f
			break
		}
	}
	if targetFile == nil {
		return fmt.Errorf("down file not found for version %d; restore the missing migration or run repair", rec.Version)
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

type migrationFile struct {
	Number   int
	Name     string
	UpPath   string
	DownPath string
}

var migNumRegexp = regexp.MustCompile(`^(\d+)_.*\.up\.sql$`)

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
		var n int
		_, _ = fmt.Sscanf(matches[1], "%d", &n)
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
