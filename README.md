Architecture & Request Flow
===========================

Summary
-------
This repository is a modular Go web application framework for backend services. The documentation here explains the architecture, request flow, important environment variables, and how to run development tools (server and console).

Table of contents
-----------------
- Overview
- Quick Start
- What's included
- What's NOT included (implement via plugins)
- Main components
- Bootstrapping
- Request flow
- Plugin system
- Environment variables
- Auth
  - JWT utilities available
  - Testing JWT functions
- DB
- Console commands overview
- Migration commands (console)
- Plugin quickstart (CLI generator)
- Plugin registration (manual)

Overview
--------
- This is a **minimalist, plugin-based Go web framework** for building modular backend services
- Main binaries: `cmd/server/main.go` (HTTP server) and `cmd/console/main.go` (CLI/migrations)
- Core philosophy: provide essential infrastructure (DB, routing, plugins) and let you implement features via plugins or custom code
- **No built-in authentication or user management** - implement via plugins (recommended) or custom middleware
- Application bootstrap and dependency wiring are in `internal/app/bootstrap.go`
- Extensible via a plugin system that supports middleware, routes, services, migrations, and console commands

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

See sections below for more details.

What's included
---------------
✅ Database connectivity (GORM - PostgreSQL, MySQL, MariaDB)  
✅ Database migrations system  
✅ Plugin architecture (middleware, routes, services, migrations, console commands)  
✅ Plugin generator CLI (`plugin new`)  
✅ JWT utilities (token generation/verification)  
✅ CORS configuration  
✅ Console commands (migrations, plugin generator, user management stub)  
✅ KeyDB/Redis client (for flash messages)  
✅ Mailer utilities  
✅ Transaction helpers  
✅ Swagger documentation support  

What's NOT included (implement via plugins)
--------------------------------------------
❌ Authentication service & middleware  
❌ User/Admin models  
❌ Authorization/permissions system  
❌ Built-in CRUD endpoints  
❌ Session management  
❌ Password hashing/verification utilities  

**Recommended:** Create an `auth` plugin using the plugin generator to implement authentication features.

Main components
---------------
- Bootstrap & wiring: `internal/app/bootstrap.go`
- Business services: `internal/admin/services`
- Database (GORM): `internal/db/gorm.go`
- Auth utilities (JWT): `internal/auth` (token generation/verification helpers)
- Plugin system: `internal/pluginloader` and `internal/plugins`
- CLI / migrations: `internal/console`
- Mailer: `internal/mail`
- KeyDB/Redis client: `internal/keydb`
- Storage abstraction: `internal/storage`
- Events: `internal/events`
- UUID v7 generator: `internal/uuid`

Events
------
A lightweight internal event bus is provided by `internal/events`. Handlers are invoked asynchronously in separate goroutines by default. Use `Subscribe` to register a handler (it returns an unsubscribe function) and `Publish` to emit events.

Example:

```go
// subscribe to an event
unsub := events.Subscribe("user.created", func(ctx context.Context, payload interface{}) {
   // handle event (payload can be any value)
   // run quick background work or forward to worker queues
})
defer unsub()

// publish an event (delivered asynchronously to subscribers)
events.Publish("user.created", map[string]interface{}{"id": "user123"})
```

See `internal/events/events_example_test.go` for a runnable test demonstrating Subscribe/Publish.

Request–Reply (safe pattern)
---------------------------
For synchronous data-sync between plugins the repo provides a helper `RequestReply` in `internal/events`.
It creates a unique reply topic for each request, publishes the request with a `ReplyTo` field, and waits (with timeout) for a single reply.

Key properties:
- Uses a per-request reply topic (no shared global reply channel) to avoid cross-talk.
- Caller supplies a timeout and context for cancellation.
- Subscribers must read `ReplyTo` from the request and publish their response to that topic.

See `internal/events/request_reply.go` and `internal/events/request_reply_test.go` for a concrete example with concurrent requesters.

