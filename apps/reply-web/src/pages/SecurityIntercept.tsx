import { useState } from 'react'
import { gql } from '@apollo/client'
import { useQuery, useMutation, useSubscription } from '@apollo/client/react'
import { ShieldAlert, ShieldCheck, Lock, AlertTriangle, ExternalLink } from 'lucide-react'

const GET_AGENTS = gql`
  query GetAgentsForIntercept {
    agents {
      id
      name
      department
      status
      allowedTools
      targetModel
    }
  }
`

const INVOKE_TOOL = gql`
  mutation InvokeToolForIntercept($input: InvokeToolInput!) {
    invokeTool(input: $input) {
      executed
      cached
      result
    }
  }
`

const AUDIT_ENTRY_ADDED = gql`
  subscription AuditEntryAddedForIntercept {
    auditEntryAdded {
      id
      tenantId
      sessionId
      toolName
      decision
      prevHash
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
  allowedTools: string[]
  targetModel: string
}

interface AgentsData {
  agents: Agent[]
}

interface AuditEntry {
  id: string
  tenantId: string
  sessionId: string
  toolName: string | null
  decision: string
  prevHash: string
  rowHash: string
  createdAt: string
}

interface AuditSubscriptionData {
  auditEntryAdded: AuditEntry
}

interface InvokeToolData {
  invokeTool: {
    executed: boolean
    cached: boolean
    result: Record<string, unknown>
  }
}

interface InterceptResult {
  timestamp: string
  agentName: string
  toolName: string
  outcome: 'blocked' | 'executed'
  errorMessage: string | null
  real: true
}

const UNAUTHORIZED_TOOL = 'issue_refund'

export const SecurityIntercept = () => {
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null)
  const [results, setResults] = useState<InterceptResult[]>([])
  const [liveAudit, setLiveAudit] = useState<AuditEntry[]>([])
  const [sessionId] = useState(() => `intercept-${Date.now()}`)

  const { data: agentsData, loading, error } = useQuery(GET_AGENTS)

  const [invokeTool, { loading: invoking }] = useMutation(INVOKE_TOOL)

  useSubscription(AUDIT_ENTRY_ADDED, {
    onData: ({ data: subData }) => {
      const typed = subData.data as AuditSubscriptionData | undefined
      if (!typed?.auditEntryAdded) return
      setLiveAudit((prev) => [typed.auditEntryAdded, ...prev.slice(0, 19)])
    },
  })

  const selectedAgent = (agentsData as AgentsData | undefined)?.agents.find(
    (a) => a.id === selectedAgentId
  )

  const handleAttempt = async () => {
    if (!selectedAgent) return

    const deviceId = localStorage.getItem('chrono_device_id') || 'web-device-fallback'
    const toolIsAllowed = selectedAgent.allowedTools.includes(UNAUTHORIZED_TOOL)

    try {
      const res = await invokeTool({
        variables: {
          input: {
            agentId: selectedAgent.id,
            sessionId,
            deviceId,
            toolName: UNAUTHORIZED_TOOL,
            params: {
              order_id: 'ORD-9999',
              amount: 50.0,
              reason: 'simulated unauthorized attempt',
            },
          },
        },
      })

      const data = res.data as InvokeToolData | undefined
      if (data?.invokeTool) {
        setResults((prev) => [
          {
            timestamp: new Date().toLocaleTimeString(),
            agentName: selectedAgent.name,
            toolName: UNAUTHORIZED_TOOL,
            outcome: 'executed',
            errorMessage: toolIsAllowed
              ? null
              : 'Unexpected: tool was allowed',
            real: true,
          },
          ...prev,
        ])
      }
    } catch (err: any) {
      setResults((prev) => [
        {
          timestamp: new Date().toLocaleTimeString(),
          agentName: selectedAgent.name,
          toolName: UNAUTHORIZED_TOOL,
          outcome: 'blocked',
          errorMessage: err.message,
          real: true,
        },
        ...prev,
      ])
    }
  }

  if (loading && !agentsData) {
    return (
      <div className="min-h-screen bg-slate-950 flex items-center justify-center text-slate-400 font-mono text-sm">
        Loading agent registry...
      </div>
    )
  }
  if (error) {
    return (
      <div className="min-h-screen bg-slate-950 flex items-center justify-center text-red-400 font-mono text-sm">
        Error: {error.message}
      </div>
    )
  }

  const agents = (agentsData as AgentsData)?.agents || []
  const toolIsAllowed = selectedAgent?.allowedTools.includes(UNAUTHORIZED_TOOL) ?? false

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 p-6 font-mono">
      <header className="border-b border-slate-800 pb-4 mb-6">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-red-950/40 border border-red-900/60 rounded-lg text-red-400">
            <ShieldAlert className="w-6 h-6" />
          </div>
          <div>
            <h1 className="text-xl font-black tracking-tighter text-white">
              SECURITY INTERCEPT // TOOL GATEWAY
            </h1>
            <p className="text-[10px] text-slate-400 uppercase tracking-widest">
              Prove the Ed25519 capability token blocks unauthorized actions — every event is REAL
            </p>
          </div>
        </div>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: agent selector */}
        <div className="space-y-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
            <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">
              Target Agent
            </h2>
            <div className="space-y-2">
              {agents.map((agent) => (
                <button
                  key={agent.id}
                  onClick={() => setSelectedAgentId(agent.id)}
                  className={`w-full text-left p-3 rounded-lg border transition-all ${
                    selectedAgentId === agent.id
                      ? 'border-emerald-500/60 bg-emerald-950/20'
                      : 'border-slate-800 bg-slate-950 hover:border-slate-700'
                  }`}
                >
                  <div className="flex justify-between items-center">
                    <span className="text-sm font-bold text-slate-200">{agent.name}</span>
                    <span className="text-[9px] uppercase px-2 py-0.5 rounded bg-slate-800 text-slate-400">
                      {agent.department}
                    </span>
                  </div>
                  <div className="mt-2 text-[10px] text-slate-500">
                    allowedTools: {agent.allowedTools.join(', ')}
                  </div>
                </button>
              ))}
            </div>
          </div>

          {selectedAgent && (
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
              <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">
                Attempt Unauthorized Tool
              </h2>
              <div className="bg-slate-950 border border-slate-800 rounded-lg p-3 mb-3 text-[11px]">
                <div className="flex justify-between mb-1">
                  <span className="text-slate-500">Tool:</span>
                  <span className={`font-bold ${toolIsAllowed ? 'text-emerald-400' : 'text-red-400'}`}>
                    {UNAUTHORIZED_TOOL}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">In allowedTools:</span>
                  <span className={`font-bold ${toolIsAllowed ? 'text-emerald-400' : 'text-red-400'}`}>
                    {toolIsAllowed ? 'YES' : 'NO'}
                  </span>
                </div>
              </div>
              <button
                onClick={handleAttempt}
                disabled={invoking}
                className="w-full bg-red-600 hover:bg-red-500 text-white font-bold py-2.5 rounded-lg text-xs uppercase tracking-wider transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
              >
                <Lock className="w-3.5 h-3.5" />
                {invoking ? 'Attempting...' : 'Attempt Unauthorized Invocation'}
              </button>
              <p className="text-[10px] text-slate-500 mt-2 leading-relaxed">
                The backend will issue an Ed25519 capability token scoped to{' '}
                <code className="text-slate-400">{selectedAgent.allowedTools.join(', ')}</code>. The
                Gateway will reject <code className="text-slate-400">{UNAUTHORIZED_TOOL}</code>.
              </p>
            </div>
          )}
        </div>

        {/* Center: results */}
        <div className="lg:col-span-2 space-y-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400">
                Intercept Results
              </h2>
              <span className="text-[9px] font-bold px-2 py-0.5 rounded bg-emerald-950 text-emerald-400 border border-emerald-900/60">
                REAL
              </span>
            </div>

            {results.length === 0 ? (
              <div className="text-center text-slate-600 text-xs py-12">
                Select an agent and attempt an unauthorized tool to see the Gateway block it.
              </div>
            ) : (
              <div className="space-y-2">
                {results.map((r, i) => (
                  <div
                    key={i}
                    className={`p-3 rounded-lg border ${
                      r.outcome === 'blocked'
                        ? 'bg-red-950/20 border-red-900/60'
                        : 'bg-amber-950/20 border-amber-900/60'
                    }`}
                  >
                    <div className="flex items-start justify-between mb-2">
                      <div className="flex items-center gap-2">
                        {r.outcome === 'blocked' ? (
                          <ShieldCheck className="w-4 h-4 text-red-400" />
                        ) : (
                          <AlertTriangle className="w-4 h-4 text-amber-400" />
                        )}
                        <span className="text-xs font-bold text-slate-200">
                          {r.outcome === 'blocked' ? 'BLOCKED' : 'EXECUTED'}
                        </span>
                      </div>
                      <span className="text-[10px] text-slate-500">{r.timestamp}</span>
                    </div>
                    <div className="text-[11px] space-y-1">
                      <div className="flex justify-between">
                        <span className="text-slate-500">Agent:</span>
                        <span className="text-slate-300">{r.agentName}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-slate-500">Tool:</span>
                        <span className="text-slate-300">{r.toolName}</span>
                      </div>
                      {r.errorMessage && (
                        <div className="mt-2 p-2 bg-slate-950 border border-slate-800 rounded text-[10px] text-red-300 font-mono break-all">
                          {r.errorMessage}
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400">
                Live Audit Trail (Real-time)
              </h2>
              <a
                href="/audit"
                className="text-[10px] text-emerald-400 hover:text-emerald-300 flex items-center gap-1"
              >
                View Full Chain <ExternalLink className="w-3 h-3" />
              </a>
            </div>

            {liveAudit.length === 0 ? (
              <div className="text-center text-slate-600 text-xs py-8">
                Waiting for audit events... Attempt a tool invocation above.
              </div>
            ) : (
              <div className="space-y-1 max-h-[300px] overflow-y-auto">
                {liveAudit.map((entry) => (
                  <div
                    key={entry.id}
                    className={`p-2 rounded border text-[10px] font-mono ${
                      entry.decision === 'allowed'
                        ? 'bg-emerald-950/10 border-emerald-900/40'
                        : entry.decision === 'blocked'
                        ? 'bg-red-950/10 border-red-900/40'
                        : 'bg-slate-950 border-slate-800'
                    }`}
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-slate-500">#{entry.id}</span>
                      <span
                        className={`font-bold ${
                          entry.decision === 'allowed'
                            ? 'text-emerald-400'
                            : entry.decision === 'blocked'
                            ? 'text-red-400'
                            : 'text-slate-400'
                        }`}
                      >
                        {entry.decision.toUpperCase()}
                      </span>
                    </div>
                    <div className="flex justify-between text-slate-400">
                      <span>tool: {entry.toolName || 'system'}</span>
                      <span>row_hash: {entry.rowHash.slice(0, 12)}...</span>
                    </div>
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