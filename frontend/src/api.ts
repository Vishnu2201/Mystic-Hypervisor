import {
  NetworkExposure,
  NetworkExposureStatus,
  WorkloadSummary,
  ExposureValidationResult
} from './types'

export async function checkBackendHealth(): Promise<{ healthy: boolean; status?: string }> {
  try {
    const res = await fetch('/api/v1/health')
    if (!res.ok) return { healthy: false }
    const data = await res.json()
    return { healthy: data.status === 'healthy', status: data.status }
  } catch {
    return { healthy: false }
  }
}

export async function fetchExposures(): Promise<NetworkExposure[]> {
  const res = await fetch('/api/v1/network/exposures')
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }))
    throw new Error(err.error || `Failed to fetch exposures: HTTP ${res.status}`)
  }
  const data = await res.json()
  return data.exposures || []
}

export async function fetchExposureProviderStatus(id: string): Promise<NetworkExposureStatus> {
  const res = await fetch(`/api/v1/network/exposures/${encodeURIComponent(id)}/provider`)
  const data = await res.json().catch(() => ({}))
  if (!res.ok && !data.provider_status) {
    throw new Error(data.error || `Provider inspection failed: HTTP ${res.status}`)
  }
  return data.provider_status || { active: false, state: 'UNCONFIGURED' }
}

export async function fetchWorkloads(): Promise<WorkloadSummary[]> {
  const res = await fetch('/api/v1/workloads')
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }))
    throw new Error(err.error || `Failed to fetch workloads: HTTP ${res.status}`)
  }
  const data = await res.json()
  return data.workloads || []
}

export async function createExposure(payload: Partial<NetworkExposure>): Promise<{ exposure: NetworkExposure; message?: string }> {
  const res = await fetch('/api/v1/network/exposures', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `Failed to create exposure: HTTP ${res.status}`)
  }
  return data
}

export async function validateExposure(id: string): Promise<ExposureValidationResult> {
  const res = await fetch(`/api/v1/network/exposures/${encodeURIComponent(id)}/validate`, {
    method: 'POST'
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `Validation failed: HTTP ${res.status}`)
  }
  return data.validation_result || { is_valid: false, status: 'ERROR', message: data.error || 'Unknown error', conflicts: [] }
}

export async function applyExposure(id: string): Promise<{ exposure: NetworkExposure; message?: string }> {
  const res = await fetch(`/api/v1/network/exposures/${encodeURIComponent(id)}/apply`, {
    method: 'POST'
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `Apply failed: HTTP ${res.status}`)
  }
  return data
}

export async function reconcileExposure(id: string): Promise<{ exposure: NetworkExposure }> {
  const res = await fetch(`/api/v1/network/exposures/${encodeURIComponent(id)}/reconcile`, {
    method: 'POST'
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `Reconcile failed: HTTP ${res.status}`)
  }
  return data
}

export async function deleteExposure(id: string): Promise<{ message: string }> {
  const res = await fetch(`/api/v1/network/exposures/${encodeURIComponent(id)}`, {
    method: 'DELETE'
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `Delete failed: HTTP ${res.status}`)
  }
  return data
}

export async function fetchWorkloadSnapshots(workloadId: string): Promise<import('./types').Snapshot[]> {
  const res = await fetch(`/api/v1/workloads/${encodeURIComponent(workloadId)}/snapshots`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }))
    throw new Error(err.error || `Failed to fetch snapshots: HTTP ${res.status}`)
  }
  const data = await res.json()
  return data.snapshots || []
}

export async function createSnapshot(workloadId: string, name: string, stateful: boolean = false, description: string = ''): Promise<{ snapshot: import('./types').Snapshot; message?: string }> {
  const res = await fetch(`/api/v1/workloads/${encodeURIComponent(workloadId)}/snapshots`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, stateful, description })
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `Failed to create snapshot: HTTP ${res.status}`)
  }
  return data
}

export async function restoreSnapshot(workloadId: string, name: string): Promise<{ snapshot: import('./types').Snapshot; message?: string }> {
  const res = await fetch(`/api/v1/workloads/${encodeURIComponent(workloadId)}/snapshots/${encodeURIComponent(name)}/restore`, {
    method: 'POST'
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `Restore snapshot failed: HTTP ${res.status}`)
  }
  return data
}

export async function deleteSnapshot(workloadId: string, name: string): Promise<{ message: string }> {
  const res = await fetch(`/api/v1/workloads/${encodeURIComponent(workloadId)}/snapshots/${encodeURIComponent(name)}`, {
    method: 'DELETE'
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `Delete snapshot failed: HTTP ${res.status}`)
  }
  return data
}
