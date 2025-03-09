package config

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Config содержит настройки приложения
type Config struct {
	LogLevel string         `yaml:"logLevel"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	VPN      VPNConfig      `yaml:"vpn"`
	Telegram TelegramConfig `yaml:"telegram"`
	API      APIConfig      `yaml:"api"`
}

// APIConfig содержит настройки API сервера
type APIConfig struct {
	ListenAddr      string `yaml:"listenAddr"`
	ReadTimeout     int    `yaml:"readTimeout"`
	WriteTimeout    int    `yaml:"writeTimeout"`
	ShutdownTimeout int    `yaml:"shutdownTimeout"`
}

// DatabaseConfig содержит настройки базы данных
type DatabaseConfig struct {
	ConnectionString string `yaml:"connectionString"`
}

// JWTConfig содержит настройки JWT
type JWTConfig struct {
	Secret        string `yaml:"secret"`
	ExpiryMinutes int    `yaml:"expiryMinutes"`
}

// VPNConfig содержит настройки VPN
type VPNConfig struct {
	ListenIP         string   `yaml:"listenIP"`
	ListenPort       int      `yaml:"listenPort"`
	CertDirectory    string   `yaml:"certDirectory"`
	CACommonName     string   `yaml:"caCommonName"`
	ServerCommonName string   `yaml:"serverCommonName"`
	Organization     string   `yaml:"organization"`
	Country          string   `yaml:"country"`
	DefaultRoutes    []string `yaml:"defaultRoutes"`
	DefaultASNRoutes []int    `yaml:"defaultASNRoutes"`
}

// TelegramConfig содержит настройки Telegram бота
type TelegramConfig struct {
	Token    string  `yaml:"token"`
	AdminIDs []int64 `yaml:"adminIDs"`
}

// LoadConfig загружает конфигурацию из файла
func LoadConfig(path string) (*Config, error) {
	// Читаем файл
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Парсим YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}
