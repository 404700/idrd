package config

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// parseExtendedDuration 解析扩展的时间间隔格式
// Go 标准库的 time.ParseDuration 不支持天(d)和周(w)，此函数添加支持
// 支持格式：30s, 5m, 1h, 24h, 7d, 2w
func parseExtendedDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// 尝试标准解析
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// 处理 d (天) 和 w (周)
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration number: %s", numStr)
		}
		return time.Duration(num * 24 * float64(time.Hour)), nil
	}

	if strings.HasSuffix(s, "w") {
		numStr := strings.TrimSuffix(s, "w")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration number: %s", numStr)
		}
		return time.Duration(num * 7 * 24 * float64(time.Hour)), nil
	}

	return 0, fmt.Errorf("time: unknown unit in duration %q", s)
}

// ValidateConfig 验证配置的有效性
func ValidateConfig(cfg *AppConfig) error {
	// 验证服务器配置
	if err := validateServerConfig(&cfg.Server); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	// 验证时间间隔配置
	if err := validateIntervalsConfig(&cfg.Intervals); err != nil {
		return fmt.Errorf("intervals config: %w", err)
	}

	// 验证 IP 提供者
	for i, provider := range cfg.IPProviders {
		if err := validateIPProvider(&provider); err != nil {
			return fmt.Errorf("ip_provider[%d]: %w", i, err)
		}
	}

	// 验证 Cloudflare 账户（允许为空，用户可能只想监控 IP 不更新 DNS）
	for i, account := range cfg.CloudflareAccounts {
		if err := validateCloudflareAccount(&account); err != nil {
			return fmt.Errorf("cloudflare_account[%d]: %w", i, err)
		}
	}

	return nil
}

func validateServerConfig(s *ServerConfig) error {
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be 1-65535)", s.Port)
	}

	if len(s.APIKey) < 16 {
		return fmt.Errorf("API key too short (minimum 16 characters, got %d)", len(s.APIKey))
	}

	for _, cidr := range s.TrustedSubnets {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
		}
	}

	return nil
}

func validateIntervalsConfig(i *IntervalsConfig) error {
	// 验证 IP 检查间隔
	if i.IPCheck != "" {
		d, err := parseExtendedDuration(i.IPCheck)
		if err != nil {
			return fmt.Errorf("invalid ip_check interval %s: %w", i.IPCheck, err)
		}
		if d < 1*time.Second {
			return fmt.Errorf("ip_check interval too short: %s (minimum 1s)", i.IPCheck)
		}
	}

	// 验证 DNS 更新间隔
	if i.DNSUpdate != "" {
		d, err := parseExtendedDuration(i.DNSUpdate)
		if err != nil {
			return fmt.Errorf("invalid dns_update interval %s: %w", i.DNSUpdate, err)
		}
		if d < 10*time.Second {
			return fmt.Errorf("dns_update interval too short: %s (minimum 10s)", i.DNSUpdate)
		}
	}

	// 验证历史保留时间
	if i.HistoryRetention != "" {
		d, err := parseExtendedDuration(i.HistoryRetention)
		if err != nil {
			return fmt.Errorf("invalid history_retention %s: %w", i.HistoryRetention, err)
		}
		if d < time.Hour {
			return fmt.Errorf("history_retention too short: %s (minimum 1h)", i.HistoryRetention)
		}
	}

	return nil
}

