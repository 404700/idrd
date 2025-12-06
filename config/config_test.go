package config

import (
	"os"
	"sync"
	"testing"
)

func TestSafeConfig_Concurrency(t *testing.T) {
	initialCfg := DefaultConfig()
	safeCfg := NewSafeConfig(initialCfg)

	var wg sync.WaitGroup
	// 模拟并发读取
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = safeCfg.Get()
		}()
	}

	// 模拟并发写入
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newCfg := DefaultConfig()
			newCfg.Server.Port = 9090
			safeCfg.Update(newCfg)
		}()
	}

	wg.Wait()

	// 验证最终状态是否一致（不会 panic 且能读取）
	cfg := safeCfg.Get()
	if cfg.Server.Port != 8080 && cfg.Server.Port != 9090 {
		t.Errorf("Unexpected port value: %d", cfg.Server.Port)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config_test_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	cfg := DefaultConfig()
	cfg.Server.Port = 12345
	// 使用新的 IPProviders 结构
	cfg.IPProviders = []IPProviderConfig{
		{
			Type:    "stun",
			Enabled: true,
			Properties: map[string]string{
				"server": "stun.test.com:19302",
			},
		},
	}

	// 测试保存
	if err := SaveConfig(cfg, tmpFile.Name()); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// 测试加载
	loadedCfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loadedCfg.Server.Port != 12345 {
		t.Errorf("Expected port 12345, got %d", loadedCfg.Server.Port)
	}
	if len(loadedCfg.IPProviders) != 1 {
		t.Errorf("Expected 1 IP provider, got %d", len(loadedCfg.IPProviders))
	}
	if loadedCfg.IPProviders[0].Properties["server"] != "stun.test.com:19302" {
		t.Errorf("Expected server 'stun.test.com:19302', got '%s'", loadedCfg.IPProviders[0].Properties["server"])
	}
}

func TestGenerateRandomKey(t *testing.T) {
	key, err := GenerateRandomKey(32)
	if err != nil {
		t.Fatalf("GenerateRandomKey failed: %v", err)
	}
	// GenerateRandomKey(32) 返回 32 字节的 base64 RawURL 编码，实际长度为 43 字符
	// base64 编码：每 3 字节变 4 字符，32 字节 = ceil(32/3)*4 = 44 字符（含 padding）
	// RawURLEncoding 去掉 padding，所以是 43 字符
	if len(key) != 43 {
		t.Errorf("Expected key length 43 (base64 RawURL of 32 bytes), got %d", len(key))
	}
}
