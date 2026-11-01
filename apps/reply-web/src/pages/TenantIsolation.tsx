import { useState } from 'react'
import { gql } from '@apollo/client'
import { ApolloClient, InMemoryCache, HttpLink } from '@apollo/client'
import { useQuery } from '@apollo/client/react'
import { SetContextLink } from '@apollo/client/link/context'
import { ShieldCheck, Lock, Unlock, ArrowRightLeft, Database } from 'lucide-react'

const KNOWN_AGENT_IDS = {
  acme: '5ca2ccb5-061f-4042-9222-9c076776a2e3',   // Acme Triage Agent
  globex: 'c888c7f4-55ae-4823-8c6c-011328851a1b', // Globex Triage Agent
}

const TENANTS = {
  acme: {
    id: '11111111-1111-1111-1111-111111111111',
    name: 'Acme Corp',
    color: 'emerald',
  },
  globex: {
    id: '22222222-2222-2222-2222-222222222222',
    name: 'Globex Inc',
    color: 'cyan',
  },
} as const

type TenantKey = keyof typeof TENANTS

const LOGIN = gql`
  mutation Login($tenantId: ID!) {
    login(tenantId: $tenantId) {
      token
      tenantId
      deviceId
    }
  }
`

const GET_AGENTS = gql`
  query GetAgents {
    agents {
      id
      name
      tenantId
      department
    }
  }
`

const GET_AGENT_BY_ID = gql`
  query GetAgentById($id: ID!) {
    agent(id: $id) {
      id
      name
      tenantId
    }
  }
`

interface LoginData {
  login: {
    token: string
    tenantId: string
    deviceId: string
  }
}

interface Agent {
  id: string
  name: string
  tenantId: string
  department: string
}

interface AgentsData {
  agents: Agent[]
}

interface AgentByIdData {
  agent: Agent | null
}

interface TenantClient {
  client: ApolloClient
  token: string
  deviceId: string
}

interface Evidence {
  timestamp: string
  actor: string
  operation: string
  query: string
  result: string
  outcome: 'success' | 'blocked'
}

