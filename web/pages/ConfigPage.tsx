import React, { useContext, useEffect, useState } from 'react';
import { AppContext } from '../App';
import { api } from '../services/api';
import { Config, IpProvider, CloudflareAccount, Zone } from '../types';
import { Save, Plus, Trash2, RefreshCw, Shield, Globe, Cloud, ChevronDown, ChevronUp, Settings, Check, Download } from 'lucide-react';
import { AuthModal } from '../App';
import { motion, AnimatePresence } from 'framer-motion';

// --- Sub Components ---
const SectionHeader = ({ icon: Icon, title, action }: any) => (
  <div className="flex justify-between items-center mb-6 pb-2">
    <div className="flex items-center gap-2 text-primary">
      <Icon size={20} />
      <h3 className="font-bold uppercase tracking-wider text-sm">{title}</h3>
    </div>
    {action}
  </div>
);

const InputGroup = ({ label, children }: any) => (
  <div className="space-y-2">
    <label className="block text-xs font-bold text-muted uppercase tracking-wider">{label}</label>
    {children}
  </div>
);

const StyledInput = (props: React.InputHTMLAttributes<HTMLInputElement>) => (
  <input
    {...props}
    className={`w-full bg-surface-hover rounded-lg px-4 py-2.5 text-sm text-content placeholder-muted focus:ring-1 focus:ring-primary outline-none transition-all font-mono ${props.className}`}
  />
);

