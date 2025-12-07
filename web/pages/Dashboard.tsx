import React, { useEffect, useState, useContext, useMemo, useRef } from 'react';
import { AppContext } from '../App';
import { api } from '../services/api';
import { StatusResponse, StatsResponse, EventLog } from '../types';
import { Globe, Activity, Clock, Server, ArrowUpRight, Copy, AlertTriangle, Maximize2, X, CheckCircle, Info, AlertCircle, Timer, RefreshCw, Zap, Settings } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { motion, AnimatePresence } from 'framer-motion';

// --- Font Fitting Hook ---
const useFitFontSize = (texts: string[], containerWidth: number, maxFontSize: number = 30, minFontSize: number = 12) => {
  const [fontSize, setFontSize] = useState(maxFontSize);

  useEffect(() => {
    if (!texts || texts.length === 0 || containerWidth === 0) return;

    const ctx = document.createElement('canvas').getContext('2d');
    if (!ctx) return;

    let minCalculatedSize = maxFontSize;

    texts.forEach(text => {
      let size = maxFontSize;
      ctx.font = `bold ${size}px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace`; // Match the font-mono class
      let width = ctx.measureText(text).width;

      // Binary search or simple decrement could work. Simple decrement is safer for exact fit.
      // Use 0.85 factor as a safety margin to prevent truncation due to font rendering differences
      const safeWidth = containerWidth * 0.85;

      while (width > safeWidth && size > minFontSize) {
        size -= 1;
        ctx.font = `bold ${size}px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace`;
        width = ctx.measureText(text).width;
      }

      if (size < minCalculatedSize) {
        minCalculatedSize = size;
      }
    });

    setFontSize(minCalculatedSize);
  }, [texts, containerWidth, maxFontSize, minFontSize]);

  return fontSize;
};

// --- SVG Text Component for auto-scaling ---
const SVGText = ({ text, className = '' }: { text: string, className?: string }) => (
  <svg
    viewBox="0 0 280 40"
    className={`w-full h-10 ${className}`}
    preserveAspectRatio="xMinYMid meet"
  >
    <text
      x="0"
      y="30"
      className="fill-current"
      style={{
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
        fontWeight: 'bold',
        fontSize: '28px'
      }}
    >
      {text}
    </text>
  </svg>
);

const StatCard = ({ title, value, subValue, icon: Icon, colorClass, loading, onClick }: any) => {
  return (
    <motion.div
      layout
      whileHover={{ scale: 1.02, y: -5 }}
      whileTap={{ scale: 0.98 }}
      onClick={onClick}
      className={`relative overflow-hidden rounded-2xl bg-surface shadow-sm hover:shadow-xl transition-shadow p-6 ${onClick ? 'cursor-pointer' : ''} flex flex-col h-full`}
    >
      <div className={`absolute -top-4 -right-4 p-4 opacity-[0.05] ${colorClass}`}>
        <Icon size={120} />
      </div>
      <div className="flex items-center gap-3 mb-4 relative z-10">
        <div className={`p-2.5 rounded-xl ${colorClass.replace('text-', 'bg-').replace('400', '500').replace('500', '600')}/10`}>
          <Icon size={20} className={colorClass} />
        </div>
        <span className="text-base font-bold text-content tracking-wide">{title}</span>
      </div>
      <div className="relative z-10 flex-1 flex flex-col">
        <div className="text-content" title={typeof value === 'string' ? value : ''}>
          {loading ? (
            <div className="h-10 w-24 bg-surface-hover animate-pulse rounded" />
          ) : (
            <SVGText text={value} />
          )}
        </div>
        <div className="text-sm text-muted font-mono truncate flex items-center gap-2 mt-auto pt-3">
          {subValue}
        </div>
      </div>
    </motion.div>
  );
};

