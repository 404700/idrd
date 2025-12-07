package db

import (
	"database/sql"
	"encoding/json"
)

// SettingKey 定义配置键名常量
const (
	SettingKeyInternalPort     = "internal_port"      // 内部端口（通常由环境变量覆盖，但可存储）
	SettingKeyAPIKey           = "api_key"            // API Key
	SettingKeyTrustedSubnets   = "trusted_subnets"    // 可信子网 JSON
	SettingKeyCheckInterval    = "check_interval"     // IP 检查间隔
	SettingKeyDNSInterval      = "dns_interval"       // DNS 更新间隔
	SettingKeyHistoryRetention = "history_retention"  // 历史保留时间
	SettingKeyIPv6Enabled      = "ipv6_enabled"       // IPv6 启用
	SettingKeyUpdateAAAA       = "update_aaaa"        // 更新 AAAA 记录
)

// IPProviderConfig 数据库中的 IP 提供商配置结构
type IPProviderConfig struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
	Properties string `json:"properties"` // JSON 字符串
}

// CloudflareAccountConfig 数据库中的 Cloudflare 账户配置结构
type CloudflareAccountConfig struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	APIToken string `json:"api_token"`
	Zones    string `json:"zones"` // JSON 字符串
}

// -----------------------------------------------------------------------------
// Settings (Key-Value) Operations
// -----------------------------------------------------------------------------

// GetSetting 获取单个设置
func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // 不存在返回空字符串，不报错
	}
	return value, err
}

// SetSetting 保存或更新单个设置
func (db *DB) SetSetting(key, value string) error {
	query := `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
			  ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`
	_, err := db.conn.Exec(query, key, value)
	return err
}

// GetSettingsMap 获取所有设置
func (db *DB) GetSettingsMap() (map[string]string, error) {
	rows, err := db.conn.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value
	}
	return settings, nil
}

// -----------------------------------------------------------------------------
// IP Providers Operations
// -----------------------------------------------------------------------------

// GetAllIPProviders 获取所有 IP 提供商配置
func (db *DB) GetAllIPProviders() ([]IPProviderConfig, error) {
	rows, err := db.conn.Query("SELECT id, type, enabled, properties FROM ip_providers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []IPProviderConfig
	for rows.Next() {
		var p IPProviderConfig
		if err := rows.Scan(&p.ID, &p.Type, &p.Enabled, &p.Properties); err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, nil
}

// SaveIPProviders 清空并保存所有 IP 提供商配置 (全量替换模式)
func (db *DB) SaveIPProviders(providers []IPProviderConfig) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 清空旧数据
	if _, err := tx.Exec("DELETE FROM ip_providers"); err != nil {
		return err
	}

	// 2. 插入新数据
	stmt, err := tx.Prepare("INSERT INTO ip_providers (type, enabled, properties) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range providers {
		if _, err := stmt.Exec(p.Type, p.Enabled, p.Properties); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// -----------------------------------------------------------------------------
// Cloudflare Accounts Operations
// -----------------------------------------------------------------------------

// GetAllCloudflareAccounts 获取所有 Cloudflare 账户配置
func (db *DB) GetAllCloudflareAccounts() ([]CloudflareAccountConfig, error) {
	rows, err := db.conn.Query("SELECT id, name, api_token, zones FROM cloudflare_accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []CloudflareAccountConfig
	for rows.Next() {
		var acc CloudflareAccountConfig
		if err := rows.Scan(&acc.ID, &acc.Name, &acc.APIToken, &acc.Zones); err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

// SaveCloudflareAccounts 清空并保存所有 Cloudflare 账户配置 (全量替换模式)
func (db *DB) SaveCloudflareAccounts(accounts []CloudflareAccountConfig) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 清空旧数据
	if _, err := tx.Exec("DELETE FROM cloudflare_accounts"); err != nil {
		return err
	}

	// 2. 插入新数据
	stmt, err := tx.Prepare("INSERT INTO cloudflare_accounts (name, api_token, zones) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, acc := range accounts {
		if _, err := stmt.Exec(acc.Name, acc.APIToken, acc.Zones); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// -----------------------------------------------------------------------------
// Helper: Helper to convert JSON
// -----------------------------------------------------------------------------
func ToJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
