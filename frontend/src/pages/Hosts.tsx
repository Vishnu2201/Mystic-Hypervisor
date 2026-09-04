import React from 'react'

export const Hosts: React.FC = () => {
  return (
    <div>
      <h1 style={{ marginTop: 0, fontSize: '1.5rem', color: '#38bdf8' }}>Host Hypervisor Nodes</h1>
      <div style={{ backgroundColor: '#1e293b', padding: '20px', borderRadius: '8px', border: '1px solid #334155' }}>
        <p style={{ margin: 0, color: '#94a3b8' }}>
          No host nodes connected. Live host inspection endpoints are being established in the Go backend (`/api/v1/hosts`).
        </p>
      </div>
    </div>
  )
}