Cancellation / Deadline pattern
-------------------------------
When requesters use different timeouts, handlers may finish after some callers already timed out. To avoid leaking or processing stale replies:

- Include a deadline or cancel field in the request payload (e.g. `Deadline` as a Unix nano timestamp) because `Publish` invokes handlers with a background context.
- Subscribers should check the deadline before doing expensive work and before publishing a reply; if the deadline passed, skip replying.
- Always publish replies to the specific `ReplyTo` topic provided in the request so replies don't reach other requesters.

Example (publisher):

```go
deadline := time.Now().Add(2 * time.Second).UnixNano()
req := map[string]interface{}{ "id": "42", "Deadline": deadline }
resp, err := events.RequestReply(ctx, "user.query", req, 2*time.Second)
```

Example (subscriber):

```go
events.Subscribe("user.query", func(ctx context.Context, payload interface{}) {
   m, _ := payload.(map[string]interface{})
   // read deadline (type assertions may vary)
   if d, ok := m["Deadline"].(int64); ok {
      if time.Now().UnixNano() > d {
         // too late, skip
         return
      }
   }
   replyTo, _ := m["ReplyTo"].(string)
   // do work and publish to replyTo
   events.Publish(replyTo, map[string]interface{}{"ok": true})
})
```

This pattern keeps reply scopes isolated and makes late replies harmless (they're ignored by the requester). If you need true cancellation of work, consider changing the publish API to forward a cancellable context or use a direct service call.

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

Storage
- `STORAGE_DRIVER`=local|s3
- `STORAGE_ROOT`=./storage — used when `STORAGE_DRIVER=local`
- `STORAGE_PUBLIC_URL`=http://localhost:8080/assets
- `S3_BUCKET`, `S3_REGION`, `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` — used when `STORAGE_DRIVER=s3`

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
This framework provides JWT utilities but does not include built-in authentication services or middleware. Authentication should be implemented via plugins or custom code.

JWT utilities available (`internal/auth`)
-----------------------------------------

**Basic JWT functions** (`jwt.go`):
```go
// Generate tokens
token, err := auth.SignAccessToken(userID)       // Generate access token
refresh, err := auth.SignRefreshToken(userID)    // Generate refresh token

// Parse/verify tokens
userID, err := auth.ParseAccessToken(tokenStr)   // Verify access token, returns user ID
userID, err := auth.ParseRefreshToken(tokenStr)  // Verify refresh token, returns user ID

// Get token expiry settings
accessExp := auth.AccessExpirySeconds()   // e.g., 900 (15 minutes)
refreshExp := auth.RefreshExpirySeconds() // e.g., 1209600 (14 days)
```

**Advanced JWT with custom claims** (`claims_tokens.go`):
```go
// Generate token with admin_id and level claims
token, expTime, err := auth.GenerateAccessTokenWithLevel(
    adminID,
    "admin",  // level: "admin", "user", etc.
    15 * time.Minute,
)

// Parse token and get claims
claims, err := auth.ParseAccessTokenClaims(tokenStr)
if err == nil {
    adminID := claims.AdminID
    level := claims.Level
    exp := claims.ExpiresAt
}
```

**Opaque refresh tokens** (non-JWT, for database storage):
```go
// Generate opaque token (96 random hex chars)
plainToken, hashedToken, err := auth.GenerateOpaqueRefreshToken()
// Store hashedToken in DB, return plainToken to client

// Verify opaque token
receivedHash := auth.HashOpaqueToken(plainTokenFromClient)
// Compare receivedHash with hash stored in DB
```

Implementation notes:
- The framework does NOT include built-in auth middleware or user/admin models
- **JWT functions are STATELESS and database-agnostic** - they only encode/decode data into/from token strings
- JWT tokens do NOT interact with database - you query the database separately using the ID from the token
- Column names and table structure are completely up to you - JWT only returns the data you encoded (userID, adminID, etc.)
- Implement authentication in a plugin (recommended) or in your own services
- Use the JWT utilities in `internal/auth` for token generation and verification
- Design your own middleware to validate tokens and inject user identity into `context.Context`
- Pass `context.Context` to service methods to propagate request identity

**Example workflow:**
```go
// 1. Login handler - user provides credentials
func LoginHandler(c *gin.Context) {
    // Verify credentials from YOUR database (any table structure)
    var user YourUserModel  // Could be "users", "admins", "accounts", etc.
    db.Where("email = ?", email).First(&user) // YOUR column names
    
    // Verify password (use your own hash method)
    if !verifyPassword(user.Password, providedPassword) {
        c.JSON(401, gin.H{"error": "invalid credentials"})
        return
    }
    
    // Generate JWT with the user ID (from YOUR database)
    token, _ := auth.SignAccessToken(user.ID)  // Just needs an ID string
    
    c.JSON(200, gin.H{"token": token})
}

// 2. Protected handler - verify token and get user
func ProtectedHandler(c *gin.Context) {
    tokenStr := c.GetHeader("Authorization") // "Bearer xxx"
    
    // Parse token - returns the ID you encoded earlier
    userID, err := auth.ParseAccessToken(tokenStr)
    if err != nil {
        c.JSON(401, gin.H{"error": "invalid token"})
        return
    }
    
    // Query YOUR database with YOUR schema
    var user YourUserModel
    db.First(&user, "id = ?", userID)  // Use whatever column name you have
    
    c.JSON(200, gin.H{"user": user})
}
```

**Key point:** JWT is just a container for data. The actual database queries, column names, and table structures are entirely your responsibility.

Security recommendations
- Keep `AUTH_JWT_SECRET` (or legacy `JWT_SECRET`) out of source control; use environment injection or secret managers.
- Use short `JWT_ACCESS_EXP_SECONDS` values for access tokens (recommended: 900 seconds / 15 minutes) and longer `JWT_REFRESH_EXP_SECONDS` for refresh tokens.
- Always serve authentication endpoints over HTTPS; set cookie flags `Secure`, `HttpOnly`, and `SameSite` when using cookies.
- Rotate signing keys and provide a migration/rotation plan (support key identifiers (`kid`) in tokens if you add multiple keys).

Example: implementing auth in a plugin
- Create an auth plugin using `go run ./cmd/console plugin new --id auth`
- Add user/admin models in the plugin
- Implement login/register handlers and services
- Create auth middleware that validates tokens and injects user ID into context
- Register the middleware with appropriate priority in the plugin's `RegisterMiddleware()` method

Testing JWT functions
----------------------
You can test JWT functions directly in your code or tests:

```go
package mytest

import (
    "testing"
    "time"
    "go_framework/internal/auth"
)

func TestJWT(t *testing.T) {
    // Set environment variable for testing
    t.Setenv("AUTH_JWT_SECRET", "test-secret-key")
    
    userID := "user123"
    
    // Generate access token
    token, err := auth.SignAccessToken(userID)
    if err != nil {
        t.Fatalf("failed to sign token: %v", err)
    }
    
    // Verify access token
    parsedID, err := auth.ParseAccessToken(token)
    if err != nil {
        t.Fatalf("failed to parse token: %v", err)
    }
    
    if parsedID != userID {
        t.Errorf("expected %s, got %s", userID, parsedID)
    }
}

func TestJWTWithClaims(t *testing.T) {
    t.Setenv("AUTH_JWT_SECRET", "test-secret-key")
    
    // Generate token with custom claims
    token, _, err := auth.GenerateAccessTokenWithLevel("admin123", "admin", 15*time.Minute)
    if err != nil {
        t.Fatal(err)
    }
    
    // Parse and verify claims
    claims, err := auth.ParseAccessTokenClaims(token)
    if err != nil {
        t.Fatal(err)
    }
    
    if claims.AdminID != "admin123" {
        t.Errorf("expected admin123, got %s", claims.AdminID)
    }
    if claims.Level != "admin" {
        t.Errorf("expected admin, got %s", claims.Level)
    }
}
```

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
This project uses GORM (`*gorm.DB`) as the shared database dependency and passes it to plugins through `plugins.ServiceDeps`. For file/object storage access, plugins also receive `storage.Store` from the same dependencies.

When you need transactional consistency across multiple service calls, prefer starting a transaction at the HTTP handler boundary and pass the transaction (`*gorm.DB`) explicitly into service methods. Also propagate the request `context.Context` into DB operations so cancellations/deadlines are honored.

Recommended handler pattern (Gin example):

```go
func CreateItemHandler(c *gin.Context) {
   ctx := c.Request.Context()
   gdb := deps.DB

   // start transaction
   tx := gdb.Begin()
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
   if err := yourService.CreateItem(ctx, tx, req); err != nil {
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
func (s *YourService) CreateItem(ctx context.Context, db *gorm.DB, req *CreateItemReq) error {
   // use db (transaction) which already has ctx via db = db.WithContext(ctx)
   if err := db.Create(&item).Error; err != nil {
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
- For read-only handlers that do not need transactions, use the shared DB dependency directly (for example `deps.DB`) without `Begin()`.
Helper utility
- `internal/db/tx.go` exposes `WithTransaction(ctx, gdb, fn)` which wraps begin/commit/rollback and panic handling. Use it to simplify handlers:

```go
err := db.WithTransaction(ctx, deps.DB, func(tx *gorm.DB) error {
   if err := yourService.CreateItem(ctx, tx, req); err != nil {
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
- Service deps: plugins receive shared dependencies through `plugins.ServiceDeps`, currently `DB *gorm.DB` and `Store storage.Store`.
- Middleware ordering: plugin middleware is executed according to priorities defined in `internal/plugins/middleware_priorities.go`. Valid targets are `global`, `admin`, and `api`.
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
   - `RegisterServices(deps plugins.ServiceDeps) error` — initialize plugin services using shared DB/storage deps
   - `RegisterMiddleware() []plugins.MiddlewareDescriptor` — provide middleware descriptors (Target: `global`, `admin`, `api`)
   - `RegisterRoutes(router *gin.Engine, admin *gin.RouterGroup, api *gin.RouterGroup) error` — attach routes
   - `Seed() error` — optional seed data
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
)

type MyPlugin struct{}

func New() plugins.Plugin { return &MyPlugin{} }

func (p *MyPlugin) ID() string { return "myplugin" }

func (p *MyPlugin) RegisterServices(deps plugins.ServiceDeps) error {
   // deps.DB dan deps.Store tersedia di sini
   return nil
}

func (p *MyPlugin) RegisterMiddleware() []plugins.MiddlewareDescriptor {
   return []plugins.MiddlewareDescriptor{
      {Name: "myplugin.log", Target: "global", Priority: 100, Handler: func(c *gin.Context) { /*...*/ c.Next() }},
   }
}

func (p *MyPlugin) RegisterRoutes(router *gin.Engine, admin *gin.RouterGroup, api *gin.RouterGroup) error {
   admin.GET("/myplugin/ping", func(c *gin.Context) { c.JSON(200, gin.H{"pong": true}) })
   _ = router
   _ = api
   return nil
}

func (p *MyPlugin) Seed() error { return nil }

func (p *MyPlugin) ConsoleCommands() []*cobra.Command { return nil }
```

Notes:
- Choose middleware `Priority` carefully so plugins integrate predictably with core middleware.
- Keep plugins isolated and avoid global mutable state.
- Register plugins before `plugins.AttachMiddleware` is called (bootstrap handles this via `app.Run` options).

- Limit the privileges of plugin-executed operations (DB, external APIs) where possible.


Bootstrapping (high level)
--------------------------
1. `cmd/server` calls bootstrap in `internal/app` to initialize:
   - Configuration from environment variables
   - Database connection (GORM)
   - Storage service (`local` or `s3`)
   - KeyDB/Redis connection (for flash messages)
   - Gin router with CORS configuration
   - Static `/assets` route when local storage is active
2. Core plugins are loaded via `internal/pluginloader`, then user-provided plugins registered in `cmd/server/main.go`
3. Plugin services are registered with shared deps (`plugins.ServiceDeps{DB, Store}`)
4. Plugin middleware are attached to router groups (global, admin, api) based on priority
5. Plugin routes are registered
6. Swagger documentation routes are registered (if enabled)
7. HTTP server is started on configured port

Request flow (execution order)
-----------------------------
1. Client HTTP request arrives at the server binary (`cmd/server`).
2. Router matches the route and triggers the middleware chain.
3. Global middleware execute in priority order:
   - Gin's default middlewares (Logger, Recovery)
   - CORS middleware (if configured via `CORS_ALLOWED_ORIGINS`)
   - Plugin middleware (registered according to priorities defined in `internal/plugins/middleware_priorities.go`)
   - Note: Authentication middleware is NOT included by default — implement in a plugin
4. After middleware, the matched handler runs (plugin-registered handlers or custom handlers).
5. Handler calls into plugin services or custom code for business logic, DB interactions, storage, or cache usage.
6. Shared dependencies come from bootstrap: GORM (`internal/db/gorm.go`), storage (`internal/storage`), and optional KeyDB (`internal/keydb`).
7. Handler serializes the response (JSON/HTML) and returns it to the client.
8. Plugin hooks or response middleware may modify the response before it is sent.

Plugin system notes
-------------------
- Plugins are loaded at bootstrap and may register routes, middleware, and service hooks.
- Middleware ordering for plugins is controlled by `internal/plugins/middleware_priorities.go`.
- Plugins integrate with core services via the registry in `internal/plugins/registry.go`.

Extension points & best practices
--------------------------------
**Plugin-first approach:**
- Implement features (auth, user management, business logic) as plugins rather than modifying core files
- Use `go run ./cmd/console plugin new --id <feature-name>` to generate plugin scaffolds
- Keep plugins isolated and testable - each plugin should be self-contained

**Code organization:**
- Add routes/handlers via plugins (recommended) or by modifying `internal/app/bootstrap.go`
- Register middleware with explicit priority so execution order is predictable
- Keep service layer decoupled from HTTP layer — services should accept `context.Context` and repository interfaces
- Prefer plugin-local services initialized from `plugins.ServiceDeps` instead of extending core structs unless truly necessary

**Database & transactions:**
- Use `context.Context` to pass request identity and cancellation signals into services
- Pass `*gorm.DB` transactions explicitly to service methods (see Transactions & context patterns section)
- Use the transaction helper `db.WithTransaction()` in `internal/db/tx.go` to simplify error handling

**Testing:**
- Mock DB and services for unit testing; see `internal/mail/mailer_test.go` for example patterns
- Test plugins in isolation by providing test `*gorm.DB` and, when needed, a fake `storage.Store`
- Use `go run ./cmd/console migrate --db <type> up` to run migrations in test databases

**Security:**
- Implement authentication middleware in a plugin and register with appropriate priority
- Never commit secrets to version control - use environment variables and secret managers
- Validate and sanitize all user inputs in handlers before passing to services

Quick file references
---------------------
- Server entry: `cmd/server/main.go`
- Console entry: `cmd/console/main.go`
- Bootstrap: `internal/app/bootstrap.go`
- Admin services: `internal/admin/services/services.go`
- Plugin loader: `internal/pluginloader/loader.go`
- Plugin registry: `internal/plugins/registry.go`
- Plugin types: `internal/plugins/types.go`
- DB (GORM): `internal/db/gorm.go`
- DB transactions: `internal/db/tx.go`
- Auth utilities (JWT): `internal/auth/jwt.go`
- Storage: `internal/storage/config.go`
- Console commands: `internal/console/`
- Mailer: `internal/mail/mailer.go`
- KeyDB client: `internal/keydb/client.go`

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
