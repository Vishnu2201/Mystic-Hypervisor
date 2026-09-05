import React, { useEffect, useState } from 'react'
import { AdoptionPreviewResult } from '../types'

interface AdoptionModalProps {
  instanceName: string
  onClose: () => void
  onSuccess: () => void
}

export const AdoptionModal: React.FC<AdoptionModalProps> = ({ instanceName, onClose, onSuccess }) => {
  const [preview, setPreview] = useState<AdoptionPreviewResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [adopting, setAdopting] = useState(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  useEffect(() => {
    fetchPreview()
  }, [instanceName])

  const fetchPreview = async () => {
    setLoading(true)
    setErrorMsg(null)
    try {
      const res = await fetch(`/api/v1/providers/incus/instances/${encodeURIComponent(instanceName)}/adoption-preview`)
      if (res.ok) {
        const data = await res.json()
        setPreview(data)
      } else {
        const errData = await res.json()
        setErrorMsg(errData.error || 'Failed to load adoption preview.')
      }
    } catch (e) {
      setErrorMsg('Network error fetching adoption preview.')
    }
    setLoading(false)
  }

  const handleAdopt = async () => {
    setAdopting(true)
    setErrorMsg(null)
    try {
      const res = await fetch(`/api/v1/providers/incus/instances/${encodeURIComponent(instanceName)}/adopt`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      })

      const data = await res.json()
      if (res.ok) {
        setSuccessMsg(data.message || `Instance '${instanceName}' successfully adopted into Mystic management.`)
        setTimeout(() => {
          onSuccess()
          onClose()
        }, 1200)
      } else {
        setErrorMsg(data.error || 'Adoption failed.')
      }
    } catch (e) {
      setErrorMsg('Network error requesting instance adoption.')
    }
    setAdopting(false)
  }

  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: 'rgba(15, 23, 42, 0.85)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1000,
        padding: '20px'
      }}
    >
      <div
        style={{
          backgroundColor: '#1e293b',
          border: '1px solid #334155',
          borderRadius: '12px',
          padding: '24px',
          maxWidth: '560px',
          width: '100%',
          color: '#f8fafc',
          boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.5)'
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <h2 style={{ margin: 0, fontSize: '1.25rem', color: '#38bdf8' }}>
            Adopt External Incus Instance
          </h2>
          <button
            onClick={onClose}
            style={{
              backgroundColor: 'transparent',
              border: 'none',
              color: '#94a3b8',
              fontSize: '1.2rem',
              cursor: 'pointer'
            }}
          >
            ✕
          </button>
        </div>

        {loading ? (
          <div style={{ padding: '24px 0', textAlign: 'center', color: '#94a3b8' }}>
            Fetching real provider instance preview for <strong>{instanceName}</strong>...
          </div>
        ) : errorMsg ? (
          <div
            style={{
              backgroundColor: '#450a0a',
              border: '1px solid #ef4444',
              borderRadius: '6px',
              padding: '16px',
              color: '#fca5a5',
              fontSize: '0.85rem',
              marginBottom: '16px'
            }}
          >
            <strong>Adoption Error:</strong> {errorMsg}
          </div>
        ) : preview ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            {/* Safety Banner */}
            <div
              style={{
                backgroundColor: '#0f2942',
                borderLeft: '4px solid #38bdf8',
                padding: '12px 16px',
                borderRadius: '4px',
                fontSize: '0.85rem',
                lineHeight: '1.4'
              }}
            >
              <strong style={{ color: '#38bdf8', display: 'block', marginBottom: '2px' }}>Non-Destructive Adoption</strong>
              Adopting <strong>{preview.instance_name}</strong> establishes Mystic ownership metadata on the live Incus daemon.
              No recreation, migration, restart, stop, or storage modification will occur.
            </div>

            {/* Live Specs Grid */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: '1fr 1fr',
                gap: '12px',
                backgroundColor: '#0f172a',
                padding: '16px',
                borderRadius: '8px',
                fontSize: '0.85rem'
              }}
            >
              <div>
                <span style={{ color: '#64748b', display: 'block' }}>Instance Name</span>
                <strong style={{ color: '#f8fafc' }}>{preview.instance_name}</strong>
              </div>
              <div>
                <span style={{ color: '#64748b', display: 'block' }}>Provider & Type</span>
                <span style={{ color: '#cbd5e1' }}>{preview.provider} ({preview.type})</span>
              </div>
              <div>
                <span style={{ color: '#64748b', display: 'block' }}>Live State</span>
                <span style={{ color: preview.state === 'running' ? '#4ade80' : '#f87171', fontWeight: 600 }}>
                  {preview.state.toUpperCase()}
                </span>
              </div>
              <div>
                <span style={{ color: '#64748b', display: 'block' }}>Discovered IP</span>
                <span style={{ color: '#f8fafc', fontWeight: 600 }}>{preview.ip_address || 'Unassigned / Dynamic'}</span>
              </div>
              <div>
                <span style={{ color: '#64748b', display: 'block' }}>Limits (CPU / RAM)</span>
                <span style={{ color: '#cbd5e1' }}>{preview.cpu_cores} Core(s) / {preview.memory_bytes / (1024 * 1024)} MB</span>
              </div>
              <div>
                <span style={{ color: '#64748b', display: 'block' }}>Current Ownership</span>
                <span style={{ color: preview.ownership === 'MYSTIC_OWNED' ? '#34d399' : '#fbbf24', fontWeight: 600 }}>
                  {preview.ownership}
                </span>
              </div>
            </div>

            {/* Blockers / Warnings */}
            {preview.blockers && preview.blockers.length > 0 && (
              <div style={{ backgroundColor: '#450a0a', border: '1px solid #ef4444', padding: '12px', borderRadius: '6px', fontSize: '0.85rem', color: '#fca5a5' }}>
                <strong>Adoption Blockers:</strong>
                {preview.blockers.map((b, i) => (
                  <div key={i}>• {b}</div>
                ))}
              </div>
            )}

            {/* Success Toast */}
            {successMsg && (
              <div style={{ backgroundColor: '#064e3b', border: '1px solid #10b981', padding: '12px', borderRadius: '6px', fontSize: '0.85rem', color: '#6ee7b7' }}>
                {successMsg}
              </div>
            )}

            {/* Modal Actions */}
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px', marginTop: '8px' }}>
              <button
                onClick={onClose}
                disabled={adopting}
                style={{
                  backgroundColor: '#334155',
                  color: '#f8fafc',
                  border: 'none',
                  padding: '8px 16px',
                  borderRadius: '6px',
                  cursor: 'pointer'
                }}
              >
                Cancel
              </button>
              <button
                onClick={handleAdopt}
                disabled={adopting || !preview.can_adopt}
                style={{
                  backgroundColor: preview.can_adopt ? '#0284c7' : '#475569',
                  color: '#fff',
                  border: 'none',
                  padding: '8px 18px',
                  borderRadius: '6px',
                  cursor: preview.can_adopt && !adopting ? 'pointer' : 'not-allowed',
                  fontWeight: 600
                }}
              >
                {adopting ? 'Adopting Instance...' : 'Confirm Adoption'}
              </button>
            </div>
          </div>
        ) : null}
      </div>
    </div>
  )
}
