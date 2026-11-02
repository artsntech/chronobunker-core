import { useState } from 'react'
import { gql } from '@apollo/client'
import { useMutation } from '@apollo/client/react'
import { Shield, Key } from 'lucide-react'

const LOGIN = gql`
  mutation Login($tenantId: ID!) {
    login(tenantId: $tenantId) {
      token
      tenantId
      deviceId
    }
  }
`

interface LoginData {
  login: { token: string; tenantId: string; deviceId: string }
}

export const Login = ({ onLoginSuccess }: { onLoginSuccess: () => void }) => {
  const [tenantId, setTenantId] = useState('11111111-1111-1111-1111-111111111111')
  const [login, { loading, error }] = useMutation(LOGIN)

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const result = await login({ variables: { tenantId } })
      const data = result.data as LoginData | undefined
      if (data?.login?.token) {
        localStorage.setItem('bunker_jwt', data.login.token)
        localStorage.setItem('bunker_tenant_id', data.login.tenantId)
        onLoginSuccess()
      }
    } catch (err) {
      console.error(err)
    }
  }

  return (
    <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-white border border-slate-200 rounded-2xl p-8 shadow-lg">
        <div className="flex flex-col items-center mb-8">
          <div className="p-3 bg-slate-900 rounded-xl text-white mb-4">
            <Shield className="w-8 h-8" />
          </div>
          <h1 className="text-2xl font-bold text-slate-900">Bunker Store</h1>
          <p className="text-sm text-slate-500 mt-1">Secure infrastructure for AI agents</p>
        </div>

        <form onSubmit={handleLogin} className="space-y-6">
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-500 mb-3">
              Select your account
            </label>
            <div className="grid grid-cols-2 gap-3">
              <button
                type="button"
                onClick={() => setTenantId('11111111-1111-1111-1111-111111111111')}
                className={`p-4 border-2 rounded-xl text-left transition-all ${
                  tenantId === '11111111-1111-1111-1111-111111111111'
                    ? 'border-slate-900 bg-slate-50'
                    : 'border-slate-200 hover:border-slate-300'
                }`}
              >
                <span className="block font-bold text-sm text-slate-900">Acme Corp</span>
                <span className="block text-xs text-slate-500 mt-1">Enterprise account</span>
              </button>
              <button
                type="button"
                onClick={() => setTenantId('22222222-2222-2222-2222-222222222222')}
                className={`p-4 border-2 rounded-xl text-left transition-all ${
                  tenantId === '22222222-2222-2222-2222-222222222222'
                    ? 'border-slate-900 bg-slate-50'
                    : 'border-slate-200 hover:border-slate-300'
                }`}
              >
                <span className="block font-bold text-sm text-slate-900">Globex Inc</span>
                <span className="block text-xs text-slate-500 mt-1">Enterprise account</span>
              </button>
            </div>
          </div>

          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg text-sm">
              {error.message}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-slate-900 hover:bg-slate-800 text-white font-semibold p-3 rounded-xl flex items-center justify-center gap-2 transition-colors disabled:opacity-50"
          >
            <Key className="w-4 h-4" />
            <span>{loading ? 'Signing in...' : 'Sign in'}</span>
          </button>
        </form>
      </div>
    </div>
  )
}