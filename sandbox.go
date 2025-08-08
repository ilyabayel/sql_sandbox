package sql_sandbox

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// Sandbox represents a PostgreSQL test database sandbox
type Sandbox struct {
	TemplateDB *sql.DB
	TestDB     *sql.DB
	DBName     string
	Config     *Config
	mu         sync.Mutex
}

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
	return NewWithMigrationChecker(mainDBURL, config, nil)
}

// NewWithMigrationChecker creates a new sandbox instance with a custom migration checker
func NewWithMigrationChecker(mainDBURL string, config *Config, migrationChecker MigrationChecker) (*Sandbox, error) {
	if config == nil {
		config = DefaultConfig()
	}
	config.MainDBURL = mainDBURL

	// Use default migration checker if none provided
	if migrationChecker == nil {
		migrationChecker = &DefaultMigrationChecker{}
	}

	// Ensure main database is migrated to latest version
	if err := migrationChecker.EnsureMigrated(config.MainDBURL); err != nil {
		return nil, fmt.Errorf("failed to ensure main DB is migrated: %w", err)
	}

	// Connect to main database
	mainDB, err := sql.Open("postgres", config.MainDBURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to main database: %w", err)
	}
	defer mainDB.Close()

	// Create template database if it doesn't exist
	if err := createTemplateDatabase(mainDB, config.TemplateDBName); err != nil {
		return nil, fmt.Errorf("failed to create template database: %w", err)
	}

	// Generate unique test database name
	testDBName := generateUniqueDBName(config.TestDBPrefix)

	// Create test database from main database
	_, err = createTestDatabase(mainDB, "main_db", testDBName)
	if err != nil {
		return nil, fmt.Errorf("failed to create test database: %w", err)
	}

	// Connect to test database
	testDBConn, err := sql.Open("postgres", replaceDBName(config.MainDBURL, testDBName))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	// Configure connection pool
	testDBConn.SetMaxOpenConns(config.MaxConnections)
	testDBConn.SetConnMaxLifetime(config.ConnectionTimeout)

	return &Sandbox{
		TemplateDB: mainDB,
		TestDB:     testDBConn,
		DBName:     testDBName,
		Config:     config,
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
		if err := dropTestDatabase(s.TemplateDB, s.DBName); err != nil {
			errors = append(errors, fmt.Sprintf("failed to drop test database: %v", err))
		}
	}

	// Close template database connection
	if s.TemplateDB != nil {
		if err := s.TemplateDB.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to close template DB connection: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during cleanup: %s", strings.Join(errors, "; "))
	}

	return nil
}

// ensureMainDBMigrated checks if the main database is migrated to the latest version
func ensureMainDBMigrated(mainDBURL string) error {
	// This is a placeholder implementation
	// In a real implementation, you would check your migration system
	// For example, using golang-migrate, goose, or your custom migration system

	db, err := sql.Open("postgres", mainDBURL)
	if err != nil {
		return fmt.Errorf("failed to connect to main database: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping main database: %w", err)
	}

	// Here you would typically check migration status
	// For example:
	// - Check if migration table exists
	// - Verify all migrations are applied
	// - Run any pending migrations

	log.Println("Main database migration check completed")
	return nil
}

// createTemplateDatabase creates a template database from the main database
func createTemplateDatabase(mainDB *sql.DB, templateDBName string) error {
	// Check if template database already exists
	var exists bool
	err := mainDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", templateDBName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if template database exists: %w", err)
	}

	if exists {
		log.Printf("Template database '%s' already exists", templateDBName)
		return nil
	}

	// Get current database name
	var currentDB string
	err = mainDB.QueryRow("SELECT current_database()").Scan(&currentDB)
	if err != nil {
		return fmt.Errorf("failed to get current database name: %w", err)
	}

	// Create template database
	_, err = mainDB.Exec(fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", templateDBName, currentDB))
	if err != nil {
		return fmt.Errorf("failed to create template database: %w", err)
	}

	log.Printf("Created template database '%s'", templateDBName)
	return nil
}

// createTestDatabase creates a test database from the template
func createTestDatabase(mainDB *sql.DB, templateDBName, testDBName string) (*sql.DB, error) {
	// Create test database from template
	_, err := mainDB.Exec(fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", testDBName, templateDBName))
	if err != nil {
		return nil, fmt.Errorf("failed to create test database: %w", err)
	}

	log.Printf("Created test database '%s' from template", testDBName)
	return nil, nil
}

// dropTestDatabase drops the test database
func dropTestDatabase(mainDB *sql.DB, testDBName string) error {
	// Terminate all connections to the test database
	_, err := mainDB.Exec(fmt.Sprintf(`
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = '%s' AND pid <> pg_backend_pid()
	`, testDBName))
	if err != nil {
		log.Printf("Warning: failed to terminate connections to test database: %v", err)
	}

	// Drop the test database
	_, err = mainDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	if err != nil {
		return fmt.Errorf("failed to drop test database: %w", err)
	}

	log.Printf("Dropped test database '%s'", testDBName)
	return nil
}

// generateUniqueDBName generates a unique database name
func generateUniqueDBName(prefix string) string {
	timestamp := time.Now().UnixNano()
	pid := os.Getpid()
	return fmt.Sprintf("%s%d_%d", prefix, timestamp, pid)
}

// replaceDBName replaces the database name in a connection string
func replaceDBName(connStr, newDBName string) string {
	// Parse the connection string properly
	if strings.Contains(connStr, "dbname=") {
		// Replace existing dbname parameter
		parts := strings.Split(connStr, " ")
		for i, part := range parts {
			if strings.HasPrefix(part, "dbname=") {
				parts[i] = "dbname=" + newDBName
				break
			}
		}
		return strings.Join(parts, " ")
	}

	// Add dbname parameter if it doesn't exist
	if strings.Contains(connStr, "?") {
		// Connection string has query parameters
		return connStr + "&dbname=" + newDBName
	}

	// No query parameters, add dbname
	return connStr + "?dbname=" + newDBName
}
