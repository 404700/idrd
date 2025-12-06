package ip

import (
	"fmt"
	"idrd/config"
)

// Provider 定义获取公网 IP 的接口
type Provider interface {
	GetIP() (string, string, error) // 返回 IP, Source, Error
	GetIPv6() (string, string, error) // 新增 IPv6 支持
}

// DynamicProvider 根据配置动态选择 IP 提供者
type DynamicProvider struct {
	Config *config.SafeConfig
}

// GetIP 根据配置获取 IPv4
func (d *DynamicProvider) GetIP() (string, string, error) {
	cfg := d.Config.Get()
	
	var errors []string

	// 遍历所有启用的提供者
	for _, pCfg := range cfg.IPProviders {
		if !pCfg.Enabled {
			continue
		}
		
		var p Provider
		
		switch pCfg.Type {
		case "stun":
			server := pCfg.Properties["server"]
			if server == "" {
				server = "stun.l.google.com:19302"
			}
			p = &STUNProvider{Server: server}
		case "router_ssh":
			// 解析端口
			port := 22
			if pCfg.Properties["port"] != "" {
				fmt.Sscanf(pCfg.Properties["port"], "%d", &port)
			}
			p = &RouterProvider{
				Type:      pCfg.Properties["type"],
				Host:      pCfg.Properties["host"],
				Port:      port,
				User:      pCfg.Properties["user"],
				Password:  pCfg.Properties["password"],
				Key:       pCfg.Properties["key"],
				KeyPath:   pCfg.Properties["key_path"],
				Interface: pCfg.Properties["interface"],
				HostKey:   pCfg.Properties["host_key"],
			}
		// TODO: 添加 http 和 interface 支持
		default:
			continue
		}
		
		if p != nil {
			ip, source, err := p.GetIP()
			if err == nil && ip != "" {
				// 如果底层 provider 返回了 source，使用它；否则使用配置的 Type
				if source == "" {
					source = pCfg.Type
				}
				return ip, source, nil
			}
			if err != nil {
				errors = append(errors, fmt.Sprintf("[%s] %v", pCfg.Type, err))
			}
		}
	}
	
	if len(errors) > 0 {
		return "", "", fmt.Errorf("所有启用的 IP 提供者均获取失败: %v", errors)
	}
	return "", "", fmt.Errorf("没有启用的 IP 提供者")
}

// GetIPv6 根据配置获取 IPv6
func (d *DynamicProvider) GetIPv6() (string, string, error) {
	// 暂时未实现，返回空
	return "", "", nil
}
