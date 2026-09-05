import React, { useEffect, useState } from 'react'
import { ProviderPreflightResult } from '../types'
import { AdoptionModal } from '../components/AdoptionModal'

export const Hosts: React.FC = () => {
  const [preflight, setPreflight] = useState<ProviderPreflightResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [adoptTargetName, setAdoptTargetName] = useState<string | null>(null)

  useEffect(() => {
    fetchPreflight()
  }, [])

  const fetchPreflight = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/v1/providers/incus/preflight')
      if (res.ok) {
        const data = await res.json()
        setPreflight(data)
      }
    } catch (e) {
      console.log('Preflight endpoint unavailable locally.')
    }
    setLoading(false)
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', color: '#f8fafc' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1 style={{ margin: 0, fontSize: '1.5rem', color: '#38bdf8' }}>Host Hypervisor Nodes & Provider Discovery</h1>
          <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>
            Empirical Real Hypervisor Inspection & Preflight Health Status
          </span>
        </div>
        <button
          onClick={fetchPreflight}
          style={{
            backgroundColor: '#0284c7',
            color: '#fff',
            border: 'none',
            padding: '8px 16px',
            borderRadius: '6px',
            cursor: 'pointer',
            fontWeight: 600
          }}
        >
          Re-run Preflight Discovery
        </button>
      </div>

      {/* Safety Notice Banner for Existing Infrastructure */}
      {preflight && preflight.existing_instances && preflight.existing_instances.length > 0 && (
        <div
          style={{
            backgroundColor: '#451a03',
            border: '1px solid #f59e0b',
            borderRadius: '8px',
            padding: '16px',
            color: '#fef3c7'
          }}
        >
          <div style={{ fontWeight: 'bold', fontSize: '0.95rem', marginBottom: '4px', color: '#fbbf24' }}>
            ⚠️ Infrastructure Coexistence Notice
          </div>
          <div style={{ fontSize: '0.85rem', lineHeight: '1.4' }}>
            Existing provider resources were detected on this host ({preflight.existing_instances.length} instance(s)).
            Mystic Hypervisor will preserve and NOT automatically adopt, modify, or delete resources it does not own.
          </div>
        </div>
      )}

      {/* Provider Status Card */}
      <div style={{ backgroundColor: '#1e293b', padding: '20px', borderRadius: '8px', border: '1px solid #334155' }}>
        <h2 style={{ marginTop: 0, fontSize: '1.2rem', color: '#38bdf8', marginBottom: '16px' }}>
          Incus Hypervisor Driver
        </h2>

        {loading ? (
          <div style={{ color: '#94a3b8' }}>Loading preflight health & discovery data...</div>
        ) : preflight ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            {/* Availability Badges */}
            <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
              <Badge
                label="Installed"
                active={preflight.health_status.installed}
                activeText="YES"
                inactiveText="NO"
              />
              <Badge
                label="Daemon Reachable"
                active={preflight.health_status.reachable}
                activeText="AVAILABLE"
                inactiveText="UNAVAILABLE"
              />
              <Badge
                label="Operational"
                active={preflight.health_status.operational}
                activeText="OPERATIONAL"
                inactiveText="DOWN"
              />
              <Badge
                label="Capable"
                active={preflight.health_status.capable}
                activeText="READY"
                inactiveText="INCAPABLE"
              />
            </div>

            {/* Server Info Grid */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
                gap: '12px',
                backgroundColor: '#0f172a',
                padding: '16px',
                borderRadius: '6px',
                fontSize: '0.85rem'
              }}
            >
              <div>
                <span style={{ color: '#64748b', display: 'block' }}>Version</span>
                <span style={{ fontWeight: 600 }}>{preflight.server_info.server_version || 'Unknown'}</span>
              </div>
              <div>
                <span style={{ color: '#64748b', display: 'block' }}>OS & Kernel</span>
                <span style={{ fontWeight: 600 }}>{preflight.server_info.os || 'Linux'}</span>
              </div>
              <div>
                <span style={{ color: '#64748b', display: 'block' }}>Architecture</span>
                <span style={{ fontWeight: 600 }}>{preflight.server_info.architecture || 'x86_64'}</span>
              </div>
              <div>
                <span style={{ color: '#64748b', display: 'block' }}>KVM Hardware Accel</span>
                <span style={{ fontWeight: 600, color: preflight.server_info.kvm_supported ? '#4ade80' : '#f87171' }}>
                  {preflight.server_info.kvm_supported ? 'Available' : 'Not Present'}
                </span>
              </div>
            </div>

            {/* Counts Summary */}
            <div style={{ display: 'flex', gap: '24px', fontSize: '0.9rem' }}>
              <div>
                <span style={{ color: '#94a3b8' }}>Instances: </span>
                <strong style={{ color: '#38bdf8' }}>{preflight.existing_instances.length}</strong>
              </div>
              <div>
                <span style={{ color: '#94a3b8' }}>Discovered Networks: </span>
                <strong style={{ color: '#38bdf8' }}>{preflight.networks.length}</strong>
              </div>
              <div>
                <span style={{ color: '#94a3b8' }}>Storage Pools: </span>
                <strong style={{ color: '#38bdf8' }}>{preflight.storage_pools.length}</strong>
              </div>
              <div>
                <span style={{ color: '#94a3b8' }}>Discovered Images: </span>
                <strong style={{ color: '#38bdf8' }}>{preflight.images.length}</strong>
              </div>
            </div>

            {/* Discovered Incus Instances Table */}
            {preflight.existing_instances && preflight.existing_instances.length > 0 && (
              <div style={{ marginTop: '8px' }}>
                <h4 style={{ margin: '0 0 10px 0', fontSize: '0.95rem', color: '#e2e8f0' }}>
                  Discovered Incus Instances on Host
                </h4>
                <div style={{ backgroundColor: '#0f172a', borderRadius: '6px', border: '1px solid #334155', overflow: 'hidden' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.85rem' }}>
                    <thead>
                      <tr style={{ borderBottom: '1px solid #334155', color: '#94a3b8' }}>
                        <th style={{ padding: '10px 14px' }}>Instance Name</th>
                        <th style={{ padding: '10px 14px' }}>Type</th>
                        <th style={{ padding: '10px 14px' }}>Live State</th>
                        <th style={{ padding: '10px 14px' }}>Discovered IP</th>
                        <th style={{ padding: '10px 14px' }}>Ownership</th>
                        <th style={{ padding: '10px 14px' }}>Action</th>
                      </tr>
                    </thead>
                    <tbody>
                      {preflight.existing_instances.map(inst => (
                        <tr key={inst.name} style={{ borderBottom: '1px solid #1e293b' }}>
                          <td style={{ padding: '10px 14px', fontWeight: 600, color: '#f8fafc' }}>{inst.name}</td>
                          <td style={{ padding: '10px 14px', color: '#cbd5e1' }}>{inst.type}</td>
                          <td style={{ padding: '10px 14px' }}>
                            <span style={{ color: inst.state === 'Running' || inst.state === 'running' ? '#4ade80' : '#f87171', fontWeight: 600 }}>
                              {inst.state}
                            </span>
                          </td>
                          <td style={{ padding: '10px 14px', color: '#cbd5e1' }}>{inst.ip_address || 'Unassigned'}</td>
                          <td style={{ padding: '10px 14px' }}>
                            <span
                              style={{
                                padding: '2px 8px',
                                borderRadius: '4px',
                                fontSize: '0.75rem',
                                fontWeight: 600,
                                backgroundColor: inst.ownership === 'MYSTIC_OWNED' ? '#064e3b' : inst.ownership === 'EXTERNAL' ? '#451a03' : '#334155',
                                color: inst.ownership === 'MYSTIC_OWNED' ? '#34d399' : inst.ownership === 'EXTERNAL' ? '#fbbf24' : '#94a3b8',
                                border: `1px solid ${inst.ownership === 'MYSTIC_OWNED' ? '#10b981' : inst.ownership === 'EXTERNAL' ? '#f59e0b' : '#64748b'}`
                              }}
                            >
                              {inst.ownership === 'MYSTIC_OWNED' ? 'MANAGED' : inst.ownership}
                            </span>
                          </td>
                          <td style={{ padding: '10px 14px' }}>
                            {inst.ownership === 'EXTERNAL' ? (
                              <button
                                onClick={() => setAdoptTargetName(inst.name)}
                                style={{
                                  backgroundColor: '#0284c7',
                                  color: '#fff',
                                  border: 'none',
                                  padding: '4px 12px',
                                  borderRadius: '4px',
                                  cursor: 'pointer',
                                  fontWeight: 600,
                                  fontSize: '0.8rem'
                                }}
                              >
                                Adopt
                              </button>
                            ) : (
                              <span style={{ color: '#64748b', fontSize: '0.8rem' }}>Managed</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Blockers / Warnings */}
            {preflight.blockers && preflight.blockers.length > 0 && (
              <div style={{ backgroundColor: '#450a0a', padding: '12px', borderRadius: '6px', border: '1px solid #ef4444' }}>
                <strong style={{ color: '#f87171', display: 'block', marginBottom: '4px' }}>Preflight Blockers:</strong>
                {preflight.blockers.map((b, i) => (
                  <div key={i} style={{ color: '#fca5a5', fontSize: '0.85rem' }}>• {b}</div>
                ))}
              </div>
            )}
          </div>
        ) : (
          <div style={{ color: '#94a3b8' }}>Incus preflight discovery data unavailable.</div>
        )}
      </div>

      {/* Adoption Modal */}
      {adoptTargetName && (
        <AdoptionModal
          instanceName={adoptTargetName}
          onClose={() => setAdoptTargetName(null)}
          onSuccess={() => {
            fetchPreflight()
          }}
        />
      )}
    </div>
  )
}

const Badge: React.FC<{ label: string; active: boolean; activeText: string; inactiveText: string }> = ({
  label,
  active,
  activeText,
  inactiveText
}) => (
  <div
    style={{
      display: 'inline-flex',
      alignItems: 'center',
      gap: '6px',
      padding: '4px 10px',
      borderRadius: '4px',
      backgroundColor: active ? '#064e3b' : '#450a0a',
      border: `1px solid ${active ? '#10b981' : '#ef4444'}`,
      fontSize: '0.8rem'
    }}
  >
    <span style={{ color: '#94a3b8' }}>{label}:</span>
    <span style={{ fontWeight: 600, color: active ? '#34d399' : '#f87171' }}>
      {active ? activeText : inactiveText}
    </span>
  </div>
)
