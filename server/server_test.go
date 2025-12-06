package server

import (
	"encoding/json"
	"idrd/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestHandleGetIP(t *testing.T) {
	// 创建测试服务器
	e := echo.New()
	safeCfg := config.NewSafeConfig(config.DefaultConfig())
	
	s := &Server{
		Echo:       e,
		Config:     safeCfg,
		CurrentIP:  "203.0.113.1",
		CurrentSource: "TEST",
		StartTime:  time.Now(),
	}

	// 创建请求
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// 执行处理函数
	if err := s.handleGetIP(c); err != nil {
		t.Fatalf("handleGetIP returned error: %v", err)
	}

	// 验证响应
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	expected := "203.0.113.1\n"
	if rec.Body.String() != expected {
		t.Errorf("Expected body %q, got %q", expected, rec.Body.String())
	}
}

func TestHandleGetIP_Empty(t *testing.T) {
	e := echo.New()
	safeCfg := config.NewSafeConfig(config.DefaultConfig())
	
	s := &Server{
		Echo:       e,
		Config:     safeCfg,
		CurrentIP:  "", // 空 IP
		StartTime:  time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetIP(c); err != nil {
		t.Fatalf("handleGetIP returned error: %v", err)
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestHandleGetIPJSON(t *testing.T) {
	e := echo.New()
	safeCfg := config.NewSafeConfig(config.DefaultConfig())
	
	s := &Server{
		Echo:       e,
		Config:     safeCfg,
		CurrentIP:  "10.0.0.1",
		StartTime:  time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ip", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetIPJSON(c); err != nil {
		t.Fatalf("handleGetIPJSON returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if response["ip"] != "10.0.0.1" {
		t.Errorf("Expected IP 10.0.0.1, got %s", response["ip"])
	}
}

func TestSetAndGetCurrentIP(t *testing.T) {
	s := &Server{}

	// 测试设置和获取 IP
	s.SetCurrentIP("1.2.3.4")
	if got := s.GetCurrentIP(); got != "1.2.3.4" {
		t.Errorf("Expected IP 1.2.3.4, got %s", got)
	}

	// 测试设置和获取 Source
	s.SetCurrentSource("STUN")
	if got := s.GetCurrentSource(); got != "STUN" {
		t.Errorf("Expected source STUN, got %s", got)
	}
}

func TestSetCurrentIP_Concurrent(t *testing.T) {
	s := &Server{}

	// 并发写入
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(n int) {
			s.SetCurrentIP("192.168.1.1")
			_ = s.GetCurrentIP()
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 100; i++ {
		<-done
	}

	// 验证最终值
	ip := s.GetCurrentIP()
	if ip != "192.168.1.1" {
		t.Errorf("Expected IP 192.168.1.1, got %s", ip)
	}
}

func TestTrustedSubnetMiddleware(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.TrustedSubnets = []string{"127.0.0.1/32", "192.168.1.0/24"}
	safeCfg := config.NewSafeConfig(cfg)

	e := echo.New()
	mw := TrustedSubnetMiddleware(safeCfg)

	tests := []struct {
		name       string
		clientIP   string
		expectTrusted bool
	}{
		{"localhost trusted", "127.0.0.1", true},
		{"local network trusted", "192.168.1.100", true},
		{"external not trusted", "8.8.8.8", false},
		{"different subnet not trusted", "192.168.2.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.clientIP + ":12345"
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := mw(func(c echo.Context) error {
				trusted, _ := c.Get("trusted").(bool)
				if trusted != tt.expectTrusted {
					t.Errorf("Expected trusted=%v, got %v", tt.expectTrusted, trusted)
				}
				return nil
			})

			if err := handler(c); err != nil {
				t.Errorf("Middleware returned error: %v", err)
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.APIKey = "test-api-key-12345678"
	safeCfg := config.NewSafeConfig(cfg)

	e := echo.New()
	mw := AuthMiddleware(safeCfg)

	tests := []struct {
		name       string
		apiKey     string
		trusted    bool
		expectPass bool
	}{
		{"valid key", "test-api-key-12345678", false, true},
		{"invalid key", "wrong-key", false, false},
		{"no key", "", false, false},
		{"trusted bypasses auth", "", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			if tt.trusted {
				c.Set("trusted", true)
			}

			handlerCalled := false
			handler := mw(func(c echo.Context) error {
				handlerCalled = true
				return nil
			})

			err := handler(c)
			
			if tt.expectPass {
				if err != nil {
					t.Errorf("Expected pass but got error: %v", err)
				}
				if !handlerCalled {
					t.Error("Handler was not called")
				}
			} else {
				if err == nil {
					t.Error("Expected error but got none")
				}
			}
		})
	}
}

func TestAuthMiddleware_QueryParam(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.APIKey = "query-test-key-12345"
	safeCfg := config.NewSafeConfig(cfg)

	e := echo.New()
	mw := AuthMiddleware(safeCfg)

	// 使用查询参数传递 API Key
	req := httptest.NewRequest(http.MethodGet, "/?key=query-test-key-12345", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	handler := mw(func(c echo.Context) error {
		handlerCalled = true
		return nil
	})

	if err := handler(c); err != nil {
		t.Errorf("Expected pass but got error: %v", err)
	}
	if !handlerCalled {
		t.Error("Handler was not called")
	}
}

func TestHandleGetConfig_Sanitization(t *testing.T) {
	e := echo.New()
	
	cfg := config.DefaultConfig()
	cfg.Server.APIKey = "secret-api-key-1234567890"
	cfg.CloudflareAccounts = []config.CloudflareAccount{
		{
			Name:     "Test",
			APIToken: "secret-cloudflare-token",
			Zones:    []config.Zone{{ZoneName: "example.com", Records: []string{"@"}}},
		},
	}
	cfg.IPProviders = []config.IPProviderConfig{
		{
			Type:    "router_ssh",
			Enabled: true,
			Properties: map[string]string{
				"host":     "192.168.1.1",
				"password": "secret-password",
				"key":      "secret-private-key",
			},
		},
	}
	safeCfg := config.NewSafeConfig(cfg)

	s := &Server{
		Echo:      e,
		Config:    safeCfg,
		StartTime: time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := s.handleGetConfig(c); err != nil {
		t.Fatalf("handleGetConfig returned error: %v", err)
	}

	body := rec.Body.String()

	// 验证敏感信息已被脱敏
	if strings.Contains(body, "secret-api-key") {
		t.Error("API key was not sanitized")
	}
	if strings.Contains(body, "secret-cloudflare-token") {
		t.Error("Cloudflare token was not sanitized")
	}
	if strings.Contains(body, "secret-password") {
		t.Error("SSH password was not sanitized")
	}
	if strings.Contains(body, "secret-private-key") {
		t.Error("SSH key was not sanitized")
	}

	// 验证脱敏值存在
	if !strings.Contains(body, "***") {
		t.Error("Expected sanitized values (***) not found")
	}
}