// --- Provider Form ---
const ProviderItem: React.FC<{ provider: IpProvider, onChange: (p: IpProvider) => void, onRemove: () => void, isZh: boolean }> = ({ provider, onChange, onRemove, isZh }) => {
  const [expanded, setExpanded] = useState(false); // Default collapsed for cleaner look

  const updateProp = (key: string, val: string) => {
    onChange({ ...provider, properties: { ...provider.properties, [key]: val } });
  };

  const renderFields = () => {
    switch (provider.type) {
      case 'stun':
        return <InputGroup label="STUN Server"><StyledInput value={provider.properties.server || ''} onChange={e => updateProp('server', e.target.value)} placeholder="stun.l.google.com:19302" /></InputGroup>;
      case 'http':
        return <InputGroup label="URL"><StyledInput value={provider.properties.url || ''} onChange={e => updateProp('url', e.target.value)} placeholder="https://api.ipify.org" /></InputGroup>;
      case 'interface':
        return <InputGroup label={isZh ? "接口名称" : "Interface Name"}><StyledInput value={provider.properties.name || ''} onChange={e => updateProp('name', e.target.value)} placeholder="eth0" /></InputGroup>;
      case 'router_ssh':
        return (
          <div className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <InputGroup label="Router Type">
                <select
                  value={provider.properties.type || 'routeros'}
                  onChange={e => updateProp('type', e.target.value)}
                  className="w-full bg-surface-hover rounded-lg px-4 py-2.5 text-sm text-content focus:ring-1 focus:ring-primary outline-none cursor-pointer"
                >
                  <option value="routeros">RouterOS</option>
                  <option value="openwrt">OpenWrt</option>
                </select>
              </InputGroup>
              <InputGroup label="Port"><StyledInput type="number" value={provider.properties.port || '22'} onChange={e => updateProp('port', e.target.value)} placeholder="22" /></InputGroup>
              <InputGroup label="Host"><StyledInput value={provider.properties.host || ''} onChange={e => updateProp('host', e.target.value)} placeholder="192.168.1.1" /></InputGroup>
              <InputGroup label="User"><StyledInput value={provider.properties.user || ''} onChange={e => updateProp('user', e.target.value)} placeholder="admin" /></InputGroup>
              <InputGroup label="Interface"><StyledInput value={provider.properties.interface || ''} onChange={e => updateProp('interface', e.target.value)} placeholder="wan / ether1" /></InputGroup>
            </div>
            <div className="bg-surface-hover/50 rounded-lg p-4 space-y-3">
              <div className="text-xs font-bold text-muted uppercase mb-2">{isZh ? '认证方式 (选择一种)' : 'Authentication (Choose One)'}</div>
              <InputGroup label={`${isZh ? '密码' : 'Password'} (${isZh ? '可选' : 'Optional'})`}>
                <StyledInput type="password" value={provider.properties.password || ''} onChange={e => updateProp('password', e.target.value)} placeholder={isZh ? '如果使用密码认证' : 'If using password auth'} />
              </InputGroup>
              <InputGroup label={`SSH Private Key (${isZh ? '可选' : 'Optional'})`}>
                <div className="space-y-2">
                  <div
                    className="relative"
                    onDragOver={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      e.currentTarget.classList.add('ring-2', 'ring-primary', 'ring-offset-2');
                    }}
                    onDragLeave={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      e.currentTarget.classList.remove('ring-2', 'ring-primary', 'ring-offset-2');
                    }}
                    onDrop={async (e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      e.currentTarget.classList.remove('ring-2', 'ring-primary', 'ring-offset-2');

                      const file = e.dataTransfer.files?.[0];
                      if (file) {
                        try {
                          const text = await file.text();
                          // 验证是否包含私钥标记
                          const hasValidHeader = text.includes('-----BEGIN') &&
                            (text.includes('PRIVATE KEY') || text.includes('OPENSSH PRIVATE KEY') ||
                              text.includes('RSA PRIVATE KEY') || text.includes('EC PRIVATE KEY'));

                          if (!hasValidHeader) {
                            alert(isZh
                              ? '错误：文件不包含有效的私钥格式。请确保拖拽的是私钥文件（不是公钥 .pub 文件）。'
                              : 'Error: File does not contain a valid private key format. Make sure you dropped the private key file (not the .pub public key file).');
                            return;
                          }

                          updateProp('key', text);
                          alert(isZh
                            ? `成功导入私钥！文件名: ${file.name}\n长度: ${text.length} 字符`
                            : `Private key imported successfully!\nFilename: ${file.name}\nLength: ${text.length} characters`);
                        } catch (error) {
                          alert(isZh
                            ? `导入失败: ${error}`
                            : `Import failed: ${error}`);
                        }
                      }
                    }}
                  >
                    <textarea
                      value={provider.properties.key || ''}
                      onChange={e => updateProp('key', e.target.value)}
                      placeholder={isZh
                        ? "拖拽私钥文件到此处，或粘贴内容:\n-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----"
                        : "Drop private key file here, or paste content:\n-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----"}
                      className="w-full bg-surface-hover rounded-lg px-4 py-2.5 text-sm text-content placeholder-muted focus:ring-1 focus:ring-primary outline-none transition-all font-mono resize-none"
                      rows={6}
                    />
                    {!provider.properties.key && (
                      <div className="absolute inset-0 flex items-center justify-center pointer-events-none opacity-0 hover:opacity-100 transition-opacity">
                        <div className="bg-surface/90 px-4 py-2 rounded-lg text-sm text-muted">
                          {isZh ? '拖拽文件到这里' : 'Drop file here'}
                        </div>
                      </div>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <label className="flex items-center gap-2 cursor-pointer text-xs font-bold text-primary hover:text-primary/80 transition-colors bg-primary/10 px-3 py-1.5 rounded hover:bg-primary/20">
                      <input
                        type="file"
                        accept=".pem,.key,.txt,*"
                        className="hidden"
                        onChange={async (e) => {
                          const file = e.target.files?.[0];
                          if (file) {
                            try {
                              const text = await file.text();
                              // 验证是否包含私钥标记
                              const hasValidHeader = text.includes('-----BEGIN') &&
                                (text.includes('PRIVATE KEY') || text.includes('OPENSSH PRIVATE KEY') ||
                                  text.includes('RSA PRIVATE KEY') || text.includes('EC PRIVATE KEY'));

                              if (!hasValidHeader) {
                                alert(isZh
                                  ? '错误：文件不包含有效的私钥格式。请确保选择的是私钥文件（不是公钥 .pub 文件）。'
                                  : 'Error: File does not contain a valid private key format. Make sure you selected the private key file (not the .pub public key file).');
                                return;
                              }

                              updateProp('key', text);
                              alert(isZh
                                ? `成功导入私钥！文件名: ${file.name}\n长度: ${text.length} 字符`
                                : `Private key imported successfully!\nFilename: ${file.name}\nLength: ${text.length} characters`);
                            } catch (error) {
                              alert(isZh
                                ? `导入失败: ${error}`
                                : `Import failed: ${error}`);
                            }
                          }
                        }}
                      />
                      <RefreshCw size={12} />
                      {isZh ? '从文件导入' : 'Import from file'}
                    </label>
                    {provider.properties.key && (
                      <span className="text-xs text-emerald-500 font-medium flex items-center gap-1">
                        <Check size={12} />
                        {isZh ? `已设置 (${provider.properties.key.length} 字符)` : `Set (${provider.properties.key.length} chars)`}
                      </span>
                    )}
                  </div>
                </div>
              </InputGroup>
            </div>
          </div>
        );
      default: return null;
    }
  };

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, scale: 0.95 }}
      className="bg-surface rounded-xl overflow-hidden shadow-sm hover:shadow-md transition-shadow"
    >
      <div className="p-4 flex items-center justify-between cursor-pointer select-none" onClick={() => setExpanded(!expanded)}>
        <div className="flex items-center gap-4">
          <div className={`p-2 rounded-lg font-mono text-xs font-bold uppercase ${provider.enabled ? 'bg-primary/10 text-primary' : 'bg-muted/10 text-muted'}`}>
            {provider.type}
          </div>
          <div className="flex items-center gap-2">
            <label className="relative inline-flex items-center cursor-pointer" onClick={e => e.stopPropagation()}>
              <input type="checkbox" checked={provider.enabled} onChange={(e) => onChange({ ...provider, enabled: e.target.checked })} className="sr-only peer" />
              <div className="w-9 h-5 bg-surface-hover peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-primary"></div>
            </label>
            <span className="text-sm font-medium text-muted">{provider.enabled ? (isZh ? '已启用' : 'Enabled') : (isZh ? '已禁用' : 'Disabled')}</span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={(e) => { e.stopPropagation(); onRemove(); }} className="p-2 text-muted hover:text-red-500 transition-colors rounded-full hover:bg-red-500/10"><Trash2 size={16} /></button>
          <motion.div animate={{ rotate: expanded ? 180 : 0 }}>
            <ChevronDown size={16} className="text-muted" />
          </motion.div>
        </div>
      </div>
      <AnimatePresence>
        {expanded && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="bg-surface-hover/30"
          >
            <div className="p-4 space-y-4">
              <InputGroup label={isZh ? "类型" : "Type"}>
                <select
                  value={provider.type}
                  onChange={e => {
                    const newType = e.target.value as any;
                    let defaultProps = {};
                    if (newType === 'router_ssh') {
                      defaultProps = { type: 'routeros', port: '22', user: 'admin', interface: 'wan' };
                    }
                    onChange({ ...provider, type: newType, properties: defaultProps });
                  }}
                  className="w-full bg-surface-hover rounded-lg px-4 py-2.5 text-sm text-content focus:ring-1 focus:ring-primary outline-none cursor-pointer border-none"
                >
                  <option value="stun">STUN Server ({isZh ? '推荐' : 'Recommended'})</option>
                  <option value="router_ssh">Router SSH</option>
                  <option value="http" disabled>HTTP API ({isZh ? '即将推出' : 'Coming Soon'})</option>
                  <option value="interface" disabled>Network Interface ({isZh ? '即将推出' : 'Coming Soon'})</option>
                </select>
              </InputGroup>
              {renderFields()}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
};

