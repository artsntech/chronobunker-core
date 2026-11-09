import { parse, value } from 'promtext'

export interface RuntimeMetrics {
  goroutines: number
  heapAllocMB: number
  heapObjects: number
  gcPauseMs: number
  gcCount: number
}

const METRICS_URL = '/metrics'

export async function fetchRuntimeMetrics(): Promise<RuntimeMetrics> {
  const res = await fetch(METRICS_URL)
  if (!res.ok) throw new Error(`metrics fetch failed: ${res.status}`)
  const text = await res.text()
  const families = parse(text)

  const goroutines = value(families, 'go_goroutines') ?? 0
  const heapAllocBytes = value(families, 'go_memstats_alloc_bytes') ?? 0
  const heapObjects = value(families, 'go_memstats_heap_objects') ?? 0

  // GC pause: sum / count * 1000 (ms). O histogram do Go expõe _sum e _count.
  const gcSum = value(families, 'go_gc_duration_seconds_sum') ?? 0
  const gcCount = value(families, 'go_gc_duration_seconds_count') ?? 0
  const gcPauseMs = gcCount > 0 ? (gcSum / gcCount) * 1000 : 0

  return {
    goroutines,
    heapAllocMB: heapAllocBytes / 1024 / 1024,
    heapObjects,
    gcPauseMs,
    gcCount,
  }
}