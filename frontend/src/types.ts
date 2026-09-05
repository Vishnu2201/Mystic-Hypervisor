export type ProviderAvailability = 'AVAILABLE' | 'UNAVAILABLE'
export type InstanceOwnership = 'MYSTIC_OWNED' | 'EXTERNAL' | 'UNKNOWN'

export interface ProviderHealthStatus {
  installed: boolean
  reachable: boolean
  operational: boolean
  capable: boolean
}

export interface PreflightInstance {
  name: string
  type: string
  state: string
  ownership: InstanceOwnership
  ip_address?: string
}

export interface DiscoveredNetwork {
  name: string
  type: string
  managed: boolean
  ipv4?: string
  ipv6?: string
  state?: string
}

export interface DiscoveredStoragePool {
  name: string
  driver: string
  status?: string
  used_bytes?: number
  total_bytes?: number
}

export interface DiscoveredImage {
  fingerprint: string
  alias: string
  description?: string
  os?: string
  release?: string
  architecture: string
  size_bytes: number
}

export interface ProviderServerInfo {
  server_version?: string
  os?: string
  kernel?: string
  architecture?: string
  kvm_supported: boolean
}

export interface ProviderPreflightResult {
  provider: string
  availability: ProviderAvailability
  health_status: ProviderHealthStatus
  server_info: ProviderServerInfo
  capabilities: string[]
  existing_instances: PreflightInstance[]
  networks: DiscoveredNetwork[]
  storage_pools: DiscoveredStoragePool[]
  images: DiscoveredImage[]
  warnings: string[]
  blockers: string[]
}

export interface AdoptionPreviewResult {
  instance_name: string
  provider: string
  type: string
  state: string
  ip_address?: string
  cpu_cores: number
  memory_bytes: number
  storage_gb: number
  image: string
  network: string
  ownership: InstanceOwnership
  already_managed: boolean
  can_adopt: boolean
  blockers: string[]
  warnings: string[]
}
