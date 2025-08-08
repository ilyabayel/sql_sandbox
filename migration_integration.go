package sql_sandbox

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// MigrationChecker defines the interface for checking and running migrations
type MigrationChecker interface {
	EnsureMigrated(dbURL string) error
}

// DefaultMigrationChecker provides a basic implementation
type DefaultMigrationChecker struct{}

// EnsureMigrated implements basic migration checking
func (d *DefaultMigrationChecker) EnsureMigrated(dbURL string) error {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Check if migration table exists (basic check)
	var tableExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'schema_migrations'
		)
	`).Scan(&tableExists)

	if err != nil {
		log.Printf("Warning: Could not check migration table: %v", err)
		// Don't fail the check, just log a warning
		return nil
	}

	if !tableExists {
		log.Println("Warning: No migration table found. Consider running migrations.")
	}

	log.Println("Migration check completed")
	return nil
}

// GolangMigrateChecker integrates with golang-migrate library
type GolangMigrateChecker struct {
	MigrationsPath string
}

// NewGolangMigrateChecker creates a new golang-migrate checker
func NewGolangMigrateChecker(migrationsPath string) *GolangMigrateChecker {
	return &GolangMigrateChecker{
		MigrationsPath: migrationsPath,
	}
}

// EnsureMigrated runs migrations using golang-migrate library
func (g *GolangMigrateChecker) EnsureMigrated(dbURL string) error {
	// Check if migrations directory exists
	if _, err := os.Stat(g.MigrationsPath); os.IsNotExist(err) {
		return fmt.Errorf("migrations directory not found: %s", g.MigrationsPath)
	}

	// Get absolute path to migrations
	absPath, err := filepath.Abs(g.MigrationsPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for migrations: %w", err)
	}

	// Create migration source URL
	sourceURL := fmt.Sprintf("file://%s", absPath)

	// Create migrate instance
	m, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("Migrations completed successfully")
	return nil
}

// GooseMigrateChecker integrates with goose migration tool
type GooseMigrateChecker struct {
	MigrationsPath string
	BinaryPath     string
}

// NewGooseMigrateChecker creates a new goose checker
func NewGooseMigrateChecker(migrationsPath string) *GooseMigrateChecker {
	return &GooseMigrateChecker{
		MigrationsPath: migrationsPath,
		BinaryPath:     "goose", // Assumes goose binary is in PATH
	}
}

// EnsureMigrated runs migrations using goose
func (g *GooseMigrateChecker) EnsureMigrated(dbURL string) error {
	// Check if goose binary exists
	if _, err := exec.LookPath(g.BinaryPath); err != nil {
		return fmt.Errorf("goose binary not found in PATH: %w", err)
	}

	// Check if migrations directory exists
	if _, err := os.Stat(g.MigrationsPath); os.IsNotExist(err) {
		return fmt.Errorf("migrations directory not found: %s", g.MigrationsPath)
	}

	// Run migrations
	cmd := exec.Command(g.BinaryPath, "-dir", g.MigrationsPath, "postgres", dbURL, "up")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w, output: %s", err, string(output))
	}

	log.Printf("Migrations completed successfully: %s", string(output))
	return nil
}

// CustomMigrationChecker allows for custom migration logic
type CustomMigrationChecker struct {
	CheckFunc func(dbURL string) error
}

// NewCustomMigrationChecker creates a custom migration checker
func NewCustomMigrationChecker(checkFunc func(dbURL string) error) *CustomMigrationChecker {
	return &CustomMigrationChecker{
		CheckFunc: checkFunc,
	}
}

// EnsureMigrated runs the custom migration check
func (c *CustomMigrationChecker) EnsureMigrated(dbURL string) error {
	return c.CheckFunc(dbURL)
}

// Example usage functions

// ExampleGolangMigrateIntegration shows how to use golang-migrate
func ExampleGolangMigrateIntegration() {
	mainDBURL := "postgres://user:pass@localhost:5432/main_db?sslmode=disable"

	// Create migration checker
	migrationChecker := NewGolangMigrateChecker("./migrations")

	// Create sandbox with custom migration checker
	sandbox, err := NewWithMigrationChecker(mainDBURL, nil, migrationChecker)
	if err != nil {
		log.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Close()

	log.Printf("Using golang-migrate for migrations")
	log.Printf("Sandbox created successfully with database: %s", sandbox.DBName)
}

// ExampleGooseIntegration shows how to use goose
func ExampleGooseIntegration() {
	mainDBURL := "postgres://user:pass@localhost:5432/main_db?sslmode=disable"

	// Create migration checker
	migrationChecker := NewGooseMigrateChecker("./db/migrations")

	log.Printf("Using goose for migrations")
	if err := migrationChecker.EnsureMigrated(mainDBURL); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
}

// ExampleCustomMigrationIntegration shows how to use custom migration logic
func ExampleCustomMigrationIntegration() {
	mainDBURL := "postgres://user:pass@localhost:5432/main_db?sslmode=disable"

	// Custom migration function
	customMigration := func(dbURL string) error {
		// Your custom migration logic here
		// For example, running SQL scripts, checking version tables, etc.
		log.Println("Running custom migration logic")
		return nil
	}

	migrationChecker := NewCustomMigrationChecker(customMigration)

	log.Printf("Using custom migration logic")
	if err := migrationChecker.EnsureMigrated(mainDBURL); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
}
