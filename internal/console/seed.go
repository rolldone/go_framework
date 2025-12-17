package console

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go_framework/internal/admin/services"
	"go_framework/internal/db"
	"go_framework/internal/plugins"
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
		svc := services.NewServices(gdb)
		if err := runServiceSeed(svc); err != nil {
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

func runServiceSeed(svc *services.AdminServices) error {
	return seedPlugins(svc)
}

func seedPlugins(svc *services.AdminServices) error {
	switch seedPlugin {
	case "core":
		return nil
	case "all":
		return plugins.SeedAll(svc)
	default:
		for _, p := range plugins.RegisteredPlugins() {
			if p.ID() == seedPlugin {
				return p.Seed(svc)
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
