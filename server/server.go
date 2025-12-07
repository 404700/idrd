package server

import (
	"embed"
	"fmt"
	"idrd/config"
	"idrd/db"
	"idrd/dns"
	"idrd/ip"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gopkg.in/yaml.v3"
)

//go:embed static
var staticFiles embed.FS

// Server 管理 Web 服务器
type Server struct {
	Echo             *echo.Echo
	Config           *config.SafeConfig
	DB               *db.DB
	DNSUpdater       *dns.CloudflareUpdater
	IPProvider       *ip.DynamicProvider
	Hub              *Hub // WebSocket Hub
	StartTime        time.Time
	LastCheckTime    time.Time
	CurrentIP        string
	CurrentSource    string
	ConfigUpdateChan chan struct{} // 配置更新通知通道
	ipMutex          sync.RWMutex
}

// New 创建新的 Server 实例
func New(cfg *config.SafeConfig, database *db.DB, dnsUpdater *dns.CloudflareUpdater, ipProvider *ip.DynamicProvider, startTime time.Time) *Server {
	e := echo.New()
	e.HidePort = true
	e.HideBanner = true

	s := &Server{
		Echo:             e,
		Config:           cfg,
		DB:               database,
		DNSUpdater:       dnsUpdater,
		IPProvider:       ipProvider,
		Hub:              NewHub(),
		StartTime:        startTime,
		LastCheckTime:    startTime,
		ConfigUpdateChan: make(chan struct{}, 1),
	}

	// 启动 WebSocket Hub
	go s.Hub.Run()

	// 通用中间件
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// 静态文件（嵌入）
	staticFS, _ := fs.Sub(staticFiles, "static")
	
	// 静态文件服务辅助函数
	serveStatic := func(c echo.Context, file string) error {
		data, err := fs.ReadFile(staticFS, file)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		contentType := http.DetectContentType(data)
		if strings.HasSuffix(file, ".js") {
			contentType = "application/javascript"
		} else if strings.HasSuffix(file, ".css") {
			contentType = "text/css"
		} else if strings.HasSuffix(file, ".svg") {
			contentType = "image/svg+xml"
		}
		return c.Blob(http.StatusOK, contentType, data)
	}

	// === 静态资源路由 (不需要认证，否则页面无法加载) ===
	// 这些是 JS/CSS/图片等静态文件，不包含敏感数据
	e.GET("/assets/*", func(c echo.Context) error {
		return serveStatic(c, "assets/"+c.Param("*"))
	})
	e.GET("/index.css", func(c echo.Context) error { return serveStatic(c, "index.css") })
	e.GET("/vite.svg", func(c echo.Context) error { return serveStatic(c, "vite.svg") })
	e.GET("/*.js", func(c echo.Context) error { return serveStatic(c, c.Param("*")+".js") })
	e.GET("/*.css", func(c echo.Context) error { return serveStatic(c, c.Param("*")+".css") })
	e.GET("/*.svg", func(c echo.Context) error { return serveStatic(c, c.Param("*")+".svg") })

	// === 所有其他路由都需要认证 ===
	// 必须在 trusted_subnets 内，或提供有效的 API Key
	authenticated := e.Group("")
	authenticated.Use(TrustedSubnetMiddleware(cfg))
	authenticated.Use(AuthMiddleware(cfg))

	// 主页 & SPA 路由
	authenticated.GET("/", func(c echo.Context) error {
		userAgent := c.Request().UserAgent()
		// 如果是命令行工具，返回 IP
		if strings.Contains(strings.ToLower(userAgent), "curl") ||
			strings.Contains(strings.ToLower(userAgent), "wget") ||
			strings.Contains(strings.ToLower(userAgent), "httpie") ||
			userAgent == "" {
			return c.String(http.StatusOK, c.RealIP()+"\n")
		}
		// 否则返回新版 React 应用
		data, _ := fs.ReadFile(staticFS, "index.html")
		return c.HTMLBlob(http.StatusOK, data)
	})

	// 旧版 HTML 应用
	authenticated.GET("/legacy", func(c echo.Context) error {
		data, _ := fs.ReadFile(staticFS, "legacy/index.html")
		return c.HTMLBlob(http.StatusOK, data)
	})

	authenticated.GET("/legacy/config", func(c echo.Context) error {
		data, _ := fs.ReadFile(staticFS, "legacy/config.html")
		return c.HTMLBlob(http.StatusOK, data)
	})

	// 公开查询 API (现在也需要认证)
	authenticated.GET("/ip", s.handleGetIP)
	authenticated.GET("/api/ip", s.handleGetIPJSON)
	authenticated.GET("/api/status", s.handleGetStatus)
	authenticated.GET("/api/stats/history", s.handleGetHistoryStats)

	// 配置管理 API
	authenticated.GET("/api/config", s.handleGetConfig)
	authenticated.POST("/api/config", s.handleUpdateConfig)
	authenticated.GET("/api/config/export", s.handleExportConfig)
	authenticated.POST("/api/config/import", s.handleImportConfig)
	authenticated.POST("/api/dns/update", s.handleTriggerDNSUpdate)

	// WebSocket 实时推送（不需要认证，因为只推送公开数据）
	e.GET("/ws", s.handleWebSocket)

	// 健康检查端点（不需要认证，供 Docker 健康检查使用）
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// SPA 路由回退 (捕获所有非 API 请求)
	authenticated.GET("/*", func(c echo.Context) error {
		path := c.Request().URL.Path
		// 如果是 API 路径但未匹配，返回 404 (不回退到 index.html)
		if strings.HasPrefix(path, "/api/") {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		// 返回 React 应用入口
		data, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			return c.String(http.StatusInternalServerError, "Index file not found")
		}
		return c.HTMLBlob(http.StatusOK, data)
	})

	return s
}

