package sql_sandbox

import (
	"database/sql"
	"os"
	"testing"
	"time"

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

func TestSandbox(t *testing.T) {
	// This is an example test showing how to use the sandbox
	// In a real scenario, you would use your actual database connection string
	mainDBURL := getTestDBURL()

	// Skip this test if no database is available
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Create sandbox configuration
	config := &Config{
		TemplateDBName:    "template_test",
		TestDBPrefix:      "test_db_",
		MaxConnections:    5,
		ConnectionTimeout: 10 * time.Second,
	}

	// Create new sandbox
	sandbox, err := New(mainDBURL, config)
	require.NoError(t, err)
	defer sandbox.Close()

	// Get database connection
	db := sandbox.DB()
	require.NotNil(t, db)

	// Test database connection
	err = db.Ping()
	require.NoError(t, err)

	// Example: Create a table and insert data
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	// Insert test data
	_, err = db.Exec(`
		INSERT INTO users (name, email) VALUES 
		('John Doe', 'john@example.com'),
		('Jane Smith', 'jane@example.com')
	`)
	require.NoError(t, err)

	// Query data
	rows, err := db.Query("SELECT id, name, email FROM users ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()

	var users []struct {
		ID    int
		Name  string
		Email string
	}

	for rows.Next() {
		var user struct {
			ID    int
			Name  string
			Email string
		}
		err := rows.Scan(&user.ID, &user.Name, &user.Email)
		require.NoError(t, err)
		users = append(users, user)
	}

	require.NoError(t, rows.Err())
	assert.Len(t, users, 2)
	assert.Equal(t, "John Doe", users[0].Name)
	assert.Equal(t, "jane@example.com", users[1].Email)

	// The sandbox will be automatically cleaned up when Close() is called
}

func TestSandboxWithDependencyInjection(t *testing.T) {
	// This test demonstrates how to use the sandbox with dependency injection
	mainDBURL := getTestDBURL()

	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Create sandbox
	sandbox, err := New(mainDBURL, nil) // Use default config
	require.NoError(t, err)
	defer sandbox.Close()

	// Example service that uses database dependency injection
	service := NewUserService(sandbox.DB())

	// Test the service
	user, err := service.CreateUser("Test User", "test@example.com")
	require.NoError(t, err)
	assert.NotZero(t, user.ID)
	assert.Equal(t, "Test User", user.Name)

	// Query user back
	foundUser, err := service.GetUserByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.Name, foundUser.Name)
	assert.Equal(t, user.Email, foundUser.Email)
}

// Example service demonstrating dependency injection
type User struct {
	ID        int
	Name      string
	Email     string
	CreatedAt time.Time
}

type UserService struct {
	db *sql.DB
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) CreateUser(name, email string) (*User, error) {
	query := `
		INSERT INTO users (name, email) 
		VALUES ($1, $2) 
		RETURNING id, name, email, created_at
	`

	var user User
	err := s.db.QueryRow(query, name, email).Scan(
		&user.ID, &user.Name, &user.Email, &user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *UserService) GetUserByID(id int) (*User, error) {
	query := `
		SELECT id, name, email, created_at 
		FROM users 
		WHERE id = $1
	`

	var user User
	err := s.db.QueryRow(query, id).Scan(
		&user.ID, &user.Name, &user.Email, &user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
