import React, { useState } from 'react'
import { Layout } from './components/Layout'
import { Dashboard } from './pages/Dashboard'
import { Hosts } from './pages/Hosts'
import { VirtualMachines } from './pages/VirtualMachines'
import { Containers } from './pages/Containers'
import {
  Images,
  Storage,
  Networks,
  Snapshots,
  Monitoring,
  Users,
  ApiDocs,
  AuditLogs,
  Settings,
} from './pages/Views'

export const App: React.FC = () => {
  const [activeTab, setActiveTab] = useState('dashboard')

  const renderContent = () => {
    switch (activeTab) {
      case 'dashboard':
        return <Dashboard />
      case 'hosts':
        return <Hosts />
      case 'vms':
        return <VirtualMachines />
      case 'containers':
        return <Containers />
      case 'images':
        return <Images />
      case 'storage':
        return <Storage />
      case 'networks':
        return <Networks />
      case 'snapshots':
        return <Snapshots />
      case 'monitoring':
        return <Monitoring />
      case 'users':
        return <Users />
      case 'api':
        return <ApiDocs />
      case 'audit':
        return <AuditLogs />
      case 'settings':
        return <Settings />
      default:
        return <Dashboard />
    }
  }

  return (
    <Layout activeTab={activeTab} setActiveTab={setActiveTab}>
      {renderContent()}
    </Layout>
  )
}

export default App
