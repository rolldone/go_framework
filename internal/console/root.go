package console

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go_framework/internal/pluginloader"
	"go_framework/internal/plugins"
)

var rootCmd = &cobra.Command{
	Use:   "console",
	Short: "Console tools for Umahstore",
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
	// register core plugins and their console commands
	pluginloader.RegisterCorePlugins()
	plugins.RegisterConsoleCommands(rootCmd)
}