// SetCurrentIP 设置当前 IP（线程安全）
func (s *Server) SetCurrentIP(ip string) {
	s.ipMutex.Lock()
	defer s.ipMutex.Unlock()
	s.CurrentIP = ip
}

// SetCurrentSource 更新当前 IP 来源
func (s *Server) SetCurrentSource(source string) {
	s.ipMutex.Lock()
	defer s.ipMutex.Unlock()
	s.CurrentSource = source
}

// SetLastCheck 更新最后检查时间
func (s *Server) SetLastCheck(t time.Time) {
	s.ipMutex.Lock()
	defer s.ipMutex.Unlock()
	s.LastCheckTime = t
}

// GetCurrentIP 获取当前 IP（线程安全）
func (s *Server) GetCurrentIP() string {
	s.ipMutex.RLock()
	defer s.ipMutex.RUnlock()
	return s.CurrentIP
}

// GetCurrentSource 获取当前 IP 来源
func (s *Server) GetCurrentSource() string {
	s.ipMutex.RLock()
	defer s.ipMutex.RUnlock()
	return s.CurrentSource
}

// GetLastCheck 获取最后检查时间
func (s *Server) GetLastCheck() time.Time {
	s.ipMutex.RLock()
	defer s.ipMutex.RUnlock()
	return s.LastCheckTime
}

// BroadcastIPChange 广播 IP 变化事件给所有 WebSocket 客户端
func (s *Server) BroadcastIPChange(newIP, source string) {
	if s.Hub != nil {
		s.Hub.Broadcast("ip_change", map[string]string{
			"ip":     newIP,
			"source": source,
		})
	}
}

// handleGetIP 返回纯文本 IP（返回系统监控的公网 IP）
func (s *Server) handleGetIP(c echo.Context) error {
	ip := s.GetCurrentIP()
	if ip == "" {
		return c.String(http.StatusServiceUnavailable, "IP unavailable\n")
	}
	return c.String(http.StatusOK, ip+"\n")
}

