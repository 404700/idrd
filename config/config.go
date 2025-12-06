package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// AppConfig 应用程序总配置
type AppConfig struct {
	Server             ServerConfig        `yaml:"server" json:"server"`
	IPProviders        []IPProviderConfig  `yaml:"ip_providers" json:"ip_providers"`
	CloudflareAccounts []CloudflareAccount `yaml:"cloudflare_accounts" json:"cloudflare_accounts"`
	Intervals          IntervalsConfig     `yaml:"intervals" json:"intervals"`
	IPv6               IPv6Config          `yaml:"ipv6" json:"ipv6"`
	
	// 兼容旧配置字段（读取时使用，保存时迁移）
	OldIP         *IPConfig         `yaml:"ip,omitempty" json:"-"`
	OldCloudflare *CloudflareConfig `yaml:"cloudflare,omitempty" json:"-"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port           int      `yaml:"port" json:"port"`
	APIKey         string   `yaml:"api_key" json:"api_key"`
	TrustedSubnets []string `yaml:"trusted_subnets" json:"trusted_subnets"`
}

// IPProviderConfig IP 提供者配置
type IPProviderConfig struct {
	Type          string            `yaml:"type" json:"type"` // stun, router_ssh, http, interface
	Enabled       bool              `yaml:"enabled" json:"enabled"`
	Properties    map[string]string `yaml:"properties" json:"properties"` // 存储特定类型的配置
}

// CloudflareAccount Cloudflare 账户配置
type CloudflareAccount struct {
	Name     string `yaml:"name" json:"name"`
	APIToken string `yaml:"api_token" json:"api_token"`
	Zones    []Zone `yaml:"zones" json:"zones"`
}

// Zone 域名区域配置
type Zone struct {
	ZoneName string   `yaml:"zone_name" json:"zone_name"`
	Records  []string `yaml:"records" json:"records"`
}

// IntervalsConfig 时间间隔配置
type IntervalsConfig struct {
	IPCheck          string `yaml:"ip_check" json:"ip_check"`           // IP 检查间隔
	DNSUpdate        string `yaml:"dns_update" json:"dns_update"`       // DNS 同步间隔
	HistoryRetention string `yaml:"history_retention" json:"history_retention"` // 历史保留时间
}

// IPv6Config IPv6 配置
type IPv6Config struct {
	Enabled           bool `yaml:"enabled" json:"enabled"`
	UpdateAAAARecords bool `yaml:"update_aaaa_records" json:"update_aaaa_records"`
}

// 旧配置结构（用于兼容）
type IPConfig struct {
	Provider   string       `yaml:"provider" json:"provider"`
	STUNServer string       `yaml:"stun_server" json:"stun_server"`
	Router     RouterConfig `yaml:"router" json:"router"`
}

type RouterConfig struct {
	Type      string `yaml:"type" json:"type"`
	Host      string `yaml:"host" json:"host"`
	Port      int    `yaml:"port" json:"port"`
	User      string `yaml:"user" json:"user"`
	Password  string `yaml:"password" json:"password"`
	Key       string `yaml:"key" json:"key"`
	KeyPath   string `yaml:"key_path" json:"key_path"`
	Interface string `yaml:"interface" json:"interface"`
}

type CloudflareConfig struct {
	APIToken string   `yaml:"api_token" json:"api_token"`
	ZoneName string   `yaml:"zone_name" json:"zone_name"`
	Records  []string `yaml:"records" json:"records"` // 需要更新的子域名列表，如 "vpn", "www"
}

// GenerateRandomKey 生成指定长度（字节）的随机 base64 编码字符串
func GenerateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// SaveConfig 保存配置到文件
func SaveConfig(cfg *AppConfig, path string) error {
	// 验证配置
	if err := ValidateConfig(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	
	// 确保父目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败 %s: %w", dir, err)
	}
	
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0666)
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	
	// 验证配置
	if err := ValidateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return &cfg, nil
}

// SafeConfig 线程安全的配置包装器
type SafeConfig struct {
	cfg *AppConfig
	mu  sync.RWMutex
}

// NewSafeConfig 创建新的 SafeConfig
func NewSafeConfig(cfg *AppConfig) *SafeConfig {
	return &SafeConfig{cfg: cfg}
}

// Get 获取当前配置的副本
func (s *SafeConfig) Get() AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.cfg
}

// Update 更新配置
func (s *SafeConfig) Update(newCfg *AppConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = newCfg
}

// DefaultConfig 生成默认配置
func DefaultConfig() *AppConfig {
	key, _ := GenerateRandomKey(32) // Changed GenerateAPIKey() to GenerateRandomKey(32)
	return &AppConfig{
		Server: ServerConfig{
			Port:   8080,
			APIKey: key,
			TrustedSubnets: []string{
				"127.0.0.1/32",
				"192.168.1.0/24",
			},
		},
		IPProviders: []IPProviderConfig{
			{
				Type:    "stun",
				Enabled: true,
				Properties: map[string]string{
					"server": "stun.l.google.com:19302",
				},
			},
		},
		CloudflareAccounts: []CloudflareAccount{},
		Intervals: IntervalsConfig{
			IPCheck:          "5m",
			DNSUpdate:        "1m",
			HistoryRetention: "30d",
		},
		IPv6: IPv6Config{
			Enabled:           false,
			UpdateAAAARecords: false,
		},
	}
}

// WatchConfig 监控配置文件变化并自动重载
// 实现了去抖动机制，防止编辑器保存时产生的多次事件
func WatchConfig(configPath string, safeCfg *SafeConfig) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建文件监控器失败: %w", err)
	}

	// 监控配置文件所在的父目录（而不是文件本身）
	// 这样可以支持编辑器的原子保存操作（先写临时文件，再重命名）
	configDir := filepath.Dir(configPath)
	if err := watcher.Add(configDir); err != nil {
		return fmt.Errorf("监控目录失败 %s: %w", configDir, err)
	}

	log.Printf("🔍 开始监控配置文件: %s", configPath)

	// 启动监控协程
	go func() {
		defer watcher.Close()

		// 去抖动: 使用定时器延迟处理事件
		var debounceTimer *time.Timer
		debounceDuration := 500 * time.Millisecond

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// 只处理与目标配置文件相关的事件
				if event.Name != configPath {
					continue
				}

				// 只关心写入和创建事件（重命名会触发创建）
				if event.Op&fsnotify.Write != fsnotify.Write && 
				   event.Op&fsnotify.Create != fsnotify.Create {
					continue
				}

				// 去抖动: 重置定时器
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDuration, func() {
					log.Printf("🔄 检测到配置文件变化，正在重新加载...")

					// 尝试加载新配置到临时变量
					newCfg, err := LoadConfig(configPath)
					if err != nil {
						log.Printf("❌ 重载配置失败（保持原配置）: %v", err)
						return
					}

					// 验证通过，更新配置
					safeCfg.Update(newCfg)
					log.Printf("✅ 配置已热更新")
				})

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("⚠️  配置监控错误: %v", err)
			}
		}
	}()

	return nil
}