import { Activity, ShieldAlert, Zap, Database, Terminal, LogOut } from 'lucide-react'
import { useEffect, useState } from 'react'
import { AttentionBar, type Alert } from '../components/AttentionBar'

export type CockpitView = 'monitor' | 'intercept' | 'breaker' | 'isolation' | 'audit'

interface CockpitProps {
  view: CockpitView
  onNavigate: (view: CockpitView) => void
  onLogout: () => void
  children: React.ReactNode
  threatLevel: 'NOMINAL' | 'ELEVATED' | 'CRITICAL'
  alerts: Alert[]
}

const NAV_ITEMS: { key: CockpitView; label: string; icon: React.ReactNode }[] = [
  { key: 'monitor', label: 'MONITOR', icon: <Activity className="w-4 h-4" /> },
  { key: 'intercept', label: 'INTERCEPT', icon: <ShieldAlert className="w-4 h-4" /> },
  { key: 'breaker', label: 'BREAKER', icon: <Zap className="w-4 h-4" /> },
  { key: 'isolation', label: 'ISOLATION', icon: <Database className="w-4 h-4" /> },
  { key: 'audit', label: 'AUDIT', icon: <Terminal className="w-4 h-4" /> },
]

export const Cockpit = ({ view, onNavigate, onLogout, children, threatLevel, alerts }: CockpitProps) => {
  const [uptime, setUptime] = useState('00:00:00')

  useEffect(() => {
    const start = Date.now()
    const interval = setInterval(() => {
      const elapsed = Math.floor((Date.now() - start) / 1000)
      const h = String(Math.floor(elapsed / 3600)).padStart(2, '0')
      const m = String(Math.floor((elapsed % 3600) / 60)).padStart(2, '0')
      const s = String(elapsed % 60).padStart(2, '0')
      setUptime(`${h}:${m}:${s}`)
    }, 1000)
    return () => clearInterval(interval)
  }, [])

  const threatColor = threatLevel === 'NOMINAL'
    ? 'text-emerald-400 border-emerald-500/40'
    : threatLevel === 'ELEVATED'
    ? 'text-amber-400 border-amber-500/40'
    : 'text-red-500 border-red-500/40 animate-pulse'

  return (
    <div className="min-h-screen bg-[#030a16] text-slate-100 font-mono relative overflow-hidden">
      <div className="fixed inset-0 pointer-events-none opacity-[0.03]">
        <svg width="100%" height="100%">
          <defs>
            <pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse">
              <path d="M 40 0 L 0 0 0 40" fill="none" stroke="#38bdf8" strokeWidth="0.5" />
            </pattern>
          </defs>
          <rect width="100%" height="100%" fill="url(#grid)" />
        </svg>
      </div>

      <aside className="fixed left-0 top-0 h-full w-16 lg:w-48 bg-slate-950/80 backdrop-blur-md border-r border-slate-800/60 z-40 flex flex-col">
        <div className="p-3 border-b border-slate-800/60">
          <div className="text-[10px] font-black tracking-widest text-emerald-500 uppercase">CHRONO</div>
          <div className="text-[8px] text-slate-600 tracking-wider">BUNKER // v1.0</div>
        </div>

        <nav className="flex-1 py-4 space-y-1">
          {NAV_ITEMS.map((item) => (
            <button
              key={item.key}
              onClick={() => onNavigate(item.key)}
              className={`w-full flex items-center gap-3 px-3 py-2.5 text-[10px] font-bold tracking-wider transition-all ${
                view === item.key
                  ? 'bg-emerald-500/10 text-emerald-400 border-l-2 border-emerald-500'
                  : 'text-slate-500 hover:text-slate-300 hover:bg-slate-900/50 border-l-2 border-transparent'
              }`}
            >
              {item.icon}
              <span className="hidden lg:inline">{item.label}</span>
            </button>
          ))}
        </nav>

        <button
          onClick={onLogout}
          className="p-3 border-t border-slate-800/60 text-slate-600 hover:text-red-400 transition-colors flex items-center gap-3"
        >
          <LogOut className="w-4 h-4" />
          <span className="hidden lg:inline text-[10px] font-bold tracking-wider">LOGOUT</span>
        </button>
      </aside>

      <div className="lg:ml-48 ml-16">
        <header className="border-b border-slate-800/60 bg-slate-950/60 backdrop-blur-md px-6 py-3 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div>
              <div className="text-[9px] text-slate-600 tracking-widest uppercase">Operation</div>
              <div className="text-xs font-black tracking-tighter text-white">CHRONOBUNKER</div>
            </div>
            <div className={`px-3 py-1 border rounded text-[9px] font-black tracking-widest uppercase ${threatColor}`}>
              THREAT: {threatLevel}
            </div>
          </div>

          <div className="flex items-center gap-6">
            <div className="text-right">
              <div className="text-[9px] text-slate-600 tracking-widest uppercase">Uptime</div>
              <div className="text-xs font-bold text-emerald-400 tabular-nums">{uptime}</div>
            </div>
            <div className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
              <span className="text-[9px] text-emerald-400 tracking-widest uppercase">LINK ACTIVE</span>
            </div>
          </div>
        </header>

        <AttentionBar 
        alerts={alerts} 
        onNavigate={(v) => onNavigate(v as CockpitView)} 
        />

        <main className="p-6 pb-24">{children}</main>
      </div>
    </div>
  )
}