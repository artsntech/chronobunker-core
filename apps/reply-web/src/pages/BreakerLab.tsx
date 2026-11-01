import { useState } from 'react'
import { gql } from '@apollo/client'
import { useQuery, useMutation, useSubscription } from '@apollo/client/react'
import { Activity, AlertTriangle, Zap } from 'lucide-react'

const GET_CHAOS_AGENT = gql`
  query GetChaosAgent {
    agents {
      id
      name
      department
      status
      targetModel
      breakerState
    }
  }
`

const INVOKE_AGENT = gql`
  mutation InvokeChaosAgent($input: InvokeAgentInput!) {
    invokeAgent(input: $input) {
      completion
      modelId
      latencyMs
    }
  }
`

const AUDIT_ENTRY_ADDED = gql`
  subscription BreakerAudit {
    auditEntryAdded {
      id
      toolName
      decision
      rowHash
      createdAt
    }
  }
`

interface Agent {
  id: string
  name: string
  department: string
  status: string
  targetModel: string
  breakerState: string
}

interface AgentsData {
  agents: Agent[]
}

interface AuditEntry {
  id: string
  toolName: string | null
  decision: string
  rowHash: string
  createdAt: string
}

interface AuditSubscriptionData {
  auditEntryAdded: AuditEntry
}

interface InvokeResult {
  timestamp: string
  outcome: 'failed' | 'succeeded'
  error: string | null
  latencyMs: number
}

