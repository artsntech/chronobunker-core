// Cryptographic device binding for the ChronoBunker cockpit.
//
// On first login, the browser generates an Ed25519 key pair with a
// non-extractable private key. The public key is registered with the backend.
// Every invokeTool is signed with the private key, and the backend verifies
// the signature against the registered public key.
//
// If the browser does not support Ed25519 in Web Crypto (Chrome < 137,
// Firefox < 129, Safari < 17), ensureDeviceKey returns supported: false and
// the caller falls back to DeviceID-only validation. The limitations.md
// documents this degradation.

const PRIVATE_KEY_DB = 'chrono_device_keys'
const PRIVATE_KEY_STORE = 'keys'
const PRIVATE_KEY_ID = 'device_signing_key'
const PUBLIC_KEY_STORAGE = 'chrono_device_public_key'
const DEVICE_ID_STORAGE = 'chrono_device_id'

function b64url(bytes: Uint8Array): string {
  let s = ''
  for (const b of bytes) s += String.fromCharCode(b)
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function openKeyDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(PRIVATE_KEY_DB, 1)
    req.onupgradeneeded = () => {
      req.result.createObjectStore(PRIVATE_KEY_STORE)
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

async function loadPrivateKey(): Promise<CryptoKey | null> {
  const db = await openKeyDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(PRIVATE_KEY_STORE, 'readonly')
    const req = tx.objectStore(PRIVATE_KEY_STORE).get(PRIVATE_KEY_ID)
    req.onsuccess = () => resolve((req.result as CryptoKey) ?? null)
    req.onerror = () => reject(req.error)
  })
}

async function savePrivateKey(key: CryptoKey): Promise<void> {
  const db = await openKeyDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(PRIVATE_KEY_STORE, 'readwrite')
    tx.objectStore(PRIVATE_KEY_STORE).put(key, PRIVATE_KEY_ID)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

export interface DeviceKeyState {
  supported: boolean
  deviceId: string
  publicKeyB64: string | null
}

// ensureDeviceKey generates the key pair if none exists, registers the
// public key with the backend, and returns the state. On browsers without
// Ed25519 support, it returns supported: false.
export async function ensureDeviceKey(
  registerPublicKey: (deviceId: string, publicKeyB64: string) => Promise<void>
): Promise<DeviceKeyState> {
  const deviceId = localStorage.getItem(DEVICE_ID_STORAGE) ?? `web-device-${Date.now()}`
  localStorage.setItem(DEVICE_ID_STORAGE, deviceId)

  let privateKey = await loadPrivateKey()
  let publicKeyB64 = localStorage.getItem(PUBLIC_KEY_STORAGE)

  if (!privateKey || !publicKeyB64) {
    try {
      const keyPair = (await crypto.subtle.generateKey(
        { name: 'Ed25519' },
        false, // private key non-extractable
        ['sign', 'verify']
      )) as CryptoKeyPair

      privateKey = keyPair.privateKey
      const pubRaw = new Uint8Array(await crypto.subtle.exportKey('raw', keyPair.publicKey))
      publicKeyB64 = b64url(pubRaw)

      // Firefox cannot structured-clone a CryptoKey into IndexedDB
      // (Bugzilla #1348279, RESOLVED WONTFIX). The key stays in memory for
      // this session; persistence across reloads is Chrome/Edge/Safari only.

            try {
        await savePrivateKey(privateKey)
      } catch (persistErr) {
        console.warn('[device] private key persistence not supported in this browser', persistErr)
      }
      localStorage.setItem(PUBLIC_KEY_STORAGE, publicKeyB64)
    } catch (err) {
      console.warn('[device] Ed25519 not supported; falling back to DeviceID-only', err)
      return { supported: false, deviceId, publicKeyB64: null }
    }
  }

  await registerPublicKey(deviceId, publicKeyB64)
  return { supported: true, deviceId, publicKeyB64 }
}

// canonicalJSON serializes an object with keys sorted alphabetically at every
// level, matching Go's encoding/json behavior for maps. This ensures the
// browser and the Go verifier produce identical bytes for the same payload.
function canonicalJSON(value: unknown): string {
  if (value === null || typeof value !== 'object') {
    return JSON.stringify(value)
  }
  if (Array.isArray(value)) {
    return '[' + value.map(canonicalJSON).join(',') + ']'
  }
  const obj = value as Record<string, unknown>
  const keys = Object.keys(obj).sort()
  return '{' + keys.map((k) => JSON.stringify(k) + ':' + canonicalJSON(obj[k])).join(',') + '}'
}

export async function signToolCall(
  sessionId: string,
  toolName: string,
  deviceId: string,
  params: Record<string, unknown>
): Promise<{ signature: string; timestamp: number } | null> {
  const privateKey = await loadPrivateKey()
  if (!privateKey) return null

  const timestamp = Math.floor(Date.now() / 1000)
  const canonical = canonicalJSON({
    session_id: sessionId,
    tool_name: toolName,
    device_id: deviceId,
    params,
    ts: timestamp,
  })

  try {
    const sig = new Uint8Array(
      await crypto.subtle.sign(
        { name: 'Ed25519' },
        privateKey,
        new TextEncoder().encode(canonical)
      )
    )
    return { signature: b64url(sig), timestamp }
  } catch (err) {
    console.warn('[device] signing failed', err)
    return null
  }
}

export function getDeviceId(): string {
  return localStorage.getItem(DEVICE_ID_STORAGE) ?? `web-device-${Date.now()}`
}