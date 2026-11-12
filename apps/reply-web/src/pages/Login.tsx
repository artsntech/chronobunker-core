import { useState } from 'react'
import { gql } from '@apollo/client'
import { useMutation } from '@apollo/client/react'
import { Shield, Key, Server } from 'lucide-react'
import { ensureDeviceKey } from '../lib/device'

const LOGIN = gql`
  mutation Login($tenantId: ID!) {
    login(tenantId: $tenantId) {
      token
      tenantId
      deviceId
    }
  }
`

const REGISTER_DEVICE_KEY = gql`
  mutation RegisterDeviceKey($input: RegisterDeviceKeyInput!) {
    registerDeviceKey(input: $input)
  }
`

interface LoginData {
  login: {
    token: string
    tenantId: string
    deviceId: string
  }
}

interface LoginProps {
  onLoginSuccess: () => void
}

export const Login = ({ onLoginSuccess }: LoginProps) => {
  const [tenantId, setTenantId] = useState('11111111-1111-1111-1111-111111111111')
  const [login, { loading, error }] = useMutation(LOGIN)
  const [registerDeviceKey] = useMutation(REGISTER_DEVICE_KEY)

    const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const result = await login({ variables: { tenantId } })
      const data = result.data as LoginData | undefined
      if (data?.login?.token) {
        localStorage.setItem('chrono_jwt', data.login.token)
        localStorage.setItem('chrono_tenant_id', data.login.tenantId)
        localStorage.setItem('chrono_device_id', data.login.deviceId)

        // Cryptographic device binding: generate an Ed25519 key pair in the
        // browser and register the public key with the backend. On browsers
        // without Web Crypto Ed25519, this logs a warning and the system
        // falls back to DeviceID-only validation.
        try {
          const state = await ensureDeviceKey(async (deviceId, publicKey) => {
            await registerDeviceKey({ variables: { input: { deviceId, publicKey } } })
          })
          if (!state.supported) {
            console.warn(
              '[login] this browser does not support Ed25519; device binding will use DeviceID only'
            )
          }
        } catch (deviceErr) {
          console.warn('[login] device key registration failed:', deviceErr)
        }

        onLoginSuccess()
      }
    } catch (err) {
      console.error('Login failed:', err)
    }
  }

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex items-center justify-center p-4">
      <div className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-xl p-8 shadow-2xl">
        <div className="flex flex-col items-center mb-8">
          <div className="p-3 bg-slate-950 border border-slate-800 rounded-lg text-emerald-400 mb-3">
            <Shield className="w-8 h-8 animate-pulse" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight">ChronoBunker Core</h1>
          <p className="text-xs text-slate-400 mt-1">Zero-Trust AgentOps Platform</p>
        </div>

        <form onSubmit={handleLogin} className="space-y-6">
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-2">
              Select Isolation Boundary (Tenant)
            </label>
            <div className="grid grid-cols-2 gap-3">
              <button
                type="button"
                onClick={() => setTenantId('11111111-1111-1111-1111-111111111111')}
                className={`p-3 border rounded-lg text-left transition-all ${
                  tenantId === '11111111-1111-1111-1111-111111111111'
                    ? 'border-emerald-500 bg-emerald-950/20 text-emerald-400'
                    : 'border-slate-800 bg-slate-950 text-slate-400 hover:border-slate-700'
                }`}
              >
                <span className="block font-bold text-sm">Acme Corp</span>
                <span className="block text-[10px] opacity-60">Tenant Alpha</span>
              </button>
              <button
                type="button"
                onClick={() => setTenantId('22222222-2222-2222-2222-222222222222')}
                className={`p-3 border rounded-lg text-left transition-all ${
                  tenantId === '22222222-2222-2222-2222-222222222222'
                    ? 'border-cyan-500 bg-cyan-950/20 text-cyan-400'
                    : 'border-slate-800 bg-slate-950 text-slate-400 hover:border-slate-700'
                }`}
              >
                <span className="block font-bold text-sm">Globex Ind</span>
                <span className="block text-[10px] opacity-60">Tenant Beta</span>
              </button>
            </div>
          </div>

          <div className="bg-slate-950 border border-slate-800 rounded-lg p-4 text-xs space-y-3">
            <div className="flex items-center gap-2 text-slate-400">
              <Server className="w-4 h-4 text-emerald-400" />
              <span>Identity assertion via backend RS256 JWT.</span>
            </div>
          </div>

          {error && (
            <div className="bg-red-950/40 border border-red-900 text-red-400 p-3 rounded-lg text-xs">
              {error.message}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold p-3 rounded-lg flex items-center justify-center gap-2 transition-colors disabled:opacity-50"
          >
            <Key className="w-4 h-4" />
            <span>{loading ? 'Authenticating...' : 'Assert Secure Session'}</span>
          </button>
        </form>
      </div>
    </div>
  )
}