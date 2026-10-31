interface AuditEntry {
  id: string
  prevHash: string
  rowHash: string
}

interface WorkerRequest {
  entries: AuditEntry[]
}

interface WorkerResponse {
  status: 'VALID' | 'CORRUPTED'
  corruptedIndex: number | null
}

const GENESIS_HASH = '0'.repeat(64)

self.onmessage = (e: MessageEvent<WorkerRequest>) => {
  const { entries } = e.data

  if (!entries || entries.length === 0) {
    self.postMessage({ status: 'VALID', corruptedIndex: null } as WorkerResponse)
    return
  }

  // Sort by numeric id (the ID is a string but represents a sequence)
  const sorted = [...entries].sort((a, b) => Number(a.id) - Number(b.id))

  for (let i = 0; i < sorted.length; i++) {
    const current = sorted[i]
    const expectedPrev = i === 0 ? GENESIS_HASH : sorted[i - 1].rowHash

    if (current.prevHash !== expectedPrev) {
      self.postMessage({ status: 'CORRUPTED', corruptedIndex: i } as WorkerResponse)
      return
    }
  }

  self.postMessage({ status: 'VALID', corruptedIndex: null } as WorkerResponse)
}