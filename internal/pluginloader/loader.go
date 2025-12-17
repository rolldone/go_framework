package pluginloader

import (
	"go_framework/internal/plugins"
)

// RegisterCorePlugins registers the set of core plugins supported by Umahstore.
func RegisterCorePlugins() {
	plugins.RegisterPlugins([]plugins.Plugin{
		// Add core plugins here
		// e.g., coreplugin.New(),
	})
}
