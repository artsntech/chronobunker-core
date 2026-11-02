import { motion } from 'framer-motion'

interface TelemetryPanelProps {
  title: string
  status?: 'nominal' | 'elevated' | 'critical'
  children: React.ReactNode
  className?: string
}

export const TelemetryPanel = ({ title, status = 'nominal', children, className = '' }: TelemetryPanelProps) => {
  const borderColor = status === 'nominal'
    ? 'border-slate-800/60'
    : status === 'elevated'
    ? 'border-amber-500/40 shadow-[0_0_15px_rgba(245,158,11,0.1)]'
    : 'border-red-500/40 shadow-[0_0_20px_rgba(239,68,68,0.15)] animate-pulse-slow'

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.2, ease: 'easeOut' }}
      className={`bg-slate-950/60 backdrop-blur-sm border rounded-lg ${borderColor} ${className}`}
    >
      <div className="px-4 py-2 border-b border-slate-800/40 flex items-center justify-between">
        <span className="text-[9px] font-black tracking-widest text-slate-500 uppercase">{title}</span>
        {status !== 'nominal' && (
          <span className={`text-[8px] font-black tracking-widest uppercase ${status === 'critical' ? 'text-red-500' : 'text-amber-400'}`}>
            {status === 'critical' ? 'ALERT' : 'ELEVATED'}
          </span>
        )}
      </div>
      <div className="p-4">{children}</div>
    </motion.div>
  )
}