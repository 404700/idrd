package ip

import (
	"fmt"

	"github.com/pion/stun"
)

// STUNProvider 通过 STUN 服务器获取公网 IP
type STUNProvider struct {
	Server string
}

// GetIP 从 STUN 服务器获取公网 IP
func (s *STUNProvider) GetIP() (string, string, error) {
	// 创建 STUN 客户端
	// 注意：stun.Dial 默认使用 UDP
	c, err := stun.Dial("udp", s.Server)
	if err != nil {
		return "", "", fmt.Errorf("连接 STUN 服务器失败: %w", err)
	}
	defer c.Close()

	// 构建请求
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	var ip string
	var callbackErr error // 使用单独的变量存储回调中的错误
	
	// 发送请求并等待响应
	if err := c.Do(message, func(res stun.Event) {
		if res.Error != nil {
			callbackErr = res.Error
			return
		}
		// 解析 XOR-MAPPED-ADDRESS
		var xorAddr stun.XORMappedAddress
		if getErr := xorAddr.GetFrom(res.Message); getErr != nil {
			callbackErr = getErr
			return
		}
		ip = xorAddr.IP.String()
	}); err != nil {
		return "", "", fmt.Errorf("STUN 请求失败: %w", err)
	}

	// 检查回调中是否有错误
	if callbackErr != nil {
		return "", "", fmt.Errorf("STUN 响应处理失败: %w", callbackErr)
	}

	if ip == "" {
		return "", "", fmt.Errorf("未能从 STUN 响应中获取 IP")
	}

	return ip, "STUN", nil
}

// GetIPv6 获取 IPv6 地址（STUN 通常不支持，返回空）
func (s *STUNProvider) GetIPv6() (string, string, error) {
	return "", "", nil
}