// handleGetIPJSON 返回 JSON 格式 IP
func (s *Server) handleGetIPJSON(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"ip": s.GetCurrentIP(),
	})
}



// handleGetConfig 获取配置（不脱敏，直接返回原始值）
func (s *Server) handleGetConfig(c echo.Context) error {
	// 直接返回当前配置副本（不脱敏）
	cfg := s.Config.Get()
	return c.JSON(http.StatusOK, cfg)
}

// handleUpdateConfig 更新配置并保存到文件
func (s *Server) handleUpdateConfig(c echo.Context) error {
	var newCfg config.AppConfig
	if err := c.Bind(&newCfg); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "配置格式错误: " + err.Error()})
	}

	// 直接使用前端传来的配置（不再需要回填脱敏值）

	// 在保存前验证完整配置
	if err := config.ValidateConfig(&newCfg); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "配置验证失败: " + err.Error()})
	}

	// 保存配置到数据库
	if err := config.SaveConfig(&newCfg, s.DB); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "保存配置失败: " + err.Error()})
	}

	// 更新内存中的配置（热更新）
	s.Config.Update(&newCfg)

	// 记录系统日志
	s.DB.AddErrorLog("info", "Configuration updated via Web UI")

	// 通知 monitoring loop 配置已变更（非阻塞）
	select {
	case s.ConfigUpdateChan <- struct{}{}:
	default:
		// 通道已满，说明已有挂起的变更信号，忽略
	}

	// 强制触发一次 DNS 更新（异步）
	// 确保存储了配置后立即尝试同步，解决"显示已同步但无记录"的问题
	go func() {
		// 稍微延迟确保配置完全应用
		time.Sleep(500 * time.Millisecond)
		
		ip := s.GetCurrentIP()
		// 如果还没获取到 IP，尝试获取一次
		if ip == "" && s.IPProvider != nil {
			fetchedIP, source, err := s.IPProvider.GetIP()
			if err == nil {
				ip = fetchedIP
				s.SetCurrentIP(ip)
				s.SetCurrentSource(source)
			}
		}
		
		if ip != "" && s.DNSUpdater != nil {
			fmt.Println("Configuration changed, triggering immediate DNS update...")
			// 强制更新（即使 IP 没变也检查记录是否存在）
			if err := s.DNSUpdater.UpdateIP(ip); err != nil {
				fmt.Printf("Immediate DNS update failed: %v\n", err)
				s.DB.AddErrorLog("error", "Post-config DNS update failed: "+err.Error())
			} else {
				s.DB.AddErrorLog("success", "DNS records synchronized after config update")
			}
		}
	}()

	return c.JSON(http.StatusOK, map[string]string{
		"message": "配置已保存，正在后台同步 DNS 记录",
	})
}

// handleExportConfig 导出当前配置为 YAML
func (s *Server) handleExportConfig(c echo.Context) error {
	// 直接从内存/数据库获取配置，而不是读取文件
	cfg := s.Config.Get()
	
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "生成配置 YAML 失败: " + err.Error()})
	}

	// 设置响应头，触发下载
	c.Response().Header().Set("Content-Disposition", "attachment; filename=config.yaml")
	c.Response().Header().Set("Content-Type", "application/x-yaml")
	return c.Blob(http.StatusOK, "application/x-yaml", data)
}

// handleImportConfig 导入配置文件
func (s *Server) handleImportConfig(c echo.Context) error {
	// 读取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "未找到上传文件"})
	}

	// 打开文件
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "打开文件失败"})
	}
	defer src.Close()

	// 读取内容
	data, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "读取文件失败"})
	}

	// 先尝试解析 YAML 验证格式
	var testCfg config.AppConfig
	if err := yaml.Unmarshal(data, &testCfg); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "配置格式错误: " + err.Error()})
	}

	// 验证通过，保存到数据库
	if err := config.SaveConfig(&testCfg, s.DB); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "保存配置失败: " + err.Error()})
	}

	// 更新内存中的配置
	s.Config.Update(&testCfg)

	return c.JSON(http.StatusOK, map[string]string{
		"message": "配置导入成功并已生效",
	})
}

