package console

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

var (
	pluginNewID       string
	pluginNewName     string
	pluginNewTmpl     string
	pluginSkipConsole bool
)

func init() {
	pluginNewCmd.Flags().StringVar(&pluginNewID, "id", "", "plugin id (kebab-case, required)")
	pluginNewCmd.Flags().StringVar(&pluginNewName, "name", "", "display name (optional)")
	pluginNewCmd.Flags().StringVar(&pluginNewTmpl, "template", "minimal", "template type: minimal, crud, middleware")
	pluginNewCmd.Flags().BoolVar(&pluginSkipConsole, "no-console", false, "skip generating console stub")

	pluginCmd.AddCommand(pluginNewCmd)
	rootCmd.AddCommand(pluginCmd)
}

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Plugin utilities",
}

var pluginNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Generate a minimal plugin scaffold",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pluginNewID == "" {
			return errors.New("--id is required")
		}
		tmpl := strings.ToLower(strings.TrimSpace(pluginNewTmpl))
		switch tmpl {
		case "minimal", "crud", "middleware":
		default:
			return fmt.Errorf("unknown template %q (valid: minimal, crud, middleware)", pluginNewTmpl)
		}
		// reject any plugin id that contains whitespace characters
		hasSpace := false
		for _, r := range pluginNewID {
			if unicode.IsSpace(r) {
				hasSpace = true
				break
			}
		}
		if hasSpace {
			return fmt.Errorf("invalid plugin id: %q (no spaces allowed)", pluginNewID)
		}

		id := sanitizePluginID(pluginNewID)
		if id == "" {
			return fmt.Errorf("invalid plugin id: %q", pluginNewID)
		}
		pkgName := packageNameFromID(id)
		display := pluginNewName
		if display == "" {
			display = titleCase(strings.ReplaceAll(id, "-", " "))
		}

		base := filepath.Join("plugins", id)
		paths := []string{filepath.Join(base, "handlers"), filepath.Join(base, "migrations", "postgres")}
		if tmpl == "middleware" {
			paths = append(paths, filepath.Join(base, "middleware"))
		}
		if tmpl == "crud" {
			paths = append(paths, filepath.Join(base, "services"))
		}
		for _, p := range paths {
			// use wide perms and rely on process umask to tighten
			if err := os.MkdirAll(p, 0o777); err != nil {
				return err
			}
		}

		// Files
		var pluginContent string
		switch tmpl {
		case "minimal":
			pluginContent = pluginGoTemplateMinimal(pkgName, id, display, !pluginSkipConsole)
			if err := writeFileIfMissing(filepath.Join(base, "handlers", "health.go"), pluginHandlerTemplate(id)); err != nil {
				return err
			}
		case "crud":
			pluginContent = pluginGoTemplateCRUD(pkgName, id, display, !pluginSkipConsole)
			if err := writeFileIfMissing(filepath.Join(base, "handlers", "resource.go"), pluginCRUDHandlerTemplate(id)); err != nil {
				return err
			}
			if err := writeFileIfMissing(filepath.Join(base, "services", "resource.go"), pluginCRUDServiceTemplate(id)); err != nil {
				return err
			}
		case "middleware":
			pluginContent = pluginGoTemplateMiddleware(pkgName, id, display, !pluginSkipConsole)
			if err := writeFileIfMissing(filepath.Join(base, "middleware", "sample.go"), pluginMiddlewareTemplate(id)); err != nil {
				return err
			}
		}
		if err := writeFileIfMissing(filepath.Join(base, "plugin.go"), pluginContent); err != nil {
			return err
		}
		if err := writeFileIfMissing(filepath.Join(base, "migrations", "postgres", "000001_init.up.sql"), "-- write your UP migration here\n"); err != nil {
			return err
		}
		if err := writeFileIfMissing(filepath.Join(base, "migrations", "postgres", "000001_init.down.sql"), "-- write your DOWN migration here\n"); err != nil {
			return err
		}

		fmt.Printf("plugin scaffold created at %s\n", base)
		fmt.Printf("For server: register the plugin in `cmd/server/main.go` (use the RegisterPlugins hook)\n")
		fmt.Printf("For console: register the plugin in `cmd/console/main.go` using `console.RegisterAdditionalPlugins([]plugins.Plugin{plugin.New()})`\n")
		return nil
	},
}

