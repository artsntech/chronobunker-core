import { useEffect, useState } from 'react'
import { Cpu, MemoryStick, Activity, Gauge } from 'lucide-react'
import { fetchRuntimeMetrics, type RuntimeMetrics } from '../lib/metrics'

export const RuntimePanel = () => {
  const [metrics, setMetrics] = useState<RuntimeMetrics | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    const poll = async () => {
      try {
        const m = await fetchRuntimeMetrics()
        if (!cancelled) {
          setMetrics(m)
          setError(null)
        }
      } catch (err: any) {
        if (!cancelled) setError(err.message)
      }
    }

    poll()
    const interval = setInterval(poll, 3000)
    return () => {
      cancelled = true
      clearInterval(interval)
    }
  }, [])

  if (error) {
    return (
      <div className="bg-slate-900 border border-red-900/50 rounded-xl p-4 text-xs text-red-400 font-mono">
        RUNTIME METRICS UNAVAILABLE: {error}
      </div>
    )
  }

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400 flex items-center gap-2">
          <Cpu className="w-3.5 h-3.5 text-emerald-400" />
          Go Runtime Telemetry
        </h2>
        <span className="text-[9px] text-slate-500 font-mono">polling 3s</span>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <MetricCell
          icon={<Activity className="w-3.5 h-3.5" />}
          label="Goroutines"
          value={metrics?.goroutines.toFixed(0) ?? '—'}
          color="text-emerald-400"
        />
        <MetricCell
          icon={<MemoryStick className="w-3.5 h-3.5" />}
          label="Heap Alloc"
          value={metrics ? `${metrics.heapAllocMB.toFixed(1)} MB` : '—'}
          color="text-cyan-400"
        />
        <MetricCell
          icon={<Gauge className="w-3.5 h-3.5" />}
          label="GC Pause Avg"
          value={metrics ? `${metrics.gcPauseMs.toFixed(2)} ms` : '—'}
          color={metrics && metrics.gcPauseMs > 5 ? 'text-amber-400' : 'text-slate-300'}
        />
        <MetricCell
          icon={<Cpu className="w-3.5 h-3.5" />}
          label="Heap Objects"
          value={metrics ? metrics.heapObjects.toLocaleString() : '—'}
          color="text-slate-300"
        />
      </div>
    </div>
  )
}

const MetricCell = ({ icon, label, value, color }: { icon: React.ReactNode; label: string; value: string; color: string }) => (
  <div className="bg-slate-950 border border-slate-800/60 rounded-lg p-3">
    <div className="flex items-center gap-1.5 text-slate-500 mb-1">
      {icon}
      <span className="text-[9px] uppercase font-bold tracking-wider">{label}</span>
    </div>
    <div className={`text-sm font-bold font-mono ${color}`}>{value}</div>
  </div>
)