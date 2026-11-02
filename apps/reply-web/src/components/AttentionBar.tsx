import { AlertTriangle, ShieldAlert, Zap, ChevronRight } from 'lucide-react'

export interface Alert {
  type: 'breaker' | 'rate_limit' | 'security'
  message: string
  view: string
}

interface AttentionBarProps {
  alerts: Alert[]
  onNavigate: (view: string) => void
}

export const AttentionBar = ({ alerts, onNavigate }: AttentionBarProps) => {
  if (alerts.length === 0) {
    return (
      <div className="px-6 py-2 border-b border-slate-800/40 bg-emerald-950/10 flex items-center gap-2">
        <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
        <span className="text-[9px] font-mono text-emerald-500 tracking-widest uppercase">
          All systems nominal
        </span>
      </div>
    )
  }

  return (
    <div className="px-6 py-2 border-b border-red-900/40 bg-red-950/20 flex items-center gap-4 overflow-x-auto">
      <div className="flex items-center gap-1.5 animate-pulse">
        <AlertTriangle className="w-3.5 h-3.5 text-red-500" />
        <span className="text-[9px] font-mono text-red-400 tracking-widest uppercase font-black">
          {alerts.length} Active Alert{alerts.length > 1 ? 's' : ''}
        </span>
      </div>
      {alerts.map((alert, i) => (
        <button
          key={i}
          onClick={() => onNavigate(alert.view)}
          className="flex items-center gap-1.5 text-[9px] font-mono text-red-300 hover:text-white transition-colors whitespace-nowrap"
        >
          {alert.type === 'breaker' && <Zap className="w-3 h-3" />}
          {alert.type === 'security' && <ShieldAlert className="w-3 h-3" />}
          {alert.type === 'rate_limit' && <AlertTriangle className="w-3 h-3" />}
          <span>{alert.message}</span>
          <ChevronRight className="w-3 h-3" />
        </button>
      ))}
    </div>
  )
}