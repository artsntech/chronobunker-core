import { useState } from 'react'
import { Login } from './pages/Login'
import { AgentMonitor } from './pages/AgentMonitor'

export default function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(
    () => !!localStorage.getItem('chrono_jwt')
  )

  if (!isAuthenticated) {
    return <Login onLoginSuccess={() => setIsAuthenticated(true)} />
  }

  return (
    <AgentMonitor
      onLogout={() => {
        localStorage.removeItem('chrono_jwt')
        localStorage.removeItem('chrono_tenant_id')
        setIsAuthenticated(false)
      }}
    />
  )
}