func sanitizePluginID(in string) string {
	s := strings.ToLower(strings.TrimSpace(in))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	// allow letters, numbers, underscore and dash (prefer underscore for folders)
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevUnderscore = false
		} else if r == '_' || r == '-' {
			// normalize separators to underscore, collapse repeats
			if !prevUnderscore {
				b.WriteRune('_')
				prevUnderscore = true
			}
		}
		// drop other characters
	}
	res := b.String()
	// trim leading/trailing separators
	res = strings.Trim(res, "_-")
	// collapse any accidental multiple underscores (defensive)
	for strings.Contains(res, "__") {
		res = strings.ReplaceAll(res, "__", "_")
	}
	return res
}

func writeFileIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s", path)
	}
	return os.WriteFile(path, []byte(content), 0o666)
}

// packageNameFromID produces a valid Go package name from a plugin id.
func packageNameFromID(id string) string {
	base := strings.Builder{}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			base.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			base.WriteRune(r + ('a' - 'A'))
		}
	}
	res := base.String()
	if res == "" || res[0] >= '0' && res[0] <= '9' {
		res = "plugin" + res
	}
	return res
}

// titleCase does a simple word-level title casing without pulling extra deps.
func titleCase(s string) string {
	parts := strings.Fields(s)
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		runes[0] = toUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	return r
}

func pluginGoTemplateMinimal(pkg, id, display string, includeConsole bool) string {
	consoleBlock := ""
	if includeConsole {
		consoleBlock = fmt.Sprintf(`
	cmd := &cobra.Command{
		Use:   "%s:hello",
		Short: "hello from %s",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("hello from plugin %s\\n")
		},
	}
	return []*cobra.Command{cmd}
`, id, id, id)
	} else {
		consoleBlock = "return nil\n"
	}

	return fmt.Sprintf(`package %s

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go_framework/internal/admin/services"
	"go_framework/internal/plugins"
	pluginhandlers "go_framework/plugins/%s/handlers"
)

// Plugin %s provides a minimal scaffold.
type Plugin struct{}

// New returns a new plugin instance.
func New() plugins.Plugin { return &Plugin{} }

func (p *Plugin) ID() string { return "%s" }

func (p *Plugin) RegisterServices(svcs *services.AdminServices) error { return nil }

func (p *Plugin) RegisterMiddleware() []plugins.MiddlewareDescriptor { return nil }

func (p *Plugin) RegisterRoutes(router *gin.Engine, admin *gin.RouterGroup, front *gin.RouterGroup, svcs *services.AdminServices) error {
	admin.GET("/plugins/%s/health", pluginhandlers.HealthHandler)
    return nil
}

func (p *Plugin) Seed(svcs *services.AdminServices) error { return nil }

func (p *Plugin) ConsoleCommands() []*cobra.Command {
%s}
`, pkg, id, display, id, id, consoleBlock)
}

func pluginGoTemplateCRUD(pkg, id, display string, includeConsole bool) string {
	consoleBlock := ""
	if includeConsole {
		consoleBlock = fmt.Sprintf(`
	cmd := &cobra.Command{
		Use:   "%s:hello",
		Short: "hello from %s",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("hello from plugin %s\\n")
		},
	}
	return []*cobra.Command{cmd}
`, id, id, id)
	} else {
		consoleBlock = "return nil\n"
	}

	return fmt.Sprintf(`package %s

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go_framework/internal/admin/services"
	"go_framework/internal/plugins"
	pluginhandlers "go_framework/plugins/%s/handlers"
)

// Plugin %s provides a CRUD sample scaffold.
type Plugin struct{}

func New() plugins.Plugin { return &Plugin{} }

func (p *Plugin) ID() string { return "%s" }

func (p *Plugin) RegisterServices(svcs *services.AdminServices) error { return nil }

func (p *Plugin) RegisterMiddleware() []plugins.MiddlewareDescriptor { return nil }

func (p *Plugin) RegisterRoutes(router *gin.Engine, admin *gin.RouterGroup, front *gin.RouterGroup, svcs *services.AdminServices) error {
	admin.GET("/plugins/%s/items", pluginhandlers.ListItems)
	admin.POST("/plugins/%s/items", pluginhandlers.CreateItem)
	admin.GET("/plugins/%s/items/:id", pluginhandlers.GetItem)
	return nil
}

func (p *Plugin) Seed(svcs *services.AdminServices) error { return nil }

func (p *Plugin) ConsoleCommands() []*cobra.Command {
%s}
`, pkg, id, display, id, id, id, id, consoleBlock)
}

