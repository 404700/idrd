package dns

import (
	"context"
	"fmt"
	"idrd/config"
	"idrd/db"
	"log"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

// 重试配置常量
const (
	maxRetries     = 3               // 最大重试次数
	initialBackoff = 1 * time.Second // 初始退避时间
	maxBackoff     = 10 * time.Second // 最大退避时间
)

// CloudflareUpdater 负责更新 Cloudflare DNS 记录
type CloudflareUpdater struct {
	Config *config.SafeConfig
	DB     *db.DB
}

// UpdateIP 更新所有配置的 DNS 记录到新 IP
func (c *CloudflareUpdater) UpdateIP(newIP string) error {
	cfg := c.Config.Get()
	
	// 遍历所有账户
	for _, account := range cfg.CloudflareAccounts {
		if account.APIToken == "" {
			continue
		}

		api, err := cloudflare.NewWithAPIToken(account.APIToken)
		if err != nil {
			log.Printf("❌ 创建 Cloudflare 客户端失败 (账户: %s): %v", account.Name, err)
			continue
		}

		// DEBUG: 打印 Token 前缀以排查问题 (只显示前 4 位)
		tokenPrefix := "EMPTY"
		if len(account.APIToken) >= 4 {
			tokenPrefix = account.APIToken[:4] + "..."
		} else if len(account.APIToken) > 0 {
			tokenPrefix = account.APIToken
		}
		log.Printf("🔍 [DEBUG] 初始化 Cloudflare 账户: %s, Token前缀: %s, Token长度: %d", account.Name, tokenPrefix, len(account.APIToken))

		// 遍历该账户下的所有 Zone
		for _, zone := range account.Zones {
			// 为每个 zone 创建独立的 context，确保立即释放
			func() {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // 增加超时以容纳重试
				defer cancel()

				zoneID, err := c.getZoneIDWithRetry(ctx, api, zone.ZoneName, account.Name)
				if err != nil {
					log.Printf("❌ 获取 Zone ID 失败 (账户: %s, 域名: %s): %v", account.Name, zone.ZoneName, err)
					return
				}

				// 遍历并更新每个子域名
				for _, record := range zone.Records {
					fullDomain := record
					if record != "@" && record != "" {
						fullDomain = fmt.Sprintf("%s.%s", record, zone.ZoneName)
					} else {
						fullDomain = zone.ZoneName
					}

					if err := c.updateRecordWithRetry(ctx, api, zoneID, fullDomain, newIP, account.Name); err != nil {
						// 记录失败（重试后仍失败）
						if c.DB != nil {
							c.DB.AddDNSUpdate(account.Name, newIP, "A", fullDomain, false, err.Error())
						}
						log.Printf("❌ 更新 DNS 记录失败 (%s): %v", fullDomain, err)
						continue
					}

					// 记录成功
					if c.DB != nil {
						c.DB.AddDNSUpdate(account.Name, newIP, "A", fullDomain, true, "")
					}
				}
			}()
		}
	}

	return nil
}

// getZoneIDWithRetry 带重试的获取 Zone ID
func (c *CloudflareUpdater) getZoneIDWithRetry(ctx context.Context, api *cloudflare.API, zoneName, accountName string) (string, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("🔄 重试获取 Zone ID (账户: %s, 域名: %s, 尝试 %d/%d)", accountName, zoneName, attempt, maxRetries)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
		}

		zoneID, err := api.ZoneIDByName(zoneName)
		if err == nil {
			return zoneID, nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("重试 %d 次后仍失败: %w", maxRetries, lastErr)
}

// updateRecordWithRetry 带重试的更新单个 DNS 记录
func (c *CloudflareUpdater) updateRecordWithRetry(ctx context.Context, api *cloudflare.API, zoneID, domain, ip, accountName string) error {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("🔄 重试更新 DNS 记录 (%s, 尝试 %d/%d)", domain, attempt, maxRetries)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
		}

		err := c.updateRecord(ctx, api, zoneID, domain, ip)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("重试 %d 次后仍失败: %w", maxRetries, lastErr)
}

// updateRecord 更新单个 DNS 记录（无重试）
func (c *CloudflareUpdater) updateRecord(ctx context.Context, api *cloudflare.API, zoneID, domain, ip string) error {
	// 查找现有记录
	records, _, err := api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
		Name: domain,
		Type: "A",
	})
	if err != nil {
		return err
	}

	if len(records) == 0 {
		// 创建新记录
		_, err := api.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.CreateDNSRecordParams{
			Type:    "A",
			Name:    domain,
			Content: ip,
			TTL:     1, // Auto
			Proxied: cloudflare.BoolPtr(false),
		})
		if err != nil {
			return err
		}
		log.Printf("✅ 创建 DNS 记录成功: %s -> %s", domain, ip)
		return nil
	}

	// 更新现有记录
	record := records[0]
	if record.Content == ip {
		log.Printf("ℹ️  IP 未变化，跳过 Cloudflare 更新: %s -> %s", domain, ip)
		return nil // IP 未变化
	}

	_, err = api.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.UpdateDNSRecordParams{
		ID:      record.ID,
		Type:    "A",
		Name:    domain,
		Content: ip,
		TTL:     1,
		Proxied: record.Proxied,
	})
	if err != nil {
		return err
	}
	log.Printf("✅ 更新 DNS 记录成功: %s -> %s", domain, ip)
	return nil
}

