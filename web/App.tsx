import React, { useState, useEffect, createContext, useContext } from 'react';
import { HashRouter, Routes, Route, useLocation, Navigate } from 'react-router-dom';
import { LayoutDashboard, Settings, Lock, Languages, LogOut, Sun, Moon, Monitor, Maximize2, Minimize2, X } from 'lucide-react';
import { AnimatePresence, motion } from 'framer-motion';
import Dashboard from './pages/Dashboard';
import ConfigPage from './pages/ConfigPage';
import { ToastMessage, Language } from './types';

// --- Types & Context ---
type ThemeMode = 'light' | 'dark' | 'system';

interface AppContextType {
  lang: Language;
  toggleLang: () => void;
  theme: ThemeMode;
  setTheme: (mode: ThemeMode) => void;
  showToast: (msg: string, type?: 'success' | 'error' | 'info') => void;
  isAuthenticated: boolean;
  setAuthenticated: (val: boolean) => void;
  isFullscreenMode: boolean;
  toggleFullscreenMode: () => void;
}

export const AppContext = createContext<AppContextType>({} as AppContextType);

// --- Components ---
const SidebarItem = ({ to, icon: Icon, label, active }: any) => (
  <a href={`#${to}`} className="block relative group">
    {active && (
      <motion.div
        layoutId="activeTab"
        className="absolute inset-0 bg-primary/10 rounded-xl"
        initial={false}
        transition={{ type: "spring", stiffness: 500, damping: 30 }}
      />
    )}
    <div className={`relative flex items-center gap-3 px-4 py-3 rounded-xl transition-colors duration-200 ${active ? 'text-primary font-medium' : 'text-muted hover:text-content hover:bg-surface-hover/50'}`}>
      <Icon size={20} className={active ? 'text-primary' : 'group-hover:text-primary transition-colors'} />
      <span>{label}</span>
    </div>
  </a>
);

const ToastContainer = ({ toasts }: { toasts: ToastMessage[] }) => (
  <div className="fixed bottom-6 right-6 z-[60] flex flex-col gap-3 pointer-events-none">
    <AnimatePresence>
      {toasts.map(t => (
        <motion.div
          key={t.id}
          initial={{ opacity: 0, y: 20, scale: 0.9 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, x: 20, scale: 0.9 }}
          layout
          className={`pointer-events-auto px-4 py-3 rounded-lg shadow-xl backdrop-blur-md flex items-center gap-3 min-w-[300px] ${
            t.type === 'error' ? 'bg-red-500/10 text-red-500 dark:text-red-400 shadow-red-500/10' :
            t.type === 'success' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 shadow-emerald-500/10' :
            'bg-surface/90 text-content shadow-black/10'
          }`}
        >
          <div className={`w-2 h-2 rounded-full ${
            t.type === 'error' ? 'bg-red-500' : t.type === 'success' ? 'bg-emerald-500' : 'bg-primary'
          }`} />
          <span className="text-sm font-medium">{t.message}</span>
        </motion.div>
      ))}
    </AnimatePresence>
  </div>
);

export const AuthModal = ({ onLogin, onCancel }: { onLogin: (key: string) => void, onCancel?: () => void }) => {
  const [key, setKey] = useState('');
  return (
    <motion.div 
      initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
    >
      <motion.div 
        initial={{ scale: 0.95, y: 10 }} animate={{ scale: 1, y: 0 }}
        className="bg-surface p-8 rounded-2xl w-full max-w-sm shadow-2xl relative overflow-hidden"
      >
        <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-primary to-emerald-500"></div>
        <div className="flex justify-center mb-6">
          <div className="p-4 rounded-full bg-primary/10 text-primary">
            <Lock size={32} />
          </div>
        </div>
        <h2 className="text-xl font-bold text-center text-content mb-2">需要验证</h2>
        <p className="text-muted text-center text-sm mb-6">请输入 API 密钥以访问配置</p>
        <form onSubmit={(e) => { e.preventDefault(); onLogin(key); }}>
          <input
            type="password"
            autoFocus
            className="w-full bg-surface-hover rounded-lg px-4 py-3 text-center tracking-widest text-content focus:ring-1 focus:ring-primary outline-none transition-all mb-4 font-mono"
            placeholder="API KEY"
            value={key}
            onChange={e => setKey(e.target.value)}
          />
          <motion.button 
            whileHover={{ scale: 1.02 }} whileTap={{ scale: 0.98 }}
            type="submit" 
            className="w-full bg-primary hover:opacity-90 text-primary-foreground font-bold py-3 rounded-lg transition-all shadow-lg shadow-primary/20"
          >
            解锁
          </motion.button>
          {onCancel && (
             <button type="button" onClick={onCancel} className="w-full mt-3 text-muted hover:text-content text-sm py-2 transition-colors">
               取消
             </button>
          )}
        </form>
      </motion.div>
    </motion.div>
  );
};

