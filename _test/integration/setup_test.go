//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := testDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: open db: %v\n", err)
		os.Exit(1)
	}
	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "integration: ping db: %v (is docker-compose.test.yml running?)\n", err)
		os.Exit(1)
	}
	testDB = db

	if err := applyMigrations(db); err != nil {
		fmt.Fprintf(os.Stderr, "integration: migrations: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	db.Close()
	os.Exit(code)
}

func testDSN() string {
	host := getenv("TEST_DB_HOST", "localhost")
	port := getenv("TEST_DB_PORT", "3307")
	name := getenv("TEST_DB_NAME", "gamebot_test")
	user := getenv("TEST_DB_USER", "gamebot_test")
	pass := getenv("TEST_DB_PASSWORD", "testpassword")
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&multiStatements=true",
		user, pass, host, port, name)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// applyMigrations reads all *.up.sql files from the migrations directory and executes them.
func applyMigrations(db *sql.DB) error {
	dir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)

	ctx := context.Background()
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		// Wrap in IF NOT EXISTS equivalent by ignoring duplicate table errors;
		// we use CREATE TABLE IF NOT EXISTS style when possible, otherwise
		// ignore "table already exists" errors.
		sql := string(data)
		// Replace CREATE TABLE with CREATE TABLE IF NOT EXISTS for idempotency.
		sql = strings.ReplaceAll(sql, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ")
		if _, err := db.ExecContext(ctx, sql); err != nil {
			// Ignore ALTER TABLE errors for columns that already exist.
			if !strings.Contains(err.Error(), "Duplicate column") {
				return fmt.Errorf("execute %s: %w", f, err)
			}
		}
	}
	return nil
}

// cleanupGame removes a game and all related data.
func cleanupGame(t *testing.T, db *sql.DB, gameID uint64) {
	t.Helper()
	_, err := db.Exec("DELETE FROM games WHERE id = ?", gameID)
	if err != nil {
		t.Logf("cleanup: delete game %d: %v", gameID, err)
	}
}
