// @title Umahstore Admin API
// @version 1.0
// @description Admin API for Umahstore - auto-generated swagger docs
// @termsOfService http://example.com/terms/
// @contact.name API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
// @BasePath /admin
package main

import (
	"log"
	"syscall"

	"go_framework/internal/app"
)

// main is a thin entrypoint; core boot logic lives in internal/app.
// Register custom plugins here if needed, similar to Laravel's bootstrap/app.php.
func main() {
	// Set permissive umask so created files/dirs are group-writable by default
	syscall.Umask(0o002)

	err := app.Run(app.Options{
		RegisterPlugins: func() {
			// Register your custom plugins here. Example:
			// import "go_framework/plugins/myplugin"
			// plugins.RegisterPlugins([]plugins.Plugin{
			//     myplugin.New(),
			// })
		},
	})
	if err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
