import { useState } from 'react'
import { gql } from '@apollo/client'
import { useQuery, useSubscription } from '@apollo/client/react'
import { ShieldCheck, ShieldAlert, RefreshCw } from 'lucide-react'
import HashWorker from '../workers/hash.worker?worker'

const GET_AUDIT_ENTRIES = gql`
  query GetAuditEntries($limit: Int!) {
    auditEntries(limit: $limit) {
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

interface AuditData {
  auditEntries: AuditEntry[]
}

interface AuditSubscriptionData {
  auditEntryAdded: AuditEntry
}

export const AuditTrail = () => {
  const { data, loading, error, refetch } = useQuery(GET_AUDIT_ENTRIES, {
    variables: { limit: 200 },
  })

  useSubscription(AUDIT_ENTRY_ADDED, {
  onData: ({ client, data: subData }) => {
    const typedSubData = subData.data as AuditSubscriptionData | undefined
    if (!typedSubData?.auditEntryAdded) return
    const newEntry = typedSubData.auditEntryAdded

    client.cache.modify({
      fields: {
        auditEntries(existing = []) {
          const newRef = client.cache.writeFragment({
            data: newEntry,
            fragment: gql`
              fragment NewAuditEntry on AuditEntry {
                id tenantId sessionId toolName decision prevHash rowHash createdAt
              }
            `,
          })
          return [...existing, newRef]
        },
      },
    })
  },
})

  const [verificationStatus, setVerificationStatus] = useState<'IDLE' | 'VERIFYING' | 'VALID' | 'CORRUPTED'>('IDLE')
  const [corruptedIndex, setCorruptedIndex] = useState<number | null>(null)

  const runVerification = () => {
    const typedData = data as AuditData | undefined
    if (!typedData?.auditEntries?.length) return

    setVerificationStatus('VERIFYING')
    setCorruptedIndex(null)

    const worker = new HashWorker()
    worker.postMessage({ entries: typedData.auditEntries })

    worker.onmessage = (e) => {
      const { status, corruptedIndex } = e.data
      setVerificationStatus(status)
      setCorruptedIndex(corruptedIndex)
      worker.terminate()
    }
  }

  if (loading && !data) {
    return <div className="min-h-screen bg-slate-950 flex items-center justify-center text-slate-400">Loading audit ledger...</div>
  }
  if (error) {
    return <div className="min-h-screen bg-slate-950 flex items-center justify-center text-red-400">Audit error: {error.message}</div>
  }

  const entries = (data as AuditData)?.auditEntries || []

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 p-6">
      <header className="flex justify-between items-center border-b border-slate-800 pb-4 mb-6">
        <div>
          <h1 className="text-xl font-bold flex items-center gap-2">
            {verificationStatus === 'CORRUPTED' ? (
              <ShieldAlert className="text-red-500 w-5 h-5 animate-pulse" />
            ) : (
              <ShieldCheck className="text-amber-400 w-5 h-5" />
            )}
            Audit Trail // Hash Chain Integrity
          </h1>
          <p className="text-xs text-slate-400">Cryptographic non-repudiation ledger ({entries.length} blocks)</p>
        </div>
        
        <div className="flex gap-2">
          <button
            onClick={() => {
              setVerificationStatus('IDLE')
              setCorruptedIndex(null)
              refetch()
            }}
            className="px-3 py-2 rounded text-xs font-bold bg-slate-800 border border-slate-700 text-slate-300 hover:bg-slate-700 transition-all flex items-center gap-1.5"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            Refresh
          </button>

          <button
            onClick={runVerification}
            disabled={verificationStatus === 'VERIFYING'}
            className={`px-4 py-2 rounded text-xs font-bold border transition-all ${
              verificationStatus === 'VALID'
                ? 'bg-emerald-950 border-emerald-500 text-emerald-400'
                : verificationStatus === 'CORRUPTED'
                ? 'bg-red-950 border-red-500 text-red-400'
                : 'bg-amber-600 border-amber-500 text-slate-950 hover:bg-amber-500'
            }`}
          >
            {verificationStatus === 'IDLE' && '🕵️ Verify Chain Integrity'}
            {verificationStatus === 'VERIFYING' && '⚙️ Recomputing Hashes...'}
            {verificationStatus === 'VALID' && '✅ Chain 100% Intact'}
            {verificationStatus === 'CORRUPTED' && '🚨 Tampering Detected!'}
          </button>
        </div>
      </header>

      <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead>
            <tr className="border-b border-slate-800 text-slate-500 uppercase text-[10px]">
              <th className="p-3">Block ID</th>
              <th className="p-3">Session</th>
              <th className="p-3">Tool</th>
              <th className="p-3">Decision</th>
              <th className="p-3">Prev Hash</th>
              <th className="p-3">Row Hash</th>
              <th className="p-3">Status</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((entry, index) => {
              const isCorrupted = corruptedIndex !== null && index >= corruptedIndex
              return (
                <tr key={entry.id} className={`border-b border-slate-800/40 ${isCorrupted ? 'bg-red-950/20' : ''}`}>
                  <td className="p-3 font-mono text-amber-500">#{entry.id}</td>
                  <td className="p-3 font-mono text-slate-400">{entry.sessionId.slice(0, 16)}...</td>
                  <td className="p-3 text-slate-300">{entry.toolName || 'system'}</td>
                  <td className="p-3">
                    <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                      entry.decision === 'allowed' ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'
                    }`}>
                      {entry.decision}
                    </span>
                  </td>
                  <td className="p-3 font-mono text-slate-500 text-[10px]">{entry.prevHash.slice(0, 12)}...</td>
                  <td className="p-3 font-mono text-cyan-400 text-[10px]">{entry.rowHash.slice(0, 12)}...</td>
                  <td className="p-3">
                    {verificationStatus === 'VALID' && <span className="text-emerald-400">✓</span>}
                    {verificationStatus === 'CORRUPTED' && (
                      <span className={isCorrupted ? 'text-red-500 font-bold' : 'text-emerald-400'}>
                        {isCorrupted ? '✗' : '✓'}
                      </span>
                    )}
                    {verificationStatus === 'IDLE' && <span className="text-slate-600">—</span>}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}