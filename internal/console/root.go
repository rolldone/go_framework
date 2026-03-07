package console

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"go_framework/internal/pluginloader"
	"go_framework/internal/plugins"
)

var rootCmd = &cobra.Command{
	Use:   "console",
	Short: "Console tools",
}

// Execute executes the root command using process args.
func Execute() {
	if err := Run(os.Args[1:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Run executes the root command with the provided arguments.
func Run(args []string) error {
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// RegisterAdditionalPlugins registers additional plugins and their console commands.
func RegisterAdditionalPlugins(p []plugins.Plugin) {
	plugins.RegisterPlugins(p)
	plugins.RegisterConsoleCommands(rootCmd)
}

// NewRootCmd returns the configured root cobra command.
func NewRootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	// load .env using project standard (godotenv). Use Overload to match other packages
	// which ensures .env values are loaded for console commands.
	_ = godotenv.Overload()

	// set short description from APP_NAME env (loaded from .env or process env)
	appName := os.Getenv("APP_NAME")
	if appName == "" {
		appName = "unknown app"
	}
	rootCmd.Short = fmt.Sprintf("Console tools for %s", appName)

	// register core plugins and their console commands
	pluginloader.RegisterCorePlugins()
	plugins.RegisterConsoleCommands(rootCmd)
}
