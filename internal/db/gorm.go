package db

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	gormDB   *gorm.DB
	gormOnce sync.Once
)

// GetGormDB creates a *gorm.DB using the same DSN logic as GetDB.
func GetGormDB() (*gorm.DB, error) {
	var err error
	gormOnce.Do(func() {
		gormDB, err = openGormDB()
	})
	return gormDB, err
}

func openGormDB() (*gorm.DB, error) {
	// Load .env file
	_ = godotenv.Overload()
	// Reuse MIGRATE_DB_URL or DB_* env vars like GetDB()
	// Build DSN from individual DB_* env vars (we don't use MIGRATE_DB_URL)
	dsn := ""
	{
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		user := os.Getenv("DB_USER")
		pass := os.Getenv("DB_PASSWORD")
		name := os.Getenv("DB_NAME")
		if host == "" || port == "" || user == "" || name == "" {
			return nil, fmt.Errorf("database configuration is not set (MIGRATE_DB_URL or DB_* vars)")
		}
		ssl := os.Getenv("DB_SSLMODE")
		if ssl == "" {
			ssl = "disable"
		}
		// Build DSN depending on DB type. Use DB_TYPE if present; otherwise assume postgres style for now.
		if strings.ToLower(os.Getenv("DB_TYPE")) == "mysql" || strings.ToLower(os.Getenv("DB_TYPE")) == "mariadb" {
			dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, pass, host, port, name)
		} else {
			dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, pass, name, ssl)
		}
	}

	// Detect DB type; prefer DB_TYPE env var, fall back to DSN parsing
	dbType := strings.ToLower(os.Getenv("DB_TYPE"))
	if dbType == "" {
		dbType = detectDBType(dsn)
	}

	// DSN is masked and not printed to avoid leaking credentials
	log.Printf("Env DB: host=%s port=%s name=%s", os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME"))

	// Log connection details
	var host, port, name string
	// Attempt to extract info from DSN for logging
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" && (dbType == "postgres" || dbType == "mysql" || dbType == "mariadb") {
		// MIGRATE_DB_URL style
		host = u.Hostname()
		port = u.Port()
		name = strings.TrimPrefix(u.Path, "/")
	} else {
		// If DSN looks like MySQL native format user:pass@tcp(host:port)/name?params, parse it
		if dbType == "mysql" || dbType == "mariadb" {
			_, mh, mp, mn := parseMySQLDSN(dsn)
			if mh != "" {
				host = mh
				port = mp
				name = mn
			} else {
				// Fallback to environment variables
				host = os.Getenv("DB_HOST")
				port = os.Getenv("DB_PORT")
				name = os.Getenv("DB_NAME")
			}
		} else {
			host = os.Getenv("DB_HOST")
			port = os.Getenv("DB_PORT")
			name = os.Getenv("DB_NAME")
		}
	}
	log.Printf("Connecting DB with DSN")

	var dialector gorm.Dialector
	switch dbType {
	case "postgres", "postgresql":
		dialector = postgres.Open(dsn)
	case "mysql", "mariadb":
		dialector = mysql.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	gdb, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open gorm database: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB from gorm DB: %w", err)
	}
	configurePool(sqlDB)
	// Ping to verify connectivity
	if err := sqlDB.Ping(); err != nil {
		fmt.Printf("Database connection FAILED: host=%s port=%s name=%s error=%v\n", host, port, name, err)
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	log.Printf("Database connected")
	return gdb, nil
}

func configurePool(sqlDB *sql.DB) {
	maxOpen := intFromEnv("DB_MAX_OPEN_CONNS", 50)
	maxIdle := intFromEnv("DB_MAX_IDLE_CONNS", 10)
	maxLifeSec := intFromEnv("DB_CONN_MAX_LIFETIME_SEC", 300)

	if maxOpen > 0 {
		sqlDB.SetMaxOpenConns(maxOpen)
	}
	if maxIdle >= 0 {
		sqlDB.SetMaxIdleConns(maxIdle)
	}
	if maxLifeSec > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(maxLifeSec) * time.Second)
	}
}

