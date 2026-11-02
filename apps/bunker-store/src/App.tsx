import { useState } from 'react'
import { Login } from './pages/Login'
import { Store } from './pages/Store'

export default function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(
    () => !!localStorage.getItem('bunker_jwt')
  )

  const handleLogout = () => {
    localStorage.removeItem('bunker_jwt')
    localStorage.removeItem('bunker_tenant_id')
    setIsAuthenticated(false)
  }

  if (!isAuthenticated) {
    return <Login onLoginSuccess={() => setIsAuthenticated(true)} />
  }

  return <Store onLogout={handleLogout} />
}