package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

// GetDB returns a *sql.DB connected using MIGRATE_DB_URL from environment (.env loaded if present)
func GetDB() (*sql.DB, error) {
	// Use Overload so running the CLI (`./bin/console ...`) re-reads the
	// `.env` file and overwrites any existing process environment values.
	// This makes the CLI pick up updated `.env` without restarting the
	// containing Docker container. In long-running services you may prefer
	// a more explicit reload strategy instead.
	_ = godotenv.Overload()
	dsn := ""
	dbType := os.Getenv("DB_TYPE")
	var driverName string
	// Build DSN from individual DB_* env vars.
	// MIGRATE_DB_URL is intentionally ignored.
	{
		// Build DSN from individual DB_* env vars if MIGRATE_DB_URL is not provided.
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		user := os.Getenv("DB_USER")
		pass := os.Getenv("DB_PASSWORD")
		name := os.Getenv("DB_NAME")
		if host == "" || port == "" || user == "" || name == "" {
			return nil, fmt.Errorf("database configuration is not set (MIGRATE_DB_URL or DB_* vars)")
		}

		if dbType == "mysql" {
			dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, pass, host, port, name)
			if ssl := os.Getenv("DB_SSLMODE"); ssl != "" && ssl != "disable" {
				dsn += "&tls=" + ssl
			}
			driverName = "mysql"
		} else {
			u := &url.URL{
				Scheme: "postgres",
				User:   url.UserPassword(user, pass),
				Host:   fmt.Sprintf("%s:%s", host, port),
				Path:   "/" + name,
			}
			q := u.Query()
			ssl := os.Getenv("DB_SSLMODE")
			if ssl == "" {
				ssl = "disable"
			}
			q.Set("sslmode", ssl)
			u.RawQuery = q.Encode()
			dsn = u.String()
			driverName = "pgx"
		}
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}
