package main

import (
	"os"
	"testing"

	"github.com/ilyabayel/sql_sandbox"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestDBURL returns the test database URL from environment or default
func getTestDBURL() string {
	if url := os.Getenv("POSTGRES_URL"); url != "" {
		return url
	}
	return "postgres://testuser:testpass@127.0.0.1:5433/main_db_with_migrations?sslmode=disable"
}

func TestSandboxWithMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Use static migrations directory
	migrationsDir := "./migrations"

	// Create migration checker using golang-migrate
	migrationChecker := sql_sandbox.NewGolangMigrateChecker(migrationsDir)

	// Create sandbox with migration checker
	mainDBURL := getTestDBURL()
	sandbox, err := sql_sandbox.NewWithMigrationChecker(mainDBURL, nil, migrationChecker)
	require.NoError(t, err)
	defer sandbox.Close()

	// Get database connection
	db := sandbox.DB()

	// Test database connection
	err = db.Ping()
	require.NoError(t, err)

	// Create user service
	userService := NewUserService(db)

	// Test creating a user
	user, err := userService.CreateUser("Migration Test User", "migration@example.com")
	require.NoError(t, err)
	assert.NotZero(t, user.ID)
	assert.Equal(t, "Migration Test User", user.Name)
	assert.Equal(t, "migration@example.com", user.Email)

	// Test retrieving the user
	retrievedUser, err := userService.GetUserByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, retrievedUser.ID)
	assert.Equal(t, user.Name, retrievedUser.Name)
	assert.Equal(t, user.Email, retrievedUser.Email)
}

func TestMigrationChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Use static migrations directory
	migrationsDir := "./migrations"

	// Test the migration checker directly
	migrationChecker := sql_sandbox.NewGolangMigrateChecker(migrationsDir)
	mainDBURL := getTestDBURL()

	err := migrationChecker.EnsureMigrated(mainDBURL)
	require.NoError(t, err)

	// Verify that the migration was applied by checking if the table exists
	db, err := sql_sandbox.New(mainDBURL, nil)
	require.NoError(t, err)
	defer db.Close()

	// Check if users table exists
	var tableExists bool
	err = db.DB().QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'users'
		)
	`).Scan(&tableExists)

	require.NoError(t, err)
	assert.True(t, tableExists, "Users table should exist after migration")
}
