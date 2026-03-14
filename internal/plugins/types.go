package plugins

import (
	"go_framework/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// MiddlewareDescriptor describes a plugin-provided middleware.
type MiddlewareDescriptor struct {
	Name     string          // unique middleware id
	Target   string          // "global", "admin", or "store"
	Priority int             // lower runs earlier
	Handler  gin.HandlerFunc // actual middleware function
}

// ServiceDeps groups shared service dependencies passed into plugins.
type ServiceDeps struct {
	DB    *gorm.DB
	Store storage.Store
}

// RouteDeps groups shared route registration dependencies passed into plugins.
// (RouteDeps removed) route-specific router/groups are passed directly to RegisterRoutes.

// Plugin defines the hooks a plugin can implement.
type Plugin interface {
	ID() string
	RegisterServices(deps ServiceDeps) error
	RegisterMiddleware() []MiddlewareDescriptor
	RegisterRoutes(router *gin.Engine, admin *gin.RouterGroup, api *gin.RouterGroup) error
	Seed() error
	ConsoleCommands() []*cobra.Command
}