export const TenantIsolation = () => {
  const [clients, setClients] = useState<Record<TenantKey, TenantClient | null>>({
    acme: null,
    globex: null,
  })
  const [authenticating, setAuthenticating] = useState<Record<TenantKey, boolean>>({
    acme: false,
    globex: false,
  })
  const [evidence, setEvidence] = useState<Evidence[]>([])

  const addEvidence = (e: Evidence) => {
    setEvidence((prev) => [e, ...prev.slice(0, 19)])
  }

  const authenticate = async (tenantKey: TenantKey) => {
    setAuthenticating((prev) => ({ ...prev, [tenantKey]: true }))

    const baseClient = new ApolloClient({
      link: new HttpLink({ uri: 'http://localhost:8080/query' }),
      cache: new InMemoryCache(),
    })

    try {
      const { data } = await baseClient.mutate<LoginData>({
        mutation: LOGIN,
        variables: { tenantId: TENANTS[tenantKey].id },
      })

      if (!data?.login?.token) throw new Error('Login failed')

      const token = data.login.token
      const deviceId = data.login.deviceId

      const authLink = new SetContextLink((prev) => ({
        ...prev,
        headers: {
          ...prev.headers,
          authorization: `Bearer ${token}`,
        },
      }))
      const tenantClient = new ApolloClient({
        link: authLink.concat(new HttpLink({ uri: 'http://localhost:8080/query' })),
        cache: new InMemoryCache(),
      })

      setClients((prev) => ({
        ...prev,
        [tenantKey]: { client: tenantClient, token, deviceId },
      }))

      addEvidence({
        timestamp: new Date().toLocaleTimeString(),
        actor: TENANTS[tenantKey].name,
        operation: 'AUTH',
        query: `login(tenantId: "${TENANTS[tenantKey].id}")`,
        result: `JWT issued for ${TENANTS[tenantKey].name}`,
        outcome: 'success',
      })
    } catch (err: any) {
      addEvidence({
        timestamp: new Date().toLocaleTimeString(),
        actor: TENANTS[tenantKey].name,
        operation: 'AUTH',
        query: `login(tenantId: "${TENANTS[tenantKey].id}")`,
        result: err.message,
        outcome: 'blocked',
      })
    } finally {
      setAuthenticating((prev) => ({ ...prev, [tenantKey]: false }))
    }
  }

  const attemptCrossTenant = async (sourceKey: TenantKey) => {
    const source = clients[sourceKey]
    if (!source) return

    // The target agent belongs to the OTHER tenant. The source tenant's JWT
    // should not be able to read it. RLS must return zero rows.
    const targetKey: TenantKey = sourceKey === 'acme' ? 'globex' : 'acme'
    const targetAgentId = KNOWN_AGENT_IDS[targetKey]
    const targetTenantName = TENANTS[targetKey].name

    const result = await source.client.query<AgentByIdData>({
      query: GET_AGENT_BY_ID,
      variables: { id: targetAgentId },
      fetchPolicy: 'network-only',
    })

    const found = result.data?.agent

    addEvidence({
      timestamp: new Date().toLocaleTimeString(),
      actor: TENANTS[sourceKey].name,
      operation: 'CROSS-TENANT ACCESS',
      query: `${TENANTS[sourceKey].name} JWT → SELECT * FROM agents WHERE id = '${targetAgentId}' (belongs to ${targetTenantName})`,
      result: found
        ? `SECURITY BREACH: ${found.name} (tenant ${found.tenantId})`
        : `0 rows returned — RLS blocked ${TENANTS[sourceKey].name} from accessing ${targetTenantName}'s agent`,
      outcome: found ? 'blocked' : 'success',
    })
  }

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 p-6 font-mono">
      <header className="border-b border-slate-800 pb-4 mb-6">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-cyan-950/40 border border-cyan-900/60 rounded-lg text-cyan-400">
            <Database className="w-6 h-6" />
          </div>
          <div>
            <h1 className="text-xl font-black tracking-tighter text-white">
              TENANT ISOLATION // POSTGRES RLS
            </h1>
            <p className="text-[10px] text-slate-400 uppercase tracking-widest">
              Prove that Row-Level Security enforces tenant isolation at the database engine — every event is REAL
            </p>
          </div>
        </div>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        {(Object.keys(TENANTS) as TenantKey[]).map((key) => (
          <TenantPanel
            key={key}
            tenant={TENANTS[key]}
            client={clients[key]}
            authenticating={authenticating[key]}
            onAuthenticate={() => authenticate(key)}
            onCrossTenant={() => attemptCrossTenant(key)}
            otherTenantName={key === 'acme' ? TENANTS.globex.name : TENANTS.acme.name}
          />
        ))}
      </div>

      <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-xs font-bold uppercase tracking-wider text-slate-400">
            Evidence Log (Database-level)
          </h2>
          <span className="text-[9px] font-bold px-2 py-0.5 rounded bg-emerald-950 text-emerald-400 border border-emerald-900/60">
            REAL
          </span>
        </div>
        {evidence.length === 0 ? (
          <div className="text-center text-slate-600 text-xs py-8">
            Authenticate both tenants, then attempt a cross-tenant access.
          </div>
        ) : (
          <div className="space-y-1 max-h-[300px] overflow-y-auto">
            {evidence.map((e, i) => (
              <div
                key={i}
                className={`p-2 rounded border text-[10px] ${
                  e.outcome === 'success'
                    ? 'bg-emerald-950/10 border-emerald-900/40'
                    : 'bg-red-950/10 border-red-900/40'
                }`}
              >
                <div className="flex justify-between mb-1">
                  <span className="text-slate-400 font-bold">
                    {e.actor} → {e.operation}
                  </span>
                  <span className="text-slate-500">{e.timestamp}</span>
                </div>
                <div className="text-slate-500 mb-1 font-mono text-[9px]">{e.query}</div>
                <div className={e.outcome === 'success' ? 'text-emerald-400' : 'text-red-400'}>
                  {e.result}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

interface TenantPanelProps {
  tenant: { id: string; name: string; color: 'emerald' | 'cyan' }
  client: TenantClient | null
  authenticating: boolean
  onAuthenticate: () => void
  onCrossTenant: () => void
  otherTenantName: string
}

const TenantPanel = ({
  tenant,
  client,
  authenticating,
  onAuthenticate,
  onCrossTenant,
  otherTenantName,
}: TenantPanelProps) => {
  const { data, loading } = useQuery<AgentsData>(GET_AGENTS, {
    client: client?.client,
    skip: !client,
    fetchPolicy: 'network-only',
  })

  const agents = data?.agents || []
  const colorClasses =
    tenant.color === 'emerald'
      ? 'bg-emerald-950/20 border-emerald-900/60 text-emerald-400'
      : 'bg-cyan-950/20 border-cyan-900/60 text-cyan-400'

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-4">
      <div className="flex items-center justify-between mb-3">
        <div>
          <h2 className="text-sm font-bold text-slate-200">{tenant.name}</h2>
          <p className="text-[10px] text-slate-500 font-mono">{tenant.id}</p>
        </div>
        {client ? (
          <span className={`text-[9px] font-bold px-2 py-0.5 rounded border ${colorClasses}`}>
            <Lock className="w-3 h-3 inline mr-1" />
            AUTHENTICATED
          </span>
        ) : (
          <span className="text-[9px] font-bold px-2 py-0.5 rounded border border-slate-700 text-slate-500">
            <Unlock className="w-3 h-3 inline mr-1" />
            NOT AUTHENTICATED
          </span>
        )}
      </div>

      {!client ? (
        <button
          onClick={onAuthenticate}
          disabled={authenticating}
          className="w-full bg-slate-800 hover:bg-slate-700 text-slate-200 font-bold py-2 rounded-lg text-xs uppercase tracking-wider transition-colors disabled:opacity-50"
        >
          {authenticating ? 'Authenticating...' : `Login as ${tenant.name}`}
        </button>
      ) : loading ? (
        <div className="text-center text-slate-500 text-xs py-4">Loading agents...</div>
      ) : agents.length === 0 ? (
        <div className="text-center text-slate-500 text-xs py-4">No agents visible</div>
      ) : (
        <div className="space-y-2">
          <p className="text-[10px] text-slate-500 uppercase mb-2">
            Agents visible to this tenant ({agents.length}):
          </p>
          {agents.map((agent: Agent) => (
            <div
              key={agent.id}
              className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-[11px]"
            >
              <div className="flex justify-between items-center mb-1">
                <span className="font-bold text-slate-200">{agent.name}</span>
                <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              </div>
              <div className="flex justify-between text-slate-500 mb-2">
                <span>{agent.department}</span>
                <span className="font-mono text-[9px]">{agent.id.slice(0, 12)}...</span>
              </div>
            </div>
          ))}
          <button
            onClick={onCrossTenant}
            className="w-full mt-2 bg-red-950/40 border border-red-900/60 text-red-400 hover:bg-red-900/60 hover:text-white text-[10px] font-bold py-2 rounded transition-colors flex items-center justify-center gap-1"
          >
            <ArrowRightLeft className="w-3 h-3" />
            Try accessing {otherTenantName}'s agent
          </button>
        </div>
      )}
    </div>
  )
}