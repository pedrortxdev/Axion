'use client';

import React, { useState, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { toast, Toaster } from 'sonner';
import { Play, Square, RefreshCw, SquareTerminal, Loader2, Copy, Cpu, MemoryStick, HardDrive, Network, Terminal } from 'lucide-react';

import { Instance } from '@/types';
import WebTerminal from '@/components/WebTerminal';
import { InstanceInfoCard } from '@/components/InstanceInfoCard';
import { formatBytes } from '@/lib/utils';
import { useVmStats } from '@/hooks/useVmStats';

// Components
const ConnectionCard = ({ ip, portMap }: { ip: string, portMap?: Record<string, number> }) => {
    // Find published port for internal port 22
    let sshPort = 22;
    if (portMap) {
        // user provided portMap in types as { "external": internal } or similar?
        // Let's assume the map is "external_port": internal_port
        // We need to find the key where value is 22.
        const entry = Object.entries(portMap).find(([_, internal]) => internal === 22);
        if (entry) sshPort = parseInt(entry[0]);
    }

    const command = `ssh root@${window.location.hostname} -p ${sshPort}`;

    const copyCommand = () => {
        navigator.clipboard.writeText(command);
        toast.success("SSH Command copied!");
    };

    return (
        <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-6">
            <h3 className="font-semibold text-zinc-100 flex items-center gap-2 mb-4">
                <Terminal size={18} /> Connectivity
            </h3>
            <div className="bg-zinc-950 rounded-md p-4 flex items-center justify-between border border-zinc-800 group hover:border-zinc-700 transition-colors cursor-pointer" onClick={copyCommand}>
                <code className="text-sm font-mono text-emerald-400">
                    {command}
                </code>
                <Copy size={14} className="text-zinc-500 group-hover:text-zinc-300" />
            </div>
            <p className="text-xs text-zinc-500 mt-2 ml-1">
                Internal IP: <span className="text-zinc-400">{ip}</span>
            </p>
        </div>
    );
};

const MetricsPanel = ({ metrics, speed }: { metrics: any, speed: { down: string, up: string } }) => {
    if (!metrics) return <div className="h-32 flex items-center justify-center text-zinc-500">Loading metrics...</div>;

    // Limits (Hardcoded for Free Tier MVP display or derived?)
    // AxHV reports limits in GetVmStats response? or we use instance config?
    // Metrics object from AxHV: cpu_time_us, memory_used_bytes, disk_allocated_bytes, net_rx_bytes...
    // We assume simple progress for now.

    // We need Max RAM to show progress. 
    // Assuming backend passes 'limits' in the Instance object, not the Metrics object. We'll pass instance config down or just show raw values.
    // User asked for "Simple progress bars". We need a total to show progress.
    // Let's rely on visuals without fixed % if total is missing, or static max (e.g. 1GB Free Tier).

    return (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* Network Speed */}
            <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4">
                <div className="flex items-center gap-2 text-zinc-400 mb-2">
                    <Network size={16} /> <span className="text-sm font-medium">Network Speed</span>
                </div>
                <div className="flex flex-col gap-1">
                    <div className="flex justify-between items-end">
                        <span className="text-xs text-zinc-500">Global RX</span>
                        <span className="text-xl font-bold text-emerald-400">{speed.down}</span>
                    </div>
                    <div className="flex justify-between items-end">
                        <span className="text-xs text-zinc-500">Global TX</span>
                        <span className="text-xl font-bold text-indigo-400">{speed.up}</span>
                    </div>
                </div>
            </div>

            {/* RAM */}
            <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4">
                <div className="flex items-center gap-2 text-zinc-400 mb-2">
                    <MemoryStick size={16} /> <span className="text-sm font-medium">Memory Usage</span>
                </div>
                <div className="mt-2">
                    <span className="text-2xl font-bold text-zinc-100">{formatBytes(metrics.memoryUsedBytes)}</span>
                    <div className="w-full bg-zinc-800 h-1.5 rounded-full mt-3 overflow-hidden">
                        <div className="h-full bg-purple-500 rounded-full animate-pulse" style={{ width: '50%' }}></div>
                    </div>
                    <p className="text-xs text-zinc-500 mt-2">Allocated by Guest OS</p>
                </div>
            </div>

            {/* Disk */}
            <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4">
                <div className="flex items-center gap-2 text-zinc-400 mb-2">
                    <HardDrive size={16} /> <span className="text-sm font-medium">Disk Usage</span>
                </div>
                <div className="mt-2">
                    <span className="text-2xl font-bold text-zinc-100">{formatBytes(metrics.diskAllocatedBytes)}</span>
                    <p className="text-xs text-zinc-500 mt-1">Physical Space Occupied</p>
                </div>
            </div>
        </div>
    );
}

