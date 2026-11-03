import { useState } from 'react'
import { Login } from './pages/Login'
import { Store } from './pages/Store'
import { MissionLab } from './pages/MissionLab'

export default function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(
    () => !!localStorage.getItem('bunker_jwt')
  )
  const [view, setView] = useState<'store' | 'mission'>('store')

  const handleLogout = () => {
    localStorage.removeItem('bunker_jwt')
    localStorage.removeItem('bunker_tenant_id')
    setIsAuthenticated(false)
  }

  if (!isAuthenticated) {
    return <Login onLoginSuccess={() => setIsAuthenticated(true)} />
  }

  return (
    <div>
      <div className="fixed bottom-4 left-1/2 -translate-x-1/2 z-50 bg-white border border-slate-200 p-1.5 rounded-xl flex gap-1 shadow-lg">
        <button
          onClick={() => setView('store')}
          className={`px-3 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-wider ${
            view === 'store' ? 'bg-slate-900 text-white' : 'text-slate-500 hover:text-slate-900'
          }`}
        >
          Store
        </button>
        <button
          onClick={() => setView('mission')}
          className={`px-3 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-wider ${
            view === 'mission' ? 'bg-purple-600 text-white' : 'text-slate-500 hover:text-slate-900'
          }`}
        >
          Mission Lab
        </button>
      </div>

      {view === 'store' && <Store onLogout={handleLogout} />}
      {view === 'mission' && <MissionLab />}
    </div>
  )
}