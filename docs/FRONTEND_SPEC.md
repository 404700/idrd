# IDRD Frontend Specification

## 项目概述

IDRD 前端是一个现代化的单页应用 (SPA)，提供实时 IP 监控、DNS 状态展示和系统配置管理界面。

---

## 技术栈

| 组件 | 技术 | 版本 |
|------|------|------|
| 框架 | React | 19.x |
| 构建工具 | Vite | 6.x |
| 语言 | TypeScript | 5.x |
| 图表 | Recharts | 2.x |
| 动画 | Framer Motion | 11.x |
| 图标 | Lucide React | - |
| 样式 | Vanilla CSS + CSS Variables | - |

---

## 核心页面

### 1. Dashboard (首页)

#### 1.1 实时更新机制

**WebSocket 实时推送** (优先):
```typescript
useEffect(() => {
  const ws = new WebSocket(`${wsProtocol}//${host}/ws`);
  
  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (msg.type === 'ip_change') {
      fetchData(); // 立即刷新
    }
  };
  
  ws.onclose = () => {
    setTimeout(connectWebSocket, 5000); // 自动重连
  };
}, []);
```

**轮询降级** (5 秒):
- WebSocket 不可用时自动降级到 5 秒轮询

#### 1.2 实时运行时间

使用 `clientStartTime` 实现秒级更新:
```typescript
const [now, setNow] = useState(new Date());
const [clientStartTime, setClientStartTime] = useState<Date | null>(null);

// 每秒更新本地时钟
useEffect(() => {
  const clock = setInterval(() => setNow(new Date()), 1000);
  return () => clearInterval(clock);
}, []);

// 计算运行时间
const uptime = clientStartTime 
  ? Math.floor((now.getTime() - clientStartTime.getTime()) / 1000)
  : status?.uptime_seconds || 0;
```

### 2. Config (配置页面)

#### 2.1 全局保存按钮

- **只有一个**"保存更改"按钮在页面右上角
- 移除了各板块独立的保存按钮
- 保存时提交整个配置对象

#### 2.2 无脱敏显示

- 配置加载后，密码/Token 显示**原始值**
- 前端使用 `type="password"` 隐藏输入
- 保存时直接发送当前值，无需回填逻辑

```typescript
<StyledInput
  type="password"
  value={account.api_token}
  onChange={e => onChange({ ...account, api_token: e.target.value })}
/>
```

---

## API 服务

### services/api.ts

```typescript
const API_BASE = '/api';

const getHeaders = () => ({
  'X-API-Key': localStorage.getItem('idrd_api_key') || ''
});

export const api = {
  getStatus: async (): Promise<StatusResponse> => {
    const res = await fetch(`${API_BASE}/status`, { headers: getHeaders() });
    if (res.status === 401) throw new Error('UNAUTHORIZED');
    return res.json();
  },

  getStats: async (range: string): Promise<StatsResponse> => {
    const res = await fetch(`${API_BASE}/stats/history?range=${range}`, { 
      headers: getHeaders() 
    });
    return res.json();
  },

  getConfig: async (): Promise<Config> => {
    const res = await fetch(`${API_BASE}/config`, { headers: getHeaders() });
    return res.json();
  },

  saveConfig: async (config: Config): Promise<void> => {
    const res = await fetch(`${API_BASE}/config`, {
      method: 'POST',
      headers: { ...getHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(config)
    });
    if (!res.ok) {
      const data = await res.json();
      throw new Error(data.error || 'Save failed');
    }
  }
};
```

---

## TypeScript 类型定义

### types.ts

```typescript
interface StatusResponse {
  current_ip: string;
  source: string;
  last_updated: string;
  last_changed: string;
  start_time: string;
  uptime_seconds: number;
  ip_change_count: number;
  dns_status: {
    synced: boolean;
    records: DNSRecord[];
  };
  check_stats: {
    ip: CheckStats;
    dns: CheckStats;
  };
}

interface Config {
  server: {
    port: number;
    api_key: string;
    trusted_subnets: string[];
  };
  intervals: {
    ip_check: string;
    dns_update: string;
  };
  ip_providers: IpProvider[];
  cloudflare_accounts: CloudflareAccount[];
}

interface IpProvider {
  type: 'stun' | 'router_ssh' | 'http' | 'interface';
  enabled: boolean;
  properties: Record<string, string>;
}

interface CloudflareAccount {
  name: string;
  api_token: string;
  zones: Zone[];
}
```

---

## UI/UX 设计规范

### 1. 主题系统

支持深色/浅色/系统自动三种模式:
```css
:root {
  --bg-primary: #0f172a;
  --bg-surface: #1e293b;
  --text-content: #f8fafc;
  --text-muted: #94a3b8;
  --primary: #3b82f6;
}

.light {
  --bg-primary: #f8fafc;
  --bg-surface: #ffffff;
  --text-content: #1e293b;
}
```

### 2. 国际化

支持中文 (zh) 和英文 (en):
- 默认根据浏览器语言自动选择
- 可手动切换

### 3. 响应式设计

| 断点 | 宽度 | 布局 |
|------|------|------|
| sm | < 640px | 单列，侧边栏收起 |
| md | 640-1024px | 两列 |
| lg | > 1024px | 完整布局 |

### 4. 全屏监控模式

- 点击 "Monitor Mode" 进入全屏
- 隐藏侧边栏，只显示核心数据卡片
- ESC 或按钮退出

---

## 构建配置

### vite.config.ts

```typescript
export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: 'dist',
    assetsDir: 'assets'
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true
      }
    }
  }
});
```
