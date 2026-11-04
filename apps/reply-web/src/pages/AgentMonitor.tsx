import { useState, useMemo } from 'react'
import { gql } from '@apollo/client'
import { useQuery, useMutation, useSubscription } from '@apollo/client/react'
import { Bot, Activity, Zap, ShieldAlert, LogOut } from 'lucide-react'
import { RuntimePanel } from '../components/RuntimePanel'
import { LatencyChart } from '../components/LatencyChart'
import { DirectiveConsole } from '../components/DirectiveConsole'

const GET_AGENTS_AND_COSTS = gql`
  query GetAgentsAndCosts($from: Time!, $to: Time!) {
    agents {
      id
      name
      department
      status
      healthScore
      allowedTools
      targetModel
      breakerState
    }
    costs(from: $from, to: $to) {
      totalUsd
      eventCount
    }
  }
`

const INVOKE_AGENT = gql`
  mutation InvokeAgent($input: InvokeAgentInput!) {
    invokeAgent(input: $input) {
      completion
      modelId
      costUsd
      latencyMs
    }
  }
`

const UPDATE_AGENT_STATUS = gql`
  mutation UpdateAgentStatus($input: UpdateAgentStatusInput!) {
    updateAgentStatus(input: $input) {
      id
      status
    }
  }
`

