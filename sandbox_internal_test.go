package sql_sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractDBName(t *testing.T) {
	tests := []struct {
		name     string
		connStr  string
		expected string
	}{
		{
			name:     "standard DSN",
			connStr:  "user=postgres password=secret dbname=main_db sslmode=disable",
			expected: "main_db",
		},
		{
			name:     "DSN with dbname at start",
			connStr:  "dbname=main_db user=postgres password=secret sslmode=disable",
			expected: "main_db",
		},
		{
			name:     "URL format",
			connStr:  "postgres://user:pass@localhost:5432/main_db",
			expected: "main_db",
		},
		{
			name:     "URL with query params",
			connStr:  "postgres://user:pass@localhost:5432/main_db?sslmode=disable",
			expected: "main_db",
		},
		{
			name:     "URL with dbname query param replacing path",
			connStr:  "postgres://user:pass@localhost:5432/dummy_db?dbname=main_db&sslmode=disable",
			expected: "main_db",
		},
		{
			name:     "URL with dbname query param and no path",
			connStr:  "postgres://user:pass@localhost:5432?dbname=main_db&sslmode=disable",
			expected: "main_db",
		},
		{
			name:     "empty",
			connStr:  "",
			expected: "",
		},
		{
			name:     "DSN without dbname",
			connStr:  "user=postgres password=secret sslmode=disable",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := extractDBName(tc.connStr)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestReplaceDBName(t *testing.T) {
	tests := []struct {
		name      string
		connStr   string
		newDBName string
		expected  string
	}{
		{
			name:      "standard DSN",
			connStr:   "user=postgres password=secret dbname=main_db sslmode=disable",
			newDBName: "test_db",
			expected:  "user=postgres password=secret dbname=test_db sslmode=disable",
		},
		{
			name:      "DSN without dbname",
			connStr:   "user=postgres password=secret sslmode=disable",
			newDBName: "test_db",
			expected:  "user=postgres password=secret sslmode=disable dbname=test_db",
		},
		{
			name:      "URL format",
			connStr:   "postgres://user:pass@localhost:5432/main_db",
			newDBName: "test_db",
			expected:  "postgres://user:pass@localhost:5432/test_db",
		},
		{
			name:      "URL format with different scheme",
			connStr:   "postgresql://user:pass@localhost:5432/main_db",
			newDBName: "test_db",
			expected:  "postgresql://user:pass@localhost:5432/test_db",
		},
		{
			name:      "URL with query params",
			connStr:   "postgres://user:pass@localhost:5432/main_db?sslmode=disable",
			newDBName: "test_db",
			expected:  "postgres://user:pass@localhost:5432/test_db?sslmode=disable",
		},
		{
			name:      "URL with dbname query param",
			connStr:   "postgres://user:pass@localhost:5432/dummy_db?dbname=main_db&sslmode=disable",
			newDBName: "test_db",
			expected:  "postgres://user:pass@localhost:5432/test_db?sslmode=disable",
		},
		{
			name:      "unparseable fallback with query",
			connStr:   "invalid-conn-string?foo=bar",
			newDBName: "test_db",
			expected:  "invalid-conn-string?foo=bar dbname=test_db",
		},
		{
			name:      "unparseable fallback without query",
			connStr:   "invalid-conn-string",
			newDBName: "test_db",
			expected:  "invalid-conn-string?dbname=test_db",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := replaceDBName(tc.connStr, tc.newDBName)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
