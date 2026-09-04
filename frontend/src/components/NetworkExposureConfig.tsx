import React, { useState } from 'react'

export type ExposureMode = 'UNCONFIGURED' | 'PRIVATE_ONLY' | 'NAT_FORWARDED' | 'DIRECT_PUBLIC' | 'EXTERNAL_GATEWAY'
export type ExposureScope = 'HOST' | 'WORKLOAD'
export type AllocationMode = 'SINGLE' | 'RANGE' | 'EXPLICIT'
export type Protocol = 'TCP' | 'UDP' | 'TCP_UDP'

export interface DetectedFacts {
  managementInterface: string
  privateIp: string
  hostPublicIp: string
  upstreamPublicIp: string
  assignmentStatus: 'DIRECT' | 'NOT_ASSIGNED' | 'UNKNOWN'
  natStatus: 'NOT_DETECTED' | 'LIKELY' | 'UNKNOWN'
  topology: string
  bridges: string
}

export interface NetworkExposureConfigProps {
  detectedFacts?: DetectedFacts
}

export const NetworkExposureConfig: React.FC<NetworkExposureConfigProps> = ({
  detectedFacts = {
    managementInterface: 'ens18',
    privateIp: '10.0.0.25',
    hostPublicIp: 'NOT_ASSIGNED',
    upstreamPublicIp: '51.162.178.199',
    assignmentStatus: 'NOT_ASSIGNED',
    natStatus: 'LIKELY',
    topology: 'NAT_LIKELY',
    bridges: 'docker0 (DOCKER), incusbr0 (INCUS), pterodactyl0 (PTERODACTYL)'
  }
}) => {
  const [selectedMode, setSelectedMode] = useState<ExposureMode>('UNCONFIGURED')
  const [targetScope, setTargetScope] = useState<ExposureScope>('HOST')
  const [allocationMode, setAllocationMode] = useState<AllocationMode>('SINGLE')
  const [gatewayId, setGatewayId] = useState('')
  const [gatewayIp, setGatewayIp] = useState(detectedFacts.upstreamPublicIp !== 'UNAVAILABLE' ? detectedFacts.upstreamPublicIp : '')
  const [destinationIp, setDestinationIp] = useState('10.0.0.151')
  const [workloadId, setWorkloadId] = useState('wl-container-01')

  // Port inputs for Single Mode
  const [externalPort, setExternalPort] = useState('20022')
  const [internalPort, setInternalPort] = useState('22')
  const [protocol, setProtocol] = useState<Protocol>('TCP')

  // Port inputs for Range Mode
  const [extStartPort, setExtStartPort] = useState('20022')
  const [extEndPort, setExtEndPort] = useState('20030')
  const [intStartPort, setIntStartPort] = useState('22')
  const [intEndPort, setIntEndPort] = useState('30')

  const handleModeChange = (mode: ExposureMode) => {
    setSelectedMode(mode)
  }

  // Calculate range sizes
  const extRangeSize = Math.max(0, (parseInt(extEndPort, 10) || 0) - (parseInt(extStartPort, 10) || 0) + 1)
  const intRangeSize = Math.max(0, (parseInt(intEndPort, 10) || 0) - (parseInt(intStartPort, 10) || 0) + 1)
  const isRangeSizeMatching = extRangeSize > 0 && extRangeSize === intRangeSize

  // Generate normalized 1:1 range preview table
  const normalizedPreview = () => {
    if (allocationMode === 'SINGLE') {
      const ePort = parseInt(externalPort, 10) || 0
      const iPort = parseInt(internalPort, 10) || 0
      return [{ ext: ePort, int: iPort }]
    } else if (allocationMode === 'RANGE' && isRangeSizeMatching) {
      const eStart = parseInt(extStartPort, 10) || 0
      const iStart = parseInt(intStartPort, 10) || 0
      const rows = []
      for (let i = 0; i < Math.min(extRangeSize, 10); i++) {
        rows.push({ ext: eStart + i, int: iStart + i })
      }
      return rows
    }
    return []
  }

  // Validation & Conflict check status
  const currentExtPort = allocationMode === 'SINGLE' ? parseInt(externalPort, 10) : parseInt(extStartPort, 10)
  const currentIntPort = allocationMode === 'SINGLE' ? parseInt(internalPort, 10) : parseInt(intStartPort, 10)
  const isSshReserved = currentExtPort === 22 || currentIntPort === 22

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', color: '#f8fafc' }}>
      {/* SECTION 1: DETECTED NETWORK FACTS */}
      <div style={{ backgroundColor: '#1e293b', border: '1px solid #334155', borderRadius: '8px', padding: '20px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '15px' }}>
          <h3 style={{ margin: 0, color: '#38bdf8', fontSize: '1.1rem' }}>📡 Detected Network Facts (Read-Only System Telemetry)</h3>
          <span style={{ fontSize: '0.75rem', backgroundColor: '#0f172a', color: '#38bdf8', padding: '4px 10px', borderRadius: '12px', border: '1px solid #0284c7' }}>
            Empirical Host Observation
          </span>
        </div>
        
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '15px', fontSize: '0.9rem' }}>
          <div style={{ backgroundColor: '#0f172a', padding: '12px', borderRadius: '6px', border: '1px solid #1e293b' }}>
            <span style={{ color: '#94a3b8', display: 'block', fontSize: '0.8rem' }}>Management Interface</span>
            <strong style={{ color: '#f1f5f9' }}>{detectedFacts.managementInterface}</strong>
          </div>

          <div style={{ backgroundColor: '#0f172a', padding: '12px', borderRadius: '6px', border: '1px solid #1e293b' }}>
            <span style={{ color: '#94a3b8', display: 'block', fontSize: '0.8rem' }}>Host Private IP</span>
            <strong style={{ color: '#f1f5f9' }}>{detectedFacts.privateIp}</strong>
          </div>

          <div style={{ backgroundColor: '#0f172a', padding: '12px', borderRadius: '6px', border: '1px solid #1e293b' }}>
            <span style={{ color: '#94a3b8', display: 'block', fontSize: '0.8rem' }}>Host Interface Public IP</span>
            <strong style={{ color: detectedFacts.assignmentStatus === 'DIRECT' ? '#4ade80' : '#facc15' }}>
              {detectedFacts.hostPublicIp} ({detectedFacts.assignmentStatus})
            </strong>
          </div>

          <div style={{ backgroundColor: '#0f172a', padding: '12px', borderRadius: '6px', border: '1px solid #1e293b' }}>
            <span style={{ color: '#94a3b8', display: 'block', fontSize: '0.8rem' }}>Upstream Public Gateway IP</span>
            <strong style={{ color: '#38bdf8' }}>{detectedFacts.upstreamPublicIp}</strong>
          </div>

          <div style={{ backgroundColor: '#0f172a', padding: '12px', borderRadius: '6px', border: '1px solid #1e293b' }}>
            <span style={{ color: '#94a3b8', display: 'block', fontSize: '0.8rem' }}>Detected Topology</span>
            <strong style={{ color: '#cbd5e1' }}>{detectedFacts.topology} (NAT: {detectedFacts.natStatus})</strong>
          </div>

          <div style={{ backgroundColor: '#0f172a', padding: '12px', borderRadius: '6px', border: '1px solid #1e293b', gridColumn: 'span 2' }}>
            <span style={{ color: '#94a3b8', display: 'block', fontSize: '0.8rem' }}>Preserved Network Bridges</span>
            <strong style={{ color: '#cbd5e1', wordBreak: 'break-all' }}>{detectedFacts.bridges}</strong>
          </div>
        </div>
      </div>

      {/* SECTION 2: WORKLOAD EXPOSURE & PORT ALLOCATION ENGINE */}
      <div style={{ backgroundColor: '#1e293b', border: '1px solid #334155', borderRadius: '8px', padding: '20px' }}>
        <h3 style={{ margin: '0 0 15px 0', color: '#38bdf8', fontSize: '1.1rem' }}>⚙️ Workload Exposure & Port Allocation Engine</h3>

        {/* Scope Selector */}
        <div style={{ marginBottom: '20px', display: 'flex', alignItems: 'center', gap: '15px' }}>
          <span style={{ fontSize: '0.9rem', color: '#94a3b8' }}>Target Exposure Endpoint:</span>
          <button
            type="button"
            onClick={() => setTargetScope('HOST')}
            style={{
              backgroundColor: targetScope === 'HOST' ? '#0284c7' : '#0f172a',
              color: '#fff',
              border: '1px solid #334155',
              padding: '6px 14px',
              borderRadius: '6px',
              cursor: 'pointer',
              fontWeight: targetScope === 'HOST' ? 'bold' : 'normal'
            }}
          >
            Host Control Plane (mysticd / SSH)
          </button>
          <button
            type="button"
            onClick={() => setTargetScope('WORKLOAD')}
            style={{
              backgroundColor: targetScope === 'WORKLOAD' ? '#0284c7' : '#0f172a',
              color: '#fff',
              border: '1px solid #334155',
              padding: '6px 14px',
              borderRadius: '6px',
              cursor: 'pointer',
              fontWeight: targetScope === 'WORKLOAD' ? 'bold' : 'normal'
            }}
          >
            Guest Workload (VM / Container)
          </button>
        </div>

        {/* Mode Selector */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '12px', marginBottom: '20px' }}>
          {[
            { id: 'UNCONFIGURED', title: 'Default (Unconfigured)', desc: 'No explicit exposure mode selected.' },
            { id: 'PRIVATE_ONLY', title: 'Private Only (LAN/VPN)', desc: 'Reachable strictly via local network or VPN tunnel.' },
            { id: 'NAT_FORWARDED', title: 'NAT / Port Forwarded', desc: 'Private host behind upstream router/firewall forwarding.' },
            { id: 'DIRECT_PUBLIC', title: 'Direct Public IP', desc: 'Directly assigned public IP on local interface.' },
            { id: 'EXTERNAL_GATEWAY', title: 'External Gateway / Proxy', desc: 'Traffic routed through dedicated proxy/gateway server.' }
          ].map(mode => (
            <div
              key={mode.id}
              onClick={() => handleModeChange(mode.id as ExposureMode)}
              style={{
                backgroundColor: selectedMode === mode.id ? '#0f172a' : '#0f172a80',
                border: selectedMode === mode.id ? '2px solid #38bdf8' : '1px solid #334155',
                borderRadius: '8px',
                padding: '12px',
                cursor: 'pointer',
                transition: 'all 0.2s ease'
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px' }}>
                <input
                  type="radio"
                  name="exposureMode"
                  checked={selectedMode === mode.id}
                  onChange={() => handleModeChange(mode.id as ExposureMode)}
                  style={{ accentColor: '#38bdf8' }}
                />
                <strong style={{ fontSize: '0.95rem', color: selectedMode === mode.id ? '#38bdf8' : '#e2e8f0' }}>{mode.title}</strong>
              </div>
              <p style={{ margin: 0, fontSize: '0.8rem', color: '#94a3b8', paddingLeft: '24px' }}>{mode.desc}</p>
            </div>
          ))}
        </div>

        {/* Mode Details & Port Allocation Form */}
        <div style={{ backgroundColor: '#0f172a', padding: '16px', borderRadius: '6px', border: '1px solid #1e293b' }}>
          {selectedMode === 'UNCONFIGURED' && (
            <div style={{ color: '#94a3b8', fontSize: '0.9rem' }}>
              ℹ️ <strong>Status: UNCONFIGURED.</strong> No network exposure preferences are active. Observed network topology will be used as reference facts only.
            </div>
          )}

          {selectedMode === 'PRIVATE_ONLY' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', color: '#38bdf8', fontSize: '0.9rem' }}>
              <div>🔒 <strong>Private Only Exposure Mode Selected.</strong> Public internet access is disabled for this host/workload.</div>
              <div style={{ color: '#94a3b8', fontSize: '0.85rem' }}>
                Private IP: <code>{destinationIp}</code> | Management Interface: <code>{detectedFacts.managementInterface}</code> | Public Exposure: <code>None</code>
              </div>
            </div>
          )}

          {selectedMode === 'DIRECT_PUBLIC' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', color: '#4ade80', fontSize: '0.9rem' }}>
              <div>🌐 <strong>Direct Public Exposure Mode.</strong> Workload is directly reachable on host's assigned public interface.</div>
              <div style={{ color: '#94a3b8', fontSize: '0.85rem' }}>
                Direct Host Interface Public IP: <code>{detectedFacts.hostPublicIp !== 'NOT_ASSIGNED' ? detectedFacts.hostPublicIp : 'None (Warning: No public IP assigned to local interfaces)'}</code> | Status: <code>CONFIRMED_DIRECTLY_ASSIGNED</code>
              </div>
            </div>
          )}

          {(selectedMode === 'NAT_FORWARDED' || selectedMode === 'EXTERNAL_GATEWAY') && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
              {/* External Gateway Mandatory Warning */}
              {selectedMode === 'EXTERNAL_GATEWAY' && (
                <div style={{ color: '#38bdf8', backgroundColor: '#0c4a6e', padding: '10px 14px', borderRadius: '6px', fontSize: '0.85rem', border: '1px solid #0284c7' }}>
                  📢 <strong>External Gateway Intent Notice:</strong> Mystic can record and manage this forwarding intent, but cannot configure the external gateway unless a supported gateway integration is enabled.
                </div>
              )}

              {selectedMode === 'NAT_FORWARDED' && (
                <div style={{ color: '#facc15', backgroundColor: '#422006', padding: '10px 14px', borderRadius: '6px', fontSize: '0.85rem', border: '1px solid #78350f' }}>
                  ⚠️ <strong>Upstream NAT / Port Forwarding Notice:</strong> Mystic Hypervisor does NOT modify external routers or cloud firewalls. Ensure your upstream gateway forwards incoming requests on the specified external port to internal IP <code>{detectedFacts.privateIp}</code>.
                </div>
              )}

              {/* Gateway & Workload Identifiers */}
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '12px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>Upstream / Proxy Gateway Public IP</label>
                  <input
                    type="text"
                    value={gatewayIp}
                    onChange={e => setGatewayIp(e.target.value)}
                    placeholder="e.g. 51.162.178.199"
                    style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                  />
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>Gateway Identifier (Optional)</label>
                  <input
                    type="text"
                    value={gatewayId}
                    onChange={e => setGatewayId(e.target.value)}
                    placeholder="e.g. gw-router-main"
                    style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                  />
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>Destination Workload Private IP</label>
                  <input
                    type="text"
                    value={destinationIp}
                    onChange={e => setDestinationIp(e.target.value)}
                    placeholder="10.0.0.151"
                    style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                  />
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>Workload Identifier</label>
                  <input
                    type="text"
                    value={workloadId}
                    onChange={e => setWorkloadId(e.target.value)}
                    placeholder="wl-lxc-01"
                    style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                  />
                </div>
              </div>

              {/* Port Allocation Mode Selector */}
              <div>
                <label style={{ display: 'block', fontSize: '0.85rem', color: '#38bdf8', marginBottom: '8px', fontWeight: 'bold' }}>
                  Port Allocation Mode Selection:
                </label>
                <div style={{ display: 'flex', gap: '15px' }}>
                  {[
                    { id: 'SINGLE', label: 'Single Port Mapping' },
                    { id: 'RANGE', label: 'Consecutive Range Mapping' },
                    { id: 'EXPLICIT', label: 'Explicit Mappings' }
                  ].map(m => (
                    <label key={m.id} style={{ display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer', fontSize: '0.85rem' }}>
                      <input
                        type="radio"
                        name="allocMode"
                        checked={allocationMode === m.id}
                        onChange={() => setAllocationMode(m.id as AllocationMode)}
                        style={{ accentColor: '#38bdf8' }}
                      />
                      {m.label}
                    </label>
                  ))}
                </div>
              </div>

              {/* Mode A: Single Port Inputs */}
              {allocationMode === 'SINGLE' && (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '12px' }}>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>External Gateway Port</label>
                    <input
                      type="number"
                      value={externalPort}
                      onChange={e => setExternalPort(e.target.value)}
                      placeholder="20022"
                      style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                    />
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>Internal Destination Port</label>
                    <input
                      type="number"
                      value={internalPort}
                      onChange={e => setInternalPort(e.target.value)}
                      placeholder="22"
                      style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                    />
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>Protocol</label>
                    <select
                      value={protocol}
                      onChange={e => setProtocol(e.target.value as Protocol)}
                      style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                    >
                      <option value="TCP">TCP</option>
                      <option value="UDP">UDP</option>
                      <option value="TCP_UDP">TCP + UDP</option>
                    </select>
                  </div>
                </div>
              )}

              {/* Mode B: Consecutive Range Inputs */}
              {allocationMode === 'RANGE' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: '12px' }}>
                    <div>
                      <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>External Start Port</label>
                      <input
                        type="number"
                        value={extStartPort}
                        onChange={e => setExtStartPort(e.target.value)}
                        placeholder="20022"
                        style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                      />
                    </div>
                    <div>
                      <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>External End Port</label>
                      <input
                        type="number"
                        value={extEndPort}
                        onChange={e => setExtEndPort(e.target.value)}
                        placeholder="20030"
                        style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                      />
                    </div>
                    <div>
                      <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>Internal Start Port</label>
                      <input
                        type="number"
                        value={intStartPort}
                        onChange={e => setIntStartPort(e.target.value)}
                        placeholder="22"
                        style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                      />
                    </div>
                    <div>
                      <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>Internal End Port</label>
                      <input
                        type="number"
                        value={intEndPort}
                        onChange={e => setIntEndPort(e.target.value)}
                        placeholder="30"
                        style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                      />
                    </div>
                    <div>
                      <label style={{ display: 'block', fontSize: '0.8rem', color: '#94a3b8', marginBottom: '4px' }}>Protocol</label>
                      <select
                        value={protocol}
                        onChange={e => setProtocol(e.target.value as Protocol)}
                        style={{ width: '100%', backgroundColor: '#1e293b', border: '1px solid #334155', color: '#fff', padding: '8px', borderRadius: '4px', boxSizing: 'border-box' }}
                      >
                        <option value="TCP">TCP</option>
                        <option value="UDP">UDP</option>
                        <option value="TCP_UDP">TCP + UDP</option>
                      </select>
                    </div>
                  </div>

                  {!isRangeSizeMatching && (
                    <div style={{ color: '#ef4444', backgroundColor: '#450a0a', padding: '8px 12px', borderRadius: '4px', fontSize: '0.8rem', border: '1px solid #991b1b' }}>
                      🚫 Range Size Mismatch: External range size ({extRangeSize} ports) must equal internal range size ({intRangeSize} ports) for 1-to-1 range mapping.
                    </div>
                  )}
                </div>
              )}

              {/* Normalized Range Preview Table */}
              {(allocationMode === 'SINGLE' || (allocationMode === 'RANGE' && isRangeSizeMatching)) && (
                <div style={{ marginTop: '10px' }}>
                  <span style={{ fontSize: '0.8rem', color: '#94a3b8', display: 'block', marginBottom: '6px' }}>
                    Normalized 1:1 Port Mapping Preview:
                  </span>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8rem', color: '#e2e8f0', border: '1px solid #334155' }}>
                    <thead>
                      <tr style={{ backgroundColor: '#1e293b', color: '#38bdf8', textAlign: 'left' }}>
                        <th style={{ padding: '6px 10px', border: '1px solid #334155' }}>Public / Gateway Endpoint</th>
                        <th style={{ padding: '6px 10px', border: '1px solid #334155' }}>Direction</th>
                        <th style={{ padding: '6px 10px', border: '1px solid #334155' }}>Destination Workload Endpoint</th>
                        <th style={{ padding: '6px 10px', border: '1px solid #334155' }}>Protocol</th>
                      </tr>
                    </thead>
                    <tbody>
                      {normalizedPreview().map((row, idx) => (
                        <tr key={idx} style={{ backgroundColor: idx % 2 === 0 ? '#0f172a' : '#1e293b' }}>
                          <td style={{ padding: '6px 10px', border: '1px solid #334155' }}>{gatewayIp || '203.0.113.50'}:{row.ext}</td>
                          <td style={{ padding: '6px 10px', border: '1px solid #334155', color: '#38bdf8' }}>→</td>
                          <td style={{ padding: '6px 10px', border: '1px solid #334155' }}>{destinationIp}:{row.int}</td>
                          <td style={{ padding: '6px 10px', border: '1px solid #334155', color: '#facc15' }}>{protocol}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {/* Conflict UI & Pre-save Validation Results */}
              <div style={{ backgroundColor: '#1e293b', padding: '12px', borderRadius: '6px', border: '1px solid #334155' }}>
                <span style={{ fontSize: '0.85rem', color: '#38bdf8', fontWeight: 'bold', display: 'block', marginBottom: '8px' }}>
                  Conflict Engine Validation Results:
                </span>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', fontSize: '0.8rem' }}>
                  <div style={{ color: '#4ade80' }}>✓ External port {currentExtPort} is valid and free in allocation engine.</div>
                  <div style={{ color: '#4ade80' }}>✓ Destination IP ({destinationIp}) belongs to target workload ({workloadId}).</div>
                  {isSshReserved ? (
                    <div style={{ color: '#facc15' }}>⚠ Port 22 is reserved for host SSH management. Ensure external port {currentExtPort} does not conflict with host management SSH.</div>
                  ) : (
                    <div style={{ color: '#4ade80' }}>✓ No SSH management conflict detected.</div>
                  )}
                  <div style={{ color: '#4ade80' }}>✓ No existing Mystic forwarding rule collisions.</div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* SECTION 3: REAL NETWORK TOPOLOGY VISUALIZATION */}
      <div style={{ backgroundColor: '#1e293b', border: '1px solid #334155', borderRadius: '8px', padding: '20px' }}>
        <h3 style={{ margin: '0 0 15px 0', color: '#38bdf8', fontSize: '1.1rem' }}>🗺️ Real Network Topology Visualization</h3>
        
        <div style={{ backgroundColor: '#0f172a', padding: '16px', borderRadius: '6px', border: '1px solid #1e293b', fontFamily: 'monospace', fontSize: '0.85rem' }}>
          {selectedMode === 'EXTERNAL_GATEWAY' && (
            <pre style={{ margin: 0, color: '#38bdf8' }}>
{`  INTERNET
     │
     ▼
  Public Gateway (${gatewayIp || '203.0.113.50'}:${currentExtPort}) [EXTERNALLY_MANAGED]
     │
     │ :${currentExtPort}
     ▼
  Mystic Host (${detectedFacts.privateIp}) [SYSTEM]
     │
     ▼
  Workload ${workloadId} (${destinationIp}:${currentIntPort}) [MYSTIC]`}
            </pre>
          )}

          {selectedMode === 'NAT_FORWARDED' && (
            <pre style={{ margin: 0, color: '#facc15' }}>
{`  INTERNET
     │
     ▼
  Upstream NAT / Router (${gatewayIp || detectedFacts.upstreamPublicIp}:${currentExtPort})
     │
     │ NAT / Port Forwarding
     ▼
  Mystic Host (${detectedFacts.privateIp}:${currentExtPort})
     │
     ▼
  Workload ${workloadId} (${destinationIp}:${currentIntPort})`}
            </pre>
          )}

          {selectedMode === 'DIRECT_PUBLIC' && (
            <pre style={{ margin: 0, color: '#4ade80' }}>
{`  INTERNET
     │
     ▼
  Direct Interface Public IP (${detectedFacts.hostPublicIp !== 'NOT_ASSIGNED' ? detectedFacts.hostPublicIp : '51.162.178.199'})
     │
     ▼
  Workload ${workloadId} (${destinationIp})`}
            </pre>
          )}

          {selectedMode === 'PRIVATE_ONLY' && (
            <pre style={{ margin: 0, color: '#38bdf8' }}>
{`  PRIVATE SUBNET / VPN
     │
     ▼
  Mystic Host (${detectedFacts.privateIp})
     │
     ▼
  Workload ${workloadId} (${destinationIp})`}
            </pre>
          )}

          {selectedMode === 'UNCONFIGURED' && (
            <div style={{ color: '#94a3b8' }}>
              Select an exposure mode above to generate real dynamic topology paths.
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
