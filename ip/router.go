package ip

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// RouterProvider 通过 SSH 连接路由器获取 WAN IP
type RouterProvider struct {
	Type            string
	Host            string
	Port            int
	User            string
	Password        string
	Key             string
	KeyPath         string
	Interface       string
	HostKey         string // 预期的主机公钥 (base64 编码)
	StrictHostCheck bool   // 是否严格检查主机密钥
}

// GetIPv6 获取 IPv6 地址（暂未实现）
func (r *RouterProvider) GetIPv6() (string, string, error) {
	return "", "", nil
}

// GetIP 从路由器获取 WAN 接口的公网 IP
func (r *RouterProvider) GetIP() (string, string, error) {
	// 构建 SSH 配置
	config := &ssh.ClientConfig{
		User:            r.User,
		Auth:            r.getAuthMethods(),
		HostKeyCallback: r.getHostKeyCallback(),
		Timeout:         10 * time.Second,
	}

	// 连接 SSH
	addr := fmt.Sprintf("%s:%d", r.Host, r.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", "", fmt.Errorf("SSH 连接失败 %s: %w", addr, err)
	}
	defer client.Close()

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("创建 SSH 会话失败: %w", err)
	}
	defer session.Close()

	// 根据路由器类型执行不同的命令
	var cmd string
	switch strings.ToLower(r.Type) {
	case "routeros":
		// RouterOS: 使用 put 输出 WAN 接口 IP（格式：x.x.x.x/x）
		cmd = fmt.Sprintf(`:put [/ip address get [find interface="%s"] address]`, r.Interface)
	case "openwrt":
		// OpenWrt: ubus call network.interface.wan status | jsonfilter -e '@["ipv4-address"][0].address'
		cmd = fmt.Sprintf("ubus call network.interface.%s status | jsonfilter -e '@[\"ipv4-address\"][0].address'", r.Interface)
	default:
		return "", "", fmt.Errorf("不支持的路由器类型: %s（仅支持 routeros 和 openwrt）", r.Type)
	}

	// 执行命令
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return "", "", fmt.Errorf("执行命令失败: %w, 输出: %s", err, string(output))
	}

	// 解析输出
	ip, err := r.parseIP(string(output))
	if err != nil {
		return "", "", err
	}
	return ip, "ROUTER_SSH", nil
}

// getAuthMethods 返回 SSH 认证方法
func (r *RouterProvider) getAuthMethods() []ssh.AuthMethod {
	var auth []ssh.AuthMethod

	// 方式 1：优先使用直接配置的私钥内容（Key 字段）
	if r.Key != "" {
		// 清理 YAML 多行字符串可能带来的额外缩进
		cleanKey := cleanPEMKey(r.Key)
		signer, err := parsePrivateKey([]byte(cleanKey))
		if err == nil {
			auth = append(auth, ssh.PublicKeys(signer))
			log.Printf("🔑 SSH: 添加公钥认证方法")
		} else {
			log.Printf("⚠️ SSH 私钥解析失败: %v. 请检查格式是否正确 (支持 OpenSSH/PEM/PKCS#8 格式的 RSA/ECDSA/ed25519 密钥)", err)
		}
	}

	// 方式 2：从文件读取私钥（KeyPath 字段）
	if r.KeyPath != "" {
		key, err := os.ReadFile(r.KeyPath)
		if err == nil {
			signer, err := parsePrivateKey(key)
			if err == nil {
				auth = append(auth, ssh.PublicKeys(signer))
				log.Printf("🔑 SSH: 添加公钥认证方法 (从文件: %s)", r.KeyPath)
			}
		}
	}

	// 方式 3：密码认证
	// 注意：keyboard-interactive 必须放在 password 之前，因为 RouterOS 等设备使用此方式
	if r.Password != "" {
		// keyboard-interactive 认证（RouterOS 等使用此方式，必须优先）
		auth = append(auth, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			log.Printf("🔑 SSH keyboard-interactive: user=%s, questions=%d", user, len(questions))
			// 对于每个问题，都回答密码
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = r.Password
			}
			return answers, nil
		}))
		log.Printf("🔑 SSH: 添加 keyboard-interactive 认证方法")
		
		// 标准 password 认证（备用）
		auth = append(auth, ssh.Password(r.Password))
		log.Printf("🔑 SSH: 添加 password 认证方法")
	}

	log.Printf("🔑 SSH: 共配置 %d 种认证方法", len(auth))
	return auth
}

