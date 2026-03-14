package console

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go_framework/internal/db"
	"go_framework/internal/plugins"
	"go_framework/internal/storage"

	"gorm.io/gorm"
)

var (
	seedFile   string
	useSQL     bool
	seedPlugin string
)

const (
	sampleRegionName    = "Global (USD)"
	sampleCurrency      = "USD"
	sampleVariantSKU    = "UST-TEE-001"
	samplePriceListName = "Staging Price List"
)

func init() {
	seedCmd.Flags().StringVar(&seedFile, "file", "docs/design/seed-staging.sql", "path to SQL seed file")
	seedCmd.Flags().BoolVar(&useSQL, "sql", false, "run SQL seed file instead of service-based seed")
	seedCmd.Flags().StringVar(&seedPlugin, "plugin", "core", "run seeds for core, all plugins, or a specific plugin id")
	rootCmd.AddCommand(seedCmd)
}

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database (service-based by default, SQL file via --sql)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if useSQL {
			return runSQLSeed()
		}
		gdb, err := db.GetGormDB()
		if err != nil {
			return fmt.Errorf("connect to database: %w", err)
		}
		if err := runServiceSeed(gdb); err != nil {
			return err
		}
		fmt.Println("seed applied via services")
		return nil
	},
}

func runSQLSeed() error {
	content, err := os.ReadFile(seedFile)
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}
	dbConn, err := db.GetDB()
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer dbConn.Close()

	if _, err := dbConn.Exec(string(content)); err != nil {
		return fmt.Errorf("apply seed %s: %w", seedFile, err)
	}
	fmt.Printf("seed applied: %s\n", seedFile)
	return nil
}

func runServiceSeed(gdb *gorm.DB) error {
	// Try to initialize storage; non-fatal for seeding
	var store storage.Store
	if cfg, err := storage.LoadConfig(); err == nil {
		if s, err := storage.NewStore(cfg); err == nil {
			store = s
		} else {
			fmt.Printf("[WARN] storage init failed, continuing without store: %v\n", err)
		}
	} else {
		fmt.Printf("[WARN] storage config load failed, continuing without store: %v\n", err)
	}
	deps := plugins.ServiceDeps{DB: gdb, Store: store}
	// ensure plugins can register services with deps
	if err := plugins.RegisterAllServices(deps.DB, deps.Store); err != nil {
		return err
	}
	return seedPlugins()
}

func seedPlugins() error {
	switch seedPlugin {
	case "core":
		return nil
	case "all":
		return plugins.SeedAll()
	default:
		for _, p := range plugins.RegisteredPlugins() {
			if p.ID() == seedPlugin {
				return p.Seed()
			}
		}
		return fmt.Errorf("plugin %q is not registered", seedPlugin)
	}
}

func strPtr(v string) *string {
	return &v
}

func optionalStringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
