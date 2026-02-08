package mysqlkill

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
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
func resolveConfig(cli *CLI) (AppConfig, error) {
	cfg := AppConfig{
		MySQL:       resolveMySQLConfig(cli),
		SSH:         resolveSSHConfig(cli),
		AllowWriter: cli.AllowWriter,
	}

	fileCfg, err := loadConfigFile()
	if err != nil {
		return cfg, err
	}
	if fileCfg != nil {
		applyFileConfig(&cfg, fileCfg)
	}

	cfg.SSH.KeyPath = expandTilde(cfg.SSH.KeyPath)
	cfg.SSH.KnownHostsPath = expandTilde(cfg.SSH.KnownHostsPath)

	return cfg, nil
}

// resolveMySQLConfig merges CLI flags with environment variables for MySQL.
func resolveMySQLConfig(cli *CLI) MySQLConfig {
	cfg := MySQLConfig{
		DSN:      cli.DSN,
		Host:     cli.Host,
		Port:     cli.Port,
		User:     cli.User,
		Password: cli.Password,
		DB:       cli.DB,
		Socket:   cli.Socket,
		TLS:      cli.TLS,
	}

	if v := os.Getenv(envDSN); v != "" {
		cfg.DSN = v
	}
	if v := os.Getenv(envHost); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv(envUser); v != "" {
		cfg.User = v
	}
	if v := os.Getenv(envPassword); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv(envDB); v != "" {
		cfg.DB = v
	}
	if v := os.Getenv(envSocket); v != "" {
		cfg.Socket = v
	}
	if v := os.Getenv(envTLS); v != "" {
		cfg.TLS = v
	}

	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}

	portStr := os.Getenv(envPort)
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = p
		}
	}
	if cfg.Port == 0 {
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
		Host:            cli.SSHHost,
		Port:            cli.SSHPort,
		User:            cli.SSHUser,
		KeyPath:         cli.SSHKey,
		KnownHostsPath:  cli.SSHKnownHosts,
		NoStrictHostKey: cli.SSHNoStrictHostKey,
		Timeout:         10 * time.Second,
	}

	if v := os.Getenv(envSSHHost); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv(envSSHUser); v != "" {
		cfg.User = v
	}
	if v := os.Getenv(envSSHKey); v != "" {
		cfg.KeyPath = v
	}
	if v := os.Getenv(envSSHKnownHosts); v != "" {
		cfg.KnownHostsPath = v
	}
	cfg.NoStrictHostKey = envBoolOr(envSSHNoStrictHostKey, cfg.NoStrictHostKey)

	if cfg.User == "" {
		cfg.User = firstNonEmpty(os.Getenv("USER"), os.Getenv("USERNAME"))
	}

	portStr := os.Getenv(envSSHPort)
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = p
		}
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}

	if cfg.KnownHostsPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cfg.KnownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
		}
	}

	return cfg
}

// fileConfig represents settings loaded from config.toml.
type fileConfig struct {
	MySQL       fileMySQLConfig     `toml:"mysql"`
	SSH         fileSSHConfig       `toml:"ssh"`
	MySQLKill   fileMySQLKillConfig `toml:"mysql-kill"`
	AllowWriter *bool               `toml:"allow_writer"`
}

type fileMySQLConfig struct {
	DSN      *string     `toml:"dsn"`
	Host     *string     `toml:"host"`
	Port     any `toml:"port"`
	User     *string     `toml:"user"`
	Password *string     `toml:"password"`
	DB       *string     `toml:"db"`
	Socket   *string     `toml:"socket"`
	TLS      *string     `toml:"tls"`
}

type fileSSHConfig struct {
	Host            *string     `toml:"host"`
	Port            any `toml:"port"`
	User            *string     `toml:"user"`
	KeyPath         *string     `toml:"key"`
	KnownHostsPath  *string     `toml:"known_hosts"`
	NoStrictHostKey *bool       `toml:"no_strict_host_key"`
}

type fileMySQLKillConfig struct {
	AllowWriter *bool `toml:"allow_writer"`
}

// loadConfigFile loads config.toml from the first found location:
// 1. $XDG_CONFIG_HOME/mysql-kill/config.toml
// 2. os.UserConfigDir()/mysql-kill/config.toml (e.g. ~/Library/Application Support on macOS)
// 3. ~/.config/mysql-kill/config.toml (fallback)
func loadConfigFile() (*fileConfig, error) {
	var candidates []string

	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "mysql-kill", "config.toml"))
	}
	if dir, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(dir, "mysql-kill", "config.toml"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "mysql-kill", "config.toml"))
	}

	var path string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			path = c
			break
		}
	}
	if path == "" {
		return nil, nil
	}

	var cfg fileConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// applyFileConfig overrides config with values from file.
func applyFileConfig(cfg *AppConfig, fileCfg *fileConfig) {
	if fileCfg.AllowWriter != nil {
		cfg.AllowWriter = *fileCfg.AllowWriter
	}
	if fileCfg.MySQLKill.AllowWriter != nil {
		cfg.AllowWriter = *fileCfg.MySQLKill.AllowWriter
	}

	applyFileMySQLConfig(&cfg.MySQL, fileCfg.MySQL)
	applyFileSSHConfig(&cfg.SSH, fileCfg.SSH)
}

func applyFileMySQLConfig(cfg *MySQLConfig, fileCfg fileMySQLConfig) {
	if fileCfg.DSN != nil {
		cfg.DSN = *fileCfg.DSN
	}
	if fileCfg.Host != nil {
		cfg.Host = *fileCfg.Host
	}
	if p, ok := toInt(fileCfg.Port); ok {
		cfg.Port = p
	}
	if fileCfg.User != nil {
		cfg.User = *fileCfg.User
	}
	if fileCfg.Password != nil {
		cfg.Password = *fileCfg.Password
	}
	if fileCfg.DB != nil {
		cfg.DB = *fileCfg.DB
	}
	if fileCfg.Socket != nil {
		cfg.Socket = *fileCfg.Socket
	}
	if fileCfg.TLS != nil {
		cfg.TLS = *fileCfg.TLS
	}
}

func applyFileSSHConfig(cfg *SSHConfig, fileCfg fileSSHConfig) {
	if fileCfg.Host != nil {
		cfg.Host = *fileCfg.Host
	}
	if p, ok := toInt(fileCfg.Port); ok {
		cfg.Port = p
	}
	if fileCfg.User != nil {
		cfg.User = *fileCfg.User
	}
	if fileCfg.KeyPath != nil {
		cfg.KeyPath = *fileCfg.KeyPath
	}
	if fileCfg.KnownHostsPath != nil {
		cfg.KnownHostsPath = *fileCfg.KnownHostsPath
	}
	if fileCfg.NoStrictHostKey != nil {
		cfg.NoStrictHostKey = *fileCfg.NoStrictHostKey
	}
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

// toInt converts an any (int64 or string) to int.
func toInt(v any) (int, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case int64:
		return int(val), true
	case string:
		if p, err := strconv.Atoi(val); err == nil {
			return p, true
		}
	}
	return 0, false
}

// expandTilde replaces a leading "~/" or "~" with the user's home directory.
func expandTilde(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
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
