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

export interface NetworkExposure {
  id: string
  workload_id: string
  workload_name?: string
  gateway_id?: string
  exposure_mode: string
  public_ip?: string
  public_port: number
  internal_ip: string
  internal_port: number
  protocol: string
  desired_state: string
  actual_state: string
  sync_status: string
  description?: string
  created_at: string
  updated_at: string
  last_sync?: string
}

export interface NetworkExposureStatus {
  active: boolean
  state: string
  ip_address?: string
  port?: number
  device_name?: string
  device_type?: string
  listen?: string
  connect?: string
  nat?: string
  instance_name?: string
  raw_device?: Record<string, string>
}

export interface SSHAccessInfo {
  public_port: number
  public_host: string
  private_ip?: string
  username: string
  status: string
  connection_command: string
}

export interface WorkloadSummary {
  id: string
  name: string
  status: string
  type: string
  ip_address?: string
  ssh?: SSHAccessInfo
  created_at?: string
}

export interface ValidationConflict {
  code?: string
  message: string
  severity?: string
}

export interface ExposureValidationResult {
  is_valid: boolean
  status: string
  message: string
  conflicts: ValidationConflict[]
}

export type ServiceType = 'SSH' | 'HTTP' | 'HTTPS' | 'TCP' | 'UDP' | 'CONSOLE'

export interface Service {
  id: string
  workload_id: string
  workload_name?: string
  name: string
  type: ServiceType
  internal_ip: string
  internal_port: number
  protocol: string
  exposure_id?: string
  desired_state: string
  actual_state: string
  sync_status: string
  is_public: boolean
  description?: string
  created_at: string
  updated_at: string
}

export interface ConnectionProfile {
  id: string
  service_id: string
  workload_id: string
  label: string
  protocol: string
  endpoint_host: string
  endpoint_port: number
  target_user?: string
  credential_id?: string
  connection_url?: string
  cli_command?: string
  created_at: string
  updated_at: string
}

export interface Snapshot {
  id: string
  name: string
  workload_id: string
  workload_name: string
  stateful: boolean
  size_bytes?: number
  created_at: string
  status: string
  description?: string
}