// getHostKeyCallback 返回主机密钥验证回调
func (r *RouterProvider) getHostKeyCallback() ssh.HostKeyCallback {
	// 如果未启用严格检查，但提供了 HostKey，记录警告并使用固定密钥验证
	if r.HostKey != "" {
		expectedKeyBytes, err := base64.StdEncoding.DecodeString(r.HostKey)
		if err != nil {
			log.Printf("⚠️ 主机密钥格式错误: %v，将使用不安全模式（接受任何主机密钥）", err)
			return ssh.InsecureIgnoreHostKey()
		}

		expectedKey, err := ssh.ParsePublicKey(expectedKeyBytes)
		if err != nil {
			log.Printf("⚠️ 主机密钥解析失败: %v，将使用不安全模式（接受任何主机密钥）", err)
			return ssh.InsecureIgnoreHostKey()
		}

		return ssh.FixedHostKey(expectedKey)
	}

	// 如果未配置主机密钥，使用不安全模式（自动接受所有 Host Key，类似 ssh -o StrictHostKeyChecking=no）
	// log.Printf("⚠️ WARNING: SSH host key verification DISABLED for %s", addr)
	return ssh.InsecureIgnoreHostKey()
}

// parseIP 解析路由器输出中的 IP 地址
func (r *RouterProvider) parseIP(output string) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return "", fmt.Errorf("输出为空")
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// RouterOS 输出格式：192.168.1.1/24 或 192.168.1.1
		if strings.Contains(line, "/") {
			ip := strings.Split(line, "/")[0]
			if net.ParseIP(ip) != nil {
				return ip, nil
			}
		}

		// OpenWrt jsonfilter 输出：直接是 IP
		if net.ParseIP(line) != nil {
			return line, nil
		}

		// 兼容旧版 RouterOS print 输出格式：address=192.168.1.1/24
		if strings.Contains(line, "address=") {
			parts := strings.Split(line, "address=")
			if len(parts) > 1 {
				ipCIDR := strings.Fields(parts[1])[0]
				ip := strings.Split(ipCIDR, "/")[0]
				if net.ParseIP(ip) != nil {
					return ip, nil
				}
			}
		}
	}

	return "", fmt.Errorf("无法从输出中解析 IP 地址: %s", output)
}

// cleanPEMKey 清理 PEM 密钥中的额外缩进
// YAML 多行字符串（使用 |-）可能会在每行前添加额外空格
func cleanPEMKey(key string) string {
	lines := strings.Split(key, "\n")
	var cleaned []string
	
	for _, line := range lines {
		// 去除每行开头和结尾的空格
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	
	return strings.Join(cleaned, "\n")
}

// parsePrivateKey 解析多种格式的 SSH 私钥
// 支持格式：OpenSSH, PEM (RSA/ECDSA), PKCS#8 (RSA/ECDSA/ed25519)
func parsePrivateKey(keyBytes []byte) (ssh.Signer, error) {
	// 首先尝试 ssh.ParsePrivateKey（支持 OpenSSH 原生格式）
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err == nil {
		return signer, nil
	}

	// 如果失败，尝试解析 PEM 格式
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, fmt.Errorf("无法解析 PEM 格式: %w", err)
	}

	// 根据 PEM 块类型尝试不同的解析方法
	switch block.Type {
	case "PRIVATE KEY":
		// PKCS#8 格式（可能是 RSA/ECDSA/ed25519）
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析 PKCS#8 私钥失败: %w", err)
		}

		// 根据密钥类型转换为 ssh.Signer
		switch k := key.(type) {
		case ed25519.PrivateKey:
			return ssh.NewSignerFromKey(k)
		case *ed25519.PrivateKey:
			return ssh.NewSignerFromKey(*k)
		default:
			return ssh.NewSignerFromKey(key)
		}

	case "RSA PRIVATE KEY":
		// PEM 格式的 RSA 私钥
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析 RSA 私钥失败: %w", err)
		}
		return ssh.NewSignerFromKey(key)

	case "EC PRIVATE KEY":
		// PEM 格式的 ECDSA 私钥
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析 ECDSA 私钥失败: %w", err)
		}
		return ssh.NewSignerFromKey(key)

	default:
		return nil, fmt.Errorf("不支持的 PEM 块类型: %s", block.Type)
	}
}