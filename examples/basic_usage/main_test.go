package main

import (
	"os"
	"testing"
	"time"

	"sql_sandbox"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestDBURL returns the test database URL from environment or default
func getTestDBURL() string {
	if url := os.Getenv("POSTGRES_URL"); url != "" {
		return url
	}
	return "postgres://testuser:testpass@localhost:5433/main_db?sslmode=disable"
}

func TestUserService_CreateUser(t *testing.T) {
	// Skip if no database is available
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Create sandbox
	mainDBURL := getTestDBURL()
	sandbox, err := sql_sandbox.New(mainDBURL, nil)
	require.NoError(t, err)
	defer sandbox.Close()

	// Setup database
	db := sandbox.DB()
	err = setupDatabase(db)
	require.NoError(t, err)

	// Create service
	service := NewUserService(db)

	// Test creating a user
	user, err := service.CreateUser("Test User", "test@example.com")
	require.NoError(t, err)
	assert.NotZero(t, user.ID)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "test@example.com", user.Email)
	assert.False(t, user.CreatedAt.IsZero())
}

func TestUserService_GetUserByID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	mainDBURL := getTestDBURL()
	sandbox, err := sql_sandbox.New(mainDBURL, nil)
	require.NoError(t, err)
	defer sandbox.Close()

	db := sandbox.DB()
	err = setupDatabase(db)
	require.NoError(t, err)

	service := NewUserService(db)

	// Create a user first
	createdUser, err := service.CreateUser("John Doe", "john@example.com")
	require.NoError(t, err)

	// Retrieve the user
	retrievedUser, err := service.GetUserByID(createdUser.ID)
	require.NoError(t, err)
	assert.Equal(t, createdUser.ID, retrievedUser.ID)
	assert.Equal(t, createdUser.Name, retrievedUser.Name)
	assert.Equal(t, createdUser.Email, retrievedUser.Email)
}

func TestUserService_ListUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	mainDBURL := getTestDBURL()
	sandbox, err := sql_sandbox.New(mainDBURL, nil)
	require.NoError(t, err)
	defer sandbox.Close()

	db := sandbox.DB()
	err = setupDatabase(db)
	require.NoError(t, err)

	service := NewUserService(db)

	// Create multiple users
	user1, err := service.CreateUser("User 1", "user1@example.com")
	require.NoError(t, err)

	user2, err := service.CreateUser("User 2", "user2@example.com")
	require.NoError(t, err)

	// List all users
	users, err := service.ListUsers()
	require.NoError(t, err)
	assert.Len(t, users, 2)

	// Check that both users are in the list
	userIDs := make(map[int]bool)
	for _, user := range users {
		userIDs[user.ID] = true
	}
	assert.True(t, userIDs[user1.ID])
	assert.True(t, userIDs[user2.ID])
}

func TestUserService_Isolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// This test demonstrates that each test gets its own isolated database
	mainDBURL := getTestDBURL()
	sandbox, err := sql_sandbox.New(mainDBURL, nil)
	require.NoError(t, err)
	defer sandbox.Close()

	db := sandbox.DB()
	err = setupDatabase(db)
	require.NoError(t, err)

	service := NewUserService(db)

	// Create a user
	user, err := service.CreateUser("Isolation Test", "isolation@example.com")
	require.NoError(t, err)

	// Verify the user exists
	retrievedUser, err := service.GetUserByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, retrievedUser.ID)

	// This test should be isolated from other tests
	// Each test gets its own database copy
}

func TestUserService_Parallel(t *testing.T) {
	// This test can run in parallel with other tests
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	mainDBURL := getTestDBURL()
	sandbox, err := sql_sandbox.New(mainDBURL, nil)
	require.NoError(t, err)
	defer sandbox.Close()

	db := sandbox.DB()
	err = setupDatabase(db)
	require.NoError(t, err)

	service := NewUserService(db)

	// Create a user with a unique name to avoid conflicts
	uniqueName := "Parallel User " + time.Now().String()
	user, err := service.CreateUser(uniqueName, "parallel@example.com")
	require.NoError(t, err)
	assert.NotZero(t, user.ID)
	assert.Equal(t, uniqueName, user.Name)
}

// Benchmark test to show performance
func BenchmarkUserService_CreateUser(b *testing.B) {
	mainDBURL := getTestDBURL()
	sandbox, err := sql_sandbox.New(mainDBURL, nil)
	if err != nil {
		b.Skip("Database not available")
	}
	defer sandbox.Close()

	db := sandbox.DB()
	err = setupDatabase(db)
	if err != nil {
		b.Skip("Failed to setup database")
	}

	service := NewUserService(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.CreateUser("Benchmark User", "benchmark@example.com")
		if err != nil {
			b.Fatal(err)
		}
	}
}
