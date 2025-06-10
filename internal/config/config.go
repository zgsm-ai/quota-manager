package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Database  DatabaseConfig  `mapstructure:"database"`
	AiGateway AiGatewayConfig `mapstructure:"aigateway"`
	Server    ServerConfig    `mapstructure:"server"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Voucher   VoucherConfig   `mapstructure:"voucher"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type AiGatewayConfig struct {
	BaseURL    string `mapstructure:"base_url"`
	AdminPath  string `mapstructure:"admin_path"`
	AuthHeader string `mapstructure:"auth_header"`
	AuthValue  string `mapstructure:"auth_value"`
}

type ServerConfig struct {
	Port        int    `mapstructure:"port"`
	Mode        string `mapstructure:"mode"`
	TokenHeader string `mapstructure:"token_header"`
}

type SchedulerConfig struct {
	ScanInterval string `mapstructure:"scan_interval"`
}

type VoucherConfig struct {
	SigningKey string `mapstructure:"signing_key"`
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

func (a *AiGatewayConfig) GetBaseURL() string {
	if a.BaseURL != "" {
		return a.BaseURL
	}
	return "http://localhost:8002"
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}
