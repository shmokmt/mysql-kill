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

func TestEnvBoolOr(t *testing.T) {
	t.Setenv("TEST_BOOL", "true")
	if !envBoolOr("TEST_BOOL", false) {
		t.Fatalf("expected true")
	}
	t.Setenv("TEST_BOOL", "false")
	if envBoolOr("TEST_BOOL", true) {
		t.Fatalf("expected false")
	}
	if !envBoolOr("MISSING_BOOL", true) {
		t.Fatalf("expected fallback true")
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

func TestResolveSSHConfigEnv(t *testing.T) {
	t.Setenv("SSH_HOST", "bastion.example.com")
	t.Setenv("SSH_PORT", "2222")
	t.Setenv("SSH_USER", "ec2-user")
	t.Setenv("SSH_KEY", "/tmp/testkey")
	t.Setenv("SSH_KNOWN_HOSTS", "/tmp/known_hosts")
	t.Setenv("SSH_NO_STRICT_HOST_KEY", "true")

	cfg := resolveSSHConfig(&CLI{})
	if cfg.Host != "bastion.example.com" {
		t.Fatalf("unexpected host: %s", cfg.Host)
	}
	if cfg.Port != 2222 {
		t.Fatalf("unexpected port: %d", cfg.Port)
	}
	if cfg.User != "ec2-user" {
		t.Fatalf("unexpected user: %s", cfg.User)
	}
	if cfg.KeyPath != "/tmp/testkey" {
		t.Fatalf("unexpected key path: %s", cfg.KeyPath)
	}
	if cfg.KnownHostsPath != "/tmp/known_hosts" {
		t.Fatalf("unexpected known_hosts path: %s", cfg.KnownHostsPath)
	}
	if !cfg.NoStrictHostKey {
		t.Fatalf("expected NoStrictHostKey true")
	}
}

func TestResolveConfigFilePrecedence(t *testing.T) {
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

[ssh]
host = "file-bastion"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("MYSQL_HOST", "env-host")
	t.Setenv("SSH_HOST", "env-bastion")

	appCfg, err := resolveConfig(context.Background(), &CLI{
		Host:    "flag-host",
		SSHHost: "flag-bastion",
	})
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}

	if appCfg.MySQL.Host != "file-host" {
		t.Fatalf("mysql host precedence mismatch: %s", appCfg.MySQL.Host)
	}
	if appCfg.SSH.Host != "file-bastion" {
		t.Fatalf("ssh host precedence mismatch: %s", appCfg.SSH.Host)
	}
	if !appCfg.AllowWriter {
		t.Fatalf("expected allow_writer=true from config file")
	}
}
