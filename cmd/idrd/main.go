package main

import (
	"context"
	"fmt"
	"idrd/config"
	"idrd/db"
	"idrd/dns"
	"idrd/ip"
	"idrd/server"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var startTime = time.Now()

func main() {
	// 检查是否是初始化模式
	initMode := false
	for _, arg := range os.Args[1:] {
		if arg == "--init" || arg == "-init" {
			initMode = true
			break
		}
	}

	// 配置文件路径
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/data/config.yaml"
	}

	// 加载或生成配置
	cfg, isNew, err := loadOrGenerateConfig(configPath)
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 如果是初始化模式，生成完配置后直接退出
	if initMode {
		if isNew {
			log.Println("✅ 初始化完成，容器即将退出")
		} else {
			// 配置已存在，显示现有配置信息
			log.Println("========================================")
			log.Printf("ℹ️  配置文件已存在: %s", configPath)
			log.Printf("🔑 API Key: %s", cfg.Server.APIKey)
			log.Println("========================================")
			generateDockerCompose(configPath, cfg)
			log.Println("✅ 初始化检查完成，容器即将退出")
		}
		return
	}

	// 创建线程安全的配置包装器
	safeCfg := config.NewSafeConfig(cfg)

	// 启动配置文件热更新监控
	if err := config.WatchConfig(configPath, safeCfg); err != nil {
		log.Printf("⚠️  配置监控启动失败（继续运行但无法热更新）: %v", err)
	}

	// 初始化数据库
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/idrd.db"
	}
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer database.Close()
	log.Printf("📊 数据库已初始化: %s", dbPath)

	// 创建动态 IP 提供者
	ipProvider := &ip.DynamicProvider{Config: safeCfg}

	// 创建 DNS 同步器（传入数据库）
	dnsUpdater := &dns.CloudflareUpdater{Config: safeCfg, DB: database}

	// 创建 Web 服务器（传入数据库、DNS 同步器、IP 提供者和启动时间）
	srv := server.New(safeCfg, database, dnsUpdater, ipProvider, startTime)

	// 启动 IP 监控协程
	go monitorIP(ipProvider, dnsUpdater, srv, database, safeCfg)

	// 使用错误通道来同步服务器启动失败
	serverErr := make(chan error, 1)

	// 启动 Web 服务器
	go func() {
		currentCfg := safeCfg.Get()
		port := currentCfg.Server.Port
		
		// 环境变量 PORT 优先于配置文件
		if envPort := os.Getenv("PORT"); envPort != "" {
			if p, err := fmt.Sscanf(envPort, "%d", &port); err == nil && p == 1 {
				log.Printf("ℹ️  使用环境变量 PORT=%d 覆盖配置文件端口", port)
			}
		}
		
		addr := fmt.Sprintf(":%d", port)
		log.Printf("🚀 服务器启动在端口 %d", port)
		if err := srv.Echo.Start(addr); err != nil && err.Error() != "http: Server closed" {
			serverErr <- err
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	// 等待关闭信号或服务器错误
	select {
	case <-quit:
		log.Println("正在关闭服务器...")
	case err := <-serverErr:
		log.Printf("❌ 服务器启动失败: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Echo.Shutdown(ctx); err != nil {
		log.Printf("❌ 服务器关闭出错: %v", err)
	}
}

// loadOrGenerateConfig 加载配置或生成默认配置，返回配置、是否新生成、错误
func loadOrGenerateConfig(path string) (*config.AppConfig, bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Println("⚠️  未找到配置文件，生成默认配置...")
		cfg := config.DefaultConfig()
		if err := config.SaveConfig(cfg, path); err != nil {
			return nil, false, err
		}

		log.Println("========================================")
		log.Printf("✅ 配置文件已生成: %s", path)
		log.Printf("🔑 API Key: %s", cfg.Server.APIKey)
		log.Println("⚠️  请务必记录上述 API Key!")
		log.Println("========================================")

		// 生成 docker-compose.yml 示例文件
		generateDockerCompose(path, cfg)

		return cfg, true, nil
	}

	cfg, err := config.LoadConfig(path)
	return cfg, false, err
}

// generateDockerCompose 生成 docker-compose.yml 文件
// 使用类似 config.yaml 的方式动态生成，而非从模板复制
// docker-compose.yml.template 作为完整配置参考保留
func generateDockerCompose(configPath string, cfg *config.AppConfig) {
	// 确定 docker-compose.yml 的路径
	dataDir := filepath.Dir(configPath) // /data
	composePath := filepath.Join(dataDir, "docker-compose.yml")

	// 如果已存在则不覆盖
	if _, err := os.Stat(composePath); err == nil {
		log.Printf("ℹ️  docker-compose.yml 已存在: %s", composePath)
		return
	}

	// 动态生成最小化的 docker-compose.yml
	// 使用 config.yaml 中的配置值
	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}

	composeContent := fmt.Sprintf(`# ╔═══════════════════════════════════════════════════════════════════════════╗
# ║                    IDRD Docker Compose 配置                              ║
# ║                                                                           ║
# ║  此文件由 --init 命令自动生成，包含最小必要配置                            ║
# ║  如需高级配置（Traefik、网络模式、资源限制等），请参考:                      ║
# ║  docker-compose.yml.template                                              ║
# ╚═══════════════════════════════════════════════════════════════════════════╝

services:
  idrd:
    # 使用预构建镜像（推荐）
    image: ghcr.io/404700/idrd:latest
    
    # 本地构建（需要源代码，取消注释以下两行并注释上方 image）
    # image: idrd:latest
    # build: .
    
    container_name: idrd
    restart: unless-stopped
    
    # 用户权限：必须与运行 --init 时的用户一致
    # 启动前运行: export UID=$(id -u) GID=$(id -g)
    user: "${UID:-1000}:${GID:-1000}"
    
    environment:
      - CONFIG_PATH=/data/config.yaml
      - DB_PATH=/data/idrd.db
      - PORT=%d
      - TZ=Asia/Shanghai
    
    volumes:
      - ./data:/data:rw
    
    ports:
      - "%d:%d"
    
    # 健康检查
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:%d/api/ip"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

networks:
  default:
    name: idrd-network

# ═══════════════════════════════════════════════════════════════════════════
# 更多配置选项请参考 docker-compose.yml.template:
# - Traefik 反向代理 + 自动 HTTPS
# - 宿主机网络模式（用于 Router SSH）
# - 资源限制
# - SSH 密钥挂载
# ═══════════════════════════════════════════════════════════════════════════
`, port, port, port, port)

	if err := os.WriteFile(composePath, []byte(composeContent), 0666); err != nil {
		log.Printf("⚠️  生成 docker-compose.yml 失败: %v", err)
		return
	}

	log.Printf("📝 已生成 docker-compose.yml: %s", composePath)
	log.Println("💡 提示：请将 data/docker-compose.yml 移动到项目根目录，然后执行:")
	log.Println("   export UID=$(id -u) GID=$(id -g)")
	log.Println("   docker compose up -d")
}

