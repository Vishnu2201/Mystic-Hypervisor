import React from 'react'

export interface NavigationItem {
  id: string
  label: string
  icon: string
}

const NAV_ITEMS: NavigationItem[] = [
  { id: 'dashboard', label: 'Dashboard', icon: '📊' },
  { id: 'hosts', label: 'Hosts', icon: '🖥️' },
  { id: 'vms', label: 'Virtual Machines', icon: '💻' },
  { id: 'containers', label: 'Containers', icon: '📦' },
  { id: 'images', label: 'Images', icon: '💿' },
  { id: 'storage', label: 'Storage', icon: '💾' },
  { id: 'networks', label: 'Networks', icon: '🌐' },
  { id: 'snapshots', label: 'Snapshots', icon: '📸' },
  { id: 'monitoring', label: 'Monitoring', icon: '📈' },
  { id: 'users', label: 'Users & RBAC', icon: '👥' },
  { id: 'api', label: 'API & Tokens', icon: '🔑' },
  { id: 'audit', label: 'Audit Logs', icon: '📜' },
  { id: 'settings', label: 'Settings', icon: '⚙️' },
]

interface LayoutProps {
  activeTab: string
  setActiveTab: (tab: string) => void
  children: React.ReactNode
}

export const Layout: React.FC<LayoutProps> = ({ activeTab, setActiveTab, children }) => {
  return (
    <div style={{ display: 'flex', minHeight: '100vh', backgroundColor: '#0f172a' }}>
      {/* Sidebar Navigation */}
      <aside
        style={{
          width: '260px',
          backgroundColor: '#1e293b',
          borderRight: '1px solid #334155',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        <div style={{ padding: '20px', borderBottom: '1px solid #334155' }}>
          <h2 style={{ margin: 0, color: '#38bdf8', fontSize: '1.25rem' }}>🔮 Mystic Hypervisor</h2>
          <span style={{ fontSize: '0.75rem', color: '#94a3b8' }}>Milestone 1 — Foundation</span>
        </div>

        <nav style={{ flex: 1, padding: '12px' }}>
          {NAV_ITEMS.map((item) => {
            const isActive = activeTab === item.id
            return (
              <button
                key={item.id}
                onClick={() => setActiveTab(item.id)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '12px',
                  width: '100%',
                  padding: '10px 14px',
                  marginBottom: '4px',
                  backgroundColor: isActive ? '#0284c7' : 'transparent',
                  color: isActive ? '#ffffff' : '#cbd5e1',
                  border: 'none',
                  borderRadius: '6px',
                  cursor: 'pointer',
                  fontSize: '0.9rem',
                  fontWeight: isActive ? 600 : 400,
                  textAlign: 'left',
                  transition: 'background-color 0.15s ease',
                }}
              >
                <span>{item.icon}</span>
                <span>{item.label}</span>
              </button>
            )
          })}
        </nav>

        <div style={{ padding: '16px', borderTop: '1px solid #334155', fontSize: '0.8rem', color: '#64748b' }}>
          Real Infrastructure Only • No Fake Data
        </div>
      </aside>

      {/* Main Content View Area */}
      <main style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
        <header
          style={{
            height: '60px',
            borderBottom: '1px solid #334155',
            backgroundColor: '#1e293b',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '0 24px',
          }}
        >
          <div style={{ fontSize: '1rem', fontWeight: 600, color: '#f8fafc' }}>
            {NAV_ITEMS.find((i) => i.id === activeTab)?.label}
          </div>
          <div
            style={{
              fontSize: '0.8rem',
              padding: '4px 10px',
              borderRadius: '12px',
              backgroundColor: '#0369a1',
              color: '#e0f2fe',
            }}
          >
            System Mode: Foundation (No Daemon Active)
          </div>
        </header>

        <div style={{ flex: 1, padding: '24px', overflowY: 'auto' }}>{children}</div>
      </main>
    </div>
  )
}
