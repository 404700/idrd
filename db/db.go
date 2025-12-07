package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// IPHistory IP 变化历史记录
type IPHistory struct {
	ID        int64     `json:"id"`
	IP        string    `json:"ip"`
	IPVersion string    `json:"ip_version"` // "v4" or "v6"
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

// DNSUpdate DNS 更新记录
type DNSUpdate struct {
	ID          int64     `json:"id"`
	AccountName string    `json:"account_name"`
	IP          string    `json:"ip"`
	RecordType  string    `json:"record_type"` // "A" or "AAAA"
	Domain      string    `json:"domain"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// ErrorLog 错误日志
type ErrorLog struct {
	ID        int64     `json:"id"`
	Level     string    `json:"level"` // "error", "warning", "info"
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// CheckLog 检查日志（记录每次 IP/DNS 检查）
type CheckLog struct {
	ID         int64     `json:"id"`
	CheckType  string    `json:"check_type"` // "ip", "dns"
	Success    bool      `json:"success"`
	Result     string    `json:"result,omitempty"`     // IP 地址或 DNS 记录
	DurationMs int       `json:"duration_ms,omitempty"` // 检查耗时（毫秒）
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// DB 数据库连接
type DB struct {
	conn *sql.DB
}

// New 创建新的数据库连接
func New(dbPath string) (*DB, error) {
	// 使用 WAL 模式和超时设置
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_timeout=5000&_parseTime=true&_loc=Local")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 配置连接池参数
	conn.SetMaxOpenConns(25)                 // 最大打开连接数
	conn.SetMaxIdleConns(5)                  // 最大空闲连接数
	conn.SetConnMaxLifetime(5 * time.Minute) // 连接最大生存时间

	// 验证数据库连接
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("数据库连接验证失败: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.initialize(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

// initialize 初始化数据库表
func (db *DB) initialize() error {
	schema := `
	CREATE TABLE IF NOT EXISTS ip_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ip TEXT NOT NULL,
		ip_version TEXT DEFAULT 'v4',
		source TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_ip_history_timestamp ON ip_history(timestamp DESC);

	CREATE TABLE IF NOT EXISTS dns_updates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_name TEXT DEFAULT '',
		ip TEXT NOT NULL,
		record_type TEXT DEFAULT 'A',
		domain TEXT NOT NULL,
		success BOOLEAN NOT NULL,
		error TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_dns_updates_timestamp ON dns_updates(timestamp DESC);

	CREATE TABLE IF NOT EXISTS error_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_error_logs_timestamp ON error_logs(timestamp DESC);

	CREATE TABLE IF NOT EXISTS check_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		check_type TEXT NOT NULL,
		success BOOLEAN NOT NULL,
		result TEXT,
		duration_ms INTEGER,
		error TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_check_logs_timestamp ON check_logs(timestamp DESC);
	`

	// 新增配置表
	schema += `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ip_providers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		properties TEXT NOT NULL, -- JSON
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS cloudflare_accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		api_token TEXT NOT NULL,
		zones TEXT NOT NULL, -- JSON
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}
	
	// 尝试添加新列（如果表已存在）
	// SQLite 不支持 IF NOT EXISTS 添加列，所以忽略错误
	db.conn.Exec("ALTER TABLE ip_history ADD COLUMN ip_version TEXT DEFAULT 'v4'")
	db.conn.Exec("ALTER TABLE dns_updates ADD COLUMN account_name TEXT DEFAULT ''")
	db.conn.Exec("ALTER TABLE dns_updates ADD COLUMN record_type TEXT DEFAULT 'A'")
	
	return nil
}

// AddIPHistory 添加 IP 历史记录
func (db *DB) AddIPHistory(ip, ipVersion, source string) error {
	_, err := db.conn.Exec(
		"INSERT INTO ip_history (ip, ip_version, source, timestamp) VALUES (?, ?, ?, ?)",
		ip, ipVersion, source, time.Now(),
	)
	return err
}