// --- Cloudflare Form ---
const AccountItem: React.FC<{ account: CloudflareAccount, onChange: (a: CloudflareAccount) => void, onRemove: () => void, isZh: boolean }> = ({ account, onChange, onRemove, isZh }) => {
  const addZone = () => {
    onChange({ ...account, zones: [...account.zones, { zone_name: '', records: [] }] });
  };

  const updateZone = (idx: number, field: keyof Zone, val: any) => {
    const newZones = [...account.zones];
    if (field === 'records') {
      newZones[idx] = { ...newZones[idx], records: val.split(',').map((s: string) => s.trim()) };
    } else {
      newZones[idx] = { ...newZones[idx], [field]: val };
    }
    onChange({ ...account, zones: newZones });
  };

  const removeZone = (idx: number) => {
    const newZones = account.zones.filter((_, i) => i !== idx);
    onChange({ ...account, zones: newZones });
  };

  return (
    <motion.div
      layout
      initial={{ opacity: 0, x: -20 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, scale: 0.9 }}
      className="bg-surface rounded-xl p-5 space-y-5 relative group hover:shadow-md transition-shadow"
    >
      <button onClick={onRemove} className="absolute top-4 right-4 text-muted hover:text-red-500 opacity-0 group-hover:opacity-100 transition-all p-2 rounded-full hover:bg-red-500/10"><Trash2 size={16} /></button>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <InputGroup label={isZh ? "账户名称" : "Account Name"}>
          <StyledInput value={account.name} onChange={e => onChange({ ...account, name: e.target.value })} placeholder="My Account" />
        </InputGroup>
        <InputGroup label="API Token">
          <StyledInput
            type="password"
            value={account.api_token}
            onChange={e => onChange({ ...account, api_token: e.target.value })}
            placeholder={isZh ? "输入 Cloudflare API Token" : "Enter Cloudflare API Token"}
          />
        </InputGroup>
      </div>

      <div className="bg-surface-hover/30 rounded-lg p-4">
        <div className="flex justify-between items-center mb-3">
          <span className="text-xs font-bold text-muted uppercase">{isZh ? '托管区域' : 'Managed Zones'}</span>
          <button onClick={addZone} className="text-xs flex items-center gap-1 text-primary hover:text-primary/80 font-bold px-2 py-1 rounded bg-primary/10 hover:bg-primary/20 transition-colors"><Plus size={12} /> ADD ZONE</button>
        </div>
        <div className="space-y-3">
          <AnimatePresence>
            {account.zones.map((zone, idx) => (
              <motion.div
                key={idx}
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={{ opacity: 0, height: 0 }}
                className="flex gap-2 items-start"
              >
                <div className="flex-1 grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <StyledInput
                    value={zone.zone_name}
                    onChange={e => updateZone(idx, 'zone_name', e.target.value)}
                    placeholder="example.com"
                    className="bg-surface"
                  />
                  <input
                    type="text"
                    value={zone.records.join(', ')}
                    onChange={e => updateZone(idx, 'records', e.target.value)}
                    placeholder="Records (e.g., @, www, vpn)"
                    className="w-full bg-surface rounded-lg px-4 py-2.5 text-sm text-content placeholder-muted focus:ring-1 focus:ring-primary outline-none transition-all font-mono"
                  />
                </div>
                <button onClick={() => removeZone(idx)} className="p-3 text-muted hover:text-red-500 mt-0"><Trash2 size={16} /></button>
              </motion.div>
            ))}
          </AnimatePresence>
          {account.zones.length === 0 && <div className="text-xs text-muted text-center italic py-2">No zones configured</div>}
        </div>
      </div>
    </motion.div>
  );
};