// handleGetStatus 获取系统综合状态
func (s *Server) handleGetStatus(c echo.Context) error {
	cfg := s.Config.Get()
	
	// 获取 IP 历史（最近20条）
	ipHistory, _ := s.DB.GetRecentIPHistory(20)
	
	// 获取 DNS 更新记录（最近10条）
	dnsUpdates, _ := s.DB.GetRecentDNSUpdates(10)
	
	// 获取错误日志（最近5条）
	errorLogs, _ := s.DB.GetRecentErrorLogs(5)
	
	// 获取 IP 变化次数
	ipChangeCount, _ := s.DB.GetIPChangeCount()
	
	// 计算运行时长
	uptime := time.Since(s.StartTime)
	
	// 构建 DNS 记录状态（基于最近的更新记录）
	dnsStatus := make(map[string]interface{})
	for _, update := range dnsUpdates {
		key := fmt.Sprintf("%s|%s", update.AccountName, update.Domain)
		if _, exists := dnsStatus[key]; !exists {
			dnsStatus[key] = map[string]interface{}{
				"account":   update.AccountName,
				"domain":    update.Domain,
				"success":   update.Success,
				"ip":        update.IP,
				"timestamp": update.Timestamp,
				"error":     update.Error,
			}
		}
	}
	
	// 获取当前使用的 IP 来源
	// 逻辑：如果 currentSource 对应的 provider 仍然启用，使用它；否则使用第一个启用的 provider
	source := s.GetCurrentSource()
	firstEnabledSource := ""
	
	for _, p := range cfg.IPProviders {
		if p.Enabled {
			// 记录第一个启用的 provider 类型
			if firstEnabledSource == "" {
				firstEnabledSource = strings.ToUpper(p.Type)
			}
		}
	}
	
	// 如果当前 source 为空（尚未获取到 IP），使用第一个启用的作为展示占位
	if source == "" && firstEnabledSource != "" {
		source = firstEnabledSource
	}

	// 获取最后一次IP变更时间
	var lastChanged time.Time
	if len(ipHistory) > 0 {
		lastChanged = ipHistory[0].Timestamp
	} else {
		lastChanged = s.StartTime
	}

	// 获取最后一次检查时间（使用实际记录的检查时间）
	lastUpdated := s.GetLastCheck()
	if lastUpdated.IsZero() {
		// 如果还没检查过，回退到启动时间
		lastUpdated = s.StartTime
	}

	// 构建简化的DNS状态（去重，只显示每个域名的最新状态）
	dnsRecordMap := make(map[string]struct{
		domain  string
		account string
		success bool
	})
	// 默认为 false，只有当至少有一个成功记录时才设为 true（除非根本没配置 DNS）
	dnsSynced := false
	if len(cfg.CloudflareAccounts) == 0 {
		// 如果没配置账户，状态显示为 synced（避免报错），但记录数为 0
		dnsSynced = true
	} else if len(dnsUpdates) > 0 {
		// 如果有配置且有更新记录，初始设为 true，遇到失败则置 false
		dnsSynced = true
	}
	
	for _, update := range dnsUpdates {
		// 使用域名作为唯一键（同一域名只保留最新的记录）
		if _, exists := dnsRecordMap[update.Domain]; !exists {
			dnsRecordMap[update.Domain] = struct {
				domain  string
				account string
				success bool
			}{
				domain:  update.Domain,
				account: update.AccountName,
				success: update.Success,
			}
			if !update.Success {
				dnsSynced = false
			}
		}
	}
	
	// 将 map 转换为 slice
	dnsRecords := []string{}
	for _, record := range dnsRecordMap {
		recordStr := fmt.Sprintf("%s (%s)", record.domain, record.account)
		if !record.success {
			recordStr += " - failed"
		}
		dnsRecords = append(dnsRecords, recordStr)
	}

	// 获取检查统计数据（过去 24 小时）
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	ipCheckStats, _ := s.DB.GetCheckStats("ip", oneDayAgo)
	dnsCheckStats, _ := s.DB.GetCheckStats("dns", oneDayAgo)
	
	// 获取最近的检查日志
	recentChecks, _ := s.DB.GetRecentCheckLogs(10)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"current_ip":    s.GetCurrentIP(),
		"source":        source,
		"last_updated":  lastUpdated,
		"last_changed":  lastChanged,
		"uptime_seconds": int(uptime.Seconds()),
		"start_time":    s.StartTime,
		"ip_history":    ipHistory,
		"ip_change_count": ipChangeCount,
		"dns_updates":   dnsUpdates,
		"dns_status": map[string]interface{}{
			"synced":  dnsSynced,
			"records": dnsRecords,
		},
		"error_logs": errorLogs,
		"check_stats": map[string]interface{}{
			"ip":  ipCheckStats,
			"dns": dnsCheckStats,
		},
		"recent_checks": recentChecks,
		"config": map[string]interface{}{
			"dns_enabled": len(cfg.CloudflareAccounts) > 0,
			"accounts":    cfg.CloudflareAccounts,
			"intervals":   cfg.Intervals,
		},
	})
}