// monitorIP 定期检查 IP 变化并同步 DNS
func monitorIP(provider ip.Provider, updater *dns.CloudflareUpdater, srv *server.Server, database *db.DB, safeCfg *config.SafeConfig) {
	// 从数据库获取最后记录的 IP，避免每次重启都触发 IP 变化
	var lastIP string
	if history, err := database.GetRecentIPHistory(1); err == nil && len(history) > 0 {
		lastIP = history[0].IP
		log.Printf("📜 从历史记录恢复上次 IP: %s", lastIP)
	}

	for {
		cfg := safeCfg.Get()
		
		// 解析检查间隔
		checkInterval := 5 * time.Minute
		if cfg.Intervals.IPCheck != "" {
			if d, err := time.ParseDuration(cfg.Intervals.IPCheck); err == nil {
				checkInterval = d
				
				// 根据 provider 类型设置不同的最小间隔
				minInterval := 30 * time.Second // 默认最小 30 秒（适用于 STUN/HTTP 等外部服务）
				
				// 检查当前启用的 provider 类型
				for _, p := range cfg.IPProviders {
					if p.Enabled {
						// Router SSH 和本地接口可以更频繁检查（最小 1 秒）
						if p.Type == "router_ssh" || p.Type == "interface" {
							minInterval = 1 * time.Second
							break
						}
					}
				}
				
				if checkInterval < minInterval {
					log.Printf("⚠️  IP 检查间隔 %v 小于最小值 %v，已调整", checkInterval, minInterval)
					checkInterval = minInterval
				}
			}
		}

		currentIP, source, err := provider.GetIP()
		if err != nil {
			log.Printf("❌ 获取 IP 失败: %v", err)
			// 记录错误到数据库
			database.AddErrorLog("error", fmt.Sprintf("获取 IP 失败: %v", err))
			time.Sleep(30 * time.Second)
			continue
		}

		// 每次成功获取 IP 时都更新 CurrentIP 和 CurrentSource
		// 这样即使 IP 没变但 provider 切换了，source 也会正确更新
		srv.SetCurrentIP(currentIP)
		srv.SetCurrentSource(source)

		if currentIP != lastIP {
			log.Printf("🔄 检测到 IP 变化: %s -> %s (Source: %s)", lastIP, currentIP, source)
			
			// 记录 IP 变化到数据库
			if err := database.AddIPHistory(currentIP, "v4", source); err != nil {
				log.Printf("⚠️  记录 IP 历史失败: %v", err)
			}
			
			if err := updater.UpdateIP(currentIP); err != nil {
				log.Printf("❌ DNS 同步失败: %v", err)
				// DNS 同步失败已在 CloudflareUpdater 中记录
			} else {
				log.Printf("✅ DNS 记录已同步为: %s", currentIP)
			}

			lastIP = currentIP
		}

		time.Sleep(checkInterval)
	}
}
