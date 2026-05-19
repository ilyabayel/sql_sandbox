package sql_sandbox

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"
)

// Sandbox represents a PostgreSQL test database sandbox
type Sandbox struct {
	TestDB *sql.DB
	DBName string
	Config *Config
	mu     sync.Mutex
}

type setupState struct {
	once sync.Once
	err  error
}

var setupMap sync.Map // map[string]*setupState

// Config holds the configuration for the sandbox
type Config struct {
	MainDBURL         string
	TemplateDBName    string
	TestDBPrefix      string
	MaxConnections    int
	ConnectionTimeout time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		TemplateDBName:    "template_test",
		TestDBPrefix:      "test_db_",
		MaxConnections:    10,
		ConnectionTimeout: 30 * time.Second,
	}
}

// New creates a new sandbox instance
func New(mainDBURL string, config *Config) (*Sandbox, error) {
	return NewWithContext(context.Background(), mainDBURL, config)
}

// NewWithContext creates a new sandbox instance with context
func NewWithContext(ctx context.Context, mainDBURL string, config *Config) (*Sandbox, error) {
	return NewWithMigrationCheckerAndContext(ctx, mainDBURL, config, nil)
}

// NewWithMigrationChecker creates a new sandbox instance with a custom migration checker
func NewWithMigrationChecker(mainDBURL string, config *Config, migrationChecker MigrationChecker) (*Sandbox, error) {
	return NewWithMigrationCheckerAndContext(context.Background(), mainDBURL, config, migrationChecker)
}

// NewWithMigrationCheckerAndContext creates a new sandbox instance with a custom migration checker and context
func NewWithMigrationCheckerAndContext(ctx context.Context, mainDBURL string, config *Config, migrationChecker MigrationChecker) (*Sandbox, error) {
	if config == nil {
		config = DefaultConfig()
	}
	config.MainDBURL = mainDBURL

	// Use default migration checker if none provided
	if migrationChecker == nil {
		migrationChecker = &DefaultMigrationChecker{}
	}

	// Determine source database name from connection string
	sourceDBName := extractDBName(config.MainDBURL)
	if sourceDBName == "" {
		sourceDBName = "main_db"
	}

	// Connect to maintenance database (postgres) to manage DB-level operations
	adminConnStr := replaceDBName(config.MainDBURL, "postgres")
	adminDB, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to admin database: %w", err)
	}
	defer adminDB.Close()

	// Test admin database connection with context
	if err := adminDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping admin database: %w", err)
	}

	// Ensure template DB setup is done only once per sourceDBName + TemplateDBName
	setupKey := sourceDBName + "|" + config.TemplateDBName
	stateAny, _ := setupMap.LoadOrStore(setupKey, &setupState{})
	state := stateAny.(*setupState)

	state.once.Do(func() {
		// Ensure main database is migrated to latest version
		if err := migrationChecker.EnsureMigratedWithContext(ctx, config.MainDBURL); err != nil {
			state.err = fmt.Errorf("failed to ensure main DB is migrated: %w", err)
			return
		}

		// Create template database if it doesn't exist
		if err := createTemplateDatabase(ctx, adminDB, sourceDBName, config.TemplateDBName); err != nil {
			state.err = fmt.Errorf("failed to create template database: %w", err)
			return
		}
	})

	if state.err != nil {
		return nil, state.err
	}

	// Generate unique test database name
	testDBName := generateUniqueDBName(config.TestDBPrefix)

	// Create test database from the template database
	_, err = createTestDatabase(ctx, adminDB, config.TemplateDBName, testDBName)
	if err != nil {
		return nil, fmt.Errorf("failed to create test database: %w", err)
	}

	// Connect to test database
	testDBConn, err := sql.Open("postgres", replaceDBName(config.MainDBURL, testDBName))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	// Test test database connection with context
	if err := testDBConn.PingContext(ctx); err != nil {
		testDBConn.Close()
		return nil, fmt.Errorf("failed to ping test database: %w", err)
	}

	// Configure connection pool
	testDBConn.SetMaxOpenConns(config.MaxConnections)
	testDBConn.SetConnMaxLifetime(config.ConnectionTimeout)

	return &Sandbox{
		TestDB: testDBConn,
		DBName: testDBName,
		Config: config,
	}, nil
}

// DB returns the test database connection
func (s *Sandbox) DB() *sql.DB {
	return s.TestDB
}