// handleGetHistoryStats 获取历史统计数据
func (s *Server) handleGetHistoryStats(c echo.Context) error {
	timeRange := c.QueryParam("range")
	if timeRange == "" {
		timeRange = "24h"
	}

	var start time.Time
	var groupBy string

	now := time.Now()

	switch timeRange {
	case "1h":
		start = now.Add(-1 * time.Hour)
		groupBy = "minute"
	case "24h":
		start = now.Add(-24 * time.Hour)
		groupBy = "hour"
	case "7d":
		start = now.AddDate(0, 0, -7)
		groupBy = "day"
	case "30d":
		start = now.AddDate(0, 0, -30)
		groupBy = "day"
	case "365d", "1y":
		start = now.AddDate(-1, 0, 0)
		groupBy = "month"
	case "all":
		start = time.Time{} // 从最开始
		groupBy = "month"
	default: // 默认 24h
		start = now.Add(-24 * time.Hour)
		groupBy = "hour"
	}

	stats, err := s.DB.GetIPHistoryStats(start, groupBy)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "查询统计失败: " + err.Error()})
	}

	dnsFailures, err := s.DB.GetDNSFailures(start)
	if err != nil {
		fmt.Printf("查询 DNS 失败记录失败: %v\n", err)
	}

	errorLogs, err := s.DB.GetErrorLogs(start)
	if err != nil {
		fmt.Printf("查询错误日志失败: %v\n", err)
	}
	
	ipHistory, err := s.DB.GetIPHistoryLogs(start)
	if err != nil {
		fmt.Printf("查询 IP 历史记录失败: %v\n", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"range":        timeRange,
		"group_by":     groupBy,
		"data":         stats,
		"dns_failures": dnsFailures,
		"error_logs":   errorLogs,
		"ip_history":   ipHistory,
	})
}

// handleTriggerDNSUpdate 手动触发 DNS 更新
func (s *Server) handleTriggerDNSUpdate(c echo.Context) error {
	// 获取当前 IP
	currentIP := s.GetCurrentIP()
	if currentIP == "" {
		// 如果当前 IP 为空，先获取一次
		ip, source, err := s.IPProvider.GetIP()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "获取当前 IP 失败: " + err.Error()})
		}
		currentIP = ip
		s.SetCurrentIP(currentIP)
		s.SetCurrentSource(source)
	}

	// 触发 DNS 更新
	if s.DNSUpdater != nil {
		go func() {
			if err := s.DNSUpdater.UpdateIP(currentIP); err != nil {
				fmt.Printf("DNS 更新失败: %v\n", err)
			}
		}()
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "DNS 更新已触发",
		"ip":      currentIP,
	})
}
