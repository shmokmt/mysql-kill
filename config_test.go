package mysqlkill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDSN(t *testing.T) {
	cfg := MySQLConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "pass",
		DB:       "testdb",
	}
	got := buildDSN(cfg)
	want := "root:pass@tcp(127.0.0.1:3306)/testdb?parseTime=true"
	if got != want {
		t.Fatalf("buildDSN mismatch: got %q want %q", got, want)
	}
}

func TestBuildDSNWithSocketAndTLS(t *testing.T) {
	cfg := MySQLConfig{
		User:   "root",
		Socket: "/tmp/mysql.sock",
		TLS:    "custom",
	}
	got := buildDSN(cfg)
	if !strings.Contains(got, "unix(/tmp/mysql.sock)") {
		t.Fatalf("expected unix socket in dsn: %q", got)
	}
	if !strings.Contains(got, "parseTime=true") {
		t.Fatalf("expected parseTime=true in dsn: %q", got)
	}
	if !strings.Contains(got, "tls=custom") {
		t.Fatalf("expected tls=custom in dsn: %q", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	got := firstNonEmpty("", "a", "b")
	if got != "a" {
		t.Fatalf("expected a, got %q", got)
	}
	got = firstNonEmpty("", "")
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestResolveConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfgDir := filepath.Join(dir, "mysql-kill")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	configPath := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[mysql-kill]
allow_writer = true

[mysql]
host = "file-host"
port = 3307
user = "file-user"
password = "file-pass"
db = "file-db"

[ssh]
host = "file-bastion"
port = 2222
user = "file-ssh-user"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	appCfg, err := resolveConfig(context.Background(), &CLI{})
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}

	if appCfg.MySQL.Host != "file-host" {
		t.Fatalf("mysql host: got %q, want %q", appCfg.MySQL.Host, "file-host")
	}
	if appCfg.MySQL.Port != 3307 {
		t.Fatalf("mysql port: got %d, want %d", appCfg.MySQL.Port, 3307)
	}
	if appCfg.MySQL.User != "file-user" {
		t.Fatalf("mysql user: got %q, want %q", appCfg.MySQL.User, "file-user")
	}
	if appCfg.MySQL.Password != "file-pass" {
		t.Fatalf("mysql password: got %q, want %q", appCfg.MySQL.Password, "file-pass")
	}
	if appCfg.MySQL.DB != "file-db" {
		t.Fatalf("mysql db: got %q, want %q", appCfg.MySQL.DB, "file-db")
	}
	if appCfg.SSH.Host != "file-bastion" {
		t.Fatalf("ssh host: got %q, want %q", appCfg.SSH.Host, "file-bastion")
	}
	if appCfg.SSH.Port != 2222 {
		t.Fatalf("ssh port: got %d, want %d", appCfg.SSH.Port, 2222)
	}
	if appCfg.SSH.User != "file-ssh-user" {
		t.Fatalf("ssh user: got %q, want %q", appCfg.SSH.User, "file-ssh-user")
	}
	if !appCfg.AllowWriter {
		t.Fatalf("expected allow_writer=true from config file")
	}
}

func TestResolveConfigDSNFlagOverridesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfgDir := filepath.Join(dir, "mysql-kill")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	configPath := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`
[mysql]
dsn = "file-user:file-pass@tcp(file-host:3306)/filedb"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	appCfg, err := resolveConfig(context.Background(), &CLI{
		DSN: "flag-user:flag-pass@tcp(flag-host:3306)/flagdb",
	})
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}

	if appCfg.MySQL.DSN != "flag-user:flag-pass@tcp(flag-host:3306)/flagdb" {
		t.Fatalf("--dsn flag should override file dsn: got %q", appCfg.MySQL.DSN)
	}
}

func TestResolveConfigCustomPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "custom.toml")
	if err := os.WriteFile(configPath, []byte(`
[mysql]
host = "custom-host"
user = "custom-user"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	appCfg, err := resolveConfig(context.Background(), &CLI{
		Config: configPath,
	})
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}

	if appCfg.MySQL.Host != "custom-host" {
		t.Fatalf("mysql host: got %q, want %q", appCfg.MySQL.Host, "custom-host")
	}
	if appCfg.MySQL.User != "custom-user" {
		t.Fatalf("mysql user: got %q, want %q", appCfg.MySQL.User, "custom-user")
	}
}

func TestResolveConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	appCfg, err := resolveConfig(context.Background(), &CLI{})
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}

	if appCfg.MySQL.Host != "127.0.0.1" {
		t.Fatalf("default host: got %q, want %q", appCfg.MySQL.Host, "127.0.0.1")
	}
	if appCfg.MySQL.Port != 3306 {
		t.Fatalf("default port: got %d, want %d", appCfg.MySQL.Port, 3306)
	}
	if appCfg.MySQL.User != "root" {
		t.Fatalf("default user: got %q, want %q", appCfg.MySQL.User, "root")
	}
	if appCfg.SSH.Port != 22 {
		t.Fatalf("default ssh port: got %d, want %d", appCfg.SSH.Port, 22)
	}
	if appCfg.AllowWriter {
		t.Fatalf("default allow_writer should be false")
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := loadConfigFile("/nonexistent/path/config.toml")
	if err == nil {
		t.Fatalf("expected error for nonexistent config file")
	}
}
