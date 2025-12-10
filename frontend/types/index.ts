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

export interface InstanceMetric {
  location?: string;
  name: string;
  status: string;
  memory_usage_bytes: number;
  cpu_usage_seconds: number;
  config: InstanceConfig;
  devices?: InstanceDevices; // Adicionado
}

export interface WSEvent {
  type: 'job_update' | 'state_change';
  job_id?: string;
  target?: string;
  payload: any;
  timestamp: number;
}
