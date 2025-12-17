export type JobStatus = 'PENDING' | 'IN_PROGRESS' | 'COMPLETED' | 'FAILED' | 'CANCELED';

export type JobType = 'state_change' | 'update_limits' | 'create_instance' | 'delete_instance' | 'create_snapshot' | 'restore_snapshot' | 'delete_snapshot' | 'add_port' | 'remove_port';

export interface Job {
  id: string;
  type: JobType;
  target: string;
  payload: string;
  status: JobStatus;
  error?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  attempt_count: number;
  requested_by?: string;
}

export interface Schedule {
  id: string;
  cron_expr: string;
  type: JobType;
  target: string;
  payload: string;
  created_at: string;
}

export interface InstanceConfig {
  "limits.memory"?: string;
  "limits.cpu"?: string;
  [key: string]: string | undefined;
}

// Mapa de devices do LXD
// Ex: "proxy-8080": { "type": "proxy", "listen": "tcp:0.0.0.0:8080", "connect": "tcp:127.0.0.1:80" }
export interface InstanceDevices {
  [deviceName: string]: {
    type: string;
    listen?: string;
    connect?: string;
    bind?: string;
    [key: string]: string | undefined;
  };
}

export interface InstanceBackupInfo {
  enabled: boolean;
  schedule: string;
  retention?: number; // Added this line
  next_run?: string; // ISO date string
  last_run?: string; // ISO date string
  last_status?: string; // "completed", "failed", etc.
}

export interface InstanceState {
    status: string;
    status_code: number;
    disk: {
        [deviceName: string]: {
            usage: number;
        };
    };
    memory: {
        usage: number;
        usage_peak: number;
        swap_usage: number;
        swap_usage_peak: number;
        total: number;
    };
    root_device?: {
        usage: number;
        total: number;
    };
    network: {
        [interfaceName: string]: {
            addresses: {
                family: string;
                address: string;
                netmask: string;
                scope: string;
            }[];
            counters: {
                bytes_received: number;
                bytes_sent: number;
                packets_received: number;
                packets_sent: number;
            };
            hwaddr: string;
            mtu: number;
            state: string;
            type: string;
        };
    };
    pid: number;
    processes: number;
    cpu: {
        usage: number;
    };
}

export interface InstanceMetric {
  location?: string;
  name: string;
  type?: string;
  status: string;
  memory_usage_bytes: number;
  cpu_usage_seconds: number;
  disk_usage_bytes: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
  config: InstanceConfig;
  devices?: InstanceDevices;
  state?: InstanceState; 
  backup_info?: InstanceBackupInfo; // Added for backup observability
}

export interface WSEvent {
  type: 'job_update' | 'state_change'; // Reverted this line
  job_id?: string;
  target?: string;
  payload: unknown; // Changed from any to unknown
  timestamp: number;
}

export interface HostStats {
  // Supporting both camelCase and snake_case field names
  cpuPercent?: number;
  cpu_percent?: number;
  memoryUsedMB?: number;
  memory_used_mb?: number;
  memoryTotalMB?: number;
  memory_total_mb?: number;
  diskUsedGB?: number;
  disk_used_gb?: number;
  diskTotalGB?: number;
  disk_total_gb?: number;
  netRxKB?: number;
  net_rx_kb?: number;
  netTxKB?: number;
  net_tx_kb?: number;
  cpuCores?: number;
  cpu_cores?: number;
  cpuModel?: string;
  cpu_model?: string;
}

export interface MetricHistory {
  timestamp: string;
  cpu_usage: number;
  memory_usage: number;
}

