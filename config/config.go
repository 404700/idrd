package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"idrd/db"
)

// AppConfig 应用程序总配置
type AppConfig struct {
	Server             ServerConfig        `yaml:"server" json:"server"`
	IPProviders        []IPProviderConfig  `yaml:"ip_providers" json:"ip_providers"`
	CloudflareAccounts []CloudflareAccount `yaml:"cloudflare_accounts" json:"cloudflare_accounts"`
	Intervals          IntervalsConfig     `yaml:"intervals" json:"intervals"`
	IPv6               IPv6Config          `yaml:"ipv6" json:"ipv6"`
	

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
	DNSUpdate        string `yaml:"dns_update" json:"dns_update"`       // DNS 更新间隔
	HistoryRetention string `yaml:"history_retention" json:"history_retention"` // 历史保留时间
}

// IPv6Config IPv6 配置
type IPv6Config struct {
	Enabled           bool `yaml:"enabled" json:"enabled"`
	UpdateAAAARecords bool `yaml:"update_aaaa_records" json:"update_aaaa_records"`
}



// GenerateRandomKey 生成指定长度（字节）的随机 base64 编码字符串
func GenerateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// SaveConfig 保存配置到数据库
func SaveConfig(cfg *AppConfig, database *db.DB) error {
	// 验证配置
	if err := ValidateConfig(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// 1. 保存设置 (Settings)
	if err := database.SetSetting(db.SettingKeyAPIKey, cfg.Server.APIKey); err != nil {
		return err
	}
	// Port 通常只读（启动参数），但也存一下
	if err := database.SetSetting(db.SettingKeyInternalPort, strconv.Itoa(cfg.Server.Port)); err != nil {
		return err
	}
	
	trustedJSON, _ := json.Marshal(cfg.Server.TrustedSubnets)
	if err := database.SetSetting(db.SettingKeyTrustedSubnets, string(trustedJSON)); err != nil {
		return err
	}

	if err := database.SetSetting(db.SettingKeyCheckInterval, cfg.Intervals.IPCheck); err != nil {
		return err
	}
	if err := database.SetSetting(db.SettingKeyDNSInterval, cfg.Intervals.DNSUpdate); err != nil {
		return err
	}
	if err := database.SetSetting(db.SettingKeyHistoryRetention, cfg.Intervals.HistoryRetention); err != nil {
		return err
	}
	
	if err := database.SetSetting(db.SettingKeyIPv6Enabled, strconv.FormatBool(cfg.IPv6.Enabled)); err != nil {
		return err
	}
	if err := database.SetSetting(db.SettingKeyUpdateAAAA, strconv.FormatBool(cfg.IPv6.UpdateAAAARecords)); err != nil {
		return err
	}

	// 2. 保存 IP Providers
	var dbProviders []db.IPProviderConfig
	for _, p := range cfg.IPProviders {
		propsJSON, _ := json.Marshal(p.Properties)
		dbProviders = append(dbProviders, db.IPProviderConfig{
			Type:       p.Type,
			Enabled:    p.Enabled,
			Properties: string(propsJSON),
		})
	}
	if err := database.SaveIPProviders(dbProviders); err != nil {
		return err
	}

	// 3. 保存 Cloudflare Accounts
	var dbAccounts []db.CloudflareAccountConfig
	for _, acc := range cfg.CloudflareAccounts {
		zonesJSON, _ := json.Marshal(acc.Zones)
		dbAccounts = append(dbAccounts, db.CloudflareAccountConfig{
			Name:     acc.Name,
			APIToken: acc.APIToken,
			Zones:    string(zonesJSON),
		})
	}
	if err := database.SaveCloudflareAccounts(dbAccounts); err != nil {
		return err
	}

	return nil
}

// LoadConfig 从数据库加载配置
// 如果数据库为空，返回 nil, nil (需要调用者处理初始化)
// 这里的 path 参数是为了兼容性预留的，实际不再从文件加载除迁移外的配置
func LoadConfig(database *db.DB) (*AppConfig, error) {
	// 检查是否有配置数据（通过检查 API Key 是否存在）
	apiKey, err := database.GetSetting(db.SettingKeyAPIKey)
	if err != nil {
		return nil, err
	}
	if apiKey == "" {
		return nil, nil // 数据库无配置
	}

	cfg := &AppConfig{}

	// 1. 加载设置
	cfg.Server.APIKey = apiKey
	
	portStr, _ := database.GetSetting(db.SettingKeyInternalPort)
	if portStr != "" {
		cfg.Server.Port, _ = strconv.Atoi(portStr)
	}

	trustedJSON, _ := database.GetSetting(db.SettingKeyTrustedSubnets)
	if trustedJSON != "" {
		json.Unmarshal([]byte(trustedJSON), &cfg.Server.TrustedSubnets)
	}

	cfg.Intervals.IPCheck, _ = database.GetSetting(db.SettingKeyCheckInterval)
	cfg.Intervals.DNSUpdate, _ = database.GetSetting(db.SettingKeyDNSInterval)
	cfg.Intervals.HistoryRetention, _ = database.GetSetting(db.SettingKeyHistoryRetention)

	ipv6EnabledStr, _ := database.GetSetting(db.SettingKeyIPv6Enabled)
	cfg.IPv6.Enabled, _ = strconv.ParseBool(ipv6EnabledStr)

	updateAAAAStr, _ := database.GetSetting(db.SettingKeyUpdateAAAA)
	cfg.IPv6.UpdateAAAARecords, _ = strconv.ParseBool(updateAAAAStr)

	// 2. 加载 IP Providers
	cfg.IPProviders = []IPProviderConfig{} // 初始化为空切片，避免 JSON 输出 null
	dbProviders, err := database.GetAllIPProviders()
	if err != nil {
		return nil, err
	}
	for _, p := range dbProviders {
		var props map[string]string
		json.Unmarshal([]byte(p.Properties), &props)
		cfg.IPProviders = append(cfg.IPProviders, IPProviderConfig{
			Type:       p.Type,
			Enabled:    p.Enabled,
			Properties: props,
		})
	}

	// 3. 加载 Cloudflare Accounts
	cfg.CloudflareAccounts = []CloudflareAccount{} // 初始化为空切片，避免 JSON 输出 null
	dbAccounts, err := database.GetAllCloudflareAccounts()
	if err != nil {
		return nil, err
	}
	for _, acc := range dbAccounts {
		var zones []Zone
		json.Unmarshal([]byte(acc.Zones), &zones)
		cfg.CloudflareAccounts = append(cfg.CloudflareAccounts, CloudflareAccount{
			Name:     acc.Name,
			APIToken: acc.APIToken,
			Zones:    zones,
		})
	}

	return cfg, nil
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
	key, _ := GenerateRandomKey(32)
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