export default function InstanceDetailPage() {
    const params = useParams();
    const router = useRouter();
    const instanceName = params.name as string;

    const [instance, setInstance] = useState<Instance | null>(null);
    const [token, setToken] = useState<string | null>(null);
    const [isTerminalOpen, setTerminalOpen] = useState(false);

    // New Metrics Hook
    const { metrics, networkSpeed } = useVmStats(instanceName);

    useEffect(() => {
        const storedToken = localStorage.getItem('axion_token');
        if (!storedToken) router.push('/login');
        else setToken(storedToken);
    }, [router]);

    useEffect(() => {
        if (!token || !instanceName) return;

        const fetchData = async () => {
            try {
                const res = await fetch(`${window.location.protocol}//${window.location.hostname}:8500/api/v1/instances/${instanceName}`, {
                    headers: { 'Authorization': `Bearer ${token}` }
                });

                if (res.status === 401) { router.push('/login'); return; }
                if (res.ok) setInstance(await res.json());
            } catch (err) {
                console.error("Failed to fetch instance:", err);
                toast.error("Failed to load instance data.");
            }
        };

        fetchData();
    }, [token, instanceName, router]);

    const handlePowerAction = (action: 'start' | 'stop' | 'restart') => {
        if (!token) return;
        // Map restart to reboot for backend compatibility if needed, or backend handles "restart"
        // UpdateInstanceState expects "reboot" usually.
        const apiAction = action === 'restart' ? 'reboot' : action;

        toast.promise(
            fetch(`${window.location.protocol}//${window.location.hostname}:8500/api/v1/instances/${instanceName}/action`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
                body: JSON.stringify({ action: apiAction }),
            }).then(res => { if (!res.ok) throw new Error(); }),
            {
                loading: `Requesting to ${action} instance...`,
                success: `Instance ${action} request queued.`,
                error: `Failed to ${action} instance.`,
            }
        );
    };

    if (!instance) {
        return <div className="flex h-screen w-full items-center justify-center bg-zinc-950"><Loader2 className="animate-spin text-zinc-500" size={32} /></div>;
    }

    // Determine status (AxHV: "RUNNING", "STOPPED")
    // If backend returns lowercase, we handle it.
    const status = instance.status?.toUpperCase() || 'UNKNOWN';
    const isRunning = status === 'RUNNING';
    const ipAddress = instance.ipAddress || 'N/A';

    // Fake port map if missing (Free Tier MVP usually has it, but if backend doesn't send it yet...)
    // const portMap = instance.portMap || { "2202": 22 }; 

    return (
        <div className="min-h-screen bg-zinc-950 text-zinc-300 font-sans p-4 sm:p-6">
            <Toaster position="bottom-right" theme="dark" richColors />
            {isTerminalOpen && <WebTerminal instanceName={instanceName} onClose={() => setTerminalOpen(false)} />}

            {/* Header */}
            <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4 mb-6">
                <div className="flex flex-col sm:flex-row justify-between sm:items-center">
                    <div className="flex items-center gap-3">
                        <h1 className="text-2xl font-bold text-zinc-100">{instance.name}</h1>
                        <span className={`flex items-center gap-2 px-2 py-0.5 rounded-full text-xs ${isRunning ? 'bg-emerald-900/50 text-emerald-300' : 'bg-red-900/50 text-red-300'}`}>
                            <span className={`h-2 w-2 rounded-full ${isRunning ? 'bg-emerald-500' : 'bg-red-500'}`}></span>
                            {status}
                        </span>
                        {instance.plan === 'free' && <span className="ml-2 px-2 py-0.5 bg-blue-900/30 text-blue-400 text-[10px] rounded border border-blue-900/50 uppercase tracking-wide">Free Tier</span>}
                    </div>
                    <div className="flex items-center gap-2 text-sm text-zinc-400 mt-3 sm:mt-0">
                        {isRunning ? (
                            <>
                                <button onClick={() => handlePowerAction('stop')} className="px-3 py-1.5 text-sm rounded-md flex items-center gap-2 bg-red-900/50 border border-red-700/40 hover:bg-red-900/80 text-red-300 transition-colors"><Square size={14} /> Stop</button>
                                <button onClick={() => handlePowerAction('restart')} className="px-3 py-1.5 text-sm rounded-md flex items-center gap-2 bg-zinc-800 border border-zinc-700 hover:bg-zinc-700/80 text-zinc-200 transition-colors"><RefreshCw size={14} /> Reboot</button>
                            </>
                        ) : (
                            <button onClick={() => handlePowerAction('start')} className="px-3 py-1.5 text-sm rounded-md flex items-center gap-2 bg-emerald-900/50 border border-emerald-700/40 hover:bg-emerald-900/80 text-emerald-300 transition-colors"><Play size={14} /> Start</button>
                        )}
                        <button onClick={() => setTerminalOpen(true)} disabled={!isRunning} className="px-3 py-1.5 text-sm rounded-md flex items-center gap-2 bg-zinc-800 border border-zinc-700 hover:bg-zinc-700/80 text-zinc-200 transition-colors disabled:opacity-50"><SquareTerminal size={14} /> Console</button>
                    </div>
                </div>
            </div>

            <main className="space-y-6">
                {/* Visual Lock / Warning for Free Tier Limits could go here if needed globally */}

                {/* New Metrics Panel */}
                <MetricsPanel metrics={metrics} speed={networkSpeed} />

                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                    {/* Connection Info */}
                    <ConnectionCard ip={ipAddress} portMap={instance.portMap} />

                    {/* Basic Info */}
                    <InstanceInfoCard
                        node={instance.name ? 'AxHV-Node-01' : 'N/A'} // Placeholder
                        ipAddress={ipAddress}
                        vcpu={instance.limits?.['limits.cpu'] ? parseInt(instance.limits['limits.cpu']) : 1}
                        ram={instance.limits?.['limits.memory'] || '512MB'}
                        disk={metrics ? formatBytes(metrics.diskAllocatedBytes) : 'N/A'}
                    />
                </div>
            </main>
        </div>
    );
}
