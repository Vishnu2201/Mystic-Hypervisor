import React from 'react'

export const Dashboard: React.FC = () => {
  return (
    <div style={{ color: '#f8fafc' }}>
      <h1 style={{ marginTop: 0, fontSize: '1.5rem', color: '#38bdf8' }}>System Dashboard</h1>
      <p style={{ color: '#94a3b8' }}>
        Mystic Hypervisor control plane engineering foundation (Milestone 1).
      </p>

      <div
        style={{
          backgroundColor: '#1e293b',
          border: '1px solid #334155',
          borderRadius: '8px',
          padding: '20px',
          marginTop: '20px',
        }}
      >
        <h3 style={{ marginTop: 0, color: '#fbbf24' }}>⚠️ System Notice: No Active Daemon Connection</h3>
        <p style={{ margin: 0, color: '#cbd5e1', lineHeight: '1.6' }}>
          The web application shell is initialized. Live host metrics and real instance telemetry will populate automatically once the <code>mysticd</code> daemon service is started and hypervisor drivers are configured in Milestone 3.
        </p>
        <div style={{ marginTop: '16px', fontSize: '0.85rem', color: '#64748b' }}>
          Strict Policy Enforced: Fake, mock, or simulated production data is prohibited by <code>PROJECT_CONSTITUTION.md</code>.
        </div>
      </div>
    </div>
  )
}