export const BreakerLab = () => {
  const [results, setResults] = useState<InvokeResult[]>([])
  const [liveAudit, setLiveAudit] = useState<AuditEntry[]>([])
  const [sessionId] = useState(() => `chaos-${Date.now()}`)

  const { data: agentsData, loading } = useQuery(GET_CHAOS_AGENT, {
    pollInterval: 2000,
  })

  const [invokeAgent, { loading: invoking }] = useMutation(INVOKE_AGENT)

  useSubscription(AUDIT_ENTRY_ADDED, {
    onData: ({ data: subData }) => {
      const typed = subData.data as AuditSubscriptionData | undefined
      if (!typed?.auditEntryAdded) return
      setLiveAudit((prev) => [typed.auditEntryAdded, ...prev.slice(0, 19)])
    },
  })

  const chaosAgent = (agentsData as AgentsData | undefined)?.agents.find(
    (a) => a.department === 'chaos'
  )

  const handleFire = async () => {
    if (!chaosAgent) return
    const start = Date.now()
    try {
      await invokeAgent({
        variables: {
          input: {
            agentId: chaosAgent.id,
            prompt: 'chaos probe',
            sessionId,
          },
        },
      })
      setResults((prev) => [
        { timestamp: new Date().toLocaleTimeString(), outcome: 'succeeded', error: null, latencyMs: Date.now() - start },
        ...prev,
      ])
    } catch (err: any) {
      setResults((prev) => [
        { timestamp: new Date().toLocaleTimeString(), outcome: 'failed', error: err.message, latencyMs: Date.now() - start },
        ...prev,
      ])
    }
  }

  if (loading && !agentsData) {
    return <div className="min-h-screen bg-slate-950 flex items-center justify-center text-slate-400 font-mono text-sm">Loading chaos agent...</div>
  }

  const failureCount = results.filter((r) => r.outcome === 'failed').length

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 p-6 font-mono">
      <header className="border-b border-slate-800 pb-4 mb-6">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-amber-950/40 border border-amber-900/60 rounded-lg text-amber-400">
            <Activity className="w-6 h-6" />
          </div>
          <div>
            <h1 className="text-xl font-black tracking-tighter text-white">BREAKER LAB // CIRCUIT BREAKER FSM</h1>
            <p className="text-[10px] text-slate-400 uppercase tracking-widest">Force real failures, watch the breaker open — every event is REAL</p>
          </div>
        </div>
      </header>

      <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
  <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">
    Breaker State Monitor
  </h2>
  <div className="text-center py-4">
    <div className={`text-3xl font-black ${
      chaosAgent?.breakerState === 'closed' ? 'text-emerald-400' :
      chaosAgent?.breakerState === 'half_open' ? 'text-amber-400' : 'text-red-500 animate-pulse'
    }`}>
      {chaosAgent?.breakerState.toUpperCase() || 'UNKNOWN'}
    </div>
    <div className="text-[10px] text-slate-500 uppercase mt-1">
      current FSM state (polling 2s)
    </div>
  </div>
</div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="space-y-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
            <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">Chaos Agent</h2>
            {chaosAgent ? (
              <div className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-[11px] space-y-1">
                <div className="flex justify-between"><span className="text-slate-500">Name:</span><span className="text-slate-300">{chaosAgent.name}</span></div>
                <div className="flex justify-between"><span className="text-slate-500">Department:</span><span className="text-amber-400">{chaosAgent.department}</span></div>
                <div className="flex justify-between"><span className="text-slate-500">Target Model:</span><span className="text-red-400 text-[10px]">{chaosAgent.targetModel}</span></div>
              </div>
            ) : (
              <div className="text-red-400 text-xs">Chaos agent not found. Run db-seed.</div>
            )}
            <button
              onClick={handleFire}
              disabled={invoking || !chaosAgent}
              className="w-full mt-3 bg-red-600 hover:bg-red-500 text-white font-bold py-2.5 rounded-lg text-xs uppercase tracking-wider transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
            >
              <Zap className="w-3.5 h-3.5" />
              {invoking ? 'Firing...' : 'Fire Request (Force Failure)'}
            </button>
            <p className="text-[10px] text-slate-500 mt-2 leading-relaxed">
              Each request targets a non-existent model. After 5 consecutive failures, the breaker opens.
            </p>
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
            <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">Failure Counter</h2>
            <div className="text-center">
              <div className={`text-4xl font-black ${failureCount >= 5 ? 'text-red-500 animate-pulse' : 'text-slate-300'}`}>
                {failureCount}
              </div>
              <div className="text-[10px] text-slate-500 uppercase mt-1">consecutive failures</div>
              {failureCount >= 5 && (
                <div className="mt-2 text-[10px] text-red-400 font-bold flex items-center justify-center gap-1">
                  <AlertTriangle className="w-3 h-3" /> BREAKER SHOULD BE OPEN
                </div>
              )}
            </div>
          </div>
        </div>

        <div className="lg:col-span-2 space-y-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400">Invocation Results</h2>
              <span className="text-[9px] font-bold px-2 py-0.5 rounded bg-emerald-950 text-emerald-400 border border-emerald-900/60">REAL</span>
            </div>
            {results.length === 0 ? (
              <div className="text-center text-slate-600 text-xs py-8">Fire a request to begin.</div>
            ) : (
              <div className="space-y-1 max-h-[300px] overflow-y-auto">
                {results.map((r, i) => (
                  <div key={i} className={`p-2 rounded border text-[10px] ${r.outcome === 'failed' ? 'bg-red-950/10 border-red-900/40' : 'bg-emerald-950/10 border-emerald-900/40'}`}>
                    <div className="flex justify-between mb-1">
                      <span className={r.outcome === 'failed' ? 'text-red-400 font-bold' : 'text-emerald-400 font-bold'}>
                        {r.outcome === 'failed' ? 'FAILED' : 'SUCCEEDED'}
                      </span>
                      <span className="text-slate-500">{r.timestamp} · {r.latencyMs}ms</span>
                    </div>
                    {r.error && <div className="text-red-300 break-all">{r.error}</div>}
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
            <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">Live Audit Trail (Breaker Transitions)</h2>
            {liveAudit.length === 0 ? (
              <div className="text-center text-slate-600 text-xs py-6">Waiting for audit events...</div>
            ) : (
              <div className="space-y-1 max-h-[250px] overflow-y-auto">
                {liveAudit.map((e) => (
                  <div key={e.id} className="p-2 rounded border border-slate-800 bg-slate-950 text-[10px] flex justify-between">
                    <span className="text-slate-500">#{e.id}</span>
                    <span className="text-slate-400">{e.toolName || 'system'}</span>
                    <span className={e.decision === 'blocked' ? 'text-red-400' : 'text-emerald-400'}>{e.decision}</span>
                    <span className="text-slate-600 font-mono">{e.rowHash.slice(0, 8)}...</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}