// --- Main Page ---
const ConfigPage = () => {
  const { lang, showToast, isAuthenticated, setAuthenticated } = useContext(AppContext);
  const isZh = lang === 'zh';
  const [config, setConfig] = useState<Config | null>(null);
  const [saving, setSaving] = useState(false);
  const [showAuth, setShowAuth] = useState(!isAuthenticated);

  const loadConfig = async () => {
    try {
      const data = await api.getConfig();
      setConfig(data);
    } catch (e: any) {
      if (e.message === 'UNAUTHORIZED') {
        setAuthenticated(false);
        setShowAuth(true);
      } else {
        showToast(isZh ? '加载配置失败' : 'Failed to load config', 'error');
      }
    }
  };

  useEffect(() => {
    if (isAuthenticated) {
      loadConfig();
    } else {
      setShowAuth(true);
    }
  }, [isAuthenticated]);

  const handleLogin = (key: string) => {
    localStorage.setItem('idrd_api_key', key);
    setAuthenticated(true);
    setShowAuth(false);
  };

  const handleSave = async () => {
    if (!config) return;
    setSaving(true);
    try {
      await api.saveConfig(config);
      showToast(isZh ? '全局配置已保存' : 'Configuration saved successfully');
      await loadConfig();
    } catch (e: any) {
      showToast(e.message, 'error');
    } finally {
      setSaving(false);
    }
  };

  if (showAuth) return <AuthModal onLogin={handleLogin} />;

  if (!config) return (
    <div className="flex flex-col items-center justify-center h-[50vh] text-muted gap-4">
      <div className="w-8 h-8 border-4 border-primary border-t-transparent rounded-full animate-spin"></div>
      Loading Configuration...
    </div>
  );

  return (
    <div className="max-w-4xl mx-auto space-y-8 pb-20">
      <motion.div
        initial={{ opacity: 0, y: -20 }} animate={{ opacity: 1, y: 0 }}
        className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4"
      >
        <h2 className="text-3xl font-bold text-content flex items-center gap-3">
          <Settings className="text-primary" />
          {isZh ? '系统配置' : 'Configuration'}
        </h2>
        <div className="flex gap-3 w-full sm:w-auto">
          {/* Global Apply Button */}
          <motion.button
            whileHover={{ scale: 1.05 }} whileTap={{ scale: 0.95 }}
            onClick={handleSave}
            disabled={saving}
            className={`flex-1 sm:flex-none px-6 py-2 rounded-lg text-sm font-bold transition-all flex items-center gap-2 text-primary-foreground shadow-lg hover:shadow-xl ${saving ? 'bg-primary/70 cursor-not-allowed' : 'bg-primary hover:bg-primary/90'}`}
          >
            {saving ? <RefreshCw className="animate-spin" size={16} /> : <Save size={16} />}
            {isZh ? '保存更改' : 'APPLY CHANGES'}
          </motion.button>

          <motion.button
            whileHover={{ scale: 1.05 }} whileTap={{ scale: 0.95 }}
            onClick={() => {
              const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' });
              const url = URL.createObjectURL(blob);
              const a = document.createElement('a');
              a.href = url;
              a.download = 'idrd_config.json';
              a.click();
            }}
            className="flex-1 sm:flex-none px-4 py-2 bg-surface hover:bg-surface-hover text-content rounded-lg text-sm font-bold transition-colors shadow-sm flex items-center gap-2"
          >
            <Download size={16} />
            {isZh ? '导出' : 'EXPORT'}
          </motion.button>
        </div>
      </motion.div>

      {/* General Settings */}
      <motion.div
        initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 }}
        className="bg-surface rounded-2xl p-6 shadow-sm"
      >
        <SectionHeader
          icon={Shield}
          title={isZh ? "通用设置" : "General Settings"}
          action={null} // Removed individual button
        />
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          <InputGroup label="Server Port">
            <StyledInput type="number" value={config.server.port} onChange={e => setConfig({ ...config, server: { ...config.server, port: parseInt(e.target.value) } })} />
          </InputGroup>
          <div className="lg:col-span-2">
            <InputGroup label="API Key">
              <div className="relative">
                <StyledInput value={config.server.api_key} onChange={e => setConfig({ ...config, server: { ...config.server, api_key: e.target.value } })} className="pr-10" />
                <button onClick={() => setConfig({ ...config, server: { ...config.server, api_key: Array(32).fill(0).map(() => Math.random().toString(36)[2]).join('') } })} className="absolute right-3 top-2.5 text-muted hover:text-primary"><RefreshCw size={16} /></button>
              </div>
            </InputGroup>
          </div>
          <div></div>
          <div className="lg:col-span-2">
            <InputGroup label="Trusted Subnets (CIDR)">
              <StyledInput value={config.server.trusted_subnets.join(', ')} onChange={e => setConfig({ ...config, server: { ...config.server, trusted_subnets: e.target.value.split(',').map(s => s.trim()) } })} />
            </InputGroup>
          </div>
          <InputGroup label={isZh ? "IP 检查间隔" : "IP Check Interval"}>
            <StyledInput
              value={config.intervals.ip_check}
              onChange={e => setConfig({ ...config, intervals: { ...config.intervals, ip_check: e.target.value } })}
              placeholder="5m, 30s, 1h (Router SSH min: 1s, STUN min: 30s)"
            />
            <div className="text-xs text-muted mt-1">
              {isZh ? '格式示例: 1s, 30s, 5m, 1h。Router SSH/Interface 最小 1 秒，STUN/HTTP 最小 30 秒' : 'Format: 1s, 30s, 5m, 1h. Router SSH/Interface min: 1s, STUN/HTTP min: 30s'}
            </div>
          </InputGroup>
          <InputGroup label={isZh ? "DNS 更新间隔" : "DNS Update Interval"}>
            <StyledInput
              value={config.intervals.dns_update}
              onChange={e => setConfig({ ...config, intervals: { ...config.intervals, dns_update: e.target.value } })}
              placeholder="1m, 5m, 10m"
            />
            <div className="text-xs text-muted mt-1">
              {isZh ? '格式示例: 30s, 1m, 5m, 1h。仅在 IP 变化时执行' : 'Format: 30s, 1m, 5m, 1h. Only executes when IP changes'}
            </div>
          </InputGroup>
        </div>
      </motion.div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* IP Providers */}
        <motion.div
          initial={{ opacity: 0, x: -20 }} animate={{ opacity: 1, x: 0 }} transition={{ delay: 0.2 }}
          className="bg-surface rounded-2xl p-6 h-fit shadow-sm"
        >
          <SectionHeader
            icon={Globe}
            title={isZh ? "IP 提供商" : "IP Providers"}
            action={
              <button onClick={() => setConfig({ ...config, ip_providers: [...config.ip_providers, { type: 'stun', enabled: true, properties: {} }] })} className="text-primary hover:text-primary/80 text-xs font-bold flex items-center gap-1 bg-primary/10 px-2 py-1.5 rounded hover:bg-primary/20 transition-colors"><Plus size={12} /> {isZh ? "添加" : "ADD"}</button>
            }
          />
          <div className="space-y-4">
            <AnimatePresence>
              {config.ip_providers.map((p, idx) => (
                <ProviderItem
                  key={idx}
                  provider={p}
                  isZh={isZh}
                  onChange={newP => { const newArr = [...config.ip_providers]; newArr[idx] = newP; setConfig({ ...config, ip_providers: newArr }); }}
                  onRemove={() => { const newArr = config.ip_providers.filter((_, i) => i !== idx); setConfig({ ...config, ip_providers: newArr }); }}
                />
              ))}
            </AnimatePresence>
          </div>
        </motion.div>

        {/* Cloudflare Accounts */}
        <motion.div
          initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} transition={{ delay: 0.3 }}
          className="bg-surface rounded-2xl p-6 h-fit shadow-sm"
        >
          <SectionHeader
            icon={Cloud}
            title="Cloudflare Accounts"
            action={
              <button onClick={() => setConfig({ ...config, cloudflare_accounts: [...config.cloudflare_accounts, { name: '', api_token: '', zones: [] }] })} className="text-primary hover:text-primary/80 text-xs font-bold flex items-center gap-1 bg-primary/10 px-2 py-1.5 rounded hover:bg-primary/20 transition-colors"><Plus size={12} /> {isZh ? "添加" : "ADD"}</button>
            }
          />
          <div className="space-y-6">
            <AnimatePresence>
              {config.cloudflare_accounts.map((acc, idx) => (
                <AccountItem
                  key={idx}
                  account={acc}
                  isZh={isZh}
                  onChange={newAcc => { const newArr = [...config.cloudflare_accounts]; newArr[idx] = newAcc; setConfig({ ...config, cloudflare_accounts: newArr }); }}
                  onRemove={() => { const newArr = config.cloudflare_accounts.filter((_, i) => i !== idx); setConfig({ ...config, cloudflare_accounts: newArr }); }}
                />
              ))}
            </AnimatePresence>
          </div>
        </motion.div>
      </div>
    </div>
  );
};

export default ConfigPage;