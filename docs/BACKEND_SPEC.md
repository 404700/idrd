# IDRD Backend Specification

## 项目概述

IDRD (IP & DNS Records Dashboard) 是一个自托管的动态 DNS 管理系统，用于监控公网 IP 变化并自动更新 Cloudflare DNS 记录。

---

## 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go 1.23+ |
| Web 框架 | Echo v4 |
| WebSocket | gorilla/websocket |
| 数据库 | SQLite 3 (嵌入式) |
| DNS 提供商 | Cloudflare API |
| 容器化 | Docker + Docker Compose |
| 配置存储 | SQLite (settings 表) |

---

## 核心功能模块

### 1. IP 检测模块 (`ip/`)

#### 1.1 STUN 提供商 (`stun.go`)
- 使用 STUN 协议获取公网 IP
- 支持配置 STUN 服务器地址
- 默认服务器: `stun.l.google.com:19302`
- **最小检查间隔**: 30 秒

#### 1.2 路由器 SSH 提供商 (`router.go`)
- 通过 SSH 连接路由器获取 WAN IP
- 支持 RouterOS (MikroTik) 和 OpenWrt
- 认证方式: 密码或 SSH 密钥
- 可指定网络接口名称
- **最小检查间隔**: 1 秒

#### 1.3 动态提供商 (`provider.go`)
- 根据配置自动选择 IP 提供商
- 支持多提供商回退
- 统一接口: `GetIP() (string, string, error)` 返回 IP 和来源

### 2. DNS 更新模块 (`dns/`)

#### 2.1 Cloudflare 更新器 (`cloudflare.go`)
- 使用 Cloudflare API v4
- 支持 API Token 认证
- 自动处理 Zone ID 查询
- A/AAAA 记录更新
- 内置重试机制 (3 次重试，指数退避)

### 3. 数据库模块 (`db/`)

#### 3.1 表结构

```sql
-- 配置存储（键值对）
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- IP 历史记录
CREATE TABLE ip_history (
    id INTEGER PRIMARY KEY,
    ip TEXT NOT NULL,
    type TEXT DEFAULT 'v4',
    source TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- DNS 更新记录
CREATE TABLE dns_updates (
    id INTEGER PRIMARY KEY,
    account_name TEXT,
    domain TEXT NOT NULL,
    ip TEXT NOT NULL,
    success INTEGER NOT NULL,
    error TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 错误日志
CREATE TABLE error_logs (
    id INTEGER PRIMARY KEY,
    type TEXT NOT NULL,
    message TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### 3.2 配置存储 (`config_store.go`)
- 配置以 JSON 格式存储在 `settings` 表
- 启动时从数据库加载配置
- 支持热更新，无需重启

### 4. 配置模块 (`config/`)

#### 4.1 配置验证 (`validation.go`)

| 参数 | 最小值 | 说明 |
|------|--------|------|
| `ip_check` | 1s | Router SSH 可设置 1 秒 |
| `dns_update` | 10s | DNS 更新最小 10 秒 |

#### 4.2 配置不脱敏
- **重要**: API 返回配置时**不脱敏**
- 密码、Token 等敏感字段直接返回原始值
- 前端输入框使用 `type="password"` 保护显示

### 5. HTTP 服务模块 (`server/`)

#### 5.1 API 端点

| 方法 | 路径 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/ip` | 获取当前 IP (JSON) | 必需 |
| GET | `/api/status` | 获取系统状态 | 必需 |
| GET | `/api/stats/history` | 获取历史统计 | 必需 |
| GET | `/api/config` | 获取配置 (不脱敏) | 必需 |
| POST | `/api/config` | 更新配置 | 必需 |
| GET | `/ws` | WebSocket 实时推送 | 无 |

#### 5.2 WebSocket 实时推送 (`websocket.go`)

```go
// 消息格式
type WSMessage struct {
    Type string      `json:"type"` // "ip_change"
    Data interface{} `json:"data"`
}

// IP 变化推送
{
    "type": "ip_change",
    "data": {
        "ip": "1.2.3.4",
        "source": "ROUTER_SSH"
    }
}
```

- **Hub 模式**: 管理所有 WebSocket 连接
- **自动重连**: 客户端断开后 5 秒重连
- **广播**: IP 变化时推送到所有客户端

#### 5.3 认证中间件 (`middleware.go`)

```go
// 认证检查顺序:
// 1. X-API-Key 请求头
// 2. ?key= 查询参数
// 3. 客户端 IP 是否在 trusted_subnets
```

### 6. 主程序 (`cmd/idrd/`)

#### 6.1 启动流程
1. 解析环境变量 (`DB_PATH`, `CONFIG_PATH`)
2. 初始化数据库
3. 从数据库加载配置 (或生成默认配置)
4. 创建 IP 提供商
5. 创建 DNS 更新器
6. 创建 HTTP 服务 (含 WebSocket Hub)
7. 启动 IP 监控协程

#### 6.2 初始化模式 (`--init`)
- 生成默认配置到数据库
- 生成 docker-compose.yml
- 完成后自动退出

#### 6.3 IP 监控循环
```go
for {
    currentIP, source := provider.GetIP()
    srv.SetCurrentIP(currentIP)
    srv.SetCurrentSource(source)
    
    if currentIP != lastIP {
        db.AddIPHistory(currentIP, source)
        updater.UpdateIP(currentIP)
        // WebSocket 广播
        srv.BroadcastIPChange(currentIP, source)
        lastIP = currentIP
    }
    
    time.Sleep(checkInterval)
}
```

---

## Docker 部署

### 环境变量

| 变量 | 默认值 | 描述 |
|------|--------|------|
| `DB_PATH` | `/data/idrd.db` | 数据库路径 |
| `TZ` | `Asia/Shanghai` | 时区 |

---

## 安全考量

1. **API Key**
   - 256 位熵 (32 字节)
   - URL 安全编码
   - 首次生成后需用户保存

2. **可信子网**
   - 内网 IP 可跳过认证
   - 支持 CIDR 格式

3. **敏感信息**
   - 配置 API **不脱敏**，直接返回原始值
   - 前端负责隐藏显示 (`type="password"`)

4. **SSH 连接**
   - 支持密钥认证
   - 默认跳过主机密钥验证
