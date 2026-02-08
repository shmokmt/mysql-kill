//go:build integration

package mysqlkill

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestBuildDSNConnect(t *testing.T) {
	cfg := MySQLConfig{
		Host:     testEnvOr(t, "MYSQL_TEST_HOST", "127.0.0.1"),
		Port:     testEnvIntOr(t, "MYSQL_TEST_PORT", 3307),
		User:     testEnvOr(t, "MYSQL_TEST_USER", "root"),
		Password: testEnvOr(t, "MYSQL_TEST_PASSWORD", "testpass"),
		DB:       testEnvOr(t, "MYSQL_TEST_DB", "testdb"),
	}
	cfg.DSN = buildDSN(cfg)
	if cfg.DSN == "" {
		t.Fatalf("buildDSN returned empty")
	}

	db := openTestDB(t, cfg.DSN)
	defer func() { _ = db.Close() }()

	if err := pingWithRetry(context.Background(), db, 30, 1*time.Second); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestDetectRDS(t *testing.T) {
	dsn := buildDSN(MySQLConfig{
		Host:     testEnvOr(t, "MYSQL_TEST_HOST", "127.0.0.1"),
		Port:     testEnvIntOr(t, "MYSQL_TEST_PORT", 3307),
		User:     testEnvOr(t, "MYSQL_TEST_USER", "root"),
		Password: testEnvOr(t, "MYSQL_TEST_PASSWORD", "testpass"),
		DB:       testEnvOr(t, "MYSQL_TEST_DB", "testdb"),
	})

	db := openTestDB(t, dsn)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := pingWithRetry(ctx, db, 30, 1*time.Second); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	isRDS, err := detectRDS(ctx, db)
	if err != nil {
		t.Fatalf("detectRDS error: %v", err)
	}
	if isRDS {
		t.Fatalf("expected non-RDS by default")
	}

	cleanupRoutines(t, db)
	createRoutines(t, db)
	defer cleanupRoutines(t, db)

	isRDS, err = detectRDS(ctx, db)
	if err != nil {
		t.Fatalf("detectRDS error after routines: %v", err)
	}
	if !isRDS {
		t.Fatalf("expected RDS detection to be true after creating routines")
	}
}

func openTestDB(t *testing.T, dsn string) *sql.DB {
	if dsn == "" {
		t.Fatalf("dsn is empty")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func pingWithRetry(ctx context.Context, db *sql.DB, retries int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < retries; i++ {
		if err := db.PingContext(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(delay)
	}
	return lastErr
}

func createRoutines(t *testing.T, db *sql.DB) {
	_, err := db.Exec(`CREATE PROCEDURE mysql.rds_kill(IN pid BIGINT)
BEGIN
  SELECT pid;
END`)
	if err != nil {
		t.Fatalf("create rds_kill: %v", err)
	}
	_, err = db.Exec(`CREATE PROCEDURE mysql.rds_kill_query(IN pid BIGINT)
BEGIN
  SELECT pid;
END`)
	if err != nil {
		t.Fatalf("create rds_kill_query: %v", err)
	}
}

func cleanupRoutines(t *testing.T, db *sql.DB) {
	_, _ = db.Exec("DROP PROCEDURE IF EXISTS mysql.rds_kill")
	_, _ = db.Exec("DROP PROCEDURE IF EXISTS mysql.rds_kill_query")
}

func testEnvOr(t *testing.T, key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func testEnvIntOr(t *testing.T, key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		t.Fatalf("invalid %s: %s", key, v)
	}
	return fallback
}
