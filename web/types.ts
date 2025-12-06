// API Response Types
export interface StatusResponse {
  current_ip: string;
  source: string;
  last_updated: string;
  last_changed: string;
  dns_status: {
    synced: boolean;
    records: string[];
  };
  // Extended fields from actual API
  uptime_seconds?: number;
  start_time?: string;
  ip_change_count?: number;
  ip_history?: IPHistory[];
  dns_updates?: DNSUpdate[];
  error_logs?: ErrorLog[];
  config?: StatusConfig;
  check_stats?: {
    ip?: CheckStats;
    dns?: CheckStats;
  };
  recent_checks?: CheckLog[];
}

export interface CheckStats {
  total_checks: number;
  success_count: number;
  fail_count: number;
  success_rate: number;
  avg_duration_ms: number;
}

export interface CheckLog {
  id: number;
  check_type: string;
  success: boolean;
  result?: string;
  duration_ms?: number;
  error?: string;
  timestamp: string;
}

export interface StatusConfig {
  dns_enabled?: boolean;
  intervals?: {
    ip_check: string;
    dns_update: string;
  };
}

export interface DNSUpdate {
  account: string;
  domain: string;
  record_type: string;
  ip: string;
  success: boolean;
  error?: string;
  timestamp: string;
}

export interface ErrorLog {
  level: string;
  message: string;
  timestamp: string;
}

export interface EventLog {
  id: string;
  time: string;
  type: 'success' | 'warning' | 'error' | 'info';
  category: string;
  message: string;
}

export interface IPHistory {
  id: number;
  ip: string;
  ip_version: string;
  source: string;
  timestamp: string;
}

export interface StatsResponse {
  range: string;
  group_by: string;
  data: { time: string; count: number }[];
  dns_failures?: { time: string; count: number; domain: string; error: string }[];
  error_logs?: { time: string; level: string; message: string }[];
  ip_history?: IPHistory[];
  events?: EventLog[]; // For mock/fallback
}

export interface Config {
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

export interface IpProvider {
  type: 'stun' | 'router_ssh' | 'http' | 'interface';
  enabled: boolean;
  properties: Record<string, string>;
}

export interface CloudflareAccount {
  name: string;
  api_token: string;
  zones: Zone[];
}

export interface Zone {
  zone_name: string;
  records: string[]; // API expects string array, UI handles comma-separated
}

// UI Types
export type Language = 'en' | 'zh';

export interface ToastMessage {
  id: string;
  type: 'success' | 'error' | 'info';
  message: string;
}