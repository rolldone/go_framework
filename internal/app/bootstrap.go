package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	// Value is a comma-separated list of allowed origins or patterns, e.g.
	// "http://localhost:5173,http://localhost:4321,*.emergentagent.com"
	if originsEnv := os.Getenv("CORS_ALLOWED_ORIGINS"); originsEnv != "" {
		raw := strings.Split(originsEnv, ",")
		var exactOrigins []string
		var patterns []string
		for _, o := range raw {
			o = strings.TrimSpace(o)
			if o == "" {
				continue
			}
			if strings.Contains(o, "*") {
				patterns = append(patterns, o)
			} else {
				exactOrigins = append(exactOrigins, o)
			}
		}

		corsCfg := cors.Config{
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Authorization", "Content-Type", "Accept", "x-artywiz_service-access-token"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}

		// If we have patterns, use AllowOriginFunc to dynamically validate origins.
		if len(patterns) > 0 {
			corsCfg.AllowOriginFunc = func(origin string) bool {
				// Exact match check first
				for _, e := range exactOrigins {
					if origin == e {
						return true
					}
				}

				u, err := url.Parse(origin)
				if err != nil {
					return false
				}
				host := u.Hostname()
				scheme := u.Scheme

				for _, p := range patterns {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}

					// Pattern includes scheme (e.g. https://*.domain.com)
					if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
						pu, err := url.Parse(p)
						if err != nil {
							continue
						}
						ph := pu.Hostname()
						if strings.HasPrefix(ph, "*.") {
							base := strings.TrimPrefix(ph, "*.")
							if host == base || strings.HasSuffix(host, "."+base) {
								if pu.Scheme == scheme {
									return true
								}
							}
						} else {
							if pu.Scheme == scheme && host == ph {
								return true
							}
						}
					} else {
						// Pattern without scheme, e.g. *.domain.com or domain.com
						ph := p
						if strings.HasPrefix(ph, "*.") {
							base := strings.TrimPrefix(ph, "*.")
							if host == base || strings.HasSuffix(host, "."+base) {
								return true
							}
						} else {
							if host == ph {
								return true
							}
						}
					}
				}
				return false
			}

			// Add exact origins as a fast-path list (optional)
			corsCfg.AllowOrigins = exactOrigins
		} else {
			corsCfg.AllowOrigins = exactOrigins
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
	// Root endpoint
	a.router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Hello World"})
	})

	// NOTE: assetlinks.json is served by the auth plugin to allow per-plugin
	// control. Plugin `plugins/auth/plugin.go` registers the handler for
	// `/.well-known/assetlinks.json`. The global handler was removed to avoid
	// duplicate route registration.

	// API root endpoint - match Python FastAPI behavior
	apiGroup := a.router.Group("/api")
	apiGroup.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Hello World"})
	})
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
