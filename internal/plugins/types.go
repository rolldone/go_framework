package plugins

import (
	"go_framework/internal/admin/services"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// MiddlewareDescriptor describes a plugin-provided middleware.
type MiddlewareDescriptor struct {
	Name     string          // unique middleware id
	Target   string          // "global", "admin", or "store"
	Priority int             // lower runs earlier
	Handler  gin.HandlerFunc // actual middleware function
}

// Plugin defines the hooks a plugin can implement.
type Plugin interface {
	ID() string
	RegisterServices(svcs *services.AdminServices) error
	RegisterMiddleware() []MiddlewareDescriptor
	RegisterRoutes(router *gin.Engine, admin *gin.RouterGroup, store *gin.RouterGroup, svcs *services.AdminServices) error
	Seed(svcs *services.AdminServices) error
	ConsoleCommands() []*cobra.Command
}
