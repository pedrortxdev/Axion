import { Cpu, HardDrive, MemoryStick, Activity } from 'lucide-react';
import clsx from 'clsx';

interface HostStats {
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

interface HostStatsCardProps {
  data: HostStats | null;
}

// Helper function to get CPU load color based on percentage
const getCpuLoadColor = (percent: number) => {
  if (percent < 50) return 'from-green-500 to-green-400';
  if (percent < 80) return 'from-yellow-500 to-yellow-400';
  return 'from-red-500 to-red-400';
};

export default function HostStatsCard({ data }: HostStatsCardProps) {
  if (!data) {
    return (
      <div className="h-24 bg-zinc-900/50 backdrop-blur rounded-lg border border-zinc-800 flex overflow-hidden">
        <div className="flex-1 border-r border-white/5 p-3">
          <div className="h-3 bg-zinc-800/50 rounded mb-2 w-1/3"></div>
          <div className="h-6 bg-zinc-800/50 rounded mb-2 w-1/4"></div>
          <div className="h-2 bg-zinc-800/50 rounded w-1/2"></div>
        </div>
        <div className="flex-1 border-r border-white/5 p-3">
          <div className="h-3 bg-zinc-800/50 rounded mb-2 w-1/3"></div>
          <div className="h-6 bg-zinc-800/50 rounded mb-2 w-1/4"></div>
          <div className="h-2 bg-zinc-800/50 rounded w-1/2"></div>
        </div>
        <div className="flex-1 border-r border-white/5 p-3">
          <div className="h-3 bg-zinc-800/50 rounded mb-2 w-1/3"></div>
          <div className="h-6 bg-zinc-800/50 rounded mb-2 w-1/4"></div>
          <div className="h-2 bg-zinc-800/50 rounded w-1/2"></div>
        </div>
        <div className="flex-1 p-3">
          <div className="h-3 bg-zinc-800/50 rounded mb-2 w-1/3"></div>
          <div className="flex gap-4">
            <div className="h-6 bg-zinc-800/50 rounded w-1/4"></div>
            <div className="h-6 bg-zinc-800/50 rounded w-1/4"></div>
          </div>
        </div>
      </div>
    );
  }

  // Safe access to data properties with support for both camelCase and snake_case
  const cpuPercent =
    typeof data.cpuPercent === 'number' ? data.cpuPercent :
    typeof data.cpu_percent === 'number' ? data.cpu_percent : 0;

  const memoryUsedMB =
    typeof data.memoryUsedMB === 'number' ? data.memoryUsedMB :
    typeof data.memory_used_mb === 'number' ? data.memory_used_mb : 0;

  const memoryTotalMB =
    typeof data.memoryTotalMB === 'number' ? data.memoryTotalMB :
    typeof data.memory_total_mb === 'number' ? data.memory_total_mb : 1;

  const diskUsedGB =
    typeof data.diskUsedGB === 'number' ? data.diskUsedGB :
    typeof data.disk_used_gb === 'number' ? data.disk_used_gb : 0;

  const diskTotalGB =
    typeof data.diskTotalGB === 'number' ? data.diskTotalGB :
    typeof data.disk_total_gb === 'number' ? data.disk_total_gb : 1;

  const netRxKB =
    typeof data.netRxKB === 'number' ? data.netRxKB :
    typeof data.net_rx_kb === 'number' ? data.net_rx_kb : 0;

  const netTxKB =
    typeof data.netTxKB === 'number' ? data.netTxKB :
    typeof data.net_tx_kb === 'number' ? data.net_tx_kb : 0;

  const cpuCores =
    typeof data.cpuCores === 'number' ? data.cpuCores :
    typeof data.cpu_cores === 'number' ? data.cpu_cores : 0;

  const cpuModel =
    typeof data.cpuModel === 'string' ? data.cpuModel :
    typeof data.cpu_model === 'string' ? data.cpu_model : '';

  // Calculate disk free space
  const diskFreeGB = diskTotalGB - diskUsedGB;

  // Calculate memory usage percentage
  const memoryPercent = memoryTotalMB > 0
    ? (memoryUsedMB / memoryTotalMB) * 100
    : 0;

  // Calculate disk usage percentage (used disk as percentage)
  const diskPercent = diskTotalGB > 0
    ? (diskUsedGB / diskTotalGB) * 100
    : 0;

  // Calculate memory usage in GB
  const memoryUsedGB = memoryUsedMB / 1024;
  const memoryTotalGB = memoryTotalMB / 1024;
  const memoryUsedGBStr = memoryUsedGB.toFixed(2);
  const memoryTotalGBStr = memoryTotalGB.toFixed(2);

  // Determine if there is high network traffic (for pulsing icon)
  const highNetworkTraffic = netRxKB > 5000 || netTxKB > 5000; // Threshold is arbitrary, can be adjusted

  return (
    <div className="h-24 bg-zinc-900/50 backdrop-blur rounded-lg border border-zinc-800 flex overflow-hidden">
      {/* CPU Section */}
      <div className="flex-1 border-r border-white/5 p-3 flex flex-col justify-between">
        <div>
          <p className="text-xs tracking-widest text-zinc-500">CPU LOAD</p>
          <p className="text-2xl font-mono text-zinc-100">{cpuPercent.toFixed(1)}%</p>
        </div>
        <div className="w-full bg-zinc-800/50 rounded-full h-1">
          <div
            className={clsx(
              "h-1 rounded-full bg-gradient-to-r",
              getCpuLoadColor(cpuPercent)
            )}
            style={{ width: `${Math.min(cpuPercent, 100)}%` }}
          ></div>
        </div>
        <p className="text-xs text-zinc-500">
          {cpuModel || `${cpuCores} Cores`}
        </p>
      </div>

      {/* RAM Section */}
      <div className="flex-1 border-r border-white/5 p-3 flex flex-col justify-between">
        <div>
          <p className="text-xs tracking-widest text-zinc-500">MEMORY</p>
          <p className="text-2xl font-mono text-zinc-100">{memoryUsedGBStr} GB / {memoryTotalGBStr} GB</p>
        </div>
        <div className="w-full bg-zinc-800/50 rounded-full h-1">
          <div
            className="h-1 rounded-full bg-gradient-to-r from-indigo-500 to-violet-500"
            style={{ width: `${Math.min(memoryPercent, 100)}%` }}
          ></div>
        </div>
      </div>

      {/* STORAGE Section */}
      <div className="flex-1 border-r border-white/5 p-3 flex flex-col justify-between">
        <div>
          <p className="text-xs tracking-widest text-zinc-500">LOCAL DISK</p>
          <p className="text-2xl font-mono text-zinc-100">{diskPercent.toFixed(1)}%</p>
        </div>
        <div className="w-full bg-zinc-800/50 rounded-full h-1">
          <div
            className="h-1 rounded-full bg-gradient-to-r from-emerald-500 to-emerald-400"
            style={{ width: `${Math.min(diskPercent, 100)}%` }}
          ></div>
        </div>
        <p className="text-xs text-zinc-500">
          Free: {diskFreeGB.toFixed(2)} GB
        </p>
      </div>

      {/* NETWORK Section */}
      <div className="flex-1 p-3 flex flex-col justify-between">
        <div>
          <p className="text-xs tracking-widest text-zinc-500">I/O TRAFFIC</p>
          <div className="flex items-end gap-4">
            <div className="flex items-center gap-1">
              <span className="text-blue-400 text-2xl font-mono">▼</span>
              <span className="text-blue-400 text-lg font-mono">{netRxKB} KB/s</span>
            </div>
            <div className="flex items-center gap-1">
              <span className="text-emerald-400 text-2xl font-mono">▲</span>
              <span className="text-emerald-400 text-lg font-mono">{netTxKB} KB/s</span>
            </div>
          </div>
        </div>
        <div className="flex justify-end">
          <Activity 
            className={clsx(
              "h-4 w-4 text-zinc-500",
              highNetworkTraffic && "animate-pulse text-blue-400"
            )} 
          />
        </div>
      </div>
    </div>
  );
}