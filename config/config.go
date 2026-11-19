// Package config 从 YAML 文件加载运行参数（监听、连接、超时、采集器开关）。
// 指标定义不在此配置中，仍固化在 collector 代码内。
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config exporter 运行配置。
type Config struct {
	Web      WebConfig      `yaml:"web"`
	Database DatabaseConfig `yaml:"database"`
	// Collectors 采集器开关：key 为 Scraper.Name()，value 为是否启用。
	// 留空表示全部启用。
	Collectors map[string]bool `yaml:"collectors"`
}

// WebConfig HTTP 服务参数。
type WebConfig struct {
	ListenAddress string `yaml:"listen_address"`
	TelemetryPath string `yaml:"telemetry_path"`
}

// DatabaseConfig OMS Repository 连接参数。
type DatabaseConfig struct {
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
	QueryTimeout string `yaml:"query_timeout"`
}

// Load 读取指定路径的 YAML 配置并填充默认值。
// 环境变量 OMS_DSN 存在时覆盖文件中的 database.dsn，避免明文落在配置文件里。
func Load(path string) (*Config, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(buf, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.applyDefaults()

	if dsn := os.Getenv("OMS_DSN"); dsn != "" {
		cfg.Database.DSN = dsn
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Web.ListenAddress == "" {
		c.Web.ListenAddress = ":9161"
	}
	if c.Web.TelemetryPath == "" {
		c.Web.TelemetryPath = "/metrics"
	}
	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 5
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 2
	}
	if c.Database.QueryTimeout == "" {
		c.Database.QueryTimeout = "30s"
	}
}

// QueryTimeoutDuration 解析查询超时。
func (c *Config) QueryTimeoutDuration() (time.Duration, error) {
	return time.ParseDuration(c.Database.QueryTimeout)
}

// Enabled 判断指定采集器是否启用。Collectors 未配置时默认全部启用。
func (c *Config) Enabled(name string) bool {
	if len(c.Collectors) == 0 {
		return true
	}
	return c.Collectors[name]
}
