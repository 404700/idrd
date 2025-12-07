import { Config, StatusResponse, StatsResponse, EventLog } from '../types';

const API_BASE = '/api';

const getHeaders = () => {
  const key = localStorage.getItem('idrd_api_key');
  return {
    'Content-Type': 'application/json',
    ...(key ? { 'X-API-Key': key } : {}),
  };
};

// --- Mock Data Generators ---

const generateMockStatus = (): StatusResponse => ({
  current_ip: '203.0.113.86',
  source: 'STUN (Mock)',
  last_updated: new Date().toISOString(),
  last_changed: new Date(Date.now() - 1000 * 60 * 60 * 48).toISOString(), // 2 days ago
  dns_status: {
    synced: true,
    records: [
      'example.com: 203.0.113.86',
      'www.example.com: 203.0.113.86',
      'vpn.example.com: 203.0.113.86'
    ]
  }
});

const generateMockEvents = (count: number, intervalMs: number): EventLog[] => {
  const events: EventLog[] = [];
  const now = Date.now();
  const types: EventLog['type'][] = ['info', 'success', 'warning', 'error'];
  const categories = ['SYSTEM', 'DNS', 'IP CHECK', 'NETWORK'];
  const messages = [
    'Scheduled IP check completed successfully',
    'DNS record updated for zone example.com',
    'Public IP address changed detected',
    'Connection timed out, retrying...',
    'Service initialized',
    'Cloudflare API latency high',
    'STUN server response received'
  ];

  for (let i = 0; i < count; i++) {
    // Random time distribution within the range
    const timeOffset = Math.floor(Math.random() * (count * intervalMs));
    const typeIdx = Math.random() > 0.8 ? 2 : (Math.random() > 0.9 ? 3 : (Math.random() > 0.6 ? 1 : 0));

    events.push({
      id: Math.random().toString(36).substr(2, 9),
      time: new Date(now - timeOffset).toISOString(),
      type: types[typeIdx],
      category: categories[Math.floor(Math.random() * categories.length)],
      message: messages[Math.floor(Math.random() * messages.length)]
    });
  }

  // Always sort events by time descending (newest first)
  return events.sort((a, b) => new Date(b.time).getTime() - new Date(a.time).getTime());
};

const generateMockStats = (range: string): StatsResponse => {
  let count = 24;
  let interval = 3600000; // 1h

  switch (range) {
    case '24h': count = 24; interval = 3600000; break;
    case '7d': count = 7; interval = 86400000; break;
    case '30d': count = 30; interval = 86400000; break;
    case '365d': count = 12; interval = 2592000000; break; // Monthly approx
    case 'all': count = 50; interval = 604800000; break; // Weekly points
  }

  const now = Date.now();

  const data = Array.from({ length: count }).map((_, i) => ({
    time: new Date(now - (count - 1 - i) * interval).toISOString(),
    count: Math.floor(Math.random() * (range === '365d' || range === 'all' ? 50 : 8))
  }));

  // Generate more events for longer ranges
  const eventCount = range === '24h' ? 15 : range === '7d' ? 50 : 100;

  return {
    range,
    group_by: range === '24h' ? 'hour' : range === '7d' ? 'day' : range === '30d' ? 'day' : range === '365d' ? 'month' : 'week',
    data,
    dns_failures: [],
    error_logs: [],
    events: generateMockEvents(eventCount, interval)
  };
};

const mockConfig: Config = {
  server: {
    port: 8080,
    api_key: 'mock-key',
    trusted_subnets: ['127.0.0.1/32', '192.168.1.0/24']
  },
  intervals: {
    ip_check: '5m',
    dns_update: '1m'
  },
  ip_providers: [
    { type: 'stun', enabled: true, properties: { server: 'stun.l.google.com:19302' } },
    { type: 'http', enabled: false, properties: { url: 'https://api.ipify.org' } }
  ],
  cloudflare_accounts: [
    {
      name: 'Personal',
      api_token: '****************',
      zones: [
        { zone_name: 'example.com', records: ['@', 'www', 'vpn'] }
      ]
    }
  ]
};

// --- Helper for safe fetching ---
async function fetchWithFallback<T>(url: string, mockData: T | (() => T), errorMessage: string): Promise<T> {
  try {
    const res = await fetch(url, { headers: getHeaders() });
    if (!res.ok) {
      console.warn(`[API] ${errorMessage} (Status: ${res.status}). Falling back to mock data.`);
      return typeof mockData === 'function' ? (mockData as () => T)() : mockData;
    }
    return await res.json();
  } catch (e) {
    console.warn(`[API] Connection failed for ${url}. Falling back to mock data.`);
    return typeof mockData === 'function' ? (mockData as () => T)() : mockData;
  }
}

// --- API Implementation ---

export const api = {
  getStatus: async (): Promise<StatusResponse> => {
    return fetchWithFallback<StatusResponse>(
      `${API_BASE}/status`,
      generateMockStatus,
      'Failed to fetch status'
    );
  },

  getStats: async (range: string = '24h'): Promise<StatsResponse> => {
    return fetchWithFallback<StatsResponse>(
      `${API_BASE}/stats/history?range=${range}`,
      () => generateMockStats(range),
      'Failed to fetch stats'
    );
  },

  getConfig: async (): Promise<Config> => {
    try {
      const res = await fetch(`${API_BASE}/config`, {
        headers: getHeaders(),
      });
      if (res.status === 401) throw new Error('UNAUTHORIZED');
      if (!res.ok) throw new Error('Failed to fetch config');
      return await res.json();
    } catch (e: any) {
      if (e.message === 'UNAUTHORIZED') throw e;
      console.warn('API connection failed (Config). Using mock data.', e);
      return mockConfig;
    }
  },

  saveConfig: async (config: Config): Promise<void> => {
    try {
      const res = await fetch(`${API_BASE}/config`, {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(config),
      });
      if (res.status === 401) throw new Error('UNAUTHORIZED');
      if (!res.ok) {
        const contentType = res.headers.get('content-type');
        if (contentType?.includes('application/json')) {
          const errorData = await res.json();
          throw new Error(errorData.error || 'Failed to save config');
        } else {
          const text = await res.text();
          throw new Error(text || 'Failed to save config');
        }
      }
    } catch (e: any) {
      if (e.message === 'UNAUTHORIZED') throw e;
      // 如果是其他错误（如验证失败），直接抛出
      if (e.message && e.message !== 'Failed to fetch') throw e;
      console.warn('API connection failed (Save Config). Simulating success.', e);
      await new Promise(resolve => setTimeout(resolve, 800));
    }
  },
};