const DNSModal = ({ data, onClose }: { data: string[], onClose: () => void }) => (
  <motion.div
    initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
    className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
    onClick={onClose}
  >
    <motion.div
      initial={{ scale: 0.9, opacity: 0, y: 20 }}
      animate={{ scale: 1, opacity: 1, y: 0 }}
      exit={{ scale: 0.9, opacity: 0, y: 20 }}
      className="bg-surface rounded-2xl w-full max-w-lg shadow-2xl max-h-[80vh] flex flex-col overflow-hidden"
      onClick={e => e.stopPropagation()}
    >
      <div className="p-6 flex justify-between items-center bg-surface-hover/30">
        <h3 className="text-lg font-bold text-content flex items-center gap-2">
          <Server size={20} className="text-emerald-500" />
          DNS Records
        </h3>
        <button onClick={onClose} className="text-muted hover:text-content p-2 rounded-full hover:bg-surface-hover transition-colors">
          <X size={20} />
        </button>
      </div>
      <div className="p-4 overflow-y-auto custom-scrollbar">
        {data.length === 0 ? (
          <p className="text-center text-muted py-8 flex flex-col items-center gap-2">
            <AlertTriangle size={32} className="opacity-50" />
            No Active Records
          </p>
        ) : (
          <motion.ul className="space-y-2">
            {data.map((record, idx) => (
              <motion.li
                key={idx}
                initial={{ opacity: 0, x: -10 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: idx * 0.05 }}
                className="flex items-center gap-3 p-3 rounded-lg bg-surface-hover/50 font-mono text-sm text-content hover:bg-surface-hover transition-colors"
              >
                <span className={`w-2.5 h-2.5 rounded-full shadow-sm ${record.includes('failed') ? 'bg-red-500' : 'bg-emerald-500'}`}></span>
                {record}
              </motion.li>
            ))}
          </motion.ul>
        )}
      </div>
    </motion.div>
  </motion.div>
);

