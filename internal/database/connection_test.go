package database

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	customLog "github.com/glennprays/log"

	"github.com/glennprays/whatsapp-gateway/config"
)

func newTestLogger(t *testing.T) *customLog.Logger {
	t.Helper()
	logger, err := customLog.New(customLog.Config{
		Service: "database-test",
		Env:     "dev",
		Level:   customLog.ErrorLevel,
		Output:  customLog.OutputStdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	return logger
}

func newSQLiteTestConfig() *config.Config {
	return &config.Config{
		DBMaxOpenConns:    25, // must be ignored for sqlite
		DBMaxIdleConns:    5,
		DBConnMaxLifeMins: 5,
	}
}

func TestNewConnection_SQLitePoolClamped(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "test.db")

	db, err := NewConnection(newTestLogger(t), newSQLiteTestConfig(), "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if got := db.Stats().MaxOpenConnections; got != 1 {
		t.Errorf("MaxOpenConnections = %d, want 1 for sqlite", got)
	}
}

// TestNewConnection_SQLiteConcurrentWrites reproduces the "database is
// locked" failure mode: many goroutines writing simultaneously. With the
// DSN normalization and pool clamp it must complete without lock errors.
func TestNewConnection_SQLiteConcurrentWrites(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "concurrent.db")

	db, err := NewConnection(newTestLogger(t), newSQLiteTestConfig(), "sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY AUTOINCREMENT, val TEXT)`); err != nil {
		t.Fatal(err)
	}

	const goroutines = 50
	const writesPerGoroutine = 20

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*writesPerGoroutine)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < writesPerGoroutine; i++ {
				if _, err := db.Exec(`INSERT INTO items (val) VALUES (?)`, fmt.Sprintf("g%d-i%d", g, i)); err != nil {
					errCh <- err
				}
			}
		}(g)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if strings.Contains(err.Error(), "database is locked") {
			t.Fatalf("got 'database is locked' under concurrent writes: %v", err)
		}
		t.Errorf("unexpected write error: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if want := goroutines * writesPerGoroutine; count != want {
		t.Errorf("row count = %d, want %d", count, want)
	}
}
