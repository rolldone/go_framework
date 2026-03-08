Architecture & Request Flow
===========================

Ringkasan
---------
Repository ini adalah kerangka aplikasi web Go modular untuk layanan backend. Dokumentasi di sini menjelaskan arsitektur, alur request, variabel lingkungan penting, dan cara menjalankan alat pengembangan (server dan console).

Table of contents
-----------------
- Overview
- Quick Start
- Main components
- Bootstrapping
- Request flow
- Plugin system
- Environment variables
- Auth
- DB
- Console commands overview
- Migration commands (console)
- Plugin quickstart (CLI generator)
- Plugin registration (manual)

Overview
--------
- This project is a modular Go web service with CLI utilities. Main binaries are `cmd/server/main.go` (HTTP server) and `cmd/console/main.go` (CLI/migrations).
- Application bootstrap and dependency wiring are implemented in `internal/app/bootstrap.go`.

Quick Start
-----------
1. Copy example env and edit values:

```bash
cp .env.example .env
# edit .env as needed
```

2. Run migrations (applies core then plugin migrations):

```bash
go run ./cmd/console migrate up
```

3. Start the server:

```bash
go run ./cmd/server
```

4. (Optional) Generate a new plugin:

```bash
go run ./cmd/console plugin new --id my-plugin
```

Untuk detail lainnya lihat bagian di bawah.

Main components
---------------
- Bootstrap & wiring: `internal/app/bootstrap.go`
- HTTP server & middleware: `internal/server/middleware.go`
- Routing & handlers: `internal/front/handler`
- Business services: domain services under `internal/*/services`
- Database (GORM): `internal/db/gorm.go`
- Auth (JWT): `internal/auth`
- Plugin system: `internal/pluginloader` and `internal/plugins`
- CLI / migrations: `internal/console`
- Mailer: `internal/mail`

Environment variables
---------------------
Configuration is read from environment variables. Use `./.env.example` as a starting point.

Below are recommended variables with example values and short notes.

App
- `APP_ENV`=development|staging|production — runtime environment, affects logging and error modes.
- `APP_HOST`=0.0.0.0
- `APP_PORT`=8080
- `APP_DEBUG`=true|false — enable verbose debug logs only in non-production.

Database (GORM)
- `DB_TYPE`=postgres|mysql|mariadb
- `DB_HOST`=localhost
- `DB_PORT`=5432
- `DB_NAME`=app_db
- `DB_USER`=postgres
- `DB_PASSWORD`=secret — do NOT commit secrets; use secret manager in production.
- `DB_SSLMODE`=disable|require (Postgres only)
- `DB_MAX_OPEN_CONNS`=50
- `DB_MAX_IDLE_CONNS`=10
- `DB_CONN_MAX_LIFETIME_SEC`=300 — connection max lifetime in seconds

Auth / Security
- `AUTH_JWT_SECRET`=very_long_random_string — canonical secret used to sign JWTs (HS256). Keep secret and rotate periodically.
- `JWT_ACCESS_EXP_SECONDS`=900 — access token TTL in seconds (15 minutes recommended)
- `JWT_REFRESH_EXP_SECONDS`=1209600 — refresh token TTL in seconds (14 days recommended)

Note: legacy env names such as `JWT_SECRET`, `JWT_ACCESS_SECRET`, and `JWT_REFRESH_SECRET` are deprecated. The application will prefer `AUTH_JWT_SECRET` when present and fall back to legacy names for compatibility. Remove legacy vars from production `.env` to avoid confusion.

Mailer
- `SMTP_HOST`=smtp.example.com
- `SMTP_PORT`=587
- `SMTP_USER`=
- `SMTP_PASS`=
- `SMTP_FROM`=admin@example.com
- `SMTP_USE_TLS`=true|false — enable TLS when supported by SMTP server.
- `SMTP_STARTTLS`=true|false — enable STARTTLS

Cache / Flash Messages (KeyDB/Redis)
- `KEYDB_HOST`=127.0.0.1
- `KEYDB_PORT`=6379
- `KEYDB_PASS`= — optional password
- `KEYDB_DB`=0 — database number

CORS
- `CORS_ALLOWED_ORIGINS`="http://localhost:5173,http://localhost:4321" — comma-separated list of allowed origins

Logging
- `LOG_LEVEL`=debug|info|warn|error

Misc
- `APP_URL`=http://localhost:3651
- `ADMIN_URL`=http://localhost:5173
- `FRONT_URL`=http://localhost:4321
- `DOCKER_HOST_IP`=127.0.0.1

