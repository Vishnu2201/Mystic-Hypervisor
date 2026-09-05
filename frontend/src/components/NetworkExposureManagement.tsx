import React, { useState, useEffect, useCallback } from 'react'
import {
  NetworkExposure,
  NetworkExposureStatus,
  WorkloadSummary,
  ExposureValidationResult
} from '../types'
import {
  checkBackendHealth,
  fetchExposures,
  fetchExposureProviderStatus,
  fetchWorkloads,
  createExposure,
  validateExposure,
  applyExposure,
  reconcileExposure,
  deleteExposure
} from '../api'

interface SessionOperationLog {
  id: string
  action: 'APPLY' | 'RECONCILE' | 'VALIDATE' | 'DELETE' | 'CREATE'
  target: string
  timestamp: string
  status: 'SUCCESS' | 'FAILED'
  message: string
}

export const NetworkExposureManagement: React.FC = () => {
  const [exposures, setExposures] = useState<NetworkExposure[]>([])
  const [workloads, setWorkloads] = useState<WorkloadSummary[]>([])
  const [selectedExposure, setSelectedExposure] = useState<NetworkExposure | null>(null)
  const [providerStatus, setProviderStatus] = useState<NetworkExposureStatus | null>(null)
  const [loadingProviderStatus, setLoadingProviderStatus] = useState(false)
  const [providerError, setProviderError] = useState<string | null>(null)

  const [isBackendHealthy, setIsBackendHealthy] = useState<boolean | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [lastRefreshed, setLastRefreshed] = useState<Date | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(false)

  const [actionLoading, setActionLoading] = useState<Record<string, string>>({})
  const [feedback, setFeedback] = useState<{ type: 'success' | 'error' | 'info'; message: string } | null>(null)
  const [sessionLogs, setSessionLogs] = useState<SessionOperationLog[]>([])

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [applyConfirmExp, setApplyConfirmExp] = useState<NetworkExposure | null>(null)
  const [deleteConfirmExp, setDeleteConfirmExp] = useState<NetworkExposure | null>(null)
  const [validationModalData, setValidationModalData] = useState<{ exp: NetworkExposure; result: ExposureValidationResult } | null>(null)

  // Create Form State
  const [createForm, setCreateForm] = useState({
    id: '',
    workload_id: '',
    exposure_mode: 'NAT_FORWARDED',
    public_ip: '',
    public_port: '2222',
    internal_ip: '',
    internal_port: '22',
    protocol: 'TCP',
    description: ''
  })

  const addSessionLog = (action: SessionOperationLog['action'], target: string, status: 'SUCCESS' | 'FAILED', message: string) => {
    const entry: SessionOperationLog = {
      id: `log-${Date.now()}-${Math.random().toString(36).substr(2, 4)}`,
      action,
      target,
      timestamp: new Date().toLocaleTimeString(),
      status,
      message
    }
    setSessionLogs((prev) => [entry, ...prev.slice(0, 19)])
  }

  const loadData = useCallback(async (isManualRefresh = false) => {
    if (isManualRefresh) setIsRefreshing(true)
    try {
      const health = await checkBackendHealth()
      setIsBackendHealthy(health.healthy)

      if (health.healthy) {
        const [expList, wlList] = await Promise.all([
          fetchExposures().catch((err) => {
            setFeedback({ type: 'error', message: `Failed to fetch exposures: ${err.message}` })
            return []
          }),
          fetchWorkloads().catch(() => [])
        ])
        setExposures(expList)
        setWorkloads(wlList)

        // If an exposure is selected in detail panel, refresh its provider status as well
        if (selectedExposure) {
          const updatedSelected = expList.find((e) => e.id === selectedExposure.id)
          if (updatedSelected) {
            setSelectedExposure(updatedSelected)
            loadProviderStatus(updatedSelected.id)
          }
        }
      }
    } catch (err: any) {
      setIsBackendHealthy(false)
      setFeedback({ type: 'error', message: `Backend connection error: ${err.message}` })
    } finally {
      setIsLoading(false)
      setIsRefreshing(false)
      setLastRefreshed(new Date())
    }
  }, [selectedExposure])

  const loadProviderStatus = async (id: string) => {
    setLoadingProviderStatus(true)
    setProviderError(null)
    try {
      const status = await fetchExposureProviderStatus(id)
      setProviderStatus(status)
    } catch (err: any) {
      setProviderError(err.message || 'Failed to inspect provider state')
      setProviderStatus(null)
    } finally {
      setLoadingProviderStatus(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  // Auto Refresh Interval (10s)
  useEffect(() => {
    if (!autoRefresh) return
    const timer = setInterval(() => {
      loadData(false)
    }, 10000)
    return () => clearInterval(timer)
  }, [autoRefresh, loadData])

  const handleSelectExposure = (exp: NetworkExposure) => {
    setSelectedExposure(exp)
    loadProviderStatus(exp.id)
  }

  const handleValidate = async (exp: NetworkExposure) => {
    setActionLoading((prev) => ({ ...prev, [exp.id]: 'validating' }))
    setFeedback(null)
    try {
      const result = await validateExposure(exp.id)
      setValidationModalData({ exp, result })
      addSessionLog('VALIDATE', exp.id, result.is_valid ? 'SUCCESS' : 'FAILED', result.message || 'Validation evaluation completed')
    } catch (err: any) {
      setFeedback({ type: 'error', message: `Validation error for ${exp.id}: ${err.message}` })
      addSessionLog('VALIDATE', exp.id, 'FAILED', err.message)
    } finally {
      setActionLoading((prev) => {
        const next = { ...prev }
        delete next[exp.id]
        return next
      })
    }
  }

  const handleApplyExecute = async () => {
    if (!applyConfirmExp) return
    const exp = applyConfirmExp
    setApplyConfirmExp(null)
    setActionLoading((prev) => ({ ...prev, [exp.id]: 'applying' }))
    setFeedback(null)
    try {
      const res = await applyExposure(exp.id)
      const msg = res.message || `Exposure '${exp.id}' successfully applied on Incus provider.`
      setFeedback({ type: 'success', message: msg })
      addSessionLog('APPLY', exp.id, 'SUCCESS', msg)
      await loadData()
      if (selectedExposure?.id === exp.id) {
        await loadProviderStatus(exp.id)
      }
    } catch (err: any) {
      const errMsg = err.message || `Failed to apply exposure '${exp.id}'`
      setFeedback({ type: 'error', message: errMsg })
      addSessionLog('APPLY', exp.id, 'FAILED', errMsg)
      await loadData()
    } finally {
      setActionLoading((prev) => {
        const next = { ...prev }
        delete next[exp.id]
        return next
      })
    }
  }

  const handleReconcile = async (exp: NetworkExposure) => {
    setActionLoading((prev) => ({ ...prev, [exp.id]: 'reconciling' }))
    setFeedback(null)
    try {
      await reconcileExposure(exp.id)
      const msg = `Reconciliation completed for exposure '${exp.id}'.`
      setFeedback({ type: 'success', message: msg })
      addSessionLog('RECONCILE', exp.id, 'SUCCESS', msg)
      await loadData()
      if (selectedExposure?.id === exp.id) {
        await loadProviderStatus(exp.id)
      }
    } catch (err: any) {
      const errMsg = err.message || `Reconciliation failed for '${exp.id}'`
      setFeedback({ type: 'error', message: errMsg })
      addSessionLog('RECONCILE', exp.id, 'FAILED', errMsg)
    } finally {
      setActionLoading((prev) => {
        const next = { ...prev }
        delete next[exp.id]
        return next
      })
    }
  }

  const handleDeleteExecute = async () => {
    if (!deleteConfirmExp) return
    const exp = deleteConfirmExp
    setDeleteConfirmExp(null)
    setActionLoading((prev) => ({ ...prev, [exp.id]: 'deleting' }))
    setFeedback(null)
    try {
      const res = await deleteExposure(exp.id)
      const msg = res.message || `Exposure '${exp.id}' deleted cleanly.`
      setFeedback({ type: 'success', message: msg })
      addSessionLog('DELETE', exp.id, 'SUCCESS', msg)
      if (selectedExposure?.id === exp.id) {
        setSelectedExposure(null)
        setProviderStatus(null)
      }
      await loadData()
    } catch (err: any) {
      const errMsg = err.message || `Failed to delete exposure '${exp.id}'`
      setFeedback({ type: 'error', message: errMsg })
      addSessionLog('DELETE', exp.id, 'FAILED', errMsg)
    } finally {
      setActionLoading((prev) => {
        const next = { ...prev }
        delete next[exp.id]
        return next
      })
    }
  }

  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFeedback(null)

    if (!createForm.workload_id) {
      setFeedback({ type: 'error', message: 'Workload selection is required.' })
      return
    }

    const selectedWl = workloads.find((w) => w.id === createForm.workload_id)
    const payload: Partial<NetworkExposure> = {
      id: createForm.id.trim() || undefined,
      workload_id: createForm.workload_id,
      workload_name: selectedWl ? selectedWl.name : undefined,
      exposure_mode: createForm.exposure_mode,
      public_ip: createForm.public_ip.trim() || undefined,
      public_port: parseInt(createForm.public_port, 10) || 0,
      internal_ip: createForm.internal_ip.trim() || (selectedWl?.ip_address || ''),
      internal_port: parseInt(createForm.internal_port, 10) || 0,
      protocol: createForm.protocol,
      description: createForm.description.trim() || undefined
    }

    try {
      const res = await createExposure(payload)
      const msg = res.message || `Network exposure created successfully.`
      setFeedback({ type: 'success', message: msg })
      addSessionLog('CREATE', res.exposure?.id || payload.id || 'new', 'SUCCESS', msg)
      setShowCreateModal(false)
      setCreateForm({
        id: '',
        workload_id: '',
        exposure_mode: 'NAT_FORWARDED',
        public_ip: '',
        public_port: '2222',
        internal_ip: '',
        internal_port: '22',
        protocol: 'TCP',
        description: ''
      })
      await loadData()
      if (res.exposure) {
        handleSelectExposure(res.exposure)
      }
    } catch (err: any) {
      const errMsg = err.message || 'Create failed'
      setFeedback({ type: 'error', message: `Create failed: ${errMsg}` })
      addSessionLog('CREATE', payload.id || 'new', 'FAILED', errMsg)
    }
  }

  // Calculate live summary stats
  const totalCount = exposures.length
  const configuredCount = exposures.filter((e) => e.desired_state === 'CONFIGURED').length
  const appliedCount = exposures.filter((e) => e.actual_state === 'APPLIED').length
  const driftedCount = exposures.filter((e) => e.sync_status === 'out_of_sync' || (e.desired_state === 'APPLIED' && e.actual_state !== 'APPLIED')).length
  const unconfiguredCount = exposures.filter((e) => e.actual_state === 'UNCONFIGURED').length

  const getSyncBadge = (exp: NetworkExposure) => {
    const isDrifted = exp.sync_status === 'out_of_sync' || (exp.desired_state === 'APPLIED' && exp.actual_state !== 'APPLIED')
    if (isDrifted) {
      return (
        <span style={{ backgroundColor: '#7c2d12', color: '#ff8800', padding: '4px 8px', borderRadius: '4px', border: '1px solid #ea580c', fontSize: '0.75rem', fontWeight: 'bold' }}>
          ⚠️ DRIFTED
        </span>
      )
    }
    if (exp.actual_state === 'APPLIED' && exp.sync_status === 'in_sync') {
      return (
        <span style={{ backgroundColor: '#064e3b', color: '#34d399', padding: '4px 8px', borderRadius: '4px', border: '1px solid #059669', fontSize: '0.75rem', fontWeight: 'bold' }}>
          ✓ IN SYNC
        </span>
      )
    }
    if (exp.actual_state === 'CONFIGURED') {
      return (
        <span style={{ backgroundColor: '#0c4a6e', color: '#38bdf8', padding: '4px 8px', borderRadius: '4px', border: '1px solid #0284c7', fontSize: '0.75rem', fontWeight: 'bold' }}>
          ● CONFIGURED
        </span>
      )
    }
    return (
      <span style={{ backgroundColor: '#1e293b', color: '#94a3b8', padding: '4px 8px', borderRadius: '4px', border: '1px solid #475569', fontSize: '0.75rem' }}>
        ○ {exp.actual_state || 'UNCONFIGURED'}
      </span>
    )
  }

  // Field-level comparison helper for inspection panel
  const renderComparisonRow = (label: string, desiredVal: string, providerVal: string | undefined, expectedVal: string) => {
    let matchStatus: 'MATCH' | 'DIFFERENT' | 'NOT PRESENT' = 'NOT PRESENT'
    if (providerVal) {
      matchStatus = providerVal === expectedVal ? 'MATCH' : 'DIFFERENT'
    }

    const badgeColor = matchStatus === 'MATCH' ? '#34d399' : matchStatus === 'DIFFERENT' ? '#f97316' : '#94a3b8'

    return (
      <tr style={{ borderBottom: '1px solid #1e293b' }}>
        <td style={{ padding: '6px 4px', color: '#94a3b8' }}>{label}</td>
        <td style={{ padding: '6px 4px', color: '#cbd5e1', fontFamily: 'monospace' }}>{desiredVal}</td>
        <td style={{ padding: '6px 4px', color: '#cbd5e1', fontFamily: 'monospace' }}>{providerVal || 'None'}</td>
        <td style={{ padding: '6px 4px', color: badgeColor, fontWeight: 'bold' }}>
          {matchStatus === 'MATCH' ? '✓ MATCH' : matchStatus === 'DIFFERENT' ? '⚠ DIFFERENT' : '○ ABSENT'}
        </td>
      </tr>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', color: '#f8fafc' }}>
      {/* HEADER BAR */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', backgroundColor: '#1e293b', padding: '20px', borderRadius: '8px', border: '1px solid #334155' }}>
        <div>
          <h1 style={{ margin: 0, fontSize: '1.5rem', color: '#38bdf8' }}>🌐 Live Network Exposure Engine</h1>
          <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>
            Manage, validate, apply, reconcile, and inspect live network forwarding rules.
          </span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '15px' }}>
          {/* Connection Status Badge */}
          <div style={{ fontSize: '0.8rem', padding: '6px 12px', borderRadius: '16px', border: '1px solid', backgroundColor: isBackendHealthy ? '#064e3b' : isBackendHealthy === false ? '#7f1d1d' : '#1e293b', borderColor: isBackendHealthy ? '#059669' : isBackendHealthy === false ? '#dc2626' : '#475569', color: isBackendHealthy ? '#34d399' : isBackendHealthy === false ? '#f87171' : '#cbd5e1' }}>
            {isBackendHealthy === true ? '● Backend API Connected' : isBackendHealthy === false ? '● Backend Unavailable' : '● Checking Health...'}
          </div>

          {/* Last Refreshed */}
          {lastRefreshed && (
            <span style={{ fontSize: '0.75rem', color: '#64748b' }}>
              Refreshed: {lastRefreshed.toLocaleTimeString()}
            </span>
          )}

          {/* Auto Refresh Toggle */}
          <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.8rem', color: '#cbd5e1', cursor: 'pointer', backgroundColor: '#0f172a', padding: '6px 10px', borderRadius: '6px', border: '1px solid #334155' }}>
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
              style={{ accentColor: '#38bdf8' }}
            />
            <span>Auto refresh (10s)</span>
          </label>

          {/* Refresh Button */}
          <button
            type="button"
            onClick={() => loadData(true)}
            disabled={isRefreshing}
            style={{ backgroundColor: '#0f172a', color: '#38bdf8', border: '1px solid #0284c7', padding: '6px 14px', borderRadius: '6px', cursor: 'pointer', fontWeight: 'bold', fontSize: '0.85rem' }}
          >
            {isRefreshing ? '🔄 Refreshing...' : '🔄 Refresh'}
          </button>

          {/* Create Exposure Button */}
          <button
            type="button"
            onClick={() => setShowCreateModal(true)}
            style={{ backgroundColor: '#0284c7', color: '#ffffff', border: 'none', padding: '8px 16px', borderRadius: '6px', cursor: 'pointer', fontWeight: 'bold', fontSize: '0.9rem' }}
          >
            + Create Exposure
          </button>
        </div>
      </div>

      {/* FEEDBACK BANNER */}
      {feedback && (
        <div
          style={{
            padding: '12px 16px',
            borderRadius: '6px',
            fontSize: '0.9rem',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            backgroundColor: feedback.type === 'success' ? '#064e3b' : feedback.type === 'error' ? '#7f1d1d' : '#0c4a6e',
            color: feedback.type === 'success' ? '#6ee7b7' : feedback.type === 'error' ? '#fca5a5' : '#7dd3fc',
            border: `1px solid ${feedback.type === 'success' ? '#059669' : feedback.type === 'error' ? '#dc2626' : '#0284c7'}`
          }}
        >
          <div>
            <strong>{feedback.type === 'success' ? '✓ Success: ' : feedback.type === 'error' ? '✕ Error: ' : 'ℹ Notice: '}</strong>
            {feedback.message}
          </div>
          <button
            type="button"
            onClick={() => setFeedback(null)}
            style={{ background: 'none', border: 'none', color: 'inherit', cursor: 'pointer', fontWeight: 'bold' }}
          >
            ✕
          </button>
        </div>
      )}

      {/* SUMMARY CARDS */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '15px' }}>
        <div style={{ backgroundColor: '#1e293b', padding: '16px', borderRadius: '8px', border: '1px solid #334155' }}>
          <span style={{ fontSize: '0.8rem', color: '#94a3b8', display: 'block' }}>Total Exposures</span>
          <strong style={{ fontSize: '1.6rem', color: '#f8fafc' }}>{totalCount}</strong>
        </div>

        <div style={{ backgroundColor: '#1e293b', padding: '16px', borderRadius: '8px', border: '1px solid #334155' }}>
          <span style={{ fontSize: '0.8rem', color: '#38bdf8', display: 'block' }}>Configured</span>
          <strong style={{ fontSize: '1.6rem', color: '#38bdf8' }}>{configuredCount}</strong>
        </div>

        <div style={{ backgroundColor: '#1e293b', padding: '16px', borderRadius: '8px', border: '1px solid #334155' }}>
          <span style={{ fontSize: '0.8rem', color: '#34d399', display: 'block' }}>Applied</span>
          <strong style={{ fontSize: '1.6rem', color: '#34d399' }}>{appliedCount}</strong>
        </div>

        <div style={{ backgroundColor: '#1e293b', padding: '16px', borderRadius: '8px', border: '1px solid #334155' }}>
          <span style={{ fontSize: '0.8rem', color: '#f97316', display: 'block' }}>Drifted</span>
          <strong style={{ fontSize: '1.6rem', color: '#f97316' }}>{driftedCount}</strong>
        </div>

        <div style={{ backgroundColor: '#1e293b', padding: '16px', borderRadius: '8px', border: '1px solid #334155' }}>
          <span style={{ fontSize: '0.8rem', color: '#64748b', display: 'block' }}>Unconfigured</span>
          <strong style={{ fontSize: '1.6rem', color: '#cbd5e1' }}>{unconfiguredCount}</strong>
        </div>
      </div>

      {/* MAIN CONTAINER: TABLE & INSPECTION PANEL */}
      <div style={{ display: 'grid', gridTemplateColumns: selectedExposure ? '1fr 440px' : '1fr', gap: '20px' }}>
        {/* EXPOSURES TABLE */}
        <div style={{ backgroundColor: '#1e293b', borderRadius: '8px', border: '1px solid #334155', padding: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '15px' }}>
            <h3 style={{ margin: 0, color: '#38bdf8', fontSize: '1.1rem' }}>📋 Configured Network Exposures</h3>
            <span style={{ fontSize: '0.8rem', color: '#64748b' }}>Click any row to open detailed provider inspection</span>
          </div>

          {isLoading ? (
            <div style={{ padding: '40px', textAlign: 'center', color: '#94a3b8' }}>Loading network exposures from backend API...</div>
          ) : exposures.length === 0 ? (
            <div style={{ padding: '40px', textAlign: 'center', backgroundColor: '#0f172a', borderRadius: '6px', border: '1px solid #1e293b' }}>
              <p style={{ margin: '0 0 15px 0', color: '#cbd5e1', fontSize: '1rem' }}>No network exposures configured.</p>
              <button
                type="button"
                onClick={() => setShowCreateModal(true)}
                style={{ backgroundColor: '#0284c7', color: '#fff', border: 'none', padding: '8px 16px', borderRadius: '6px', cursor: 'pointer', fontWeight: 'bold' }}
              >
                + Create First Exposure
              </button>
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem', color: '#e2e8f0' }}>
                <thead>
                  <tr style={{ backgroundColor: '#0f172a', color: '#38bdf8', textAlign: 'left', borderBottom: '2px solid #334155' }}>
                    <th style={{ padding: '10px 12px' }}>Status</th>
                    <th style={{ padding: '10px 12px' }}>Exposure ID</th>
                    <th style={{ padding: '10px 12px' }}>Workload</th>
                    <th style={{ padding: '10px 12px' }}>Public Endpoint</th>
                    <th style={{ padding: '10px 12px' }}>Destination</th>
                    <th style={{ padding: '10px 12px' }}>Proto</th>
                    <th style={{ padding: '10px 12px' }}>Desired</th>
                    <th style={{ padding: '10px 12px' }}>Actual</th>
                    <th style={{ padding: '10px 12px', textAlign: 'right' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {exposures.map((exp) => {
                    const isSelected = selectedExposure?.id === exp.id
                    const isActioning = actionLoading[exp.id]
                    const pubEndpoint = `${exp.public_ip || '0.0.0.0'}:${exp.public_port}`
                    const destEndpoint = `${exp.internal_ip}:${exp.internal_port}`

                    return (
                      <tr
                        key={exp.id}
                        onClick={() => handleSelectExposure(exp)}
                        style={{
                          backgroundColor: isSelected ? '#0f172a' : 'transparent',
                          borderBottom: '1px solid #334155',
                          cursor: 'pointer',
                          transition: 'background-color 0.15s ease'
                        }}
                      >
                        <td style={{ padding: '10px 12px' }}>{getSyncBadge(exp)}</td>
                        <td style={{ padding: '10px 12px', fontWeight: 'bold', color: '#f8fafc' }}>{exp.id}</td>
                        <td style={{ padding: '10px 12px', color: '#cbd5e1' }}>
                          <div>{exp.workload_name || exp.workload_id}</div>
                          <span style={{ fontSize: '0.75rem', color: '#64748b' }}>{exp.workload_id}</span>
                        </td>
                        <td style={{ padding: '10px 12px', color: '#38bdf8', fontFamily: 'monospace' }}>{pubEndpoint}</td>
                        <td style={{ padding: '10px 12px', color: '#cbd5e1', fontFamily: 'monospace' }}>→ {destEndpoint}</td>
                        <td style={{ padding: '10px 12px', color: '#facc15', fontWeight: 'bold' }}>{exp.protocol}</td>
                        <td style={{ padding: '10px 12px', fontSize: '0.75rem', color: '#94a3b8' }}>{exp.desired_state}</td>
                        <td style={{ padding: '10px 12px', fontSize: '0.75rem', color: '#94a3b8' }}>{exp.actual_state}</td>
                        <td style={{ padding: '10px 12px', textAlign: 'right' }} onClick={(e) => e.stopPropagation()}>
                          <div style={{ display: 'flex', gap: '6px', justifyContent: 'flex-end' }}>
                            <button
                              type="button"
                              onClick={() => handleValidate(exp)}
                              disabled={!!isActioning}
                              title="Validate port allocation"
                              style={{ backgroundColor: '#0f172a', color: '#38bdf8', border: '1px solid #334155', padding: '4px 8px', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer' }}
                            >
                              {isActioning === 'validating' ? '...' : 'Validate'}
                            </button>

                            <button
                              type="button"
                              onClick={() => setApplyConfirmExp(exp)}
                              disabled={!!isActioning}
                              title="Apply exposure on Incus provider"
                              style={{ backgroundColor: '#0284c7', color: '#fff', border: 'none', padding: '4px 8px', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer', fontWeight: 'bold' }}
                            >
                              {isActioning === 'applying' ? '...' : 'Apply'}
                            </button>

                            <button
                              type="button"
                              onClick={() => handleReconcile(exp)}
                              disabled={!!isActioning}
                              title="Reconcile provider state"
                              style={{ backgroundColor: '#0f172a', color: '#facc15', border: '1px solid #334155', padding: '4px 8px', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer' }}
                            >
                              {isActioning === 'reconciling' ? '...' : 'Reconcile'}
                            </button>

                            <button
                              type="button"
                              onClick={() => handleSelectExposure(exp)}
                              style={{ backgroundColor: '#0f172a', color: '#cbd5e1', border: '1px solid #334155', padding: '4px 8px', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer' }}
                            >
                              Inspect
                            </button>

                            <button
                              type="button"
                              onClick={() => setDeleteConfirmExp(exp)}
                              disabled={!!isActioning}
                              title="Delete exposure"
                              style={{ backgroundColor: '#7f1d1d', color: '#fca5a5', border: 'none', padding: '4px 8px', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer' }}
                            >
                              {isActioning === 'deleting' ? '...' : 'Delete'}
                            </button>
                          </div>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* INSPECTION PANEL / DRAWER */}
        {selectedExposure && (
          <div style={{ backgroundColor: '#1e293b', borderRadius: '8px', border: '1px solid #38bdf8', padding: '20px', display: 'flex', flexDirection: 'column', gap: '20px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px solid #334155', paddingBottom: '12px' }}>
              <div>
                <h3 style={{ margin: 0, color: '#38bdf8', fontSize: '1.1rem' }}>🔍 Exposure Inspection</h3>
                <span style={{ fontSize: '0.75rem', color: '#94a3b8' }}>ID: {selectedExposure.id}</span>
              </div>
              <button
                type="button"
                onClick={() => setSelectedExposure(null)}
                style={{ background: 'none', border: 'none', color: '#94a3b8', fontSize: '1.2rem', cursor: 'pointer' }}
              >
                ✕
              </button>
            </div>

            {/* SECTION A: DESIRED VS ACTUAL METADATA */}
            <div style={{ backgroundColor: '#0f172a', padding: '14px', borderRadius: '6px', border: '1px solid #1e293b', fontSize: '0.85rem' }}>
              <span style={{ fontSize: '0.8rem', color: '#38bdf8', fontWeight: 'bold', display: 'block', marginBottom: '8px' }}>
                Mystic Desired Configuration:
              </span>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px', fontSize: '0.8rem' }}>
                <div><span style={{ color: '#94a3b8' }}>Workload:</span> {selectedExposure.workload_name || selectedExposure.workload_id}</div>
                <div><span style={{ color: '#94a3b8' }}>Workload ID:</span> {selectedExposure.workload_id}</div>
                <div><span style={{ color: '#94a3b8' }}>Exposure Mode:</span> {selectedExposure.exposure_mode}</div>
                <div><span style={{ color: '#94a3b8' }}>Protocol:</span> <strong style={{ color: '#facc15' }}>{selectedExposure.protocol}</strong></div>
                <div><span style={{ color: '#94a3b8' }}>Public IP:</span> {selectedExposure.public_ip || '0.0.0.0'}</div>
                <div><span style={{ color: '#94a3b8' }}>Public Port:</span> {selectedExposure.public_port}</div>
                <div><span style={{ color: '#94a3b8' }}>Internal IP:</span> {selectedExposure.internal_ip}</div>
                <div><span style={{ color: '#94a3b8' }}>Internal Port:</span> {selectedExposure.internal_port}</div>
                <div><span style={{ color: '#94a3b8' }}>Desired State:</span> {selectedExposure.desired_state}</div>
                <div><span style={{ color: '#94a3b8' }}>Actual State:</span> {selectedExposure.actual_state}</div>
                <div><span style={{ color: '#94a3b8' }}>Sync Status:</span> {selectedExposure.sync_status}</div>
                <div><span style={{ color: '#94a3b8' }}>Created:</span> {new Date(selectedExposure.created_at).toLocaleTimeString()}</div>
              </div>
            </div>

            {/* SECTION B: REAL DYNAMIC NETWORK FLOW DIAGRAM */}
            <div style={{ backgroundColor: '#0f172a', padding: '14px', borderRadius: '6px', border: '1px solid #1e293b' }}>
              <span style={{ fontSize: '0.8rem', color: '#38bdf8', fontWeight: 'bold', display: 'block', marginBottom: '8px' }}>
                🗺️ Live Network Topology Path:
              </span>
              <pre style={{ margin: 0, fontSize: '0.75rem', fontFamily: 'monospace', color: '#38bdf8' }}>
{`  PUBLIC ENDPOINT
  ${selectedExposure.public_ip || '51.162.178.199'}:${selectedExposure.public_port}
        │
        │ ${selectedExposure.protocol} / ${selectedExposure.exposure_mode}
        ▼
  INCUS PROXY DEVICE
  mystic-exp-${selectedExposure.id}
        │
        ▼
  TARGET WORKLOAD (${selectedExposure.workload_name || selectedExposure.workload_id})
  ${selectedExposure.internal_ip}
        │
        ▼
  DESTINATION SERVICE PORT
  ${selectedExposure.internal_ip}:${selectedExposure.internal_port}`}
              </pre>
            </div>

            {/* SECTION C: REAL INCUS PROVIDER INSPECTION */}
            <div style={{ backgroundColor: '#0f172a', padding: '14px', borderRadius: '6px', border: '1px solid #1e293b' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                <span style={{ fontSize: '0.8rem', color: '#34d399', fontWeight: 'bold' }}>
                  ⚡ Real Incus Provider Observed State:
                </span>
                <button
                  type="button"
                  onClick={() => loadProviderStatus(selectedExposure.id)}
                  disabled={loadingProviderStatus}
                  style={{ backgroundColor: 'transparent', color: '#38bdf8', border: 'none', cursor: 'pointer', fontSize: '0.75rem' }}
                >
                  {loadingProviderStatus ? 'Querying socket...' : 'Re-query Incus'}
                </button>
              </div>

              {loadingProviderStatus ? (
                <div style={{ fontSize: '0.8rem', color: '#94a3b8', padding: '10px 0' }}>Executing incus query /1.0/instances...</div>
              ) : providerError ? (
                <div style={{ color: '#f87171', fontSize: '0.8rem', backgroundColor: '#450a0a', padding: '8px', borderRadius: '4px' }}>
                  ✕ Incus Query Warning: {providerError}
                </div>
              ) : providerStatus ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', fontSize: '0.8rem' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span style={{ color: '#94a3b8' }}>Provider Device Status:</span>
                    <strong style={{ color: providerStatus.active ? '#34d399' : '#f97316' }}>
                      {providerStatus.active ? 'ACTIVE & APPLIED' : 'DEVICE ABSENT / UNCONFIGURED'}
                    </strong>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '6px', backgroundColor: '#1e293b', padding: '8px', borderRadius: '4px' }}>
                    <div><span style={{ color: '#94a3b8' }}>Instance:</span> {providerStatus.instance_name || 'N/A'}</div>
                    <div><span style={{ color: '#94a3b8' }}>Device Name:</span> {providerStatus.device_name || 'N/A'}</div>
                    <div><span style={{ color: '#94a3b8' }}>Device Type:</span> {providerStatus.device_type || 'proxy'}</div>
                    <div><span style={{ color: '#94a3b8' }}>NAT Setting:</span> {providerStatus.nat || 'true'}</div>
                    <div style={{ gridColumn: 'span 2' }}>
                      <span style={{ color: '#94a3b8' }}>Listen:</span> <code>{providerStatus.listen || 'None'}</code>
                    </div>
                    <div style={{ gridColumn: 'span 2' }}>
                      <span style={{ color: '#94a3b8' }}>Connect:</span> <code>{providerStatus.connect || 'None'}</code>
                    </div>
                  </div>

                  {providerStatus.raw_device && (
                    <details style={{ marginTop: '4px' }}>
                      <summary style={{ cursor: 'pointer', color: '#38bdf8', fontSize: '0.75rem' }}>View raw Incus device JSON</summary>
                      <pre style={{ backgroundColor: '#1e293b', padding: '8px', borderRadius: '4px', fontSize: '0.7rem', color: '#cbd5e1', overflowX: 'auto' }}>
                        {JSON.stringify(providerStatus.raw_device, null, 2)}
                      </pre>
                    </details>
                  )}
                </div>
              ) : (
                <div style={{ fontSize: '0.8rem', color: '#94a3b8' }}>No provider status returned.</div>
              )}
            </div>

            {/* SECTION D: FIELD-LEVEL DESIRED VS PROVIDER DRIFT COMPARISON */}
            <div style={{ backgroundColor: '#0f172a', padding: '14px', borderRadius: '6px', border: '1px solid #1e293b' }}>
              <span style={{ fontSize: '0.8rem', color: '#facc15', fontWeight: 'bold', display: 'block', marginBottom: '8px' }}>
                ⚖️ Live Field-Level Reconciliation Comparison:
              </span>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.75rem', color: '#e2e8f0' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid #334155', color: '#94a3b8', textAlign: 'left' }}>
                    <th style={{ padding: '4px' }}>Field</th>
                    <th style={{ padding: '4px' }}>Mystic Desired</th>
                    <th style={{ padding: '4px' }}>Incus Provider</th>
                    <th style={{ padding: '4px' }}>Match</th>
                  </tr>
                </thead>
                <tbody>
                  {renderComparisonRow(
                    'Listen String',
                    `tcp:${selectedExposure.public_ip || '0.0.0.0'}:${selectedExposure.public_port}`,
                    providerStatus?.listen,
                    `tcp:${selectedExposure.public_ip || '0.0.0.0'}:${selectedExposure.public_port}`
                  )}
                  {renderComparisonRow(
                    'Connect String',
                    `tcp:${selectedExposure.internal_ip}:${selectedExposure.internal_port}`,
                    providerStatus?.connect,
                    `tcp:${selectedExposure.internal_ip}:${selectedExposure.internal_port}`
                  )}
                  {renderComparisonRow(
                    'NAT Setting',
                    'true',
                    providerStatus?.nat,
                    'true'
                  )}
                  {renderComparisonRow(
                    'Instance Name',
                    selectedExposure.workload_name || selectedExposure.workload_id,
                    providerStatus?.instance_name,
                    selectedExposure.workload_name || selectedExposure.workload_id
                  )}
                </tbody>
              </table>
            </div>

            {/* QUICK ACTIONS */}
            <div style={{ display: 'flex', gap: '10px' }}>
              <button
                type="button"
                onClick={() => setApplyConfirmExp(selectedExposure)}
                style={{ flex: 1, backgroundColor: '#0284c7', color: '#fff', border: 'none', padding: '8px', borderRadius: '6px', fontWeight: 'bold', cursor: 'pointer' }}
              >
                Apply to Incus
              </button>
              <button
                type="button"
                onClick={() => handleReconcile(selectedExposure)}
                style={{ flex: 1, backgroundColor: '#0f172a', color: '#facc15', border: '1px solid #334155', padding: '8px', borderRadius: '6px', fontWeight: 'bold', cursor: 'pointer' }}
              >
                Reconcile Now
              </button>
            </div>
          </div>
        )}
      </div>

      {/* SESSION OPERATION AUDIT LOG */}
      {sessionLogs.length > 0 && (
        <div style={{ backgroundColor: '#1e293b', borderRadius: '8px', border: '1px solid #334155', padding: '20px' }}>
          <h3 style={{ margin: '0 0 12px 0', color: '#38bdf8', fontSize: '1rem' }}>📜 Session Operation Audit Log</h3>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.75rem', color: '#e2e8f0' }}>
              <thead>
                <tr style={{ backgroundColor: '#0f172a', color: '#94a3b8', textAlign: 'left' }}>
                  <th style={{ padding: '6px 10px' }}>Time</th>
                  <th style={{ padding: '6px 10px' }}>Action</th>
                  <th style={{ padding: '6px 10px' }}>Target Exposure</th>
                  <th style={{ padding: '6px 10px' }}>Result</th>
                  <th style={{ padding: '6px 10px' }}>Response Message</th>
                </tr>
              </thead>
              <tbody>
                {sessionLogs.map((log) => (
                  <tr key={log.id} style={{ borderBottom: '1px solid #334155' }}>
                    <td style={{ padding: '6px 10px', color: '#64748b' }}>{log.timestamp}</td>
                    <td style={{ padding: '6px 10px', fontWeight: 'bold', color: '#38bdf8' }}>{log.action}</td>
                    <td style={{ padding: '6px 10px', fontFamily: 'monospace' }}>{log.target}</td>
                    <td style={{ padding: '6px 10px', fontWeight: 'bold', color: log.status === 'SUCCESS' ? '#34d399' : '#f87171' }}>
                      {log.status === 'SUCCESS' ? '✓ SUCCESS' : '✕ FAILED'}
                    </td>
                    <td style={{ padding: '6px 10px', color: '#cbd5e1' }}>{log.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* CREATE EXPOSURE MODAL */}
      {showCreateModal && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.7)', display: 'flex', justifyContent: 'center', alignItems: 'center', zIndex: 1000 }}>
          <div style={{ backgroundColor: '#1e293b', border: '1px solid #38bdf8', borderRadius: '8px', width: '550px', maxWidth: '90vw', padding: '24px', color: '#f8fafc' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px', borderBottom: '1px solid #334155', paddingBottom: '10px' }}>
              <h3 style={{ margin: 0, color: '#38bdf8' }}>+ Create Network Exposure</h3>
              <button type="button" onClick={() => setShowCreateModal(false)} style={{ background: 'none', border: 'none', color: '#94a3b8', fontSize: '1.2rem', cursor: 'pointer' }}>✕</button>
            </div>

            <form onSubmit={handleCreateSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '15px' }}>
              <div>
                <label style={{ display: 'block', fontSize: '0.85rem', color: '#cbd5e1', marginBottom: '4px' }}>Target Workload *</label>
                <select
                  value={createForm.workload_id}
                  onChange={(e) => {
                    const wlId = e.target.value
                    const wl = workloads.find((w) => w.id === wlId)
                    setCreateForm((prev) => ({
                      ...prev,
                      workload_id: wlId,
                      internal_ip: wl?.ip_address || prev.internal_ip
                    }))
                  }}
                  required
                  style={{ width: '100%', backgroundColor: '#0f172a', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px' }}
                >
                  <option value="">-- Select Workload from Backend --</option>
                  {workloads.map((w) => (
                    <option key={w.id} value={w.id}>
                      {w.name} ({w.id}) - {w.ip_address || 'No IP'}
                    </option>
                  ))}
                </select>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', color: '#cbd5e1', marginBottom: '4px' }}>Exposure ID (Optional)</label>
                  <input
                    type="text"
                    placeholder="exp-custom-01"
                    value={createForm.id}
                    onChange={(e) => setCreateForm({ ...createForm, id: e.target.value })}
                    style={{ width: '100%', backgroundColor: '#0f172a', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                  />
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', color: '#cbd5e1', marginBottom: '4px' }}>Exposure Mode</label>
                  <select
                    value={createForm.exposure_mode}
                    onChange={(e) => setCreateForm({ ...createForm, exposure_mode: e.target.value })}
                    style={{ width: '100%', backgroundColor: '#0f172a', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                  >
                    <option value="NAT_FORWARDED">NAT Forwarded / Proxy</option>
                    <option value="PRIVATE_ONLY">Private Only</option>
                    <option value="DIRECT_PUBLIC">Direct Public IP</option>
                    <option value="EXTERNAL_GATEWAY">External Gateway</option>
                  </select>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', color: '#cbd5e1', marginBottom: '4px' }}>Public Listen IP (Optional)</label>
                  <input
                    type="text"
                    placeholder="e.g. 51.162.178.199"
                    value={createForm.public_ip}
                    onChange={(e) => setCreateForm({ ...createForm, public_ip: e.target.value })}
                    style={{ width: '100%', backgroundColor: '#0f172a', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                  />
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', color: '#cbd5e1', marginBottom: '4px' }}>Public Listen Port *</label>
                  <input
                    type="number"
                    required
                    placeholder="2222"
                    value={createForm.public_port}
                    onChange={(e) => setCreateForm({ ...createForm, public_port: e.target.value })}
                    style={{ width: '100%', backgroundColor: '#0f172a', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                  />
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', color: '#cbd5e1', marginBottom: '4px' }}>Destination Internal IP</label>
                  <input
                    type="text"
                    placeholder="10.170.92.70"
                    value={createForm.internal_ip}
                    onChange={(e) => setCreateForm({ ...createForm, internal_ip: e.target.value })}
                    style={{ width: '100%', backgroundColor: '#0f172a', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                  />
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', color: '#cbd5e1', marginBottom: '4px' }}>Destination Internal Port *</label>
                  <input
                    type="number"
                    required
                    placeholder="22"
                    value={createForm.internal_port}
                    onChange={(e) => setCreateForm({ ...createForm, internal_port: e.target.value })}
                    style={{ width: '100%', backgroundColor: '#0f172a', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                  />
                </div>
              </div>

              <div>
                <label style={{ display: 'block', fontSize: '0.85rem', color: '#cbd5e1', marginBottom: '4px' }}>Protocol</label>
                <select
                  value={createForm.protocol}
                  onChange={(e) => setCreateForm({ ...createForm, protocol: e.target.value })}
                  style={{ width: '100%', backgroundColor: '#0f172a', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px' }}
                >
                  <option value="TCP">TCP</option>
                  <option value="UDP">UDP</option>
                </select>

                {createForm.protocol === 'UDP' && (
                  <div style={{ color: '#facc15', backgroundColor: '#422006', padding: '8px', borderRadius: '4px', fontSize: '0.75rem', marginTop: '6px', border: '1px solid #78350f' }}>
                    ⚠️ Protocol Notice: UDP is not supported by Incus proxy devices in Phase 3 provider driver. Backend validation will enforce TCP.
                  </div>
                )}
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '10px' }}>
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  style={{ backgroundColor: '#0f172a', color: '#cbd5e1', border: '1px solid #334155', padding: '8px 16px', borderRadius: '6px', cursor: 'pointer' }}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  style={{ backgroundColor: '#0284c7', color: '#fff', border: 'none', padding: '8px 16px', borderRadius: '6px', fontWeight: 'bold', cursor: 'pointer' }}
                >
                  Create Exposure
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* APPLY CONFIRMATION MODAL */}
      {applyConfirmExp && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.7)', display: 'flex', justifyContent: 'center', alignItems: 'center', zIndex: 1000 }}>
          <div style={{ backgroundColor: '#1e293b', border: '1px solid #0284c7', borderRadius: '8px', width: '480px', padding: '24px', color: '#f8fafc' }}>
            <h3 style={{ margin: '0 0 15px 0', color: '#38bdf8' }}>⚡ Apply Exposure to Incus Provider</h3>
            <p style={{ fontSize: '0.9rem', color: '#cbd5e1' }}>
              Are you sure you want to apply this exposure? This will execute an Incus device configuration change on the underlying hypervisor host.
            </p>

            <div style={{ backgroundColor: '#0f172a', padding: '12px', borderRadius: '6px', fontSize: '0.85rem', display: 'flex', flexDirection: 'column', gap: '6px', margin: '15px 0' }}>
              <div><span style={{ color: '#94a3b8' }}>Exposure ID:</span> {applyConfirmExp.id}</div>
              <div><span style={{ color: '#94a3b8' }}>Target Workload:</span> {applyConfirmExp.workload_name || applyConfirmExp.workload_id}</div>
              <div><span style={{ color: '#94a3b8' }}>Public Endpoint:</span> <strong style={{ color: '#38bdf8' }}>{applyConfirmExp.public_ip || '0.0.0.0'}:{applyConfirmExp.public_port}</strong></div>
              <div><span style={{ color: '#94a3b8' }}>Destination:</span> <strong style={{ color: '#cbd5e1' }}>{applyConfirmExp.internal_ip}:{applyConfirmExp.internal_port}</strong></div>
              <div><span style={{ color: '#94a3b8' }}>Protocol:</span> {applyConfirmExp.protocol}</div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px' }}>
              <button
                type="button"
                onClick={() => setApplyConfirmExp(null)}
                style={{ backgroundColor: '#0f172a', color: '#cbd5e1', border: '1px solid #334155', padding: '8px 16px', borderRadius: '6px', cursor: 'pointer' }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleApplyExecute}
                style={{ backgroundColor: '#0284c7', color: '#fff', border: 'none', padding: '8px 16px', borderRadius: '6px', fontWeight: 'bold', cursor: 'pointer' }}
              >
                Apply Exposure Now
              </button>
            </div>
          </div>
        </div>
      )}

      {/* VALIDATION RESULT MODAL */}
      {validationModalData && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.7)', display: 'flex', justifyContent: 'center', alignItems: 'center', zIndex: 1000 }}>
          <div style={{ backgroundColor: '#1e293b', border: `1px solid ${validationModalData.result.is_valid ? '#059669' : '#dc2626'}`, borderRadius: '8px', width: '500px', padding: '24px', color: '#f8fafc' }}>
            <h3 style={{ margin: '0 0 15px 0', color: validationModalData.result.is_valid ? '#34d399' : '#f87171' }}>
              {validationModalData.result.is_valid ? '✓ Port Allocation Validation Passed' : '✕ Validation Conflicts Detected'}
            </h3>

            <div style={{ backgroundColor: '#0f172a', padding: '12px', borderRadius: '6px', fontSize: '0.85rem', marginBottom: '15px' }}>
              <p style={{ margin: '0 0 8px 0' }}>{validationModalData.result.message || 'Validation evaluation completed.'}</p>
              {validationModalData.result.conflicts && validationModalData.result.conflicts.length > 0 && (
                <div style={{ marginTop: '10px' }}>
                  <span style={{ color: '#f87171', fontWeight: 'bold', display: 'block', marginBottom: '4px' }}>Conflicts:</span>
                  <ul style={{ margin: 0, paddingLeft: '20px', color: '#fca5a5' }}>
                    {validationModalData.result.conflicts.map((c, i) => (
                      <li key={i}>{c.message}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <button
                type="button"
                onClick={() => setValidationModalData(null)}
                style={{ backgroundColor: '#0284c7', color: '#fff', border: 'none', padding: '8px 16px', borderRadius: '6px', fontWeight: 'bold', cursor: 'pointer' }}
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* DELETE CONFIRMATION MODAL */}
      {deleteConfirmExp && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.7)', display: 'flex', justifyContent: 'center', alignItems: 'center', zIndex: 1000 }}>
          <div style={{ backgroundColor: '#1e293b', border: '1px solid #7f1d1d', borderRadius: '8px', width: '450px', padding: '24px', color: '#f8fafc' }}>
            <h3 style={{ margin: '0 0 15px 0', color: '#f87171' }}>⚠️ Delete Network Exposure</h3>
            <p style={{ fontSize: '0.9rem', color: '#cbd5e1' }}>
              Are you sure you want to delete exposure <strong style={{ color: '#fff' }}>{deleteConfirmExp.id}</strong>? If applied, the underlying Incus proxy device will also be removed.
            </p>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '20px' }}>
              <button
                type="button"
                onClick={() => setDeleteConfirmExp(null)}
                style={{ backgroundColor: '#0f172a', color: '#cbd5e1', border: '1px solid #334155', padding: '8px 16px', borderRadius: '6px', cursor: 'pointer' }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleDeleteExecute}
                style={{ backgroundColor: '#dc2626', color: '#fff', border: 'none', padding: '8px 16px', borderRadius: '6px', fontWeight: 'bold', cursor: 'pointer' }}
              >
                Delete Exposure
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
