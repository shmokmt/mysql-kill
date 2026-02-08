package mysqlkill

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Environment variable names for MySQL and SSH configuration.
const (
	envDSN      = "MYSQL_DSN"
	envHost     = "MYSQL_HOST"
	envPort     = "MYSQL_PORT"
	envUser     = "MYSQL_USER"
	envPassword = "MYSQL_PASSWORD"
	envDB       = "MYSQL_DB"
	envSocket   = "MYSQL_SOCKET"
	envTLS      = "MYSQL_TLS"

	envSSHHost            = "SSH_HOST"
	envSSHPort            = "SSH_PORT"
	envSSHUser            = "SSH_USER"
	envSSHKey             = "SSH_KEY"
	envSSHKnownHosts      = "SSH_KNOWN_HOSTS"
	envSSHNoStrictHostKey = "SSH_NO_STRICT_HOST_KEY"
)

// MySQLConfig holds MySQL connection settings.
type MySQLConfig struct {
	DSN      string
	Host     string
	Port     int
	User     string
	Password string
	DB       string
	Socket   string
	TLS      string
}

// SSHConfig holds SSH tunneling settings.
type SSHConfig struct {
	Host            string
	Port            int
	User            string
	KeyPath         string
	KnownHostsPath  string
	NoStrictHostKey bool
	Timeout         time.Duration
}

// AppConfig holds the resolved settings for the application.
type AppConfig struct {
	MySQL       MySQLConfig
	SSH         SSHConfig
	AllowWriter bool
}

// resolveConfig merges CLI flags with environment variables.
func resolveConfig(cli *CLI) AppConfig {
	return AppConfig{
		MySQL:       resolveMySQLConfig(cli),
		SSH:         resolveSSHConfig(cli),
		AllowWriter: cli.AllowWriter,
	}
}

// resolveMySQLConfig merges CLI flags with environment variables for MySQL.
func resolveMySQLConfig(cli *CLI) MySQLConfig {
	cfg := MySQLConfig{
		DSN:      envOr(envDSN, cli.DSN),
		Host:     envOr(envHost, cli.Host),
		User:     envOr(envUser, cli.User),
		Password: envOr(envPassword, cli.Password),
		DB:       envOr(envDB, cli.DB),
		Socket:   envOr(envSocket, cli.Socket),
		TLS:      envOr(envTLS, cli.TLS),
	}

	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}

	portStr := os.Getenv(envPort)
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = p
		}
	} else if cli.Port != 0 {
		cfg.Port = cli.Port
	} else {
		cfg.Port = 3306
	}

	if cfg.User == "" {
		cfg.User = "root"
	}

	return cfg
}

// Enabled reports whether SSH tunneling is requested.
func (c SSHConfig) Enabled() bool {
	return c.Host != ""
}

// resolveSSHConfig merges CLI flags with environment variables for SSH.
func resolveSSHConfig(cli *CLI) SSHConfig {
	cfg := SSHConfig{
		Host:            envOr(envSSHHost, cli.SSHHost),
		User:            envOr(envSSHUser, cli.SSHUser),
		KeyPath:         envOr(envSSHKey, cli.SSHKey),
		KnownHostsPath:  envOr(envSSHKnownHosts, cli.SSHKnownHosts),
		NoStrictHostKey: envBoolOr(envSSHNoStrictHostKey, cli.SSHNoStrictHostKey),
		Timeout:         10 * time.Second,
	}

	if cfg.User == "" {
		cfg.User = firstNonEmpty(os.Getenv("USER"), os.Getenv("USERNAME"))
	}

	portStr := os.Getenv(envSSHPort)
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = p
		}
	} else if cli.SSHPort != 0 {
		cfg.Port = cli.SSHPort
	} else {
		cfg.Port = 22
	}

	if cfg.KnownHostsPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cfg.KnownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
		}
	}

	return cfg
}

// envOr returns the environment value for key or fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envBoolOr parses a boolean env var or returns fallback.
func envBoolOr(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return fallback
}

// firstNonEmpty returns the first non-empty string in values.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// buildDSN builds a MySQL DSN from MySQLConfig.
func buildDSN(cfg MySQLConfig) string {
	if cfg.User == "" {
		return ""
	}

	dbcfg := mysql.NewConfig()
	dbcfg.User = cfg.User
	dbcfg.Passwd = cfg.Password
	dbcfg.DBName = cfg.DB

	if cfg.Socket != "" {
		dbcfg.Net = "unix"
		dbcfg.Addr = cfg.Socket
	} else {
		dbcfg.Net = "tcp"
		dbcfg.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	}

	if dbcfg.Params == nil {
		dbcfg.Params = make(map[string]string)
	}
	dbcfg.Params["parseTime"] = "true"
	if cfg.TLS != "" {
		dbcfg.TLSConfig = cfg.TLS
	}

	return dbcfg.FormatDSN()
}
