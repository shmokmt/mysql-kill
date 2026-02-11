package mysqlkill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-sql-driver/mysql"
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
	KnownHostsPath string
	NoStrictHostKey bool
	Timeout         time.Duration
}

// AppConfig holds the resolved settings for the application.
type AppConfig struct {
	MySQL       MySQLConfig
	SSH         SSHConfig
	AllowWriter bool
}

// resolveConfig builds the application config from TOML file and CLI flags.
// Precedence: CLI flags > config file > defaults.
func resolveConfig(ctx context.Context, cli *CLI) (AppConfig, error) {
	cfg := AppConfig{
		MySQL: MySQLConfig{
			Host: "127.0.0.1",
			Port: 3306,
			User: "root",
		},
		SSH: SSHConfig{
			Port:    22,
			Timeout: 10 * time.Second,
		},
	}

	// Set default SSH user from OS.
	cfg.SSH.User = firstNonEmpty(os.Getenv("USER"), os.Getenv("USERNAME"))

	// Set default known_hosts path.
	if home, err := os.UserHomeDir(); err == nil {
		cfg.SSH.KnownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
	}

	// Load config file (overrides defaults).
	fileCfg, err := loadConfigFile(cli.Config)
	if err != nil {
		return cfg, err
	}
	if fileCfg != nil {
		applyFileConfig(&cfg, fileCfg)
	}

	// CLI flags override config file.
	if cli.DSN != "" {
		cfg.MySQL.DSN = cli.DSN
	}
	if cli.AllowWriter {
		cfg.AllowWriter = true
	}

	cfg.SSH.KeyPath = expandTilde(cfg.SSH.KeyPath)
	cfg.SSH.KnownHostsPath = expandTilde(cfg.SSH.KnownHostsPath)

	resolved, err := resolvePassword(ctx, cfg.MySQL.Password)
	if err != nil {
		return cfg, fmt.Errorf("resolve password: %w", err)
	}
	cfg.MySQL.Password = resolved

	return cfg, nil
}

// Enabled reports whether SSH tunneling is requested.
func (c SSHConfig) Enabled() bool {
	return c.Host != ""
}

// fileConfig represents settings loaded from config.toml.
type fileConfig struct {
	MySQL     fileMySQLConfig     `toml:"mysql"`
	SSH       fileSSHConfig       `toml:"ssh"`
	MySQLKill fileMySQLKillConfig `toml:"mysql-kill"`
}

type fileMySQLConfig struct {
	DSN      *string `toml:"dsn"`
	Host     *string `toml:"host"`
	Port     any     `toml:"port"`
	User     *string `toml:"user"`
	Password *string `toml:"password"`
	DB       *string `toml:"db"`
	Socket   *string `toml:"socket"`
	TLS      *string `toml:"tls"`
}

type fileSSHConfig struct {
	Host            *string `toml:"host"`
	Port            any     `toml:"port"`
	User            *string `toml:"user"`
	KeyPath         *string `toml:"key"`
	KnownHostsPath  *string `toml:"known_hosts"`
	NoStrictHostKey *bool   `toml:"no_strict_host_key"`
}

type fileMySQLKillConfig struct {
	AllowWriter *bool `toml:"allow_writer"`
}

// loadConfigFile loads config.toml from the specified path, or from the
// first found default location:
// 1. $XDG_CONFIG_HOME/mysql-kill/config.toml
// 2. os.UserConfigDir()/mysql-kill/config.toml
// 3. ~/.config/mysql-kill/config.toml
func loadConfigFile(configPath string) (*fileConfig, error) {
	if configPath != "" {
		var cfg fileConfig
		if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
			return nil, fmt.Errorf("load config file %s: %w", configPath, err)
		}
		return &cfg, nil
	}

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
