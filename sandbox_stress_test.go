package sql_sandbox

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMigrationChecker is a mock that counts how many times EnsureMigrated is called
type MockMigrationChecker struct {
	CallCount int32
}

func (m *MockMigrationChecker) EnsureMigrated(dbURL string) error {
	return m.EnsureMigratedWithContext(context.Background(), dbURL)
}

func (m *MockMigrationChecker) EnsureMigratedWithContext(ctx context.Context, dbURL string) error {
	atomic.AddInt32(&m.CallCount, 1)
	// Simulate some heavy work
	time.Sleep(50 * time.Millisecond)
	return nil
}

func getStressTestDBURL(dbName string) string {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		url = "postgres://testuser:testpass@127.0.0.1:5433/" + dbName + "?sslmode=disable"
	} else {
		url = ReplaceDBName(url, dbName)
	}
	return url
}

func countActiveConnections(t *testing.T, dbName string) int {
	adminURL := ReplaceDBName(getStressTestDBURL("postgres"), "postgres")
	db, err := sql.Open("postgres", adminURL)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow(`
		SELECT count(*)
		FROM pg_stat_activity
		WHERE datname = $1 AND pid <> pg_backend_pid()
	`, dbName).Scan(&count)
	require.NoError(t, err)
	return count
}

func countTotalConnections(t *testing.T) int {
	adminURL := ReplaceDBName(getStressTestDBURL("postgres"), "postgres")
	db, err := sql.Open("postgres", adminURL)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow(`
		SELECT count(*)
		FROM pg_stat_activity
	`).Scan(&count)
	require.NoError(t, err)
	return count
}

func TestSandbox_MigrationDeduplicationAndParallelStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Reset global map to ensure a clean slate for the test
	setupMap.Clear()

	mainDBURL := getStressTestDBURL("main_db_root")
	checker := &MockMigrationChecker{}

	// Launch 50 parallel creations
	numRoutines := 50
	var wg sync.WaitGroup
	var errs []error
	var errMu sync.Mutex

	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			config := &Config{
				TemplateDBName:    "template_stress_test",
				TestDBPrefix:      "stress_test_",
				MaxConnections:    2,
				ConnectionTimeout: 5 * time.Second,
			}
			sandbox, err := NewWithMigrationChecker(mainDBURL, config, checker)
			if err != nil {
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
				return
			}
			// Do a quick query
			db := sandbox.DB()
			_, err = db.Exec("SELECT 1")
			if err != nil {
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
			}
			
			// Close immediately to not exhaust connections from test side
			err = sandbox.Close()
			if err != nil {
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
			}
		}()
	}

	wg.Wait()

	require.Empty(t, errs, "Expected 0 errors from parallel creation")
	// The core assertion: migrations should only be run exactly once!
	assert.Equal(t, int32(1), checker.CallCount, "EnsureMigrated should only be called once")
}

func TestSandbox_MultipleSourceDBsConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
	
	setupMap.Clear()

	db1URL := getStressTestDBURL("main_db_root")
	db2URL := getStressTestDBURL("main_db_basic_usage")
	
	checker := &MockMigrationChecker{}

	var wg sync.WaitGroup
	
	runTest := func(url, template string) {
		defer wg.Done()
		config := &Config{
			TemplateDBName: template,
			TestDBPrefix:   "multi_stress_",
			MaxConnections: 2,
		}
		sandbox, err := NewWithMigrationChecker(url, config, checker)
		require.NoError(t, err)
		sandbox.Close()
	}

	// Launch 20 for DB 1 and 20 for DB 2
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go runTest(db1URL, "template_db1")
		go runTest(db2URL, "template_db2")
	}

	wg.Wait()

	// Should be called exactly twice (once for each source DB)
	assert.Equal(t, int32(2), checker.CallCount, "EnsureMigrated should be called exactly twice")
}

func TestSandbox_ConnectionLeakVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	mainDBURL := getStressTestDBURL("main_db_root")
	
	// Get baseline connections
	baselineConns := countTotalConnections(t)

	// Create and close 50 sandboxes sequentially
	// If we were leaking the adminDB connection pool, this would fail quickly
	for i := 0; i < 50; i++ {
		config := &Config{
			TemplateDBName: "template_leak_test",
			TestDBPrefix:   "leak_test_",
			MaxConnections: 2,
		}
		sandbox, err := New(mainDBURL, config)
		require.NoError(t, err)
		
		err = sandbox.Close()
		require.NoError(t, err)
	}

	// Wait a moment for PostgreSQL to clean up terminated connections
	time.Sleep(100 * time.Millisecond)

	finalConns := countTotalConnections(t)
	
	// There shouldn't be a significant increase in total active connections
	// Allow for a small delta (e.g. background workers or other concurrent tests)
	assert.LessOrEqual(t, finalConns-baselineConns, 5, "Connection leak detected!")
}

func TestSandbox_TeardownReliability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	mainDBURL := getStressTestDBURL("main_db_root")
	
	config := &Config{
		TemplateDBName: "template_teardown",
		TestDBPrefix:   "teardown_test_",
		MaxConnections: 5,
	}
	
	sandbox, err := New(mainDBURL, config)
	require.NoError(t, err)
	dbName := sandbox.DBName

	// Open a connection that deliberately stays open and active
	db := sandbox.DB()
	
	// Start a transaction that we won't close
	tx, err := db.Begin()
	require.NoError(t, err)
	
	_, err = tx.Exec("CREATE TABLE dummy (id INT)")
	require.NoError(t, err)

	// Active connections should be > 0
	activeCount := countActiveConnections(t, dbName)
	assert.Greater(t, activeCount, 0, "Expected active connections")

	// Now close the sandbox - it should forcibly drop the database
	err = sandbox.Close()
	require.NoError(t, err)

	// Verify the database is actually gone
	adminURL := ReplaceDBName(getStressTestDBURL("postgres"), "postgres")
	adminDB, err := sql.Open("postgres", adminURL)
	require.NoError(t, err)
	defer adminDB.Close()

	var exists bool
	err = adminDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	require.NoError(t, err)
	assert.False(t, exists, "Database should be dropped despite active connections")
}