// --- Theme Switcher Component ---
const ThemeToggle = () => {
  const { theme, setTheme } = useContext(AppContext);
  
  const cycleTheme = () => {
    if (theme === 'system') setTheme('light');
    else if (theme === 'light') setTheme('dark');
    else setTheme('system');
  };

  const Icon = theme === 'dark' ? Moon : theme === 'light' ? Sun : Monitor;

  return (
    <button onClick={cycleTheme} className="flex items-center gap-3 px-4 py-3 w-full rounded-xl text-muted hover:text-content hover:bg-surface-hover/50 transition-all">
      <Icon size={20} />
      <span className="font-medium capitalize">{theme} Theme</span>
    </button>
  );
};

const PageWrapper = ({ children }: { children?: React.ReactNode }) => (
  <motion.div
    initial={{ opacity: 0, y: 20 }}
    animate={{ opacity: 1, y: 0 }}
    exit={{ opacity: 0, y: -20 }}
    transition={{ duration: 0.3 }}
    className="h-full"
  >
    {children}
  </motion.div>
);

// --- Layout ---
const Layout: React.FC<{ children?: React.ReactNode }> = ({ children }) => {
  const location = useLocation();
  const { lang, toggleLang, isAuthenticated, setAuthenticated, isFullscreenMode, toggleFullscreenMode } = useContext(AppContext);
  const isZh = lang === 'zh';

  return (
    <div className="flex min-h-screen bg-background text-content font-sans transition-colors duration-300">
      {/* Sidebar - Hidden in Fullscreen Mode */}
      <AnimatePresence>
        {!isFullscreenMode && (
          <motion.aside 
            initial={{ x: -250, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            exit={{ x: -250, opacity: 0 }}
            className="w-64 fixed h-full hidden md:flex flex-col bg-surface/80 backdrop-blur-xl z-20"
          >
            <div className="p-6">
              <h1 className="text-2xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-primary to-emerald-500 tracking-tight">IDRD</h1>
              <p className="text-xs text-muted mt-1 font-mono tracking-wide">SYSTEM V2.0</p>
            </div>
            <nav className="flex-1 p-4 space-y-2">
              <SidebarItem to="/" icon={LayoutDashboard} label={isZh ? "仪表盘" : "Dashboard"} active={location.pathname === '/'} />
              <SidebarItem to="/config" icon={Settings} label={isZh ? "系统配置" : "Configuration"} active={location.pathname === '/config'} />
            </nav>
            <div className="p-4 space-y-2">
              <ThemeToggle />
              <button onClick={toggleLang} className="flex items-center gap-3 px-4 py-3 w-full rounded-xl text-muted hover:text-content hover:bg-surface-hover/50 transition-all">
                <Languages size={20} />
                <span className="font-medium">{lang === 'zh' ? 'English' : '中文'}</span>
              </button>
              {isAuthenticated && (
                <button onClick={() => { localStorage.removeItem('idrd_api_key'); setAuthenticated(false); }} className="flex items-center gap-3 px-4 py-3 w-full rounded-xl text-red-500 hover:text-red-600 hover:bg-red-500/10 transition-all">
                  <LogOut size={20} />
                  <span className="font-medium">{isZh ? '清除凭证' : 'Logout'}</span>
                </button>
              )}
            </div>
          </motion.aside>
        )}
      </AnimatePresence>

      {/* Mobile Header */}
      {!isFullscreenMode && (
        <div className="md:hidden fixed top-0 w-full z-20 bg-surface/80 backdrop-blur-lg px-6 py-4 flex justify-between items-center shadow-sm">
          <span className="font-bold text-xl text-primary">IDRD</span>
          <div className="flex gap-4">
              <a href="#/" className={location.pathname === '/' ? 'text-primary' : 'text-muted'}><LayoutDashboard size={24}/></a>
              <a href="#/config" className={location.pathname === '/config' ? 'text-primary' : 'text-muted'}><Settings size={24}/></a>
          </div>
        </div>
      )}

      {/* Main Content */}
      <motion.main 
        layout
        className={`flex-1 p-6 md:p-10 pt-24 md:pt-10 w-full min-h-screen transition-all duration-300 ${!isFullscreenMode ? 'md:ml-64 max-w-7xl mx-auto' : 'ml-0 max-w-full'}`}
      >
        {/* Fullscreen Exit Button (Floating) */}
        <AnimatePresence>
          {isFullscreenMode && (
            <motion.button
              initial={{ opacity: 0, y: -20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20 }}
              onClick={toggleFullscreenMode}
              className="fixed top-6 right-6 z-50 p-2 bg-surface/50 hover:bg-surface backdrop-blur-md rounded-full text-muted hover:text-content transition-all shadow-lg"
            >
              <Minimize2 size={24} />
            </motion.button>
          )}
        </AnimatePresence>
        
        <AnimatePresence mode='wait'>
          <Routes location={location} key={location.pathname}>
            <Route path="/" element={
              <PageWrapper>
                <Dashboard />
              </PageWrapper>
            } />
            <Route path="/config" element={
              <PageWrapper>
                <ConfigPage />
              </PageWrapper>
            } />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </AnimatePresence>
      </motion.main>
    </div>
  );
};

// --- App Root ---
const App = () => {
  const [lang, setLang] = useState<Language>(() => (localStorage.getItem('idrd_lang') as Language) || 'zh');
  const [theme, setTheme] = useState<ThemeMode>(() => (localStorage.getItem('idrd_theme') as ThemeMode) || 'system');
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const [isAuthenticated, setAuthenticated] = useState(!!localStorage.getItem('idrd_api_key'));
  const [isFullscreenMode, setIsFullscreenMode] = useState(false);

  // Theme Logic
  useEffect(() => {
    const root = window.document.documentElement;
    const applyTheme = (t: ThemeMode) => {
      if (t === 'system') {
        const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        root.classList.toggle('dark', systemDark);
      } else {
        root.classList.toggle('dark', t === 'dark');
      }
      localStorage.setItem('idrd_theme', t);
    };
    
    applyTheme(theme);

    // Listener for system changes if in system mode
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const handleChange = () => {
      if (theme === 'system') applyTheme('system');
    };
    mediaQuery.addEventListener('change', handleChange);
    return () => mediaQuery.removeEventListener('change', handleChange);
  }, [theme]);

  const toggleLang = () => {
    const newLang = lang === 'zh' ? 'en' : 'zh';
    setLang(newLang);
    localStorage.setItem('idrd_lang', newLang);
  };

  const showToast = (message: string, type: 'success' | 'error' | 'info' = 'success') => {
    const id = Date.now().toString();
    setToasts(prev => [...prev, { id, message, type }]);
    setTimeout(() => setToasts(prev => prev.filter(t => t.id !== id)), 4000);
  };

  const toggleFullscreenMode = () => {
    if (!isFullscreenMode) {
      document.documentElement.requestFullscreen().catch((e) => {
        console.error(e);
      });
      setIsFullscreenMode(true);
    } else {
      if (document.fullscreenElement) {
        document.exitFullscreen();
      }
      setIsFullscreenMode(false);
    }
  };

  // Sync fullscreen state with ESC key or browser controls
  useEffect(() => {
    const handleFsChange = () => {
      if (!document.fullscreenElement) setIsFullscreenMode(false);
      else setIsFullscreenMode(true);
    };
    document.addEventListener('fullscreenchange', handleFsChange);
    return () => document.removeEventListener('fullscreenchange', handleFsChange);
  }, []);

  return (
    <AppContext.Provider value={{ 
      lang, toggleLang, 
      theme, setTheme,
      showToast, 
      isAuthenticated, setAuthenticated,
      isFullscreenMode, toggleFullscreenMode
    }}>
      <ToastContainer toasts={toasts} />
      <HashRouter>
        <Layout>
          {/* Children handled by routing inside Layout */}
        </Layout>
      </HashRouter>
    </AppContext.Provider>
  );
};

export default App;