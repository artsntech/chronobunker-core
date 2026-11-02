import { useState, useMemo } from 'react'
import { gql } from '@apollo/client'
import { useQuery } from '@apollo/client/react'
import { Cockpit, type CockpitView } from './lib/cockpit'
import type { Alert } from './components/AttentionBar'
import { Login } from './pages/Login'
import { AgentMonitor } from './pages/AgentMonitor'
import { SecurityIntercept } from './pages/SecurityIntercept'
import { BreakerLab } from './pages/BreakerLab'
import { TenantIsolation } from './pages/TenantIsolation'
import { AuditTrail } from './pages/AuditTrail'
import { RedTeamPanel } from './pages/RedTeamPanel'

const GET_BREAKER_STATES = gql`
  query GetBreakerStates {
    agents {
      id
      name
      breakerState
    }
  }
`

interface BreakerData {
  agents: { id: string; name: string; breakerState: string }[]
}

export default function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(
    () => !!localStorage.getItem('chrono_jwt')
  )
  const [view, setView] = useState<CockpitView>('monitor')

  const { data } = useQuery<BreakerData>(GET_BREAKER_STATES, {
    skip: !isAuthenticated,
    pollInterval: 5000,
  })

  const { threatLevel, alerts } = useMemo(() => {
    const agents = data?.agents || []
    const alertsList: Alert[] = []
    let level: 'NOMINAL' | 'ELEVATED' | 'CRITICAL' = 'NOMINAL'

    const openAgents = agents.filter((a) => a.breakerState === 'open')
    const halfOpenAgents = agents.filter((a) => a.breakerState === 'half_open')

    if (openAgents.length > 0) {
      level = 'CRITICAL'
      openAgents.forEach((a) => {
        alertsList.push({
          type: 'breaker',
          message: `BREAKER OPEN: ${a.name}`,
          view: 'breaker',
        })
      })
    } else if (halfOpenAgents.length > 0) {
      level = 'ELEVATED'
      halfOpenAgents.forEach((a) => {
        alertsList.push({
          type: 'breaker',
          message: `BREAKER HALF_OPEN: ${a.name} recovering`,
          view: 'breaker',
        })
      })
    }

    return { threatLevel: level, alerts: alertsList }
  }, [data])

  const handleLogout = () => {
    localStorage.removeItem('chrono_jwt')
    localStorage.removeItem('chrono_tenant_id')
    localStorage.removeItem('chrono_device_id')
    setIsAuthenticated(false)
  }

  if (!isAuthenticated) {
    return <Login onLoginSuccess={() => setIsAuthenticated(true)} />
  }

  return (
    <Cockpit
      view={view}
      onNavigate={setView}
      onLogout={handleLogout}
      threatLevel={threatLevel}
      alerts={alerts}
    >
      {view === 'monitor' && <AgentMonitor onLogout={handleLogout} />}
      {view === 'intercept' && <SecurityIntercept />}
      {view === 'breaker' && <BreakerLab />}
      {view === 'isolation' && <TenantIsolation />}
      {view === 'audit' && <AuditTrail />}
      {view === 'redteam' && <RedTeamPanel />}
    </Cockpit>
  )
}