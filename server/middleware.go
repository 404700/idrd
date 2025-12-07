package server

import (
	"idrd/config"
	"net"

	"github.com/labstack/echo/v4"
)

// AuthMiddleware 验证 API Key
// 如果请求来自可信子网（Context 中 trusted=true），则跳过认证
func AuthMiddleware(cfg *config.SafeConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 检查是否为可信来源
			if trusted, ok := c.Get("trusted").(bool); ok && trusted {
				// 可信子网，直接放行
				return next(c)
			}

			// 非可信来源，验证 API Key
			// 动态获取当前 API Key
			apiKey := cfg.Get().Server.APIKey

			// 从请求头或查询参数获取 key
			key := c.Request().Header.Get("X-API-Key")
			if key == "" {
				key = c.QueryParam("key")
			}

			if key != apiKey {
				return echo.ErrUnauthorized
			}

			return next(c)
		}
	}
}

// TrustedSubnetMiddleware 标记可信子网的来源
// 如果客户端 IP 在可信子网内，在 Context 中设置 "trusted" 为 true
func TrustedSubnetMiddleware(cfg *config.SafeConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 动态获取当前可信子网配置
			trustedCIDRs := cfg.Get().Server.TrustedSubnets

			// 解析 CIDR 列表
			var trustedNets []*net.IPNet
			for _, cidr := range trustedCIDRs {
				_, ipNet, err := net.ParseCIDR(cidr)
				if err == nil {
					trustedNets = append(trustedNets, ipNet)
				}
			}

			// 获取客户端 IP
			clientIP := c.RealIP()
			if clientIP == "" {
				clientIP = c.Request().RemoteAddr
			}

			// 正确处理 IPv4 和 IPv6 地址的端口分离
			// net.SplitHostPort 能正确处理 [::1]:8080 和 192.168.1.1:8080 格式
			if host, _, err := net.SplitHostPort(clientIP); err == nil {
				clientIP = host
			}
			// 如果 SplitHostPort 失败，说明没有端口，直接使用原值

			ip := net.ParseIP(clientIP)
			if ip != nil {
				// 检查是否在可信子网中
				for _, ipNet := range trustedNets {
					if ipNet.Contains(ip) {
						// 在可信子网中，设置上下文标记
						c.Set("trusted", true)
						break
					}
				}
			}

			// 继续后续处理（可能包括 AuthMiddleware）
			return next(c)
		}
	}
}
