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
      <div className="bg-zinc-900/30 backdrop-blur-sm rounded-xl border border-zinc-800 flex flex-col md:flex-row divide-y md:divide-y-0 md:divide-x divide-zinc-800 overflow-hidden">
        <div className="flex-1 p-5">
          <div className="h-3 bg-zinc-800/50 rounded mb-4 w-1/3"></div>
          <div className="h-8 bg-zinc-800/50 rounded mb-4 w-1/4"></div>
          <div className="h-2 bg-zinc-800/50 rounded w-1/2"></div>
        </div>
        <div className="flex-1 p-5">
          <div className="h-3 bg-zinc-800/50 rounded mb-4 w-1/3"></div>
          <div className="h-8 bg-zinc-800/50 rounded mb-4 w-1/4"></div>
          <div className="h-2 bg-zinc-800/50 rounded w-1/2"></div>
        </div>
        <div className="flex-1 p-5">
          <div className="h-3 bg-zinc-800/50 rounded mb-4 w-1/3"></div>
          <div className="h-8 bg-zinc-800/50 rounded mb-4 w-1/4"></div>
          <div className="h-2 bg-zinc-800/50 rounded w-1/2"></div>
        </div>
        <div className="flex-1 p-5">
          <div className="h-3 bg-zinc-800/50 rounded mb-4 w-1/3"></div>
          <div className="flex gap-4">
            <div className="h-8 bg-zinc-800/50 rounded w-1/4"></div>
            <div className="h-8 bg-zinc-800/50 rounded w-1/4"></div>
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
    <div className="bg-zinc-900/30 backdrop-blur-sm rounded-xl border border-zinc-800 flex flex-col md:flex-row divide-y md:divide-y-0 md:divide-x divide-zinc-800 overflow-hidden">
      {/* CPU Section */}
      <div className="flex-1 p-5 flex flex-col justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 mb-1">
             <Cpu size={14} className="text-zinc-500" />
             <span className="text-[10px] uppercase tracking-wider font-semibold text-zinc-500">CPU Load</span>
          </div>
          <p className="text-2xl font-mono text-zinc-200">{cpuPercent.toFixed(1)}%</p>
        </div>
        <div>
          <div className="w-full bg-zinc-800/50 rounded-full h-1.5 mb-2">
            <div
              className={clsx(
                "h-1.5 rounded-full bg-gradient-to-r",
                getCpuLoadColor(cpuPercent)
              )}
              style={{ width: `${Math.min(cpuPercent, 100)}%` }}
            ></div>
          </div>
          <p className="text-xs text-zinc-500 font-medium">
            {cpuModel || `${cpuCores} Cores`}
          </p>
        </div>
      </div>

      {/* RAM Section */}
      <div className="flex-1 p-5 flex flex-col justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 mb-1">
             <MemoryStick size={14} className="text-zinc-500" />
             <span className="text-[10px] uppercase tracking-wider font-semibold text-zinc-500">Memory</span>
          </div>
          <p className="text-2xl font-mono text-zinc-200">{memoryUsedGBStr} <span className="text-sm text-zinc-500 font-sans ml-1">/ {memoryTotalGBStr} GB</span></p>
        </div>
        <div className="w-full bg-zinc-800/50 rounded-full h-1.5">
          <div
            className="h-1.5 rounded-full bg-gradient-to-r from-indigo-500 to-violet-500"
            style={{ width: `${Math.min(memoryPercent, 100)}%` }}
          ></div>
        </div>
      </div>

      {/* STORAGE Section */}
      <div className="flex-1 p-5 flex flex-col justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 mb-1">
             <HardDrive size={14} className="text-zinc-500" />
             <span className="text-[10px] uppercase tracking-wider font-semibold text-zinc-500">Local Disk</span>
          </div>
          <p className="text-2xl font-mono text-zinc-200">{diskPercent.toFixed(1)}%</p>
        </div>
        <div>
          <div className="w-full bg-zinc-800/50 rounded-full h-1.5 mb-2">
            <div
              className="h-1.5 rounded-full bg-gradient-to-r from-emerald-500 to-emerald-400"
              style={{ width: `${Math.min(diskPercent, 100)}%` }}
            ></div>
          </div>
          <p className="text-xs text-zinc-500 font-medium">
            Free: {diskFreeGB.toFixed(2)} GB
          </p>
        </div>
      </div>

      {/* NETWORK Section */}
      <div className="flex-1 p-5 flex flex-col justify-between gap-4">
        <div>
          <div className="flex items-center justify-between mb-1">
            <div className="flex items-center gap-2">
                <Activity size={14} className="text-zinc-500" />
                <span className="text-[10px] uppercase tracking-wider font-semibold text-zinc-500">I/O Traffic</span>
            </div>
            <Activity 
                className={clsx(
                "h-4 w-4 text-zinc-500",
                highNetworkTraffic && "animate-pulse text-blue-400"
                )} 
            />
          </div>
          <div className="flex flex-col gap-1 mt-2">
            <div className="flex items-center gap-2">
              <span className="text-blue-400 text-xs">▼</span>
              <span className="text-zinc-200 font-mono text-lg">{netRxKB} <span className="text-xs text-zinc-500 font-sans">KB/s</span></span>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-emerald-400 text-xs">▲</span>
              <span className="text-zinc-200 font-mono text-lg">{netTxKB} <span className="text-xs text-zinc-500 font-sans">KB/s</span></span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}