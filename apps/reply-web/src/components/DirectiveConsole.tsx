import { useState } from 'react'
import { ShieldX } from 'lucide-react'

const MAX_LENGTH = 500

interface DirectiveConsoleProps {
  onExecute: (prompt: string) => Promise<void>
  disabled: boolean
  loading: boolean
  domain: string
}

export const DirectiveConsole = ({ onExecute, disabled, loading, domain }: DirectiveConsoleProps) => {
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!value.trim() || disabled || loading) return
    setError(null)
    try {
      await onExecute(value)
      setValue('')
    } catch (err: any) {
      setError(err.message)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-2">
      <div className="bg-slate-950 border border-slate-800 rounded-lg p-2">
        <span className="text-[9px] text-slate-500 uppercase tracking-wider">Allowed Domain</span>
        <p className="text-[10px] text-slate-400 mt-0.5">{domain}</p>
      </div>

      <div className="relative">
        <textarea
          value={value}
          onChange={(e) => setValue(e.target.value)}
          disabled={disabled || loading}
          placeholder="Ask about orders, proofs, formats, or refunds..."
          className={`w-full bg-slate-950 border rounded-lg p-3 font-mono text-xs text-slate-200 focus:outline-none resize-none h-20 transition-colors ${
            error ? 'border-red-500/60' : 'border-slate-800 focus:border-emerald-500'
          }`}
        />
        <div className={`absolute bottom-2 right-2 text-[9px] font-mono ${
          value.length > MAX_LENGTH ? 'text-red-500' : 'text-slate-600'
        }`}>
          {value.length}/{MAX_LENGTH}
        </div>
      </div>

      {error && (
        <div className="flex items-center gap-2 px-2 py-1.5 bg-red-950/30 border border-red-900/50 rounded text-[10px] text-red-400 font-mono">
          <ShieldX className="w-3 h-3" />
          <span>REJECTED: {error}</span>
        </div>
      )}

      <button
        type="submit"
        disabled={!value.trim() || disabled || loading}
        className="w-full bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold py-2 rounded-lg text-xs uppercase tracking-wider transition-colors disabled:opacity-40"
      >
        {loading ? 'Validating intent...' : 'COMMIT EXECUTION'}
      </button>
    </form>
  )
}