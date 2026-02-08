package mysqlkill

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// Execer executes SQL statements.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// openDBWithTunnel opens a DB connection, optionally via SSH tunnel.
func openDBWithTunnel(ctx context.Context, mysqlCfg MySQLConfig, sshCfg SSHConfig) (*sql.DB, *sshTunnel, error) {
	db, err := sql.Open("mysql", mysqlCfg.DSN)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}

	var tunnel *sshTunnel
	if sshCfg.Enabled() {
		targetHost := mysqlCfg.Host
		targetPort := mysqlCfg.Port
		parsed, err := parseDSN(mysqlCfg.DSN)
		if err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		if parsed != nil {
			if strings.EqualFold(parsed.Net, "unix") {
				_ = db.Close()
				return nil, nil, fmt.Errorf("ssh tunnel cannot be used with unix socket DSN")
			}
			if parsed.Addr != "" {
				host, portStr, err := net.SplitHostPort(parsed.Addr)
				if err != nil {
					_ = db.Close()
					return nil, nil, fmt.Errorf("parse dsn addr: %w", err)
				}
				port, err := strconv.Atoi(portStr)
				if err != nil {
					_ = db.Close()
					return nil, nil, fmt.Errorf("parse dsn port: %w", err)
				}
				targetHost = host
				targetPort = port
			}
		}

		tunnel, err = startSSHTunnel(ctx, sshCfg, targetHost, targetPort)
		if err != nil {
			_ = db.Close()
			return nil, nil, err
		}

		localAddr := net.JoinHostPort(tunnel.LocalHost, strconv.Itoa(tunnel.LocalPort))
		if parsed != nil {
			parsed.Net = "tcp"
			parsed.Addr = localAddr
			mysqlCfg.DSN = parsed.FormatDSN()
		} else {
			mysqlCfg.Host = tunnel.LocalHost
			mysqlCfg.Port = tunnel.LocalPort
			mysqlCfg.DSN = buildDSN(mysqlCfg)
		}

		if err := db.Close(); err != nil {
			tunnel.Close()
			return nil, nil, fmt.Errorf("close db: %w", err)
		}
		db, err = sql.Open("mysql", mysqlCfg.DSN)
		if err != nil {
			tunnel.Close()
			return nil, nil, fmt.Errorf("open db: %w", err)
		}
	}

	if err := ping(ctx, db); err != nil {
		if tunnel != nil {
			tunnel.Close()
		}
		_ = db.Close()
		return nil, nil, err
	}

	return db, tunnel, nil
}

// parseDSN parses a DSN into mysql.Config.
func parseDSN(dsn string) (*mysql.Config, error) {
	if dsn == "" {
		return nil, nil
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	return cfg, nil
}

// ping checks database connectivity.
func ping(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}

// detectRDS detects Amazon RDS/Aurora based on server metadata.
func detectRDS(ctx context.Context, db *sql.DB) (bool, error) {
	var versionComment string
	if err := db.QueryRowContext(ctx, "SELECT @@version_comment").Scan(&versionComment); err != nil {
		return false, fmt.Errorf("detect rds (version_comment): %w", err)
	}

	lower := strings.ToLower(versionComment)
	if strings.Contains(lower, "amazon rds") || strings.Contains(lower, "aurora") {
		return true, nil
	}

	var count int
	const routinesQuery = `
SELECT COUNT(*)
FROM information_schema.routines
WHERE routine_schema = 'mysql'
  AND routine_name IN ('rds_kill', 'rds_kill_query')`
	if err := db.QueryRowContext(ctx, routinesQuery).Scan(&count); err != nil {
		return false, fmt.Errorf("detect rds (routines): %w", err)
	}

	return count > 0, nil
}

// enforceReader rejects writer connections unless allowWriter is true.
func enforceReader(ctx context.Context, db *sql.DB, allowWriter bool) error {
	isReader, err := detectReader(ctx, db)
	if err != nil {
		return err
	}
	if !isReader && !allowWriter {
		return fmt.Errorf("writer detected: use --allow-writer to proceed")
	}
	return nil
}

// detectReader determines whether the instance is read-only.
func detectReader(ctx context.Context, db *sql.DB) (bool, error) {
	var innodbReadOnly sql.NullInt64
	var readOnly sql.NullInt64

	if err := db.QueryRowContext(ctx, "SELECT @@innodb_read_only, @@read_only").Scan(&innodbReadOnly, &readOnly); err != nil {
		return false, fmt.Errorf("detect reader: %w", err)
	}

	return isReaderFromValues(innodbReadOnly, readOnly), nil
}

// isReaderFromValues interprets read-only variables as reader or writer.
func isReaderFromValues(innodbReadOnly sql.NullInt64, readOnly sql.NullInt64) bool {
	if innodbReadOnly.Valid && innodbReadOnly.Int64 == 1 {
		return true
	}
	if readOnly.Valid && readOnly.Int64 == 1 {
		return true
	}
	return false
}
