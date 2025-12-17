package plugins

import (
	"sort"

	"go_framework/internal/admin/services"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var registered []Plugin

// RegisteredPlugins returns the plugins currently registered.
func RegisteredPlugins() []Plugin {
	return registered
}

// RegisterPlugins adds plugins to the registry; call once during bootstrap.
func RegisterPlugins(p []Plugin) {
	registered = append(registered, p...)
}

// RegisterAllServices lets plugins extend the service bundle.
func RegisterAllServices(svcs *services.AdminServices) error {
	for _, p := range registered {
		if err := p.RegisterServices(svcs); err != nil {
			return err
		}
	}
	return nil
}

// AttachMiddleware collects plugin middleware, sorts by priority, and attaches
// to the specified router groups.
func AttachMiddleware(routers map[string]*gin.RouterGroup) {
	var all []MiddlewareDescriptor
	for _, p := range registered {
		all = append(all, p.RegisterMiddleware()...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Priority < all[j].Priority
	})
	for _, md := range all {
		grp, ok := routers[md.Target]
		if !ok || md.Handler == nil {
			continue
		}
		grp.Use(md.Handler)
	}
}

// RegisterAllRoutes lets plugins attach routes to the shared routers.
func RegisterAllRoutes(router *gin.Engine, admin *gin.RouterGroup, store *gin.RouterGroup, svcs *services.AdminServices) error {
	for _, p := range registered {
		if err := p.RegisterRoutes(router, admin, store, svcs); err != nil {
			return err
		}
	}
	return nil
}

// SeedAll allows plugins to seed data if desired.
func SeedAll(svcs *services.AdminServices) error {
	for _, p := range registered {
		if err := p.Seed(svcs); err != nil {
			return err
		}
	}
	return nil
}

// RegisterConsoleCommands lets plugins add Cobra commands to the root CLI.
func RegisterConsoleCommands(root *cobra.Command) {
	for _, p := range registered {
		for _, cmd := range p.ConsoleCommands() {
			if cmd != nil {
				root.AddCommand(cmd)
			}
		}
	}
}
