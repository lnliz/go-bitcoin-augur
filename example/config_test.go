package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.BitcoinRpc.URL != "http://localhost:8332" {
		t.Errorf("expected URL http://localhost:8332, got %s", cfg.BitcoinRpc.URL)
	}
	if cfg.Persistence.DataDirectory != "mempool_data" {
		t.Errorf("expected data directory mempool_data, got %s", cfg.Persistence.DataDirectory)
	}
}

func TestLoadConfigEnvOverrides(t *testing.T) {
	os.Setenv("AUGUR_SERVER_HOST", "127.0.0.1")
	os.Setenv("AUGUR_SERVER_PORT", "9090")
	os.Setenv("BITCOIN_RPC_URL", "http://testnode:8332")
	os.Setenv("BITCOIN_RPC_USERNAME", "testuser")
	os.Setenv("BITCOIN_RPC_PASSWORD", "testpass")
	os.Setenv("AUGUR_DATA_DIR", "/tmp/test_data")
	defer func() {
		os.Unsetenv("AUGUR_SERVER_HOST")
		os.Unsetenv("AUGUR_SERVER_PORT")
		os.Unsetenv("BITCOIN_RPC_URL")
		os.Unsetenv("BITCOIN_RPC_USERNAME")
		os.Unsetenv("BITCOIN_RPC_PASSWORD")
		os.Unsetenv("AUGUR_DATA_DIR")
	}()

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.BitcoinRpc.URL != "http://testnode:8332" {
		t.Errorf("expected URL http://testnode:8332, got %s", cfg.BitcoinRpc.URL)
	}
	if cfg.BitcoinRpc.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", cfg.BitcoinRpc.Username)
	}
	if cfg.BitcoinRpc.Password != "testpass" {
		t.Errorf("expected password testpass, got %s", cfg.BitcoinRpc.Password)
	}
	if cfg.Persistence.DataDirectory != "/tmp/test_data" {
		t.Errorf("expected data directory /tmp/test_data, got %s", cfg.Persistence.DataDirectory)
	}
}

func TestLoadConfigFromYAMLFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `server:
  host: "192.168.1.1"
  port: 3000
bitcoinRpc:
  url: "http://bitcoin:8332"
  username: "rpcuser"
  password: "rpcpass"
persistence:
  dataDirectory: "/data/mempool"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	os.Setenv("AUGUR_CONFIG_FILE", configPath)
	defer os.Unsetenv("AUGUR_CONFIG_FILE")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server.Host != "192.168.1.1" {
		t.Errorf("expected host 192.168.1.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Server.Port)
	}
	if cfg.BitcoinRpc.URL != "http://bitcoin:8332" {
		t.Errorf("expected URL http://bitcoin:8332, got %s", cfg.BitcoinRpc.URL)
	}
}

func TestPartialConfigKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 9999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUGUR_CONFIG_FILE", path)
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 9999 || cfg.BitcoinRpc.URL != defaultConfig().BitcoinRpc.URL || cfg.MetricsAddr != defaultConfig().MetricsAddr || cfg.Persistence.DataDirectory != defaultConfig().Persistence.DataDirectory {
		t.Fatalf("lost defaults: %+v", cfg)
	}
}

func TestInvalidConfigFails(t *testing.T) {
	for _, content := range []string{"server:\n  prt: 99\n", "server:\n  port: 0\n", "bitcoinRpc:\n  url: file:///tmp/rpc\n", "metricsAddr: invalid\n", "metricsAddr: localhost:99999\n", "persistence:\n  dataDirectory: ''\n", "baseUrl: javascript:alert(1)\n", "server: [", "server: {}\n---\nserver: {}\n"} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("AUGUR_CONFIG_FILE", path)
			if _, err := loadConfig(); err == nil {
				t.Fatal("accepted invalid configuration")
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		t.Setenv("AUGUR_CONFIG_FILE", filepath.Join(t.TempDir(), "missing"))
		if _, err := loadConfig(); err == nil {
			t.Fatal("accepted missing config")
		}
	})
	t.Run("invalid environment port", func(t *testing.T) {
		t.Setenv("AUGUR_SERVER_PORT", "garbage")
		if _, err := loadConfig(); err == nil {
			t.Fatal("accepted invalid port")
		}
	})
}

func TestEnvironmentCanClearYAMLValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("bitcoinRpc:\n  password: secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUGUR_CONFIG_FILE", path)
	t.Setenv("BITCOIN_RPC_PASSWORD", "")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BitcoinRpc.Password != "" {
		t.Fatal("empty override did not clear password")
	}
}