func validateIPProvider(p *IPProviderConfig) error {
	if p.Type == "" {
		return fmt.Errorf("provider type cannot be empty")
	}

	// 如果provider未启用，跳过详细验证
	if !p.Enabled {
		return nil
	}

	switch p.Type {
	case "stun":
		server := p.Properties["server"]
		if server == "" {
			// 使用默认值
			p.Properties["server"] = "stun.l.google.com:19302"
			return nil
		}
		// 验证 server 格式 (host:port)
		if _, _, err := net.SplitHostPort(server); err != nil {
			return fmt.Errorf("invalid STUN server address %s: %w", server, err)
		}

	case "router_ssh":
		// 验证必需字段
		host := p.Properties["host"]
		if host == "" {
			return fmt.Errorf("router host required")
		}
		// 只验证格式，不做 DNS 查询（网络可能未就绪）
		// 接受 IP 地址或域名格式
		if net.ParseIP(host) == nil {
			// 不是 IP，验证是否是合理的域名格式（包含字母或点）
			if !strings.Contains(host, ".") && !strings.ContainsAny(host, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
				return fmt.Errorf("invalid router host %s (must be IP or domain)", host)
			}
		}

		user := p.Properties["user"]
		if user == "" {
			return fmt.Errorf("router user required")
		}

		// 验证端口
		port := p.Properties["port"]
		if port != "" {
			var portNum int
			if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil {
				return fmt.Errorf("invalid port %s: %w", port, err)
			}
			if portNum < 1 || portNum > 65535 {
				return fmt.Errorf("invalid port %d (must be 1-65535)", portNum)
			}
		}

		// 验证路由器类型（必填）
		routerType := p.Properties["type"]
		if routerType == "" {
			return fmt.Errorf("router type required (must be 'routeros' or 'openwrt')")
		}
		if routerType != "routeros" && routerType != "openwrt" {
			return fmt.Errorf("unsupported router type: %s (must be 'routeros' or 'openwrt')", routerType)
		}

		// 验证接口名称（必填）
		iface := p.Properties["interface"]
		if iface == "" {
			return fmt.Errorf("router interface required (e.g., 'wan' for OpenWrt, 'ether1' for RouterOS)")
		}

		// 验证认证方式：必须有密码或密钥之一
		hasPassword := p.Properties["password"] != ""
		hasKey := p.Properties["key"] != "" || p.Properties["key_path"] != ""
		if !hasPassword && !hasKey {
			return fmt.Errorf("router authentication required (password or key)")
		}

	case "http", "interface":
		// 预留给未来实现
		return fmt.Errorf("provider type %s not yet implemented", p.Type)

	default:
		return fmt.Errorf("unknown provider type: %s", p.Type)
	}

	return nil
}

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
var subdomainRegex = regexp.MustCompile(`^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)

func validateCloudflareAccount(a *CloudflareAccount) error {
	if a.Name == "" {
		return fmt.Errorf("account name cannot be empty")
	}

	// 如果 Token 是脱敏值，跳过长度检查（将在保存时回填）
	if a.APIToken != "***" && a.APIToken != "" && len(a.APIToken) < 20 {
		return fmt.Errorf("API token too short for account %s (minimum 20 characters)", a.Name)
	}

	if len(a.Zones) == 0 {
		return fmt.Errorf("account %s has no zones configured", a.Name)
	}

	for i, zone := range a.Zones {
		if err := validateZone(&zone, i); err != nil {
			return fmt.Errorf("account %s, zone[%d]: %w", a.Name, i, err)
		}
	}

	return nil
}

func validateZone(z *Zone, index int) error {
	if z.ZoneName == "" {
		return fmt.Errorf("zone name cannot be empty")
	}

	if !domainRegex.MatchString(z.ZoneName) {
		return fmt.Errorf("invalid zone name: %s (must be a valid domain)", z.ZoneName)
	}

	if len(z.Records) == 0 {
		return fmt.Errorf("zone %s has no records configured", z.ZoneName)
	}

	for _, record := range z.Records {
		if record == "" {
			return fmt.Errorf("zone %s has empty record name", z.ZoneName)
		}

		// @ 表示根域名，直接通过
		if record == "@" {
			continue
		}

		// 验证子域名格式（现在支持通配符 *）
		if !subdomainRegex.MatchString(record) {
			return fmt.Errorf("zone %s: invalid record name '%s' (must be alphanumeric with hyphens, or *.subdomain)", z.ZoneName, record)
		}
	}

	return nil
}