// GetRecentIPHistory 获取最近的 IP 历史记录
func (db *DB) GetRecentIPHistory(limit int) ([]IPHistory, error) {
	rows, err := db.conn.Query(
		"SELECT id, ip, ip_version, source, timestamp FROM ip_history ORDER BY timestamp DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []IPHistory
	for rows.Next() {
		var h IPHistory
		if err := rows.Scan(&h.ID, &h.IP, &h.IPVersion, &h.Source, &h.Timestamp); err != nil {
			return nil, err
		}
		history = append(history, h)
	}

	return history, nil
}

// AddDNSUpdate 添加 DNS 更新记录
func (db *DB) AddDNSUpdate(accountName, ip, recordType, domain string, success bool, errorMsg string) error {
	_, err := db.conn.Exec(
		"INSERT INTO dns_updates (account_name, ip, record_type, domain, success, error, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)",
		accountName, ip, recordType, domain, success, errorMsg, time.Now(),
	)
	return err
}

// GetRecentDNSUpdates 获取最近的 DNS 更新记录
func (db *DB) GetRecentDNSUpdates(limit int) ([]DNSUpdate, error) {
	rows, err := db.conn.Query(
		"SELECT id, account_name, ip, record_type, domain, success, error, timestamp FROM dns_updates ORDER BY timestamp DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var updates []DNSUpdate
	for rows.Next() {
		var u DNSUpdate
		var errMsg sql.NullString
		if err := rows.Scan(&u.ID, &u.AccountName, &u.IP, &u.RecordType, &u.Domain, &u.Success, &errMsg, &u.Timestamp); err != nil {
			return nil, err
		}
		if errMsg.Valid {
			u.Error = errMsg.String
		}
		updates = append(updates, u)
	}

	return updates, nil
}

// AddErrorLog 添加错误日志
func (db *DB) AddErrorLog(level, message string) error {
	_, err := db.conn.Exec(
		"INSERT INTO error_logs (level, message, timestamp) VALUES (?, ?, ?)",
		level, message, time.Now(),
	)
	return err
}

// GetRecentErrorLogs 获取最近的错误日志
func (db *DB) GetRecentErrorLogs(limit int) ([]ErrorLog, error) {
	rows, err := db.conn.Query(
		"SELECT id, level, message, timestamp FROM error_logs ORDER BY timestamp DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ErrorLog
	for rows.Next() {
		var l ErrorLog
		if err := rows.Scan(&l.ID, &l.Level, &l.Message, &l.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}

	return logs, nil
}

// GetIPChangeCount 获取 IP 变化次数（24小时内）
func (db *DB) GetIPChangeCount() (int, error) {
	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(DISTINCT ip) FROM ip_history WHERE timestamp > datetime('now', '-24 hours')",
	).Scan(&count)
	return count, err
}

// IPHistoryStat IP 历史统计
type IPHistoryStat struct {
	Time  string `json:"time"`
	Count int    `json:"count"`
}

// GetIPHistoryStats 获取 IP 历史统计数据
func (db *DB) GetIPHistoryStats(start time.Time, groupBy string) ([]IPHistoryStat, error) {
	// SQLite strftime 格式映射
	var format string
	switch groupBy {
	case "minute":
		format = "%Y-%m-%d %H:%M"
	case "hour":
		format = "%Y-%m-%d %H:00"
	case "day":
		format = "%Y-%m-%d"
	case "week":
		format = "%Y-%W"
	case "month":
		format = "%Y-%m"
	case "year":
		format = "%Y"
	default:
		return nil, fmt.Errorf("不支持的分组方式: %s", groupBy)
	}

	query := fmt.Sprintf(`
		SELECT strftime('%s', timestamp) as time_bucket, COUNT(*) as count
		FROM ip_history
		WHERE timestamp >= ?
		GROUP BY time_bucket
		ORDER BY time_bucket ASC
	`, format)

	rows, err := db.conn.Query(query, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []IPHistoryStat
	for rows.Next() {
		var s IPHistoryStat
		if err := rows.Scan(&s.Time, &s.Count); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// GetDNSFailureStats 获取 DNS 更新失败统计数据
func (db *DB) GetDNSFailureStats(start time.Time, groupBy string) ([]IPHistoryStat, error) {
	var format string
	switch groupBy {
	case "minute":
		format = "%Y-%m-%d %H:%M"
	case "hour":
		format = "%Y-%m-%d %H:00"
	case "day":
		format = "%Y-%m-%d"
	case "week":
		format = "%Y-%W"
	case "month":
		format = "%Y-%m"
	case "year":
		format = "%Y"
	default:
		return nil, fmt.Errorf("不支持的分组方式: %s", groupBy)
	}

	query := fmt.Sprintf(`
		SELECT strftime('%s', timestamp) as time_bucket, COUNT(*) as count
		FROM dns_updates
		WHERE timestamp >= ? AND success = 0
		GROUP BY time_bucket
		ORDER BY time_bucket ASC
	`, format)

	rows, err := db.conn.Query(query, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []IPHistoryStat
	for rows.Next() {
		var s IPHistoryStat
		if err := rows.Scan(&s.Time, &s.Count); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// GetDNSFailures 获取详细的 DNS 失败记录
func (db *DB) GetDNSFailures(start time.Time) ([]DNSUpdate, error) {
	rows, err := db.conn.Query(
		"SELECT id, account_name, ip, record_type, domain, success, error, timestamp FROM dns_updates WHERE timestamp >= ? AND success = 0 ORDER BY timestamp DESC",
		start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var updates []DNSUpdate
	for rows.Next() {
		var u DNSUpdate
		var errMsg sql.NullString
		if err := rows.Scan(&u.ID, &u.AccountName, &u.IP, &u.RecordType, &u.Domain, &u.Success, &errMsg, &u.Timestamp); err != nil {
			return nil, err
		}
		if errMsg.Valid {
			u.Error = errMsg.String
		}
		updates = append(updates, u)
	}
	return updates, nil
}

// GetErrorLogs 获取详细的错误日志
func (db *DB) GetErrorLogs(start time.Time) ([]ErrorLog, error) {
	rows, err := db.conn.Query(
		"SELECT id, level, message, timestamp FROM error_logs WHERE timestamp >= ? ORDER BY timestamp DESC",
		start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ErrorLog
	for rows.Next() {
		var l ErrorLog
		if err := rows.Scan(&l.ID, &l.Level, &l.Message, &l.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// GetIPHistoryLogs 获取详细的 IP 变更历史
func (db *DB) GetIPHistoryLogs(start time.Time) ([]IPHistory, error) {
	rows, err := db.conn.Query(
		"SELECT id, ip, ip_version, source, timestamp FROM ip_history WHERE timestamp >= ? ORDER BY timestamp DESC",
		start,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []IPHistory
	for rows.Next() {
		var h IPHistory
		if err := rows.Scan(&h.ID, &h.IP, &h.IPVersion, &h.Source, &h.Timestamp); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

// AddCheckLog 添加检查日志
func (db *DB) AddCheckLog(checkType string, success bool, result string, durationMs int, errorMsg string) error {
	_, err := db.conn.Exec(
		"INSERT INTO check_logs (check_type, success, result, duration_ms, error, timestamp) VALUES (?, ?, ?, ?, ?, ?)",
		checkType, success, result, durationMs, errorMsg, time.Now(),
	)
	return err
}

// GetRecentCheckLogs 获取最近的检查日志
func (db *DB) GetRecentCheckLogs(limit int) ([]CheckLog, error) {
	rows, err := db.conn.Query(
		"SELECT id, check_type, success, result, duration_ms, error, timestamp FROM check_logs ORDER BY timestamp DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []CheckLog
	for rows.Next() {
		var l CheckLog
		var result, errMsg sql.NullString
		var durationMs sql.NullInt64
		if err := rows.Scan(&l.ID, &l.CheckType, &l.Success, &result, &durationMs, &errMsg, &l.Timestamp); err != nil {
			return nil, err
		}
		if result.Valid {
			l.Result = result.String
		}
		if durationMs.Valid {
			l.DurationMs = int(durationMs.Int64)
		}
		if errMsg.Valid {
			l.Error = errMsg.String
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// CheckStats 检查统计数据
type CheckStats struct {
	TotalChecks    int     `json:"total_checks"`
	SuccessCount   int     `json:"success_count"`
	FailCount      int     `json:"fail_count"`
	SuccessRate    float64 `json:"success_rate"`
	AvgDurationMs  float64 `json:"avg_duration_ms"`
}

// GetCheckStats 获取检查统计数据
func (db *DB) GetCheckStats(checkType string, start time.Time) (*CheckStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) as fail_count,
			AVG(duration_ms) as avg_duration
		FROM check_logs 
		WHERE check_type = ? AND timestamp >= ?
	`
	
	var stats CheckStats
	var avgDuration sql.NullFloat64
	err := db.conn.QueryRow(query, checkType, start).Scan(
		&stats.TotalChecks, &stats.SuccessCount, &stats.FailCount, &avgDuration,
	)
	if err != nil {
		return nil, err
	}
	
	if avgDuration.Valid {
		stats.AvgDurationMs = avgDuration.Float64
	}
	if stats.TotalChecks > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalChecks) * 100
	}
	
	return &stats, nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.conn.Close()
}
