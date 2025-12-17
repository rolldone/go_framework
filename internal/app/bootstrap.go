package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"go_framework/internal/admin/services"
	"go_framework/internal/db"
	front "go_framework/internal/front/services"
	"go_framework/internal/pluginloader"
	"go_framework/internal/plugins"
	"go_framework/internal/server"
)

// Options customizes how the Umahstore app boots.
type Options struct {
	// RegisterPlugins allows callers to register additional plugins (user-provided).
	RegisterPlugins func()
}

// Run boots the application and starts the HTTP server using the provided options.
func Run(opts Options) error {
	app, err := New(opts)
	if err != nil {
		return err
	}
	return app.Run()
}

// App contains the assembled server state.
type App struct {
	router        *gin.Engine
	rootGroup     *gin.RouterGroup
	adminGroup    *gin.RouterGroup
	frontGroup    *gin.RouterGroup
	services      *services.AdminServices
	storeServices *front.StoreServices
}

// New assembles the application: DB, services, routes, plugins, swagger.
func New(opts Options) (*App, error) {
	gdb, err := db.GetGormDB()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	svc := services.NewServices(gdb)
	services.SetDefault(svc)
	storeSvc := front.NewStoreServices(svc)
	front.SetDefault(storeSvc)

	r := gin.Default()

	// Configure CORS from environment variable `CORS_ALLOWED_ORIGINS`.
	// Value is a comma-separated list of allowed origins, e.g.
	// "http://localhost:5173,http://localhost:4321"
	if originsEnv := os.Getenv("CORS_ALLOWED_ORIGINS"); originsEnv != "" {
		origins := strings.Split(originsEnv, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		corsCfg := cors.Config{
			AllowOrigins:     origins,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Authorization", "Content-Type", "Accept", "x-go_framework-access-token"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}
		r.Use(cors.New(corsCfg))
	} else {
		// Fallback to a sensible default (allow commonly used origins during development)
		r.Use(cors.Default())
	}
	root := r.Group("")

	admin := r.Group("/admin")
	admin.Use(server.AuthMiddleware())

	storeGroup := r.Group("/front")

	app := &App{
		router:        r,
		rootGroup:     root,
		adminGroup:    admin,
		frontGroup:    storeGroup,
		services:      svc,
		storeServices: storeSvc,
	}

	app.registerAdminRoutes()
	app.registerStoreRoutes()

	if err := app.attachPlugins(opts.RegisterPlugins); err != nil {
		return nil, err
	}

	app.registerSwaggerRoutes()

	return app, nil
}

// Run starts the HTTP server.
func (a *App) Run() error {
	if a == nil || a.router == nil {
		return errors.New("app not initialized")
	}
	return a.router.Run()
}

// attachPlugins registers core + user plugins, then attaches middleware and routes.
func (a *App) attachPlugins(registerPlugins func()) error {
	pluginloader.RegisterCorePlugins()
	if registerPlugins != nil {
		registerPlugins()
	}

	plugins.AttachMiddleware(map[string]*gin.RouterGroup{
		"global": a.rootGroup,
		"admin":  a.adminGroup,
		"store":  a.frontGroup,
	})

	return plugins.RegisterAllRoutes(a.router, a.adminGroup, a.frontGroup, a.services)
}

// registerAdminRoutes wires all core admin endpoints.
func (a *App) registerAdminRoutes() {

}

// registerStoreRoutes wires all core store/public endpoints.
func (a *App) registerStoreRoutes() {
}

// registerSwaggerRoutes registers handlers for serving the generated swagger JSON and UI.
func (a *App) registerSwaggerRoutes() {
	a.router.GET("/doc.json", func(c *gin.Context) {
		candidates := []string{"./docs/swagger/swagger.json", "./docs/swagger.json"}
		var data []byte
		var err error
		for _, p := range candidates {
			if fi, e := os.Stat(p); e == nil && !fi.IsDir() {
				data, err = os.ReadFile(p)
				if err == nil {
					break
				}
			}
		}
		if data == nil || err != nil {
			c.String(http.StatusNotFound, "swagger JSON not found")
			return
		}

		var doc map[string]interface{}
		if err := json.Unmarshal(data, &doc); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("failed to parse swagger JSON: %v", err))
			return
		}

		if c.Request != nil && c.Request.Host != "" {
			doc["host"] = c.Request.Host
		}

		out, err := json.MarshalIndent(doc, "", "    ")
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("failed to generate swagger JSON: %v", err))
			return
		}

		c.Data(http.StatusOK, "application/json", out)
	})

	a.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/doc.json")))
}
