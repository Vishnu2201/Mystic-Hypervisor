import React from 'react'
import { WorkloadProvisioning } from '../components/WorkloadProvisioning'

export const VirtualMachines: React.FC = () => {
  return (
    <div>
      <h1 style={{ marginTop: 0, fontSize: '1.5rem', color: '#38bdf8' }}>Virtual Machines</h1>
      <WorkloadProvisioning />
    </div>
  )
}