const AUDIT_ENTRY_ADDED = gql`
  subscription AuditEntryAdded {
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
  healthScore: number
  allowedTools: string[]
  targetModel: string
  breakerState: string
}

interface CostSummary {
  totalUsd: number
  eventCount: number
}

interface AgentsAndCostsData {
  agents: Agent[]
  costs: CostSummary
}

interface InvokeAgentResult {
  completion: string
  modelId: string
  costUsd: number
  latencyMs: number
}

interface InvokeAgentData {
  invokeAgent: InvokeAgentResult
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

interface AgentMonitorProps {
  onLogout: () => void
}

export const AgentMonitor = ({ onLogout }: AgentMonitorProps) => {
  const [activeAgentId, setActiveAgentId] = useState<string | null>(null)
  const [latencyHistory, setLatencyHistory] = useState<number[]>([])
  const [liveAudit, setLiveAudit] = useState<AuditEntry[]>([])
  const [consoleLogs, setConsoleLogs] = useState<string[]>([
    '[SYSTEM] ChronoBunker node initialized. Active session secured.',
  ])

  const variables = useMemo(() => {
    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)
    return {
      from: yesterday.toISOString(),
      to: now.toISOString(),
    }
  }, [])

  const { data, loading, error, refetch } = useQuery(GET_AGENTS_AND_COSTS, {
    variables,
    pollInterval: 10000,
  })

  const [invokeAgent, { loading: promptLoading }] = useMutation(INVOKE_AGENT)
  const [updateAgentStatus] = useMutation(UPDATE_AGENT_STATUS)

  useSubscription(AUDIT_ENTRY_ADDED, {
    onData: ({ data: subData }) => {
      const typed = subData.data as AuditSubscriptionData | undefined
      if (!typed?.auditEntryAdded) return
      setLiveAudit((prev) => [typed.auditEntryAdded, ...prev.slice(0, 49)])
    },
  })

  const addLog = (msg: string) => {
    setConsoleLogs((prev) => [`[${new Date().toLocaleTimeString()}] ${msg}`, ...prev.slice(0, 50)])
  }

  const handleKillSwitch = async (agentId: string, currentStatus: string) => {
    const nextStatus = currentStatus === 'killed' ? 'production' : 'killed'
    addLog(`[KILL SWITCH] Changing agent {${agentId}} status to ${nextStatus}...`)
    try {
      await updateAgentStatus({
        variables: {
          input: { id: agentId, status: nextStatus },
        },
      })
      addLog(`[SYSTEM] Agent status updated successfully.`)
      refetch()
    } catch (err: any) {
      addLog(`[SYSTEM ERROR] Kill Switch failed: ${err.message}`)
    }
  }

  if (loading && !data) {
    return (
      <div className="min-h-screen bg-slate-950 flex items-center justify-center text-slate-400">
        Loading cluster telemetry...
      </div>
    )
  }

  if (error) {
    return (
      <div className="min-h-screen bg-slate-950 flex items-center justify-center text-red-400">
        Cluster connection error: {error.message}
      </div>
    )
  }

  const typedData = data as AgentsAndCostsData | undefined
  const agents = typedData?.agents || []
  const costSummary = typedData?.costs
  const activeAgent = agents.find((a) => a.id === activeAgentId)

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 p-6">
      <header className="flex flex-col md:flex-row md:items-center justify-between border-b border-slate-800 pb-4 mb-6 gap-4">
        <div>
          <h1 className="text-xl font-bold tracking-tight flex items-center gap-2">
            <Activity className="text-emerald-400 w-5 h-5 animate-pulse" />
            AgentOps Monitor & Control Registry
          </h1>
          <p className="text-xs text-slate-400">Boundary Enforcement: Row-Level Security Database Isolation Active</p>
        </div>

        <div className="flex gap-4 items-center">
          <div className="bg-slate-900 border border-slate-800 rounded-lg px-4 py-2 text-right">
            <span className="block text-[10px] text-slate-400 font-semibold uppercase tracking-wider">Ledger Cost (24h)</span>
            <span className="text-sm font-mono font-bold text-teal-400">
              ${costSummary?.totalUsd.toFixed(6) || '0.000000'}
            </span>
          </div>
          <div className="bg-slate-900 border border-slate-800 rounded-lg px-4 py-2 text-right">
            <span className="block text-[10px] text-slate-400 font-semibold uppercase tracking-wider">Invocations</span>
            <span className="text-sm font-mono font-bold text-emerald-400">{costSummary?.eventCount || 0}</span>
          </div>
          <button
            onClick={onLogout}
            className="p-2 bg-slate-900 border border-slate-800 rounded-lg text-slate-400 hover:text-red-400 transition-colors"
            title="Logout"
          >
            <LogOut className="w-4 h-4" />
          </button>
        </div>
      </header>

      <RuntimePanel />

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mt-6">
        <div className="lg:col-span-2 space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {agents.map((agent) => (
              <div
                key={agent.id}
                onClick={() => agent.status !== 'KILLED' && setActiveAgentId(agent.id)}
                className={`border rounded-xl p-5 transition-all cursor-pointer bg-slate-900 ${
                  activeAgentId === agent.id
                    ? 'border-emerald-500 shadow-lg shadow-emerald-500/5'
                    : 'border-slate-800 hover:border-slate-700'
                } ${agent.status === 'KILLED' ? 'opacity-50 cursor-not-allowed' : ''}`}
              >
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-3">
                    <div className={`p-2 rounded-lg ${agent.status === 'KILLED' ? 'bg-red-950 text-red-400' : 'bg-slate-950 text-emerald-400'}`}>
                      <Bot className="w-5 h-5" />
                    </div>
                    <div>
                      <h3 className="font-bold text-sm text-slate-200">{agent.name}</h3>
                      <span className="text-[10px] uppercase font-mono px-2 py-0.5 bg-slate-950 border border-slate-800 rounded text-slate-400">
                        {agent.department}
                      </span>
                    </div>
                  </div>
                  <span className={`text-[9px] font-mono px-2 py-0.5 rounded uppercase font-bold ${
                    agent.status === 'PRODUCTION'
                      ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                      : agent.status === 'KILLED'
                      ? 'bg-red-500/10 text-red-400 border border-red-500/20'
                      : 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                  }`}>
                    {agent.status}
                  </span>
                </div>

                <div className="space-y-2 text-xs font-mono border-t border-slate-800/60 pt-3 text-slate-400">
                  <div className="flex justify-between">
                    <span>Model Router Target:</span>
                    <span className="text-slate-200">{agent.targetModel}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Health Core Score:</span>
                    <span className={agent.healthScore > 0.8 ? 'text-emerald-400' : 'text-amber-400'}>
                      {(agent.healthScore * 100).toFixed(0)}%
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span>Breaker State:</span>
                    <span className={`font-bold ${
                      agent.breakerState === 'closed' ? 'text-emerald-400' :
                      agent.breakerState === 'half_open' ? 'text-amber-400' : 'text-red-500 animate-pulse'
                    }`}>
                      ● {agent.breakerState.toUpperCase()}
                    </span>
                  </div>
                </div>

                <div className="mt-4 pt-3 border-t border-slate-800/60 flex justify-end">
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      handleKillSwitch(agent.id, agent.status)
                    }}
                    className={`text-xs px-3 py-1.5 rounded font-bold flex items-center gap-1 transition-colors ${
                      agent.status === 'KILLED'
                        ? 'bg-emerald-500 text-slate-950 hover:bg-emerald-400'
                        : 'bg-red-600 text-slate-100 hover:bg-red-500'
                    }`}
                  >
                    <ShieldAlert className="w-3.5 h-3.5" />
                    <span>{agent.status === 'KILLED' ? 'Revive Agent' : 'Emergency Kill'}</span>
                  </button>
                </div>
              </div>
            ))}
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6">
            <h2 className="text-sm font-bold uppercase tracking-wider text-slate-400 mb-4 flex items-center gap-2">
              <Zap className="text-amber-400 w-4 h-4" />
              Model Router Terminal Endpoint
            </h2>
            <DirectiveConsole
              onExecute={async (prompt) => {
                if (!activeAgentId) return
                addLog(`Invoking agent {${activeAgentId}} with prompt: "${prompt}"...`)
                try {
                  const result = await invokeAgent({
                    variables: {
                      input: {
                        agentId: activeAgentId,
                        prompt,
                        sessionId: `web-session-${Date.now()}`,
                      },
                    },
                  })
                  const invokeData = result.data as InvokeAgentData | undefined
                  if (!invokeData?.invokeAgent) return
                  const { completion, modelId, costUsd, latencyMs } = invokeData.invokeAgent
                  setLatencyHistory((prev) => [...prev.slice(-19), latencyMs])
                  addLog(`[RESPONSE] Model: ${modelId} | Latency: ${latencyMs}ms | Cost: $${costUsd.toFixed(6)}`)
                  addLog(`[AGENT] ${completion}`)
                  refetch()
                } catch (err: any) {
                  addLog(`[ERROR] ${err.message}`)
                  throw err
                }
              }}
              disabled={!activeAgentId}
              loading={promptLoading}
              domain={activeAgent?.department === 'support'
                ? 'Order tracking, proofs, file formats (CMYK, DPI, bleed), refunds, delivery'
                : 'Chaos testing and failure injection'}
            />
          </div>

          {latencyHistory.length > 0 && (
            <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
              <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400 mb-3">
                Router Latency (last 20 frames)
              </h2>
              <LatencyChart data={latencyHistory} />
            </div>
          )}
        </div>

        <div className="space-y-6">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
            <h2 className="text-sm font-bold uppercase tracking-wider text-slate-400 mb-4">
              Live Audit Stream
            </h2>
            {liveAudit.length === 0 ? (
              <div className="h-[400px] bg-slate-950 border border-slate-800 rounded p-2 flex items-center justify-center text-[11px] font-mono text-slate-600">
                Waiting for audit events...
              </div>
            ) : (
              <div className="h-[400px] bg-slate-950 border border-slate-800 rounded p-2 overflow-y-auto space-y-1 text-[11px] font-mono">
                {liveAudit.map((entry) => (
                  <div
                    key={entry.id}
                    className={`p-1.5 rounded border ${
                      entry.decision === 'blocked'
                        ? 'bg-red-950/20 text-red-400 border-red-900/30'
                        : entry.decision === 'allowed'
                        ? 'bg-emerald-950/20 text-emerald-400 border-emerald-900/30'
                        : 'bg-slate-950/40 text-slate-400 border-slate-800/40'
                    }`}
                  >
                    <div className="flex justify-between mb-1">
                      <span className="text-slate-500">#{entry.id}</span>
                      <span className={entry.decision === 'blocked' ? 'text-red-400 font-bold' : 'text-emerald-400 font-bold'}>
                        {entry.decision.toUpperCase()}
                      </span>
                    </div>
                    <div className="text-[10px] text-slate-400">
                      tool: {entry.toolName || 'system'}
                    </div>
                    <div className="text-[9px] text-slate-600 font-mono">
                      row_hash: {entry.rowHash.slice(0, 16)}...
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
            <h2 className="text-sm font-bold uppercase tracking-wider text-slate-400 mb-4">Console</h2>
            <div className="h-[300px] bg-slate-950 border border-slate-800 rounded p-2 overflow-y-auto space-y-1 text-[10px] font-mono">
              {consoleLogs.map((log, index) => (
                <div
                  key={index}
                  className={`p-1.5 rounded border border-slate-800/40 ${
                    log.includes('[ERROR]') || log.includes('[KILL')
                      ? 'bg-red-950/20 text-red-400 border-red-900/30'
                      : log.includes('[RESPONSE]')
                      ? 'bg-teal-950/20 text-teal-400 border-teal-900/30'
                      : log.includes('[AGENT]')
                      ? 'bg-slate-950 text-slate-100 border-slate-800'
                      : 'bg-slate-950/40 text-slate-400'
                  }`}
                >
                  {log}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}