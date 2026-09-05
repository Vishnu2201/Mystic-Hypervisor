import React, { useState, useEffect, useCallback } from 'react'
import {
  checkBackendHealth,
  fetchWorkloads,
  fetchWorkloadSnapshots,
  createSnapshot,
  restoreSnapshot,
  deleteSnapshot
} from '../api'
import { Snapshot } from '../types'

interface AuditLogEntry {
  timestamp: string
  operation: string
  target: string
  result: 'SUCCESS' | 'ERROR' | 'PENDING'
  message: string
}

export const SnapshotManagement: React.FC = () => {
  // Connection & Core State
  const [backendHealthy, setBackendHealthy] = useState<boolean | null>(null)
  const [workloads, setWorkloads] = useState<any[]>([])
  const [selectedWorkloadId, setSelectedWorkloadId] = useState<string>('')
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [loading, setLoading] = useState<boolean>(false)
  const [errorMsg, setErrorMsg] = useState<string>('')
  const [autoRefresh, setAutoRefresh] = useState<boolean>(false)

  // Modal / Action States
  const [showCreateModal, setShowCreateModal] = useState<boolean>(false)
  const [createSnapName, setCreateSnapName] = useState<string>('')
  const [createDescription, setCreateDescription] = useState<string>('')
  const [createStateful, setCreateStateful] = useState<boolean>(false)
  const [creating, setCreating] = useState<boolean>(false)
  const [createError, setCreateError] = useState<string>('')

  // Restore Confirmation State
  const [restoreTarget, setRestoreTarget] = useState<Snapshot | null>(null)
  const [restoring, setRestoring] = useState<boolean>(false)

  // Delete Confirmation State
  const [deleteTarget, setDeleteTarget] = useState<Snapshot | null>(null)
  const [deleting, setDeleting] = useState<boolean>(false)

  // Inspection Drawer State
  const [inspectTarget, setInspectTarget] = useState<Snapshot | null>(null)

  // Session Audit Log
  const [auditLogs, setAuditLogs] = useState<AuditLogEntry[]>([])

  const addAuditLog = useCallback((operation: string, target: string, result: 'SUCCESS' | 'ERROR' | 'PENDING', message: string) => {
    const entry: AuditLogEntry = {
      timestamp: new Date().toLocaleTimeString(),
      operation,
      target,
      result,
      message
    }
    setAuditLogs((prev) => [entry, ...prev.slice(0, 49)])
  }, [])

  // Load Workloads & Check Health
  const initSystem = useCallback(async () => {
    const health = await checkBackendHealth()
    setBackendHealthy(health.healthy)

    try {
      const list = await fetchWorkloads()
      setWorkloads(list)
      if (list.length > 0 && !selectedWorkloadId) {
        setSelectedWorkloadId(list[0].id)
      }
    } catch (e: any) {
      setErrorMsg(e.message || 'Failed to connect to backend workload manager.')
    }
  }, [selectedWorkloadId])

  // Load Snapshots for Selected Workload
  const loadSnapshots = useCallback(async (workloadId: string) => {
    if (!workloadId) return
    setLoading(true)
    setErrorMsg('')
    try {
      const data = await fetchWorkloadSnapshots(workloadId)
      setSnapshots(data)
    } catch (e: any) {
      setErrorMsg(e.message || 'Failed to load snapshots for selected workload.')
      setSnapshots([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    initSystem()
  }, [initSystem])

  useEffect(() => {
    if (selectedWorkloadId) {
      loadSnapshots(selectedWorkloadId)
    }
  }, [selectedWorkloadId, loadSnapshots])

  useEffect(() => {
    if (!autoRefresh || !selectedWorkloadId) return
    const timer = setInterval(() => {
      loadSnapshots(selectedWorkloadId)
    }, 10000)
    return () => clearInterval(timer)
  }, [autoRefresh, selectedWorkloadId, loadSnapshots])

  // Handler: Create Snapshot
  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = createSnapName.trim()
    if (!trimmed) {
      setCreateError('Snapshot name is required.')
      return
    }
    if (trimmed.startsWith('-')) {
      setCreateError('Snapshot name cannot begin with a hyphen (option injection rejected).')
      return
    }

    setCreating(true)
    setCreateError('')
    addAuditLog('CREATE_SNAPSHOT', trimmed, 'PENDING', `Initiating snapshot creation for workload ${selectedWorkloadId}`)

    try {
      const res = await createSnapshot(selectedWorkloadId, trimmed, createStateful, createDescription)
      addAuditLog('CREATE_SNAPSHOT', trimmed, 'SUCCESS', res.message || `Snapshot ${trimmed} created successfully.`)
      setShowCreateModal(false)
      setCreateSnapName('')
      setCreateDescription('')
      setCreateStateful(false)
      loadSnapshots(selectedWorkloadId)
    } catch (err: any) {
      const msg = err.message || 'Failed to create snapshot.'
      setCreateError(msg)
      addAuditLog('CREATE_SNAPSHOT', trimmed, 'ERROR', msg)
    } finally {
      setCreating(false)
    }
  }

  // Handler: Restore Snapshot
  const handleConfirmRestore = async () => {
    if (!restoreTarget) return
    setRestoring(true)
    addAuditLog('RESTORE_SNAPSHOT', restoreTarget.name, 'PENDING', `Restoring workload ${selectedWorkloadId} to snapshot ${restoreTarget.name}`)

    try {
      const res = await restoreSnapshot(selectedWorkloadId, restoreTarget.name)
      addAuditLog('RESTORE_SNAPSHOT', restoreTarget.name, 'SUCCESS', res.message || `Workload restored to snapshot ${restoreTarget.name}.`)
      setRestoreTarget(null)
      loadSnapshots(selectedWorkloadId)
    } catch (err: any) {
      addAuditLog('RESTORE_SNAPSHOT', restoreTarget.name, 'ERROR', err.message || 'Failed to restore snapshot.')
    } finally {
      setRestoring(false)
    }
  }

  // Handler: Delete Snapshot
  const handleConfirmDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    addAuditLog('DELETE_SNAPSHOT', deleteTarget.name, 'PENDING', `Deleting snapshot ${deleteTarget.name} from workload ${selectedWorkloadId}`)

    try {
      const res = await deleteSnapshot(selectedWorkloadId, deleteTarget.name)
      addAuditLog('DELETE_SNAPSHOT', deleteTarget.name, 'SUCCESS', res.message || `Snapshot ${deleteTarget.name} deleted.`)
      setDeleteTarget(null)
      loadSnapshots(selectedWorkloadId)
    } catch (err: any) {
      addAuditLog('DELETE_SNAPSHOT', deleteTarget.name, 'ERROR', err.message || 'Failed to delete snapshot.')
    } finally {
      setDeleting(false)
    }
  }

  const selectedWorkloadObj = workloads.find((w) => w.id === selectedWorkloadId)

  return (
    <div style={{ fontFamily: 'Inter, system-ui, sans-serif', color: '#e2e8f0', backgroundColor: '#0f172a', minHeight: '100vh', padding: '24px' }}>
      {/* Header Bar */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px', backgroundColor: '#1e293b', padding: '16px 24px', borderRadius: '12px', border: '1px solid #334155' }}>
        <div>
          <h1 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 700, color: '#38bdf8' }}>Workload Snapshot & Reversion Engine</h1>
          <p style={{ margin: '4px 0 0 0', fontSize: '0.875rem', color: '#94a3b8' }}>
            Capture point-in-time states and execute instant recovery for Incus hypervisor workloads
          </p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 12px', borderRadius: '20px', backgroundColor: backendHealthy ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)', border: `1px solid ${backendHealthy ? '#10b981' : '#ef4444'}` }}>
            <span style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: backendHealthy ? '#10b981' : '#ef4444' }} />
            <span style={{ fontSize: '0.75rem', fontWeight: 600, color: backendHealthy ? '#10b981' : '#ef4444' }}>
              {backendHealthy ? 'API CONNECTED' : 'API DISCONNECTED'}
            </span>
          </div>

          <button
            onClick={() => selectedWorkloadId && loadSnapshots(selectedWorkloadId)}
            disabled={loading || !selectedWorkloadId}
            style={{ padding: '8px 16px', borderRadius: '6px', border: '1px solid #38bdf8', backgroundColor: 'transparent', color: '#38bdf8', fontWeight: 600, cursor: loading ? 'not-allowed' : 'pointer' }}
          >
            {loading ? 'Refreshing...' : 'Refresh'}
          </button>
        </div>
      </div>

      {/* Workload Selector & Global Controls */}
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '16px', marginBottom: '24px', backgroundColor: '#1e293b', padding: '16px', borderRadius: '12px', border: '1px solid #334155', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <label style={{ fontWeight: 600, color: '#f8fafc', fontSize: '0.9rem' }}>Target Workload:</label>
          <select
            value={selectedWorkloadId}
            onChange={(e) => setSelectedWorkloadId(e.target.value)}
            style={{ padding: '8px 14px', borderRadius: '6px', backgroundColor: '#0f172a', color: '#f8fafc', border: '1px solid #475569', fontSize: '0.9rem', minWidth: '240px' }}
          >
            {workloads.length === 0 ? (
              <option value="">No workloads available</option>
            ) : (
              workloads.map((w) => (
                <option key={w.id} value={w.id}>
                  {w.name || w.id} ({w.id}) — {w.status || 'UNKNOWN'}
                </option>
              ))
            )}
          </select>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.85rem', color: '#94a3b8', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            Auto-Refresh (10s)
          </label>

          <button
            onClick={() => {
              setCreateError('')
              setShowCreateModal(true)
            }}
            disabled={!selectedWorkloadId}
            style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', backgroundColor: '#0284c7', color: '#ffffff', fontWeight: 600, cursor: !selectedWorkloadId ? 'not-allowed' : 'pointer' }}
          >
            + Take Snapshot
          </button>
        </div>
      </div>

      {/* Summary Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '16px', marginBottom: '24px' }}>
        <div style={{ backgroundColor: '#1e293b', padding: '16px', borderRadius: '10px', border: '1px solid #334155' }}>
          <div style={{ fontSize: '0.75rem', color: '#94a3b8', textTransform: 'uppercase', fontWeight: 600 }}>Total Snapshots</div>
          <div style={{ fontSize: '1.75rem', fontWeight: 700, color: '#f8fafc', marginTop: '4px' }}>{snapshots.length}</div>
        </div>

        <div style={{ backgroundColor: '#1e293b', padding: '16px', borderRadius: '10px', border: '1px solid #334155' }}>
          <div style={{ fontSize: '0.75rem', color: '#94a3b8', textTransform: 'uppercase', fontWeight: 600 }}>Active Workload</div>
          <div style={{ fontSize: '1.1rem', fontWeight: 600, color: '#38bdf8', marginTop: '4px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {selectedWorkloadObj ? selectedWorkloadObj.name || selectedWorkloadObj.id : 'None Selected'}
          </div>
        </div>

        <div style={{ backgroundColor: '#1e293b', padding: '16px', borderRadius: '10px', border: '1px solid #334155' }}>
          <div style={{ fontSize: '0.75rem', color: '#94a3b8', textTransform: 'uppercase', fontWeight: 600 }}>Provider Backend</div>
          <div style={{ fontSize: '1.1rem', fontWeight: 600, color: '#10b981', marginTop: '4px' }}>Incus Native Engine</div>
        </div>

        <div style={{ backgroundColor: '#1e293b', padding: '16px', borderRadius: '10px', border: '1px solid #334155' }}>
          <div style={{ fontSize: '0.75rem', color: '#94a3b8', textTransform: 'uppercase', fontWeight: 600 }}>Persistence Store</div>
          <div style={{ fontSize: '0.85rem', fontWeight: 500, color: '#cbd5e1', marginTop: '6px' }}>/var/lib/mystic/snapshots.json</div>
        </div>
      </div>

      {/* Error Banner */}
      {errorMsg && (
        <div style={{ padding: '12px 16px', backgroundColor: 'rgba(239, 68, 68, 0.1)', border: '1px solid #ef4444', color: '#fca5a5', borderRadius: '8px', marginBottom: '24px', fontSize: '0.9rem' }}>
          ⚠️ {errorMsg}
        </div>
      )}

      {/* Snapshot List Table */}
      <div style={{ backgroundColor: '#1e293b', borderRadius: '12px', border: '1px solid #334155', overflow: 'hidden', marginBottom: '24px' }}>
        <div style={{ padding: '16px 20px', borderBottom: '1px solid #334155', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h2 style={{ margin: 0, fontSize: '1.1rem', color: '#f8fafc' }}>Persisted Workload Snapshots</h2>
          <span style={{ fontSize: '0.8rem', color: '#94a3b8' }}>Showing snapshots for instance: {selectedWorkloadObj?.name || selectedWorkloadId || 'N/A'}</span>
        </div>

        {snapshots.length === 0 ? (
          <div style={{ padding: '48px', textAlign: 'center', color: '#94a3b8' }}>
            <p style={{ margin: '0 0 12px 0', fontSize: '1rem' }}>No snapshots exist for this workload.</p>
            <p style={{ margin: 0, fontSize: '0.85rem', color: '#64748b' }}>Click "Take Snapshot" above to create an immediate point-in-time recovery image.</p>
          </div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.9rem' }}>
              <thead>
                <tr style={{ backgroundColor: '#0f172a', borderBottom: '1px solid #334155', color: '#94a3b8', fontSize: '0.75rem', textTransform: 'uppercase' }}>
                  <th style={{ padding: '12px 16px' }}>Snapshot Name</th>
                  <th style={{ padding: '12px 16px' }}>Workload / Instance</th>
                  <th style={{ padding: '12px 16px' }}>Stateful</th>
                  <th style={{ padding: '12px 16px' }}>Created At</th>
                  <th style={{ padding: '12px 16px' }}>Status</th>
                  <th style={{ padding: '12px 16px', textAlign: 'right' }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {snapshots.map((snap) => (
                  <tr key={snap.id} style={{ borderBottom: '1px solid #334155' }}>
                    <td style={{ padding: '14px 16px', fontWeight: 600, color: '#38bdf8' }}>{snap.name}</td>
                    <td style={{ padding: '14px 16px', color: '#cbd5e1' }}>{snap.workload_name || snap.workload_id}</td>
                    <td style={{ padding: '14px 16px' }}>
                      <span style={{ padding: '2px 8px', borderRadius: '12px', fontSize: '0.75rem', fontWeight: 600, backgroundColor: snap.stateful ? 'rgba(168, 85, 247, 0.2)' : 'rgba(100, 116, 139, 0.2)', color: snap.stateful ? '#c084fc' : '#94a3b8' }}>
                        {snap.stateful ? 'RAM + Disk' : 'Disk Only'}
                      </span>
                    </td>
                    <td style={{ padding: '14px 16px', color: '#94a3b8' }}>{new Date(snap.created_at).toLocaleString()}</td>
                    <td style={{ padding: '14px 16px' }}>
                      <span style={{ padding: '2px 8px', borderRadius: '12px', fontSize: '0.75rem', fontWeight: 600, backgroundColor: 'rgba(16, 185, 129, 0.2)', color: '#34d399' }}>
                        {snap.status || 'ACTIVE'}
                      </span>
                    </td>
                    <td style={{ padding: '14px 16px', textAlign: 'right' }}>
                      <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
                        <button
                          onClick={() => setInspectTarget(snap)}
                          style={{ padding: '4px 10px', borderRadius: '4px', border: '1px solid #475569', backgroundColor: 'transparent', color: '#cbd5e1', fontSize: '0.8rem', cursor: 'pointer' }}
                        >
                          Inspect
                        </button>
                        <button
                          onClick={() => setRestoreTarget(snap)}
                          style={{ padding: '4px 10px', borderRadius: '4px', border: '1px solid #eab308', backgroundColor: 'rgba(234, 179, 8, 0.1)', color: '#fde047', fontSize: '0.8rem', fontWeight: 600, cursor: 'pointer' }}
                        >
                          Restore
                        </button>
                        <button
                          onClick={() => setDeleteTarget(snap)}
                          style={{ padding: '4px 10px', borderRadius: '4px', border: '1px solid #ef4444', backgroundColor: 'rgba(239, 68, 68, 0.1)', color: '#fca5a5', fontSize: '0.8rem', fontWeight: 600, cursor: 'pointer' }}
                        >
                          Delete
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Session Operation Audit Log */}
      <div style={{ backgroundColor: '#1e293b', borderRadius: '12px', border: '1px solid #334155', padding: '20px' }}>
        <h3 style={{ margin: '0 0 12px 0', fontSize: '1rem', color: '#f8fafc' }}>Session Operation Audit Log</h3>
        {auditLogs.length === 0 ? (
          <p style={{ margin: 0, fontSize: '0.85rem', color: '#64748b' }}>No operations recorded in current session.</p>
        ) : (
          <div style={{ maxHeight: '180px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {auditLogs.map((log, idx) => (
              <div key={idx} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 12px', borderRadius: '6px', backgroundColor: '#0f172a', border: '1px solid #334155', fontSize: '0.8rem' }}>
                <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
                  <span style={{ color: '#64748b' }}>[{log.timestamp}]</span>
                  <span style={{ fontWeight: 600, color: log.result === 'SUCCESS' ? '#34d399' : log.result === 'ERROR' ? '#fca5a5' : '#fde047' }}>
                    {log.operation}
                  </span>
                  <span style={{ color: '#cbd5e1' }}>Target: {log.target}</span>
                </div>
                <span style={{ color: '#94a3b8' }}>{log.message}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Create Snapshot Modal */}
      {showCreateModal && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.7)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
          <div style={{ backgroundColor: '#1e293b', borderRadius: '12px', border: '1px solid #334155', width: '100%', maxWidth: '480px', padding: '24px' }}>
            <h3 style={{ margin: '0 0 16px 0', color: '#38bdf8', fontSize: '1.2rem' }}>Create Workload Snapshot</h3>
            <form onSubmit={handleCreateSubmit}>
              <div style={{ marginBottom: '16px' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', color: '#94a3b8', marginBottom: '6px' }}>Target Workload Instance:</label>
                <input
                  type="text"
                  disabled
                  value={selectedWorkloadObj?.name || selectedWorkloadId}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: '6px', backgroundColor: '#0f172a', color: '#94a3b8', border: '1px solid #334155', boxSizing: 'border-box' }}
                />
              </div>

              <div style={{ marginBottom: '16px' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', color: '#f8fafc', marginBottom: '6px', fontWeight: 600 }}>Snapshot Name *</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. snap-pre-maintenance-01"
                  value={createSnapName}
                  onChange={(e) => setCreateSnapName(e.target.value)}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: '6px', backgroundColor: '#0f172a', color: '#f8fafc', border: '1px solid #475569', boxSizing: 'border-box' }}
                />
              </div>

              <div style={{ marginBottom: '16px' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', color: '#f8fafc', marginBottom: '6px' }}>Description (Optional)</label>
                <input
                  type="text"
                  placeholder="e.g. State backup prior to database migration"
                  value={createDescription}
                  onChange={(e) => setCreateDescription(e.target.value)}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: '6px', backgroundColor: '#0f172a', color: '#f8fafc', border: '1px solid #475569', boxSizing: 'border-box' }}
                />
              </div>

              <div style={{ marginBottom: '24px' }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.85rem', color: '#cbd5e1', cursor: 'pointer' }}>
                  <input
                    type="checkbox"
                    checked={createStateful}
                    onChange={(e) => setCreateStateful(e.target.checked)}
                  />
                  Capture Stateful RAM Memory (Requires stateful provider support)
                </label>
              </div>

              {createError && (
                <div style={{ padding: '10px', backgroundColor: 'rgba(239,68,68,0.1)', border: '1px solid #ef4444', color: '#fca5a5', borderRadius: '6px', marginBottom: '16px', fontSize: '0.85rem' }}>
                  {createError}
                </div>
              )}

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  disabled={creating}
                  style={{ padding: '8px 16px', borderRadius: '6px', border: '1px solid #475569', backgroundColor: 'transparent', color: '#cbd5e1', cursor: 'pointer' }}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={creating}
                  style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', backgroundColor: '#0284c7', color: '#ffffff', fontWeight: 600, cursor: creating ? 'not-allowed' : 'pointer' }}
                >
                  {creating ? 'Creating Snapshot...' : 'Create Snapshot'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Restore Confirmation Modal */}
      {restoreTarget && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.7)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
          <div style={{ backgroundColor: '#1e293b', borderRadius: '12px', border: '1px solid #eab308', width: '100%', maxWidth: '440px', padding: '24px' }}>
            <h3 style={{ margin: '0 0 12px 0', color: '#fde047', fontSize: '1.2rem' }}>Confirm Workload Reversion</h3>
            <p style={{ fontSize: '0.9rem', color: '#cbd5e1', margin: '0 0 16px 0' }}>
              Are you sure you want to restore workload <strong style={{ color: '#38bdf8' }}>{restoreTarget.workload_name || restoreTarget.workload_id}</strong> to snapshot <strong style={{ color: '#fde047' }}>{restoreTarget.name}</strong>?
            </p>
            <div style={{ padding: '12px', backgroundColor: 'rgba(234, 179, 8, 0.1)', border: '1px solid #eab308', borderRadius: '6px', color: '#fef08a', fontSize: '0.8rem', marginBottom: '20px' }}>
              ⚠️ WARNING: Any un-snapshot-backed changes made to the workload filesystem since {new Date(restoreTarget.created_at).toLocaleString()} will be reverted.
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
              <button
                onClick={() => setRestoreTarget(null)}
                disabled={restoring}
                style={{ padding: '8px 16px', borderRadius: '6px', border: '1px solid #475569', backgroundColor: 'transparent', color: '#cbd5e1', cursor: 'pointer' }}
              >
                Cancel
              </button>
              <button
                onClick={handleConfirmRestore}
                disabled={restoring}
                style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', backgroundColor: '#ca8a04', color: '#ffffff', fontWeight: 600, cursor: restoring ? 'not-allowed' : 'pointer' }}
              >
                {restoring ? 'Restoring...' : 'Confirm Restore'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deleteTarget && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.7)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 }}>
          <div style={{ backgroundColor: '#1e293b', borderRadius: '12px', border: '1px solid #ef4444', width: '100%', maxWidth: '440px', padding: '24px' }}>
            <h3 style={{ margin: '0 0 12px 0', color: '#fca5a5', fontSize: '1.2rem' }}>Confirm Snapshot Deletion</h3>
            <p style={{ fontSize: '0.9rem', color: '#cbd5e1', margin: '0 0 20px 0' }}>
              Are you sure you want to delete snapshot <strong style={{ color: '#fca5a5' }}>{deleteTarget.name}</strong> from workload <strong style={{ color: '#38bdf8' }}>{deleteTarget.workload_name || deleteTarget.workload_id}</strong>?
            </p>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
              <button
                onClick={() => setDeleteTarget(null)}
                disabled={deleting}
                style={{ padding: '8px 16px', borderRadius: '6px', border: '1px solid #475569', backgroundColor: 'transparent', color: '#cbd5e1', cursor: 'pointer' }}
              >
                Cancel
              </button>
              <button
                onClick={handleConfirmDelete}
                disabled={deleting}
                style={{ padding: '8px 16px', borderRadius: '6px', border: 'none', backgroundColor: '#dc2626', color: '#ffffff', fontWeight: 600, cursor: deleting ? 'not-allowed' : 'pointer' }}
              >
                {deleting ? 'Deleting...' : 'Delete Snapshot'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Inspection Drawer */}
      {inspectTarget && (
        <div style={{ position: 'fixed', top: 0, right: 0, width: '480px', height: '100vh', backgroundColor: '#1e293b', borderLeft: '1px solid #334155', padding: '24px', boxSizing: 'border-box', overflowY: 'auto', zIndex: 1000 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
            <h3 style={{ margin: 0, color: '#38bdf8', fontSize: '1.2rem' }}>Snapshot Inspection</h3>
            <button
              onClick={() => setInspectTarget(null)}
              style={{ background: 'none', border: 'none', color: '#94a3b8', fontSize: '1.2rem', cursor: 'pointer' }}
            >
              ✕
            </button>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', fontSize: '0.85rem' }}>
            <div>
              <span style={{ color: '#94a3b8' }}>Snapshot ID:</span>
              <div style={{ fontWeight: 600, color: '#f8fafc', marginTop: '2px' }}>{inspectTarget.id}</div>
            </div>

            <div>
              <span style={{ color: '#94a3b8' }}>Name:</span>
              <div style={{ fontWeight: 600, color: '#38bdf8', marginTop: '2px' }}>{inspectTarget.name}</div>
            </div>

            <div>
              <span style={{ color: '#94a3b8' }}>Target Instance:</span>
              <div style={{ fontWeight: 600, color: '#cbd5e1', marginTop: '2px' }}>{inspectTarget.workload_name || inspectTarget.workload_id}</div>
            </div>

            <div>
              <span style={{ color: '#94a3b8' }}>Created Timestamp:</span>
              <div style={{ color: '#cbd5e1', marginTop: '2px' }}>{new Date(inspectTarget.created_at).toISOString()}</div>
            </div>

            <div>
              <span style={{ color: '#94a3b8' }}>Raw Provider Metadata:</span>
              <pre style={{ marginTop: '6px', padding: '12px', backgroundColor: '#0f172a', borderRadius: '6px', border: '1px solid #334155', color: '#38bdf8', fontSize: '0.75rem', overflowX: 'auto' }}>
                {JSON.stringify(inspectTarget, null, 2)}
              </pre>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