// Close closes the sandbox and cleans up the test database
func (s *Sandbox) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errors []string

	// Close test database connection
	if s.TestDB != nil {
		if err := s.TestDB.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to close test DB connection: %v", err))
		}
	}

	// Drop test database
	if s.DBName != "" {
		adminConnStr := replaceDBName(s.Config.MainDBURL, "postgres")
		adminDB, err := sql.Open("postgres", adminConnStr)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to connect to admin DB to drop test DB: %v", err))
		} else {
			defer adminDB.Close()
			if err := dropTestDatabase(context.Background(), adminDB, s.DBName); err != nil {
				errors = append(errors, fmt.Sprintf("failed to drop test database: %v", err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during cleanup: %s", strings.Join(errors, "; "))
	}

	return nil
}

// createTemplateDatabase creates a template database from the main database
func createTemplateDatabase(ctx context.Context, adminDB *sql.DB, sourceDBName string, templateDBName string) error {
	log.Printf("Attempting to create template database '%s' from source '%s'", templateDBName, sourceDBName)
	// Terminate all connections to the source database before creating template
	_, err := adminDB.ExecContext(ctx, fmt.Sprintf(`
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = '%s' AND pid <> pg_backend_pid()
	`, sourceDBName))
	if err != nil {
		log.Printf("Warning: failed to terminate connections to source database: %v", err)
	}

	// Try to create the database first, then handle conflicts
	// This is more atomic than check-then-create
	_, err = adminDB.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE "%s" TEMPLATE "%s"`, templateDBName, sourceDBName))
	if err == nil {
		log.Printf("Created template database '%s'", templateDBName)
		return nil
	}

	// Handle various race condition scenarios
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "already exists") ||
		strings.Contains(errStr, "duplicate key value violates unique constraint") ||
		strings.Contains(errStr, "pg_database_datname_index") {
		log.Printf("Template database '%s' already exists (race condition)", templateDBName)
		return nil
	}

	// If it's not a race condition, check if the database actually exists now
	// This handles edge cases where the error message might be different
	var exists bool
	checkErr := adminDB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", templateDBName).Scan(&exists)
	if checkErr == nil && exists {
		log.Printf("Template database '%s' already exists (verified after error)", templateDBName)
		return nil
	}

	// If we get here, it's a real error
	return fmt.Errorf("failed to create template database: %w", err)
}

// createTestDatabase creates a test database from the template
func createTestDatabase(ctx context.Context, adminDB *sql.DB, templateDBName, testDBName string) (*sql.DB, error) {
	log.Printf("Attempting to create test database '%s' from template '%s'", testDBName, templateDBName)
	// Create test database from template
	_, err := adminDB.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE "%s" TEMPLATE "%s"`, testDBName, templateDBName))
	if err != nil {
		log.Printf("Failed to create test database: %v", err)
		return nil, fmt.Errorf("failed to create test database: %w", err)
	}

	log.Printf("Created test database '%s' from template", testDBName)
	return nil, nil
}

// dropTestDatabase drops the test database
func dropTestDatabase(ctx context.Context, adminDB *sql.DB, testDBName string) error {
	// Terminate all connections to the test database
	_, err := adminDB.ExecContext(ctx, fmt.Sprintf(`
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = '%s' AND pid <> pg_backend_pid()
	`, testDBName))
	if err != nil {
		log.Printf("Warning: failed to terminate connections to test database: %v", err)
	}

	// Drop the test database
	_, err = adminDB.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s" WITH (FORCE)`, testDBName))
	if err != nil {
		// Fallback for PostgreSQL < 13
		_, err = adminDB.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, testDBName))
		if err != nil {
			return fmt.Errorf("failed to drop test database: %w", err)
		}
	}

	return nil
}

var dbNameCounter int64

// generateUniqueDBName generates a unique database name
func generateUniqueDBName(prefix string) string {
	timestamp := time.Now().UnixNano()
	pid := os.Getpid()
	seq := atomic.AddInt64(&dbNameCounter, 1)
	return fmt.Sprintf("%s%d_%d_%d", prefix, timestamp, pid, seq)
}

// replaceDBName replaces the database name in a connection string
func replaceDBName(connStr, newDBName string) string {
	// Try parsing as DSN first if it contains key=value but is not a URL
	if strings.Contains(connStr, "=") && !strings.HasPrefix(connStr, "postgres://") && !strings.HasPrefix(connStr, "postgresql://") {
		parts := strings.Split(connStr, " ")
		hasDbname := false
		for i, part := range parts {
			if strings.HasPrefix(part, "dbname=") {
				parts[i] = "dbname=" + newDBName
				hasDbname = true
				break
			}
		}
		if !hasDbname {
			parts = append(parts, "dbname="+newDBName)
		}
		return strings.Join(parts, " ")
	}

	// Parse as URL
	if u, err := url.Parse(connStr); err == nil && (u.Scheme == "postgres" || u.Scheme == "postgresql") {
		u.Path = "/" + newDBName
		// Remove dbname from query string to avoid conflicts
		q := u.Query()
		q.Del("dbname")
		u.RawQuery = q.Encode()
		return u.String()
	}

	// Fallback to simple query appending if parse failed
	if strings.Contains(connStr, "?") {
		return connStr + "&dbname=" + newDBName
	}
	return connStr + "?dbname=" + newDBName
}

// extractDBName tries to determine the database name from a connection string.
// Supports URL formats like postgres://.../<db> and DSN formats with dbname=...
func extractDBName(connStr string) string {
	// URL format
	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		if u, err := url.Parse(connStr); err == nil {
			// Check query param dbname if present first
			if dbname := u.Query().Get("dbname"); dbname != "" {
				return dbname
			}
			// Path is like /main_db
			p := strings.TrimPrefix(u.Path, "/")
			if p != "" {
				return p
			}
		}
	}

	// DSN format
	if strings.Contains(connStr, "dbname=") {
		parts := strings.Split(connStr, " ")
		for _, part := range parts {
			if strings.HasPrefix(part, "dbname=") {
				return strings.TrimPrefix(part, "dbname=")
			}
		}
	}

	return ""
}