func intFromEnv(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func detectDBType(dsn string) string {
	if dsn == "" {
		if os.Getenv("DB_TYPE") != "" {
			return strings.ToLower(os.Getenv("DB_TYPE"))
		}
		port := os.Getenv("DB_PORT")
		if port == "3306" || port == "33060" {
			return "mysql"
		}
		return "postgres"
	}
	if u, err := url.Parse(dsn); err == nil {
		scheme := strings.ToLower(u.Scheme)
		if scheme == "postgresql" || scheme == "postgres" {
			return "postgres"
		}
		if scheme == "mysql" || scheme == "mariadb" {
			return "mysql"
		}
	}
	return "postgres"
}

// maskDSN masks the password in a DSN string for safe logging.
func maskDSN(dsn string) string {
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" { // URL style
		if u.User != nil {
			if username := u.User.Username(); username != "" {
				// Replace password if present
				if _, ok := u.User.Password(); ok {
					u.User = url.UserPassword(username, "***")
				}
			}
		}
		return u.String()
	}
	// MySQL style: user:pass@...
	atIdx := strings.Index(dsn, "@")
	if atIdx > 0 {
		userPart := dsn[:atIdx]
		if colonIdx := strings.Index(userPart, ":"); colonIdx > 0 {
			// mask the password portion
			return userPart[:colonIdx+1] + "***" + dsn[atIdx:]
		}
	}
	return dsn
}

// parseMySQLDSN extracts the user, host, port, and dbname from a MySQL-style DSN.
// Example DSN: user:pass@tcp(127.0.0.1:3306)/dbname?parseTime=true
func parseMySQLDSN(dsn string) (user, host, port, name string) {
	// Extract user:pass@ prefix
	atIdx := strings.Index(dsn, "@")
	if atIdx > 0 {
		userPart := dsn[:atIdx]
		if strings.Contains(userPart, ":") {
			user = strings.SplitN(userPart, ":", 2)[0]
		} else {
			user = userPart
		}
	}
	// Find tcp(host:port) or (host:port)
	// regexp to find tcp(...) or unix(...) or tcp(...) patterns
	re := regexp.MustCompile(`tcp\(([^)]+)\)`) // capture inside tcp(...)
	match := re.FindStringSubmatch(dsn)
	if len(match) >= 2 {
		addr := match[1]
		if strings.Contains(addr, ":") {
			parts := strings.SplitN(addr, ":", 2)
			host = parts[0]
			port = parts[1]
		} else {
			host = addr
		}
	} else {
		// try simple host:port after userinfo (rare)
		// fallback: parse host:port from dsn using regex ip:port
		re2 := regexp.MustCompile(`@[^/]+\(([^)]+)\)`) // @proto(addr)
		match2 := re2.FindStringSubmatch(dsn)
		if len(match2) >= 2 {
			addr := match2[1]
			if strings.Contains(addr, ":") {
				parts := strings.SplitN(addr, ":", 2)
				host = parts[0]
				port = parts[1]
			} else {
				host = addr
			}
		}
	}
	// Extract dbname: after last "/" before "?" or end
	slashIdx := strings.LastIndex(dsn, "/")
	if slashIdx >= 0 && slashIdx+1 < len(dsn) {
		rest := dsn[slashIdx+1:]
		qIdx := strings.Index(rest, "?")
		if qIdx >= 0 {
			name = rest[:qIdx]
		} else {
			name = rest
		}
	}
	// detect unix socket like @/dbname
	if strings.Contains(dsn, "@/") {
		idx := strings.Index(dsn, "@/")
		if idx > 0 {
			u := dsn[:idx]
			if strings.Contains(u, ":") {
				user = strings.SplitN(u, ":", 2)[0]
			} else {
				user = u
			}
		}
		rest := dsn[idx+2:]
		qIdx := strings.Index(rest, "?")
		if qIdx >= 0 {
			name = rest[:qIdx]
		} else {
			name = rest
		}
		host = "(socket)"
		return
	}
	// detect unix socket unix(/path)
	reUnix := regexp.MustCompile(`unix\(([^)]+)\)`) // capture inside unix(...)
	matchUnix := reUnix.FindStringSubmatch(dsn)
	if len(matchUnix) >= 2 {
		host = "unix:" + matchUnix[1]
		// name from slash after )/
		paren := strings.Index(dsn, ")/")
		if paren >= 0 {
			rest := dsn[paren+2:]
			qIdx := strings.Index(rest, "?")
			if qIdx >= 0 {
				name = rest[:qIdx]
			} else {
				name = rest
			}
		}
		return
	}
	return
}