Security notes
- Never commit `.env` with real secrets to version control; keep `.env.example` generic.
- For production, prefer secret stores (Vault, AWS Secrets Manager, Kubernetes Secrets) and inject at deploy time.
- Rotate keys/secrets and use minimal privilege for DB/service accounts.

Auth
----
This project supports JWT-based authentication. Key points and integration details:

- Token types: the codebase uses signed JWT access and refresh tokens for request auth. Token generation and verification are handled in `internal/auth/jwt.go` and `internal/auth/claims_tokens.go`.
- Token locations: auth middleware validates tokens from `Authorization: Bearer <token>` header; cookie-based tokens are supported if configured by application code.
- Verification: token verification and claims parsing occur in `internal/auth/jwt.go`. Middleware in `internal/server/middleware.go` (or the auth-specific middleware) calls these helpers and, on success, injects the authenticated identity into the request `context.Context`.
- Context usage: handlers and service functions should accept `context.Context` and retrieve actor info from context keys provided by the auth middleware (follow existing patterns in `internal/*/services.go`).

Security recommendations
- Keep `AUTH_JWT_SECRET` (or legacy `JWT_SECRET`) out of source control; use environment injection or secret managers.
- Use short `JWT_ACCESS_EXP_SECONDS` values for access tokens (recommended: 900 seconds / 15 minutes) and longer `JWT_REFRESH_EXP_SECONDS` for refresh tokens.
- Always serve authentication endpoints over HTTPS; set cookie flags `Secure`, `HttpOnly`, and `SameSite` when using cookies.
- Rotate signing keys and provide a migration/rotation plan (support key identifiers (`kid`) in tokens if you add multiple keys).

Testing
- Unit-test auth-related logic by mocking token generation/verification helpers. Look at `internal/mail/mailer_test.go` for examples of structure and patterns.

DB
--
This project uses GORM (see `internal/db/gorm.go`) as the primary ORM. Below are connection, pooling, and migration notes to help setup and operate the database safely.

