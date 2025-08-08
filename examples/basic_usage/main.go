package main

import (
	"database/sql"
	"fmt"
	"log"
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

// setupDatabase creates the users table
func setupDatabase(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}
	
	return nil
}

func main() {
	// Example database connection string
	// In a real application, this would come from environment variables
	mainDBURL := "postgres://username:password@localhost:5432/main_db?sslmode=disable"
	
	// Create sandbox configuration
	config := &sql_sandbox.Config{
		TemplateDBName:  "template_test",
		TestDBPrefix:    "test_db_",
		MaxConnections:  5,
		ConnectionTimeout: 10 * time.Second,
	}
	
	// Create sandbox
	sandbox, err := sql_sandbox.New(mainDBURL, config)
	if err != nil {
		log.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Close()
	
	// Get database connection
	db := sandbox.DB()
	
	// Setup database schema
	if err := setupDatabase(db); err != nil {
		log.Fatalf("Failed to setup database: %v", err)
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
	fmt.Println("Example completed successfully!")
}
