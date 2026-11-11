// Generates an Ed25519 key pair in the browser, registers the public key with
// the backend on first login, and signs each tool invocation.
//
// The private key never leaves the browser. In production (Expo), it must be
// stored in SecureStore/Keychain instead of localStorage.

const PRIVATE_KEY_STORAGE = 'chrono_device_private_key'
const PUBLIC_KEY_STORAGE = 'chrono_device_public_key'

function b64url(bytes: Uint8Array): string {
  let s = ''
  for (const b of bytes) s += String.fromCharCode(b)
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function fromB64url(s: string): Uint8Array {
  const pad = s.length % 4 === 0 ? '' : '='.repeat(4 - (s.length % 4))
  const bin = atob(s.replace(/-/g, '+').replace(/_/g, '/') + pad)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}

export async function getOrCreateDeviceKeyPair(): Promise<{
  publicKeyB64: string
  privateKey: CryptoKey
  deviceId: string
}> {
  const existingPriv = localStorage.getItem(PRIVATE_KEY_STORAGE)
  const existingPub = localStorage.getItem(PUBLIC_KEY_STORAGE)
  const deviceId = localStorage.getItem('chrono_device_id') || `web-device-${Date.now()}`

  if (existingPriv && existingPub) {
    // Re-import the private key from its raw PKCS8 encoding.
    const raw = fromB64url(existingPriv)
    const privateKey = await crypto.subtle.importKey(
      'pkcs8', raw, { name: 'Ed25519' }, false, ['sign']
    )
    return { publicKeyB64: existingPub, privateKey, deviceId }
  }

  const keyPair = (await crypto.subtle.generateKey(
    { name: 'Ed25519' }, true, ['sign', 'verify']
  )) as CryptoKeyPair

  const privRaw = new Uint8Array(await crypto.subtle.exportKey('pkcs8', keyPair.privateKey))
  const pubRaw = new Uint8Array(await crypto.subtle.exportKey('raw', keyPair.publicKey))

  const publicKeyB64 = b64url(pubRaw)
  localStorage.setItem(PRIVATE_KEY_STORAGE, b64url(privRaw))
  localStorage.setItem(PUBLIC_KEY_STORAGE, publicKeyB64)
  localStorage.setItem('chrono_device_id', deviceId)

  return { publicKeyB64, privateKey: keyPair.privateKey, deviceId }
}

export async function signToolCall(
  privateKey: CryptoKey,
  sessionId: string,
  toolName: string,
  deviceId: string,
  params: Record<string, unknown>
): Promise<{ signature: string; timestamp: number }> {
  const timestamp = Math.floor(Date.now() / 1000)
  const canonical = JSON.stringify({
    session_id: sessionId,
    tool_name: toolName,
    device_id: deviceId,
    params,
    ts: timestamp,
  })
  const sig = new Uint8Array(
    await crypto.subtle.sign({ name: 'Ed25519' }, privateKey, new TextEncoder().encode(canonical))
  )
  return { signature: b64url(sig), timestamp }
}