import { useState } from 'react'
import { gql } from '@apollo/client'
import { useMutation, useSubscription } from '@apollo/client/react'
import { Rocket, ShieldAlert, ShieldCheck, DollarSign, Wrench } from 'lucide-react'

const LAUNCH_MISSION = gql`
  mutation LaunchMission($input: LaunchMissionInput!) {
    launchMission(input: $input) {
      missionId
      status
      actionsExecuted
      actionsBlocked
      totalSpentUsd
    }
  }
`

const AUDIT_ENTRY_ADDED = gql`
  subscription MissionAudit {
    auditEntryAdded {
      id
      toolName
      decision
      rowHash
      createdAt
    }
  }
`

interface MissionResult {
  launchMission: {
    missionId: string
    status: string
    actionsExecuted: number
    actionsBlocked: number
    totalSpentUsd: number
  }
}

interface AuditEntry {
  id: string
  toolName: string | null
  decision: string
  rowHash: string
  createdAt: string
}

interface AuditData {
  auditEntryAdded: AuditEntry
}

const AVAILABLE_TOOLS = ['send_email', 'get_order', 'search_help_center', 'issue_refund']

export const MissionLab = () => {
  const [objective, setObjective] = useState('Recover abandoned carts with a 10% discount')
  const [budget, setBudget] = useState(50)
  const [selectedTools, setSelectedTools] = useState<string[]>(['send_email', 'get_order'])
  const [auditLog, setAuditLog] = useState<AuditEntry[]>([])
  const [result, setResult] = useState<MissionResult['launchMission'] | null>(null)

  const [launchMission, { loading }] = useMutation(LAUNCH_MISSION)

  useSubscription(AUDIT_ENTRY_ADDED, {
    onData: ({ data: subData }) => {
      const typed = subData.data as AuditData | undefined
      if (!typed?.auditEntryAdded) return
      setAuditLog((prev) => [typed.auditEntryAdded, ...prev.slice(0, 29)])
    },
  })

  const toggleTool = (tool: string) => {
    setSelectedTools((prev) =>
      prev.includes(tool) ? prev.filter((t) => t !== tool) : [...prev, tool]
    )
  }

    const handleLaunch = async () => {
    if (selectedTools.length === 0) return
    setAuditLog([])
    setResult(null)

    try {
      const res = await launchMission({
        variables: {
          input: {
            objective,
            budgetUsd: budget,
            allowedTools: selectedTools,
            autonomyLevel: 'full',
          },
        },
      })
      const typed = res.data as MissionResult | undefined
      if (typed?.launchMission) {
        setResult(typed.launchMission)
      }
    } catch (err: any) {
      console.error('Mission launch failed:', err)
    }
  }

  return (
    <div className="space-y-6">
      <header>
        <div className="flex items-center gap-3">
          <div className="p-2 bg-purple-100 rounded-lg text-purple-700">
            <Rocket className="w-6 h-6" />
          </div>
          <div>
            <h1 className="text-xl font-black tracking-tighter text-slate-900">Autonomous Mission Lab</h1>
            <p className="text-[10px] text-slate-500 uppercase tracking-widest">
              Give the agent a mission. The server enforces the budget and scope.
            </p>
          </div>
        </div>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-1 space-y-4">
          <div className="bg-white border border-slate-200 rounded-xl p-4">
            <h2 className="text-xs font-bold uppercase tracking-wider text-slate-500 mb-3">Mission Objective</h2>
            <textarea
              value={objective}
              onChange={(e) => setObjective(e.target.value)}
              className="w-full bg-slate-50 border border-slate-200 rounded-lg p-3 text-xs text-slate-900 font-mono resize-none h-20 focus:outline-none focus:border-purple-500"
            />

            <h2 className="text-xs font-bold uppercase tracking-wider text-slate-500 mb-3 mt-4">Budget (USD)</h2>
            <div className="flex items-center gap-2">
              <DollarSign className="w-4 h-4 text-emerald-600" />
              <input
                type="number"
                value={budget}
                onChange={(e) => setBudget(Number(e.target.value))}
                min={1}
                max={500}
                className="flex-1 bg-slate-50 border border-slate-200 rounded-lg p-2 text-sm text-slate-900 font-mono focus:outline-none focus:border-purple-500"
              />
            </div>

            <h2 className="text-xs font-bold uppercase tracking-wider text-slate-500 mb-3 mt-4">Allowed Tools</h2>
            <div className="space-y-1">
              {AVAILABLE_TOOLS.map((tool) => (
                <button
                  key={tool}
                  onClick={() => toggleTool(tool)}
                  className={`w-full text-left px-3 py-2 rounded-lg border text-xs font-mono transition-colors ${
                    selectedTools.includes(tool)
                      ? 'bg-purple-50 border-purple-300 text-purple-700'
                      : 'bg-slate-50 border-slate-200 text-slate-500'
                  }`}
                >
                  <Wrench className="w-3 h-3 inline mr-2" />
                  {tool}
                </button>
              ))}
            </div>

            <button
              onClick={handleLaunch}
              disabled={loading || selectedTools.length === 0}
              className="w-full mt-4 bg-purple-600 hover:bg-purple-500 text-white font-bold py-2.5 rounded-lg text-xs uppercase tracking-wider transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
            >
              <Rocket className="w-3.5 h-3.5" />
              {loading ? 'Launching...' : 'Launch Mission'}
            </button>
          </div>

          {result && (
            <div className={`bg-white border rounded-xl p-4 ${
              result.actionsBlocked > 0 ? 'border-amber-300' : 'border-emerald-300'
            }`}>
              <h2 className="text-xs font-bold uppercase tracking-wider text-slate-500 mb-3">Mission Report</h2>
              <div className="space-y-2 text-xs font-mono">
                <div className="flex justify-between">
                  <span className="text-slate-500">Mission ID:</span>
                  <span className="text-slate-900">{result.missionId}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Executed:</span>
                  <span className="text-emerald-600">{result.actionsExecuted}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Blocked:</span>
                  <span className="text-amber-600">{result.actionsBlocked}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Budget:</span>
                  <span className="text-slate-900">${result.totalSpentUsd}</span>
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="lg:col-span-2">
          <div className="bg-white border border-slate-200 rounded-xl p-4">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-xs font-bold uppercase tracking-wider text-slate-500">Mission Audit Stream</h2>
              <span className={`text-[9px] font-bold px-2 py-0.5 rounded ${
                auditLog.some((e) => e.decision === 'blocked')
                  ? 'bg-amber-100 text-amber-700'
                  : 'bg-emerald-100 text-emerald-700'
              }`}>
                {auditLog.some((e) => e.decision === 'blocked') ? 'CONTAINED' : 'REAL'}
              </span>
            </div>

            {auditLog.length === 0 ? (
              <div className="text-center text-slate-400 text-xs py-16 font-mono">
                Launch a mission to see the agent's actions and the server's enforcement.
              </div>
            ) : (
              <div className="space-y-1 max-h-[500px] overflow-y-auto">
                {auditLog.map((entry) => (
                  <div
                    key={entry.id}
                    className={`p-2 rounded border text-[10px] font-mono ${
                      entry.decision === 'blocked'
                        ? 'bg-red-50 border-red-200 text-red-700'
                        : 'bg-emerald-50 border-emerald-200 text-emerald-700'
                    }`}
                  >
                    <div className="flex justify-between mb-1">
                      <span className="text-slate-500">#{entry.id}</span>
                      <span className="font-bold">
                        {entry.decision === 'blocked' ? (
                          <ShieldAlert className="w-3 h-3 inline" />
                        ) : (
                          <ShieldCheck className="w-3 h-3 inline" />
                        )}
                        {' '}{entry.decision.toUpperCase()}
                      </span>
                    </div>
                    <div className="text-slate-600">tool: {entry.toolName || 'system'}</div>
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