const DNSDetailCard = ({ records, isZh }: { records: string[], isZh: boolean }) => (
  <motion.div
    layout
    className="bg-surface rounded-2xl p-6 shadow-sm flex flex-col h-full hover:shadow-md transition-shadow"
  >
    <h3 className="font-bold text-content mb-4 flex items-center gap-2">
      <Server size={18} className="text-emerald-500" />
      {isZh ? 'DNS 记录详情' : 'DNS Records Detail'}
    </h3>
    <div className="flex-1 overflow-y-auto custom-scrollbar pr-1 space-y-2 max-h-[250px]">
      {records.length === 0 ? (
        <div className="h-full flex flex-col items-center justify-center text-muted text-sm opacity-60">
          <AlertTriangle size={24} className="mb-2" />
          {isZh ? '无记录' : 'No Records'}
        </div>
      ) : (
        records.map((r, i) => (
          <div key={i} className="flex items-center gap-2 p-2 rounded text-sm font-mono truncate hover:bg-surface-hover/50 transition-colors">
            <div className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${r.includes('failed') ? 'bg-red-500' : 'bg-emerald-500'}`}></div>
            <span className="truncate text-content/90">{r}</span>
          </div>
        ))
      )}
    </div>
  </motion.div>
);

const Dashboard = () => {
  const { lang, showToast, isFullscreenMode, toggleFullscreenMode, theme } = useContext(AppContext);
  const isZh = lang === 'zh';

  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [processedEvents, setProcessedEvents] = useState<EventLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [timeRange, setTimeRange] = useState('24h');
  const [showDNSModal, setShowDNSModal] = useState(false);
  const [now, setNow] = useState(new Date());
  // Robust start time tracking for real-time uptime
  const [clientStartTime, setClientStartTime] = useState<Date | null>(null);

  // --- Data Fetching & Processing ---
  const fetchData = async () => {
    try {
      const [statusData, statsData] = await Promise.all([
        api.getStatus(),
        api.getStats(timeRange)
      ]);
      setStatus(statusData);
      setStats(statsData);

      // Update client-side start time reference
      if (statusData.start_time) {
        setClientStartTime(new Date(statusData.start_time));
      } else if (statusData.uptime_seconds && !clientStartTime) {
        // Fallback: Estimate start time if not provided by backend
        // We only do this once to prevent drifting on every fetch
        setClientStartTime(new Date(Date.now() - statusData.uptime_seconds * 1000));
      }

      // Process Events: Merge dns_failures and error_logs into a single events array
      const events: EventLog[] = [];

      // Add DNS Failures
      if (statsData.dns_failures) {
        statsData.dns_failures.forEach((f: any) => {
          events.push({
            id: `dns-${f.time}`,
            time: f.time,
            type: 'error',
            category: 'DNS',
            message: `DNS Update Failed: ${f.domain} (${f.error})`
          });
        });
      }

      // Add IP History
      if (statsData.ip_history) {
        statsData.ip_history.forEach((h: any) => {
          events.push({
            id: `ip-${h.id}`,
            time: h.timestamp,
            type: 'info',
            category: 'IP CHANGE',
            message: `IP Changed to ${h.ip} (${h.source})`
          });
        });
      }

      // Add System Errors
      if (statsData.error_logs) {
        statsData.error_logs.forEach((e: any) => {
          events.push({
            id: `err-${e.time}`,
            time: e.time,
            type: 'error',
            category: 'SYSTEM',
            message: e.message
          });
        });
      }

      // Add Mock/Simulated Success Events if list is empty (for better UX if no errors)
      // In a real scenario, we might want to log successful IP checks too, but backend doesn't send them yet.
      if (events.length === 0 && statsData.events) {
        // Fallback to mock events if provided (e.g. by api.ts fallback)
        setProcessedEvents(statsData.events);
      } else {
        // Sort by time descending
        events.sort((a, b) => new Date(b.time).getTime() - new Date(a.time).getTime());
        setProcessedEvents(events);
      }

    } catch (e) {
      console.warn('Data update cycle interrupted', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 5000); // 轮询作为降级方案
    const clock = setInterval(() => setNow(new Date()), 1000);

    // WebSocket 实时推送
    let ws: WebSocket | null = null;
    let reconnectTimer: NodeJS.Timeout | null = null;

    const connectWebSocket = () => {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const apiKey = localStorage.getItem('idrd_api_key') || '';
      const wsUrl = `${protocol}//${window.location.host}/ws?key=${encodeURIComponent(apiKey)}`;

      try {
        ws = new WebSocket(wsUrl);

        ws.onopen = () => {
          console.log('📡 WebSocket 已连接');
        };

        ws.onmessage = (event) => {
          try {
            const msg = JSON.parse(event.data);
            if (msg.type === 'ip_change') {
              console.log('🔄 收到 IP 变化推送:', msg.data);
              fetchData(); // 立即刷新数据
            }
          } catch (e) {
            console.warn('WebSocket 消息解析失败', e);
          }
        };

        ws.onclose = () => {
          console.log('📡 WebSocket 已断开，5 秒后重连...');
          reconnectTimer = setTimeout(connectWebSocket, 5000);
        };

        ws.onerror = (error) => {
          console.warn('📡 WebSocket 错误', error);
        };
      } catch (e) {
        console.warn('WebSocket 连接失败，使用轮询模式', e);
      }
    };

    connectWebSocket();

    return () => {
      clearInterval(interval);
      clearInterval(clock);
      if (reconnectTimer) clearTimeout(reconnectTimer);
      if (ws) ws.close();
    };
  }, [timeRange]);

  // --- Helpers ---
  const copyIp = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (status?.current_ip) {
      navigator.clipboard.writeText(status.current_ip);
      showToast(isZh ? '已复制 IP 地址' : 'IP Address Copied');
    }
  };

  const chartData = useMemo(() => {
    if (!stats) return [];
    return stats.data.map(item => ({
      time: new Date(item.time).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit', hour12: true }),
      rawTime: item.time,
      count: item.count
    }));
  }, [stats]);

  const timeAgo = (dateStr: string) => {
    if (!dateStr) return '--';
    const diff = Math.max(0, Math.floor((now.getTime() - new Date(dateStr).getTime()) / 1000));
    if (diff < 60) return `${diff}s`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
    return `${Math.floor(diff / 86400)}d`;
  };

  const format12Hour = (dateStr: string) => {
    if (!dateStr) return '--';
    return new Date(dateStr).toLocaleTimeString([], { hour12: true, hour: 'numeric', minute: '2-digit', second: '2-digit' });
  };

  const formatLogTime = (dateStr: string) => {
    if (!dateStr) return '--';
    const d = new Date(dateStr);
    return isZh
      ? `${d.getMonth() + 1}/${d.getDate()} ${d.toLocaleTimeString([], { hour12: true, hour: 'numeric', minute: '2-digit', second: '2-digit' })}`
      : `${d.toLocaleDateString([], { month: 'numeric', day: 'numeric' })} ${d.toLocaleTimeString([], { hour12: true, hour: 'numeric', minute: '2-digit', second: '2-digit' })}`;
  };

  const formatDuration = (dateStr: string) => {
    if (!dateStr) return '--';
    const diff = Math.floor((now.getTime() - new Date(dateStr).getTime()) / 1000);
    const d = Math.floor(diff / 86400);
    const h = Math.floor((diff % 86400) / 3600);
    const m = Math.floor((diff % 3600) / 60);
    const s = Math.floor(diff % 60); // Seconds
    if (isZh) return `${d > 0 ? d + '天 ' : ''}${h}小时 ${m}分 ${s}秒`;
    return `${d > 0 ? d + 'd ' : ''}${h}h ${m}m ${s}s`;
  };

  const formatUptime = (seconds: number) => {
    if (!seconds) return '--';
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60); // Seconds
    if (isZh) return d > 0 ? `${d}天 ${h}时 ${m}分 ${s}秒` : `${h}时 ${m}分 ${s}秒`;
    return d > 0 ? `${d}d ${h}h ${m}m ${s}s` : `${h}h ${m}m ${s}s`;
  };

  // --- Font Size Calculation ---
  // Prepare values for the cards
  const cardValues = useMemo(() => {
    const v1 = status?.current_ip || '--';
    const v2 = timeAgo(status?.last_updated || '') + (isZh ? ' 前' : ' ago');
    const v3 = formatDuration(status?.last_changed || '');
    const v4 = status?.dns_status?.synced ? (isZh ? '已同步' : 'Synced') : (isZh ? '异常' : 'Issue');

    // Uptime calculation using robust clientStartTime
    let uptimeSec = 0;
    if (clientStartTime) {
      uptimeSec = Math.floor((now.getTime() - clientStartTime.getTime()) / 1000);
    } else {
      uptimeSec = status?.uptime_seconds || 0;
    }
    const v5 = formatUptime(uptimeSec); // 系统运行时长 (实时)
    const v6 = `${status?.ip_change_count || 0} ${isZh ? '次' : ''}`; // 24h IP 变化
    const v7 = status?.check_stats?.ip?.success_rate
      ? `${status.check_stats.ip.success_rate.toFixed(1)}%`
      : '--'; // 检查成功率
    return [v1, v2, v3, v4, v5, v6, v7];
  }, [status, isZh, now]); // now dependency ensures time updates trigger recalc

  // Calculate unified font size
  // Assuming card width is roughly (WindowWidth / Columns) - Padding
  // This is an approximation. For better accuracy, we could use a ref on the card container.
  // Here we assume a safe minimum width for the text container (e.g., 200px on desktop)
  const cardRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState(0);

  useEffect(() => {
    if (!cardRef.current) return;

    const observer = new ResizeObserver(entries => {
      for (const entry of entries) {
        // Use contentRect.width and subtract minimal padding (24px for safe margin)
        setContainerWidth(entry.contentRect.width - 24);
      }
    });

    observer.observe(cardRef.current);
    return () => observer.disconnect();
  }, []);

  // Increase maxFontSize to 80px for larger display
  const unifiedFontSize = useFitFontSize(cardValues, containerWidth, 80, 20);


  const isDark = theme === 'dark' || (theme === 'system' && typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches);
  const chartColor = isDark ? '#38bdf8' : '#d97706';
  const chartGrid = 'transparent'; // Remove grid lines
  const chartText = isDark ? '#94a3b8' : '#78716c';

  return (
    <div className="space-y-6">
      {/* Header Info */}
      <motion.div
        layout
        className="flex flex-col md:flex-row justify-between items-end pb-6"
      >
        <div className="w-full md:w-auto">
          <h2 className="text-5xl font-mono font-bold text-content tracking-tighter tabular-nums">
            {now.toLocaleTimeString([], { hour12: true, hour: '2-digit', minute: '2-digit', second: '2-digit' })}
          </h2>
          <div className="flex items-center gap-3 mt-2">
            <span className="flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-xs font-bold tracking-wide uppercase">
              <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
              {isZh ? '系统运行中' : 'SYSTEM OPERATIONAL'}
            </span>
            <span className="text-muted text-sm font-medium">
              {now.toLocaleDateString([], { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' })}
            </span>
          </div>
        </div>

        {!isFullscreenMode && (
          <motion.button
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
            onClick={toggleFullscreenMode}
            className="mt-4 md:mt-0 px-4 py-2 bg-surface hover:bg-surface-hover rounded-lg text-sm font-medium text-muted hover:text-primary flex items-center gap-2 transition-colors shadow-sm"
          >
            <Maximize2 size={16} />
            {isZh ? '专注模式' : 'Monitor Mode'}
          </motion.button>
        )}
      </motion.div>

      {/* Config Info Bar */}
      {status?.config?.intervals && (
        <motion.div
          initial={{ opacity: 0, y: -10 }}
          animate={{ opacity: 1, y: 0 }}
          className="flex flex-wrap gap-3 pb-2"
        >
          <span className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-surface text-sm font-medium text-muted shadow-sm">
            <Clock size={14} className="text-amber-500" />
            {isZh ? 'IP检查: ' : 'IP Check: '}{status.config.intervals.ip_check}
          </span>
          <span className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-surface text-sm font-medium text-muted shadow-sm">
            <RefreshCw size={14} className="text-emerald-500" />
            {isZh ? 'DNS更新: ' : 'DNS Update: '}{status.config.intervals.dns_update}
          </span>
          <span className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-surface text-sm font-medium text-muted shadow-sm">
            <Settings size={14} className="text-primary" />
            {isZh ? '来源: ' : 'Source: '}{status?.source?.toUpperCase() || '--'}
          </span>
          {status.check_stats?.ip && (
            <span className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-surface text-sm font-medium text-muted shadow-sm">
              <Zap size={14} className="text-purple-500" />
              {isZh ? '平均延迟: ' : 'Avg Latency: '}{Math.round(status.check_stats.ip.avg_duration_ms || 0)}ms
            </span>
          )}
        </motion.div>
      )}

      {/* Stats Grid - Row 1 */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 auto-rows-fr">
        <div className="relative group h-full" ref={cardRef}>
          <StatCard
            title={isZh ? '当前公网 IP' : 'Current Public IP'}
            value={cardValues[0]}
            subValue={`${isZh ? '来源: ' : 'Source: '}${status?.source?.toUpperCase() || '--'}`}
            icon={Globe}
            colorClass="text-primary"
            loading={loading}
          />
          <motion.button
            whileHover={{ scale: 1.1 }}
            whileTap={{ scale: 0.9 }}
            onClick={copyIp}
            className="absolute top-6 right-6 text-muted hover:text-primary transition-colors bg-surface-hover/50 p-2 rounded-lg"
          >
            <Copy size={18} />
          </motion.button>
        </div>

        <StatCard
          title={isZh ? '上次检查' : 'Last Check'}
          value={cardValues[1]}
          subValue={format12Hour(status?.last_updated || '')}
          icon={Clock}
          colorClass="text-amber-500"
          loading={loading}
        />

        <StatCard
          title={isZh ? 'IP 持续时间' : 'IP Duration'}
          value={cardValues[2]}
          subValue={`${isZh ? '变更于: ' : 'Since: '}${status?.last_changed ? new Date(status.last_changed).toLocaleDateString() : '--'}`}
          icon={ArrowUpRight}
          colorClass="text-purple-500"
          loading={loading}
        />

        <StatCard
          title={isZh ? 'DNS 状态' : 'DNS Status'}
          value={cardValues[3]}
          subValue={`${status?.dns_status?.records?.length || 0} ${isZh ? '条记录' : 'Records'}`}
          icon={Server}
          colorClass={status?.dns_status?.synced ? "text-emerald-500" : "text-red-500"}
          loading={loading}
          onClick={() => setShowDNSModal(true)}
        />
      </div>

      {/* Stats Grid - Row 2: Additional Monitoring Info */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 auto-rows-fr">
        <StatCard
          title={isZh ? '系统运行时长' : 'System Uptime'}
          value={cardValues[4]}
          subValue={`${isZh ? '启动于: ' : 'Started: '}${status?.start_time ? new Date(status.start_time).toLocaleDateString() : '--'}`}
          icon={Timer}
          colorClass="text-cyan-500"
          loading={loading}
        />

        <StatCard
          title={isZh ? '24h IP 变化' : '24h IP Changes'}
          value={cardValues[5]}
          subValue={isZh ? '过去 24 小时内' : 'In the last 24 hours'}
          icon={RefreshCw}
          colorClass="text-orange-500"
          loading={loading}
        />

        <StatCard
          title={isZh ? '检查成功率' : 'Check Success Rate'}
          value={cardValues[6]}
          subValue={`${isZh ? '总计: ' : 'Total: '}${status?.check_stats?.ip?.total_checks || 0} ${isZh ? '次检查' : 'checks'}`}
          icon={Zap}
          colorClass={
            (status?.check_stats?.ip?.success_rate || 0) >= 95 ? "text-emerald-500" :
              (status?.check_stats?.ip?.success_rate || 0) >= 80 ? "text-amber-500" : "text-red-500"
          }
          loading={loading}
        />
      </div>

      {/* Chart & DNS Detail - Row 2 */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <motion.div
          layout
          className="lg:col-span-2 bg-surface rounded-2xl p-6 shadow-sm flex flex-col h-[350px] hover:shadow-md transition-shadow"
        >
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-6">
            <h3 className="font-bold text-content flex items-center gap-2">
              <Activity size={18} className="text-primary" />
              {isZh ? 'IP 变更趋势' : 'IP Change History'}
            </h3>
            <div className="flex flex-wrap bg-surface-hover rounded-lg p-1 gap-1">
              {['24h', '7d', '30d', '365d', 'all'].map(r => (
                <button
                  key={r}
                  onClick={() => setTimeRange(r)}
                  className={`px-3 py-1 text-xs font-bold rounded-md transition-all ${timeRange === r ? 'bg-surface shadow-sm text-primary' : 'text-muted hover:text-content'}`}
                >
                  {r.toUpperCase()}
                </button>
              ))}
            </div>
          </div>
          <div className="flex-1 w-full min-h-0">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData}>
                <defs>
                  <linearGradient id="colorCount" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor={chartColor} stopOpacity={0.3} />
                    <stop offset="95%" stopColor={chartColor} stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke={chartGrid} vertical={false} />
                <XAxis dataKey="time" stroke={chartText} fontSize={12} tickLine={false} axisLine={false} minTickGap={30} />
                <YAxis stroke={chartText} fontSize={12} tickLine={false} axisLine={false} allowDecimals={false} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: isDark ? '#0f172a' : '#ffffff',
                    border: 'none',
                    borderRadius: '12px',
                    color: isDark ? '#f8fafc' : '#1c1917',
                    boxShadow: '0 10px 15px -3px rgba(0, 0, 0, 0.1)'
                  }}
                  itemStyle={{ color: chartColor }}
                />
                <Area
                  type="monotone"
                  dataKey="count"
                  stroke={chartColor}
                  strokeWidth={3}
                  fillOpacity={1}
                  fill="url(#colorCount)"
                  animationDuration={1500}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </motion.div>

        <DNSDetailCard records={status?.dns_status?.records || []} isZh={isZh} />
      </div>

      {/* Full Width Recent Log - Row 3 */}
      <motion.div
        layout
        className="bg-surface rounded-2xl p-6 shadow-sm min-h-[300px] flex flex-col hover:shadow-md transition-shadow"
      >
        <h3 className="font-bold text-content mb-4 flex items-center gap-2">
          <AlertTriangle size={18} className="text-amber-500" />
          {isZh ? '最近事件' : 'Recent Events'}
          <span className="text-xs font-normal text-muted ml-auto bg-surface-hover px-2 py-1 rounded">
            {isZh ? '来源: 历史记录' : 'Source: Historical Data'}
          </span>
        </h3>

        <div className="flex-1 overflow-hidden relative">
          <div className="absolute inset-0 overflow-y-auto custom-scrollbar p-0">
            {loading && processedEvents.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center text-muted gap-2">
                <Activity className="animate-spin opacity-50" />
                Loading Events...
              </div>
            ) : processedEvents.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center text-muted gap-2 opacity-60">
                <CheckCircle size={32} />
                {isZh ? '此期间无异常事件' : 'No events found in this period'}
              </div>
            ) : (
              <table className="w-full text-left text-sm">
                <thead className="bg-surface/80 backdrop-blur sticky top-0 z-10">
                  <tr>
                    <th className="px-4 py-3 font-bold text-muted w-48">{isZh ? '时间' : 'Time'}</th>
                    <th className="px-4 py-3 font-bold text-muted w-32">{isZh ? '类型' : 'Type'}</th>
                    <th className="px-4 py-3 font-bold text-muted w-32 hidden sm:table-cell">{isZh ? '分类' : 'Category'}</th>
                    <th className="px-4 py-3 font-bold text-muted">{isZh ? '详情' : 'Details'}</th>
                  </tr>
                </thead>
                <tbody>
                  {processedEvents.map((evt) => (
                    <tr key={evt.id} className="hover:bg-surface-hover/50 transition-colors group">
                      <td className="px-4 py-3 font-mono text-content text-xs whitespace-nowrap">
                        {formatLogTime(evt.time)}
                      </td>
                      <td className="px-4 py-3">
                        <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-bold uppercase ${evt.type === 'error' ? 'bg-red-500/10 text-red-600 dark:text-red-400' :
                          evt.type === 'warning' ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400' :
                            evt.type === 'success' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' :
                              'bg-blue-500/10 text-blue-600 dark:text-blue-400'
                          }`}>
                          {evt.type === 'error' ? <AlertCircle size={10} /> :
                            evt.type === 'warning' ? <AlertTriangle size={10} /> :
                              evt.type === 'success' ? <CheckCircle size={10} /> : <Info size={10} />}
                          {evt.type}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-muted text-xs font-bold uppercase hidden sm:table-cell">
                        {evt.category}
                      </td>
                      <td className="px-4 py-3 text-content/90 font-mono text-xs break-all">
                        {evt.message}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </motion.div>

      <AnimatePresence>
        {showDNSModal && status?.dns_status && (
          <DNSModal data={status.dns_status.records} onClose={() => setShowDNSModal(false)} />
        )}
      </AnimatePresence>
    </div>
  );
};

export default Dashboard;
