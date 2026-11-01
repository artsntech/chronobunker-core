import { useState } from 'react'
import { Login } from './pages/Login'
import { AgentMonitor } from './pages/AgentMonitor'
import { AuditTrail } from './pages/AuditTrail'
import { SecurityIntercept } from './pages/SecurityIntercept'

export default function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(
    () => !!localStorage.getItem('chrono_jwt')
  )
  const [view, setView] = useState<'monitor' | 'audit' | 'intercept'>('monitor')

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
    <div>
      {/* Floating Menu */}
      <div className="fixed bottom-4 right-4 z-50 flex gap-2">
        <button
          onClick={() => setView('monitor')}
          className={`px-3 py-1.5 rounded text-xs font-bold ${
            view === 'monitor' ? 'bg-emerald-500 text-slate-950' : 'bg-slate-800 text-slate-300'
          }`}
        >
          Monitor
        </button>
        <button
          onClick={() => setView('audit')}
          className={`px-3 py-1.5 rounded text-xs font-bold ${
            view === 'audit' ? 'bg-amber-500 text-slate-950' : 'bg-slate-800 text-slate-300'
          }`}
        >
          Audit Trail
        </button>
        <button
          onClick={() => setView('intercept')}
          className={`px-3 py-1.5 rounded text-xs font-bold ${
            view === 'intercept' ? 'bg-red-500 text-white' : 'bg-slate-800 text-slate-300'
          }`}
        >
          Security Intercept
        </button>
      </div>

      {/* Rendering */}
      {view === 'monitor' && <AgentMonitor onLogout={handleLogout} />}
      {view === 'audit' && <AuditTrail />}
      {view === 'intercept' && <SecurityIntercept />}
    </div>
  )
}