func pluginGoTemplateMiddleware(pkg, id, display string, includeConsole bool) string {
	consoleBlock := ""
	if includeConsole {
		consoleBlock = fmt.Sprintf(`
	cmd := &cobra.Command{
		Use:   "%s:hello",
		Short: "hello from %s",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("hello from plugin %s\\n")
		},
	}
	return []*cobra.Command{cmd}
`, id, id, id)
	} else {
		consoleBlock = "return nil\n"
	}

	return fmt.Sprintf(`package %s

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go_framework/internal/admin/services"
	"go_framework/internal/plugins"
	pluginmiddleware "go_framework/plugins/%s/middleware"
)

// Plugin %s provides a middleware-only sample scaffold.
type Plugin struct{}

func New() plugins.Plugin { return &Plugin{} }

func (p *Plugin) ID() string { return "%s" }

func (p *Plugin) RegisterServices(svcs *services.AdminServices) error { return nil }

func (p *Plugin) RegisterMiddleware() []plugins.MiddlewareDescriptor {
    return []plugins.MiddlewareDescriptor{{
        Name:     "%s-sample-mw",
        Target:   "admin",
        Priority: 100,
		Handler:  pluginmiddleware.SampleMiddleware,
    }}
}

func (p *Plugin) RegisterRoutes(router *gin.Engine, admin *gin.RouterGroup, front *gin.RouterGroup, svcs *services.AdminServices) error {
    // No routes by default; add as needed
    return nil
}

func (p *Plugin) Seed(svcs *services.AdminServices) error { return nil }

func (p *Plugin) ConsoleCommands() []*cobra.Command {
%s}
`, pkg, id, display, id, id, consoleBlock)
}

func pluginHandlerTemplate(id string) string {
	return fmt.Sprintf(`package handlers

import "github.com/gin-gonic/gin"

// HealthHandler returns a simple health response for the plugin.
func HealthHandler(c *gin.Context) {
    c.JSON(200, gin.H{"status": "ok", "plugin": "%s"})
}
`, id)
}

func pluginCRUDHandlerTemplate(id string) string {
	return fmt.Sprintf(`package handlers

import "github.com/gin-gonic/gin"

// ListItems returns a placeholder list response.
func ListItems(c *gin.Context) {
	c.JSON(200, gin.H{"items": []string{}, "plugin": "%s"})
}

// CreateItem is a placeholder create handler.
func CreateItem(c *gin.Context) {
	c.JSON(201, gin.H{"id": "new-id", "plugin": "%s"})
}

// GetItem is a placeholder get handler.
func GetItem(c *gin.Context) {
	id := c.Param("id")
	c.JSON(200, gin.H{"id": id, "plugin": "%s"})
}
`, id, id, id)
}

func pluginCRUDServiceTemplate(id string) string {
	return `package services

import "errors"

// ResourceService is a placeholder service for CRUD operations.
type ResourceService struct{}

func NewResourceService() *ResourceService { return &ResourceService{} }

func (s *ResourceService) List() ([]string, error) { return []string{}, nil }
func (s *ResourceService) Create(name string) (string, error) { return "new-id", nil }
func (s *ResourceService) Get(id string) (string, error) { return "item:" + id, nil }
func (s *ResourceService) Delete(id string) error { return errors.New("not implemented") }
`
}

func pluginMiddlewareTemplate(id string) string {
	return fmt.Sprintf(`package middleware

import "github.com/gin-gonic/gin"

// SampleMiddleware adds a simple header to demonstrate plugin middleware wiring.
func SampleMiddleware(c *gin.Context) {
	c.Writer.Header().Set("X-Plugin-%s", "true")
	c.Next()
}
`, strings.ToUpper(strings.ReplaceAll(id, "-", "_")))
}
