package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"sql_sandbox"
)

// User represents a user in the system
type User struct {
	ID        int
	Name      string
	Email     string
	CreatedAt time.Time
}

// UserService handles user operations
type UserService struct {
	db *sql.DB
}

// NewUserService creates a new user service
func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

// CreateUser creates a new user
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
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
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
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// ListUsers retrieves all users
func (s *UserService) ListUsers() ([]*User, error) {
	query := `
		SELECT id, name, email, created_at 
		FROM users 
		ORDER BY id
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// createMigrationFiles creates example migration files
func createMigrationFiles(migrationsDir string) error {
	// Create migrations directory if it doesn't exist
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Create up migration
	upMigration := `-- +migrate Up
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on email for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
`

	// Create down migration
	downMigration := `-- +migrate Down
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
`

	// Write migration files
	upFile := filepath.Join(migrationsDir, "001_create_users_table.up.sql")
	if err := os.WriteFile(upFile, []byte(upMigration), 0644); err != nil {
		return fmt.Errorf("failed to write up migration: %w", err)
	}

	downFile := filepath.Join(migrationsDir, "001_create_users_table.down.sql")
	if err := os.WriteFile(downFile, []byte(downMigration), 0644); err != nil {
		return fmt.Errorf("failed to write down migration: %w", err)
	}

	log.Printf("Created migration files in: %s", migrationsDir)
	return nil
}

func main() {
	// Example database connection string
	// In a real application, this would come from environment variables
	mainDBURL := "postgres://username:password@localhost:5432/main_db?sslmode=disable"

	// Create migrations directory and files
	migrationsDir := "./migrations"
	if err := createMigrationFiles(migrationsDir); err != nil {
		log.Fatalf("Failed to create migration files: %v", err)
	}

	// Create migration checker using golang-migrate
	migrationChecker := sql_sandbox.NewGolangMigrateChecker(migrationsDir)

	// Create sandbox configuration
	config := &sql_sandbox.Config{
		TemplateDBName:    "template_test",
		TestDBPrefix:      "test_db_",
		MaxConnections:    5,
		ConnectionTimeout: 10 * time.Second,
	}

	// Create sandbox with migration checker
	sandbox, err := sql_sandbox.NewWithMigrationChecker(mainDBURL, config, migrationChecker)
	if err != nil {
		log.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Close()

	log.Printf("Created sandbox with database: %s", sandbox.DBName)

	// Get database connection
	db := sandbox.DB()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create user service with dependency injection
	userService := NewUserService(db)

	// Test creating users
	user1, err := userService.CreateUser("John Doe", "john@example.com")
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}
	fmt.Printf("Created user: %+v\n", user1)

	user2, err := userService.CreateUser("Jane Smith", "jane@example.com")
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}
	fmt.Printf("Created user: %+v\n", user2)

	// Test retrieving user
	retrievedUser, err := userService.GetUserByID(user1.ID)
	if err != nil {
		log.Fatalf("Failed to get user: %v", err)
	}
	fmt.Printf("Retrieved user: %+v\n", retrievedUser)

	// Test listing users
	users, err := userService.ListUsers()
	if err != nil {
		log.Fatalf("Failed to list users: %v", err)
	}
	fmt.Printf("Total users: %d\n", len(users))

	// The sandbox will be automatically cleaned up when the program exits
	fmt.Println("Example with migrations completed successfully!")
}
