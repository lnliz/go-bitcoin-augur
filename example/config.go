package main

import (
	"log"
	"os"
	"strconv"

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

func loadConfig() AppConfig {
	config := loadConfigFromSources()

	if env := os.Getenv("AUGUR_SERVER_HOST"); env != "" {
		config.Server.Host = env
	}
	if env := os.Getenv("AUGUR_SERVER_PORT"); env != "" {
		if port, err := strconv.Atoi(env); err == nil {
			config.Server.Port = port
		}
	}
	if env := os.Getenv("BITCOIN_RPC_URL"); env != "" {
		config.BitcoinRpc.URL = env
	}
	if env := os.Getenv("BITCOIN_RPC_USERNAME"); env != "" {
		config.BitcoinRpc.Username = env
	}
	if env := os.Getenv("BITCOIN_RPC_PASSWORD"); env != "" {
		config.BitcoinRpc.Password = env
	}
	if env := os.Getenv("AUGUR_DATA_DIR"); env != "" {
		config.Persistence.DataDirectory = env
	}
	if env := os.Getenv("AUGUR_BASE_URL"); env != "" {
		config.BaseURL = env
	}
	if env := os.Getenv("METRICS_ADDR"); env != "" {
		config.MetricsAddr = env
	}

	log.Printf("Loaded configuration: server.port=%d, bitcoinRpc.url=%s, persistence.dataDirectory=%s, metricsAddr=%s",
		config.Server.Port, config.BitcoinRpc.URL, config.Persistence.DataDirectory, config.MetricsAddr)

	return config
}

func loadConfigFromSources() AppConfig {
	if configPath := os.Getenv("AUGUR_CONFIG_FILE"); configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			log.Printf("Failed to read config file: %v", err)
		} else {
			var config AppConfig
			if err := yaml.Unmarshal(data, &config); err != nil {
				log.Printf("Failed to parse config file: %v", err)
			} else {
				return config
			}
		}
	}

	return defaultConfig()
}