Connection
- DSN is composed from environment variables (`DB_TYPE`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`). Example Postgres DSN format:

```text
host=localhost port=5432 user=postgres dbname=app_db password=secret sslmode=disable
```

- The connection is created in `internal/db/gorm.go`; prefer injecting the DB instance into services rather than using a global variable.
 - Transaction helper: see `internal/db/tx.go` for `WithTransaction(ctx, gdb, fn)` which simplifies begin/commit/rollback patterns.

Pooling & tuning
- Recommended env vars: `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME_SEC`. Tune based on workload and connection limits of your DB server.
- Monitor `pg_stat_activity` (Postgres) or equivalent to avoid connection exhaustion when scaling workers or background jobs.

Transactions & context
- Pass `context.Context` and, when needed, a `*gorm.DB` transaction instance from handlers into service functions so operations can participate in the same transaction.
- Avoid long-lived DB transactions across user-facing requests; keep transactions short and deterministic.

Migrations
- SQL/Go migrations live in the `migrations/` directory. Migration helper and CLI integration are available under `internal/console/migrate.go`.
- Run migrations locally via the console CLI. Example (from repo root):

```bash
go run ./cmd/console migrate
```

- Note: GORM's `AutoMigrate` can be useful for development but use structured migrations for production (reason: safer, reversible, explicit schema control).

Backup & maintenance
- Regularly backup the DB and test restores. Use read replicas for analytics/reporting to reduce load on primary.
- Apply schema changes during maintenance windows for high-traffic production systems.

Console commands overview
--------------------------
The console (`cmd/console`) provides several commands for development and operations:

```bash
# Show all available commands
go run ./cmd/console --help

# Database migrations
go run ./cmd/console migrate [make|up|down|down-all|list]

# Plugin generator
go run ./cmd/console plugin new --id <plugin-id> [--template minimal|crud|middleware]

# Seed data
go run ./cmd/console seed [--plugin core|all|<plugin-id>]

# User management (if implemented)
go run ./cmd/console user [create|list|update|delete|get]
```

See sections below for detailed usage of each command.

Migration commands (console)
----------------------------
Migrations are managed via the console CLI exposed in `cmd/console`. Migration files live under `migrations/{db_type}` for core and `plugins/{plugin_id}/migrations/{db_type}` for plugins (where {db_type} is postgres, mysql, etc.).

Common commands (run from repo root):

```bash
# create a new migration pair (up/down) for core (or use --plugin <id>)
go run ./cmd/console migrate make add_users_table

# apply pending migrations (core then plugins)
go run ./cmd/console migrate up

# apply pending migrations only for a specific plugin
go run ./cmd/console migrate --plugin myplugin up

# rollback the last migration (plugins rolled back last)
go run ./cmd/console migrate down

# rollback all migrations (plugins first, then core)
go run ./cmd/console migrate down-all

# show migration status per target
go run ./cmd/console migrate list

# override auto-detected DB type (useful for testing)
go run ./cmd/console migrate --db mysql up
```

Notes:
- The migrate commands track state in DB tables `migrations` and `migration_targets` created automatically on first run.
- If a target is reported as `dirty`, the CLI will refuse to continue; inspect the DB and migration files to resolve the issue (restore missing migration files or fix the database records), then clear `dirty` in `migration_targets`.
- For production, prefer writing explicit SQL migration files and testing rollbacks on staging before applying to production.

Transactions & context patterns
-------------------------------
This project uses GORM (`*gorm.DB`) held on the `AdminServices` struct (`internal/admin/services.AdminServices`). When you need transactional consistency across multiple service calls, prefer starting a transaction at the HTTP handler boundary and pass the transaction (`*gorm.DB`) explicitly into service methods. Also propagate the request `context.Context` into DB operations so cancellations/deadlines are honored.

Recommended handler pattern (Gin example):

```go
func CreateUserHandler(c *gin.Context) {
   svc := services.GetDefault()
   ctx := c.Request.Context()

   // start transaction
   tx := svc.DB.Begin()
   if tx.Error != nil {
      c.JSON(500, gin.H{"error": "failed to start tx"})
      return
   }

   // ensure rollback on panic or early return
   committed := false
   defer func() {
      if !committed {
         tx.Rollback()
      }
   }()

   // pass tx (with context) into service layer
   tx = tx.WithContext(ctx)
   if err := svc.Auth.CreateUser(ctx, tx, req); err != nil {
      c.JSON(400, gin.H{"error": err.Error()})
      return
   }

   if err := tx.Commit().Error; err != nil {
      c.JSON(500, gin.H{"error": "failed to commit"})
      return
   }
   committed = true
   c.Status(201)
}
```

Service method signature example (accept tx explicitly):

```go
func (s *AuthService) CreateUser(ctx context.Context, db *gorm.DB, req *CreateUserReq) error {
   // use db (transaction) which already has ctx via db = db.WithContext(ctx)
   if err := db.Create(&user).Error; err != nil {
      return err
   }
   // call other DB ops using the same `db` to participate in the transaction
   return nil
}
```

Notes & best practices
- Prefer passing `*gorm.DB` explicitly rather than storing ephemeral transactions in global state.
- Use `db.WithContext(ctx)` so query cancellation and timeouts propagate.
- Keep transactions short: perform only necessary DB work inside a transaction to avoid locking contention.
- Handle panics and ensure `Rollback()` is called unless `Commit()` succeeded.
- For read-only handlers that do not need transactions, use the shared `svc.DB` directly (without `Begin()`).
Helper utility
- `internal/db/tx.go` exposes `WithTransaction(ctx, gdb, fn)` which wraps begin/commit/rollback and panic handling. Use it to simplify handlers:

```go
err := db.WithTransaction(ctx, svc.DB, func(tx *gorm.DB) error {
   if err := svc.Auth.CreateUser(ctx, tx, req); err != nil {
      return err
   }
   // other DB ops using tx...
   return nil
})
if err != nil {
   // handle error
}
```


Plugin system
-------------
This project includes a pluggable architecture so features can be implemented as separate plugins. Core plugin-related files:

- Loader: `internal/pluginloader/loader.go` — responsible for discovering and loading plugins at bootstrap.
- Registry: `internal/plugins/registry.go` — central registry where plugins register routes, services, and hooks.
- Types & priorities: `internal/plugins/types.go` and `internal/plugins/middleware_priorities.go` — define plugin interfaces and middleware ordering.

Key concepts
- Discovery: plugins are created under the `plugins/` directory (currently empty in this starter framework). Each plugin has its own folder with optional `migrations/`, handlers, and registration code.
- Registration: plugins register themselves with the registry during application bootstrap in `cmd/server/main.go` and `cmd/console/main.go`; this allows them to add routes, middleware, and service hooks.
- Middleware ordering: plugin middleware is executed according to priorities defined in `internal/plugins/middleware_priorities.go`. When adding middleware from a plugin, choose a priority to avoid surprising ordering interactions with core middlewares.
- Migrations: plugins can include DB migrations under `plugins/{plugin_id}/migrations/{db_type}` (e.g., `migrations/postgres/`); the console migrate commands detect and apply plugin migrations in the configured order.

Integration notes
- To enable a plugin, create it in the `plugins/` directory, ensure the plugin is registered in both `cmd/server/main.go` and `cmd/console/main.go` (see Plugin registration quickstart below).
- Plugins should be written to be defensive: validate inputs, avoid global state, and return errors that the core can log and surface gracefully.
- Hot-reload is not assumed; plugins are loaded at bootstrap. For runtime reloading, add explicit support in the loader and consider concurrency/consistency implications.

Testing & safety
- Test plugins in isolation by running their handlers/services against a test instance of the registry and a sandbox DB.

Plugin quickstart (CLI generator)
---------------------------------
The fastest way to create a new plugin is using the console command:

```bash
# Generate a minimal plugin
go run ./cmd/console plugin new --id my-plugin

# Generate a CRUD plugin with handlers and services
go run ./cmd/console plugin new --id my-plugin --template crud

# Generate a middleware-focused plugin
go run ./cmd/console plugin new --id my-plugin --template middleware

# With custom display name
go run ./cmd/console plugin new --id my-plugin --name "My Awesome Plugin"

# Skip console command stub
go run ./cmd/console plugin new --id my-plugin --no-console
```

**Template options:**
- `minimal` (default) - Basic plugin with a health check handler
- `crud` - Includes CRUD handlers and service layer for resource management
- `middleware` - Focuses on middleware with sample middleware implementation

**Generated structure:**
- `plugins/my_plugin/plugin.go` - main plugin file implementing `plugins.Plugin` interface
- `plugins/my_plugin/handlers/` - HTTP handler files (health.go, resource.go, etc.)
- `plugins/my_plugin/migrations/postgres/` - migration files (000001_init.up.sql, 000001_init.down.sql)
- `plugins/my_plugin/services/` - service layer (CRUD template only)
- `plugins/my_plugin/middleware/` - middleware implementations (middleware template only)

**After generating, you must register the plugin in both:**
- `cmd/server/main.go` - for HTTP server routes and middleware
- `cmd/console/main.go` - for console commands (migrations, seeds, custom commands)

Plugin registration (manual)
-----------------------------
To manually create a plugin or understand the structure:

1. Create a plugin package under `plugins/<plugin_id>/` in your workspace.
2. Implement the `plugins.Plugin` interface (see `internal/plugins/types.go`). Minimal responsibilities:
   - `ID() string` — return plugin id
   - `RegisterServices(svcs *services.AdminServices) error` — extend shared services
   - `RegisterMiddleware() []plugins.MiddlewareDescriptor` — provide middleware descriptors (Target: `global`, `admin`, `store`)
   - `RegisterRoutes(router *gin.Engine, admin *gin.RouterGroup, store *gin.RouterGroup, svcs *services.AdminServices) error` — attach routes
   - `Seed(svcs *services.AdminServices) error` — optional seed data
   - `ConsoleCommands() []*cobra.Command` — optional CLI commands

3. Register the plugin in both `cmd/server/main.go` and `cmd/console/main.go`:

**cmd/server/main.go:**
```go
import (
   // ... other imports
   "go_framework/internal/plugins"
   myplugin "go_framework/plugins/my_plugin"  // note: use underscore for import path
)

func main() {
   err := app.Run(app.Options{
      RegisterPlugins: func() {
         plugins.RegisterPlugins([]plugins.Plugin{
            myplugin.New(),
         })
      },
   })
   // ...
}
```

**cmd/console/main.go:**
```go
import (
   // ... other imports
   "go_framework/internal/console"
   "go_framework/internal/plugins"
   myplugin "go_framework/plugins/my_plugin"  // note: use underscore for import path
)

func main() {
   console.RegisterAdditionalPlugins([]plugins.Plugin{myplugin.New()})
   console.Execute()
}
```

4. Place DB migrations under `plugins/<plugin_id>/migrations/<db_type>` if needed (e.g., `plugins/myplugin/migrations/postgres/`).

Minimal plugin skeleton (example file: `plugins/myplugin/plugin.go`):

```go
package myplugin

import (
   "github.com/gin-gonic/gin"
   "github.com/spf13/cobra"
   "go_framework/internal/plugins"
   "go_framework/internal/admin/services"
)

type MyPlugin struct{}

func New() plugins.Plugin { return &MyPlugin{} }

func (p *MyPlugin) ID() string { return "myplugin" }

func (p *MyPlugin) RegisterServices(svcs *services.AdminServices) error {
   // extend services if needed
   return nil
}

func (p *MyPlugin) RegisterMiddleware() []plugins.MiddlewareDescriptor {
   return []plugins.MiddlewareDescriptor{
      {Name: "myplugin.log", Target: "global", Priority: 100, Handler: func(c *gin.Context) { /*...*/ c.Next() }},
   }
}

func (p *MyPlugin) RegisterRoutes(router *gin.Engine, admin *gin.RouterGroup, store *gin.RouterGroup, svcs *services.AdminServices) error {
   admin.GET("/myplugin/ping", func(c *gin.Context) { c.JSON(200, gin.H{"pong": true}) })
   return nil
}

func (p *MyPlugin) Seed(svcs *services.AdminServices) error { return nil }

func (p *MyPlugin) ConsoleCommands() []*cobra.Command { return nil }
```

Notes:
- Choose middleware `Priority` carefully so plugins integrate predictably with core middleware.
- Keep plugins isolated and avoid global mutable state.
- Register plugins before `plugins.AttachMiddleware` is called (bootstrap handles this via `app.Run` options).

- Limit the privileges of plugin-executed operations (DB, external APIs) where possible.


Bootstrapping (high level)
--------------------------
1. `cmd/server` calls bootstrap in `internal/app` to initialize configuration, DB (GORM), KeyDB (for flash messages), services, and router with CORS.
2. Plugins are loaded via `internal/pluginloader` (core plugins) and user-provided plugins registered in `cmd/server/main.go`.
3. Plugin middleware and routes are attached to the router.
4. HTTP server is started.

Request flow (execution order)
-----------------------------
1. Client HTTP request arrives at the server binary (`cmd/server`).
2. Router matches the route and triggers the middleware chain.
3. Global middleware execute in priority order:
   - Core middlewares (logging, recovery, CORS, request-id).
   - Authentication middleware (`internal/auth`) — verifies header/cookie tokens and injects user identity into the request context.
   - Application / plugin middleware (plugin middleware are registered according to priorities defined in `internal/plugins/middleware_priorities.go`).
4. After middleware, the matched handler runs (e.g. handlers in `internal/front/handler` or plugin-registered handlers).
5. Handler calls into service layer (`internal/*/services`) for business logic and DB interactions (`internal/db`).
6. Services interact with GORM and return domain models to the handler.
7. Handler serializes the response (JSON/HTML) and returns it to the client.
8. Plugin hooks or response middleware may modify the response before it is sent.

Plugin system notes
-------------------
- Plugins are loaded at bootstrap and may register routes, middleware, and service hooks.
- Middleware ordering for plugins is controlled by `internal/plugins/middleware_priorities.go`.
- Plugins integrate with core services via the registry in `internal/plugins/registry.go`.

Extension points & best practices
--------------------------------
- Add routes/handlers by registering them in bootstrap or via plugins.
- Register middleware with explicit priority so execution order is predictable.
- Keep service layer decoupled from HTTP layer — services should accept `context.Context` and repository interfaces.
- Use `context` to pass identity and DB transactions into services.
- Mock DB and services for unit testing; see `internal/mail/mailer_test.go` for example patterns.

Quick file references
---------------------
- Server entry: `cmd/server/main.go`
- Bootstrap: `internal/app/bootstrap.go`
- Middleware: `internal/server/middleware.go`
- Plugin loader: `internal/pluginloader/loader.go`
- Plugins registry: `internal/plugins/registry.go`
- DB (GORM): `internal/db/gorm.go`
- Auth: `internal/auth/jwt.go`

Request Flow Diagram
--------------------
Below is a Mermaid diagram that visualizes the main request path and extension points.

```
flowchart LR
   Client[Client HTTP] --> Server[cmd/server]
   Server --> Router[Router]
   Router --> MWChain[Middleware Chain]

   subgraph Middlewares
      Core[Core middleware: logging, recovery, CORS, request-id]
      Auth[Auth middleware: internal/auth]
      PluginMW[Plugin middleware (priority-based)]
   end

   MWChain --> Core --> Auth --> PluginMW --> Handler[Handler]

   Handler --> Service[Service layer: internal/*/services]
   Service --> DB[GORM (internal/db)]
   DB --> Service
   Service --> Handler

   Handler --> Response[Response -> Client]

   PluginMW --> Hooks[Plugin hooks / response modifiers]
   Hooks --> Response
```
