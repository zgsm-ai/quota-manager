package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Database        DatabaseConfig        `mapstructure:"database"`
	AuthDatabase    DatabaseConfig        `mapstructure:"auth_database"`
	AiGateway       AiGatewayConfig       `mapstructure:"aigateway"`
	Server          ServerConfig          `mapstructure:"server"`
	Scheduler       SchedulerConfig       `mapstructure:"scheduler"`
	Voucher         VoucherConfig         `mapstructure:"voucher"`
	Log             LogConfig             `mapstructure:"log"`
	EmployeeSync    EmployeeSyncConfig    `mapstructure:"employee_sync"`
	GithubStarCheck GithubStarCheckConfig `mapstructure:"github_star_check"`
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
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	AdminPath  string `mapstructure:"admin_path"`
	AuthHeader string `mapstructure:"auth_header"`
	AuthValue  string `mapstructure:"auth_value"`
}

type ServerConfig struct {
	Port        int    `mapstructure:"port"`
	Mode        string `mapstructure:"mode"`
	TokenHeader string `mapstructure:"token_header"`
	Timezone    string `mapstructure:"timezone"`
}

type SchedulerConfig struct {
	ScanInterval string `mapstructure:"scan_interval"`
}

type VoucherConfig struct {
	SigningKey string `mapstructure:"signing_key"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type EmployeeSyncConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	HrURL   string `mapstructure:"hr_url"`
	HrKey   string `mapstructure:"hr_key"`
	DeptURL string `mapstructure:"dept_url"`
	DeptKey string `mapstructure:"dept_key"`
}

type GithubStarCheckConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	RequiredRepo string `mapstructure:"required_repo"`
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

func (a *AiGatewayConfig) GetBaseURL() string {
	if a.Host != "" && a.Port > 0 {
		return fmt.Sprintf("http://%s:%d", a.Host, a.Port)
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
