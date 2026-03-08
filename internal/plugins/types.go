package plugins

import (
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

// Plugin defines the hooks a plugin can implement.
type Plugin interface {
	ID() string
	RegisterServices(db *gorm.DB) error
	RegisterMiddleware() []MiddlewareDescriptor
	RegisterRoutes(router *gin.Engine, admin *gin.RouterGroup, api *gin.RouterGroup, db *gorm.DB) error
	Seed(db *gorm.DB) error
	ConsoleCommands() []*cobra.Command
}
