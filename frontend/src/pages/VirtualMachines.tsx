import React from 'react'

export const VirtualMachines: React.FC = () => {
  return (
    <div>
      <h1 style={{ marginTop: 0, fontSize: '1.5rem', color: '#38bdf8' }}>Virtual Machines</h1>
      <div style={{ backgroundColor: '#1e293b', padding: '20px', borderRadius: '8px', border: '1px solid #334155' }}>
        <p style={{ margin: 0, color: '#94a3b8' }}>
          No virtual machines configured. In accordance with the Provider Authoritative State policy, VM states will be retrieved directly from Incus/KVM hypervisors once activated.
        </p>
      </div>
    </div>
  )
}
