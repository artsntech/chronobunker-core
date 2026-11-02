import { useState } from 'react'
import { gql } from '@apollo/client'
import { useMutation, useSubscription } from '@apollo/client/react'
import { ShieldCheck, ShieldX, Play, AlertTriangle } from 'lucide-react'

const RUN_RED_TEAM = gql`
  mutation RunRedTeam {
    runRedTeam
  }
`

const RED_TEAM_STREAM = gql`
  subscription RedTeamStream {
    redTeamStream {
      id
      name
      owaspCategory
      status
      detail
      durationMs
      ranAt
    }
  }
`

interface RedTeamResult {
  id: string
  name: string
  owaspCategory: string
  status: string
  detail: string
  durationMs: number
  ranAt: string
}

interface RedTeamData {
  redTeamStream: RedTeamResult
}

export const RedTeamPanel = () => {
  const [results, setResults] = useState<Record<string, RedTeamResult>>({})
  const [runRedTeam, { loading }] = useMutation(RUN_RED_TEAM)

  useSubscription(RED_TEAM_STREAM, {
    onData: ({ data: subData }) => {
      const typed = subData.data as RedTeamData | undefined
      if (!typed?.redTeamStream) return
      setResults((prev) => ({ ...prev, [typed.redTeamStream.id]: typed.redTeamStream }))
    },
  })

  const handleRun = async () => {
    setResults({})
    await runRedTeam()
  }

  const resultList = Object.values(results)
  const allPass = resultList.length > 0 && resultList.every((r) => r.status === 'pass')
  const anyFail = resultList.some((r) => r.status === 'fail')

  return (
    <div className="space-y-6">
      <header className="flex items-center justify-between border-b border-slate-800/60 pb-4">
        <div>
          <h1 className="text-xl font-black tracking-tighter text-white flex items-center gap-2">
            <ShieldCheck className="w-5 h-5 text-cyan-400" />
            AUTONOMOUS RED TEAM HARNESS
          </h1>
          <p className="text-[10px] text-slate-400 uppercase tracking-widest">
            Deterministic security scenarios against the live backend. No simulation.
          </p>
        </div>
        <button
          onClick={handleRun}
          disabled={loading}
          className="bg-cyan-500 hover:bg-cyan-400 text-slate-950 font-bold px-4 py-2 rounded-lg text-xs uppercase tracking-wider transition-colors disabled:opacity-50 flex items-center gap-2"
        >
          <Play className="w-3.5 h-3.5" />
          {loading ? 'Running...' : 'Run Full Suite'}
        </button>
      </header>

      {resultList.length === 0 ? (
        <div className="text-center text-slate-600 text-xs py-16 font-mono">
          Click "Run Full Suite" to execute the OWASP-mapped scenarios.
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {resultList.map((r) => (
            <div
              key={r.id}
              className={`p-4 rounded-lg border ${
                r.status === 'running'
                  ? 'bg-slate-900 border-slate-700'
                  : r.status === 'pass'
                  ? 'bg-emerald-950/20 border-emerald-900/60'
                  : 'bg-red-950/20 border-red-900/60'
              }`}
            >
              <div className="flex items-start justify-between mb-2">
                <div>
                  <span className="text-[9px] font-mono text-slate-500">{r.id}</span>
                  <h3 className="text-xs font-bold text-slate-200">{r.name}</h3>
                </div>
                {r.status === 'pass' && <ShieldCheck className="w-4 h-4 text-emerald-400" />}
                {r.status === 'fail' && <ShieldX className="w-4 h-4 text-red-500" />}
                {r.status === 'running' && <AlertTriangle className="w-4 h-4 text-amber-400 animate-pulse" />}
              </div>
              <div className="text-[10px] font-mono text-slate-500 mb-2">
                OWASP: {r.owaspCategory} | {r.durationMs}ms
              </div>
              <p className="text-[10px] text-slate-400">{r.detail}</p>
            </div>
          ))}
        </div>
      )}

      {allPass && (
        <div className="p-4 bg-emerald-950/20 border border-emerald-900/60 rounded-lg text-emerald-400 text-xs font-mono">
          ✓ ALL SCENARIOS PASSED — the backend is safe against the tested OWASP categories.
        </div>
      )}
      {anyFail && (
        <div className="p-4 bg-red-950/20 border border-red-900/60 rounded-lg text-red-400 text-xs font-mono">
          ✗ SECURITY REGRESSION DETECTED — at least one scenario failed. Investigate immediately.
        </div>
      )}
    </div>
  )
}