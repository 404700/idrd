//go:build cgo
// +build cgo

package db

import (
	"os"
	"testing"
	"time"
)


func TestNew(t *testing.T) {
	// 创建临时数据库文件
	tmpFile, err := os.CreateTemp("", "test_db_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 测试数据库初始化
	db, err := New(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// 验证数据库连接
	if db.conn == nil {
		t.Fatal("Database connection is nil")
	}
}

func TestIPHistory(t *testing.T) {
	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "test_db_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := New(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// 测试添加 IP 历史记录
	testIP := "192.168.1.100"
	testSource := "STUN"
	if err := db.AddIPHistory(testIP, "v4", testSource); err != nil {
		t.Fatalf("Failed to add IP history: %v", err)
	}

	// 测试获取最近的 IP 历史记录
	history, err := db.GetRecentIPHistory(10)
	if err != nil {
		t.Fatalf("Failed to get recent IP history: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}

	if history[0].IP != testIP {
		t.Errorf("Expected IP %s, got %s", testIP, history[0].IP)
	}

	if history[0].Source != testSource {
		t.Errorf("Expected source %s, got %s", testSource, history[0].Source)
	}

	if history[0].IPVersion != "v4" {
		t.Errorf("Expected IP version v4, got %s", history[0].IPVersion)
	}
}

func TestDNSUpdate(t *testing.T) {
	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "test_db_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := New(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// 测试添加成功的 DNS 更新记录
	if err := db.AddDNSUpdate("TestAccount", "1.2.3.4", "A", "test.example.com", true, ""); err != nil {
		t.Fatalf("Failed to add DNS update: %v", err)
	}

	// 测试添加失败的 DNS 更新记录
	if err := db.AddDNSUpdate("TestAccount", "1.2.3.4", "A", "fail.example.com", false, "API error"); err != nil {
		t.Fatalf("Failed to add DNS update: %v", err)
	}

	// 测试获取最近的 DNS 更新记录
	updates, err := db.GetRecentDNSUpdates(10)
	if err != nil {
		t.Fatalf("Failed to get recent DNS updates: %v", err)
	}

	if len(updates) != 2 {
		t.Fatalf("Expected 2 DNS update records, got %d", len(updates))
	}

	// 验证失败记录（最近的在前）
	if updates[0].Success != false {
		t.Error("Expected first record to be failure")
	}
	if updates[0].Error != "API error" {
		t.Errorf("Expected error message 'API error', got '%s'", updates[0].Error)
	}
}

func TestErrorLog(t *testing.T) {
	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "test_db_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := New(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// 测试添加错误日志
	if err := db.AddErrorLog("error", "Test error message"); err != nil {
		t.Fatalf("Failed to add error log: %v", err)
	}

	if err := db.AddErrorLog("warning", "Test warning message"); err != nil {
		t.Fatalf("Failed to add error log: %v", err)
	}

	// 测试获取最近的错误日志
	logs, err := db.GetRecentErrorLogs(10)
	if err != nil {
		t.Fatalf("Failed to get recent error logs: %v", err)
	}

	if len(logs) != 2 {
		t.Fatalf("Expected 2 error logs, got %d", len(logs))
	}

	// 验证最近的记录
	if logs[0].Level != "warning" {
		t.Errorf("Expected level 'warning', got '%s'", logs[0].Level)
	}
}

func TestIPChangeCount(t *testing.T) {
	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "test_db_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := New(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// 添加几个不同的 IP
	ips := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "1.1.1.1"} // 4 条记录，3 个唯一 IP
	for _, ip := range ips {
		if err := db.AddIPHistory(ip, "v4", "TEST"); err != nil {
			t.Fatalf("Failed to add IP history: %v", err)
		}
	}

	count, err := db.GetIPChangeCount()
	if err != nil {
		t.Fatalf("Failed to get IP change count: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 unique IPs, got %d", count)
	}
}

func TestIPHistoryStats(t *testing.T) {
	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "test_db_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := New(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// 添加测试数据
	for i := 0; i < 5; i++ {
		if err := db.AddIPHistory("1.1.1.1", "v4", "TEST"); err != nil {
			t.Fatalf("Failed to add IP history: %v", err)
		}
	}

	// 测试按小时分组统计
	start := time.Now().Add(-1 * time.Hour)
	stats, err := db.GetIPHistoryStats(start, "hour")
	if err != nil {
		t.Fatalf("Failed to get IP history stats: %v", err)
	}

	if len(stats) == 0 {
		t.Fatal("Expected at least one stat entry")
	}

	// 验证总数
	total := 0
	for _, s := range stats {
		total += s.Count
	}
	if total != 5 {
		t.Errorf("Expected total count 5, got %d", total)
	}
}

func TestGetDNSFailures(t *testing.T) {
	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "test_db_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := New(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// 添加成功和失败的记录
	db.AddDNSUpdate("Test", "1.1.1.1", "A", "success.com", true, "")
	db.AddDNSUpdate("Test", "1.1.1.1", "A", "fail1.com", false, "Error 1")
	db.AddDNSUpdate("Test", "1.1.1.1", "A", "fail2.com", false, "Error 2")

	// 获取失败记录
	start := time.Now().Add(-1 * time.Hour)
	failures, err := db.GetDNSFailures(start)
	if err != nil {
		t.Fatalf("Failed to get DNS failures: %v", err)
	}

	if len(failures) != 2 {
		t.Errorf("Expected 2 failures, got %d", len(failures))
	}

	// 验证只返回失败记录
	for _, f := range failures {
		if f.Success {
			t.Error("Expected only failure records")
		}
	}
}
