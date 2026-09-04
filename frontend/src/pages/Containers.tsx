import React from 'react'

export const Containers: React.FC = () => (
  <div>
    <h1 style={{ marginTop: 0, fontSize: '1.5rem', color: '#38bdf8' }}>System Containers</h1>
    <div style={{ backgroundColor: '#1e293b', padding: '20px', borderRadius: '8px', border: '1px solid #334155', color: '#94a3b8' }}>
      No system containers found. Container management will be populated via Incus / LXC providers.
    </div>
  </div>
)
