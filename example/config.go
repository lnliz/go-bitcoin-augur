package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type BitcoinRpcConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type PersistenceConfig struct {
	DataDirectory string `yaml:"dataDirectory"`
}

type AppConfig struct {
	Server      ServerConfig      `yaml:"server"`
	BitcoinRpc  BitcoinRpcConfig  `yaml:"bitcoinRpc"`
	Persistence PersistenceConfig `yaml:"persistence"`
	BaseURL     string            `yaml:"baseUrl"`
	MetricsAddr string            `yaml:"metricsAddr"`
}

func defaultConfig() AppConfig {
	return AppConfig{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		BitcoinRpc: BitcoinRpcConfig{
			URL:      "http://localhost:8332",
			Username: "",
			Password: "",
		},
		Persistence: PersistenceConfig{
			DataDirectory: "mempool_data",
		},
		MetricsAddr: "127.0.0.1:9876",
	}
}

// loadConfig applies defaults, YAML, then environment overrides. Explicitly
// supplied invalid configuration is an error instead of a silent fallback.
func loadConfig() (AppConfig, error) {
	config := defaultConfig()
	if path := os.Getenv("AUGUR_CONFIG_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return config, fmt.Errorf("read config: %w", err)
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&config); err != nil {
			return config, fmt.Errorf("parse config: %w", err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return config, fmt.Errorf("config must contain a single YAML document")
		}
	}
	for key, field := range map[string]*string{
		"AUGUR_SERVER_HOST":    &config.Server.Host,
		"BITCOIN_RPC_URL":      &config.BitcoinRpc.URL,
		"BITCOIN_RPC_USERNAME": &config.BitcoinRpc.Username,
		"BITCOIN_RPC_PASSWORD": &config.BitcoinRpc.Password,
		"AUGUR_DATA_DIR":       &config.Persistence.DataDirectory,
		"AUGUR_BASE_URL":       &config.BaseURL,
		"METRICS_ADDR":         &config.MetricsAddr,
	} {
		if value, ok := os.LookupEnv(key); ok {
			*field = value
		}
	}
	if value, ok := os.LookupEnv("AUGUR_SERVER_PORT"); ok {
		port, err := strconv.Atoi(value)
		if err != nil {
			return config, fmt.Errorf("invalid AUGUR_SERVER_PORT: %w", err)
		}
		config.Server.Port = port
	}
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return config, fmt.Errorf("server port must be between 1 and 65535")
	}
	rpcURL, err := url.Parse(config.BitcoinRpc.URL)
	if err != nil || rpcURL.Hostname() == "" || (rpcURL.Scheme != "http" && rpcURL.Scheme != "https") || rpcURL.Fragment != "" {
		return config, fmt.Errorf("bitcoinRpc.url must be an HTTP or HTTPS URL")
	}
	if strings.TrimSpace(config.Persistence.DataDirectory) == "" {
		return config, fmt.Errorf("persistence.dataDirectory must not be empty")
	}
	_, port, err := net.SplitHostPort(config.MetricsAddr)
	if err != nil {
		return config, fmt.Errorf("invalid metricsAddr: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return config, fmt.Errorf("metricsAddr port must be between 1 and 65535")
	}
	base, err := url.Parse(config.BaseURL)
	if err != nil || (base.Scheme != "" && base.Scheme != "http" && base.Scheme != "https") || base.RawQuery != "" || base.Fragment != "" || base.User != nil || (base.Scheme != "" && base.Host == "") || (base.Scheme == "" && config.BaseURL != "" && !strings.HasPrefix(config.BaseURL, "/")) {
		return config, fmt.Errorf("baseUrl must be an HTTP(S) URL or an absolute path without query or fragment")
	}
	config.BaseURL = strings.TrimRight(config.BaseURL, "/")
	return config, nil
}
