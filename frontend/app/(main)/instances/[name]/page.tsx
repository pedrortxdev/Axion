'use client';

import React, { useState, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { toast, Toaster } from 'sonner';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { Play, Square, RefreshCw, SquareTerminal, Loader2, Copy, Cpu, MemoryStick, CheckCircle2, XCircle, Clock, Shield, Calendar, Server } from 'lucide-react';
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, Area, AreaChart } from 'recharts';

import { InstanceMetric, Job, MetricHistory } from '@/types';
import WebTerminal from '@/components/WebTerminal';
import { InstanceInfoCard } from '@/components/InstanceInfoCard';
import { formatBytes } from '@/lib/utils'; // Import formatBytes

// Helper to get relative time
const timeAgo = (date: string) => {
    try {
        return formatDistanceToNow(parseISO(date), { addSuffix: true });
    } catch (error) {
        return 'Invalid date';
    }
};

// --- Re-imagined Components based on feedback ---

const AuditLog = ({ jobs, instanceName }: { jobs: Job[], instanceName: string }) => {
    const instanceJobs = jobs
        .filter(j => j.target === instanceName)
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        .slice(0, 5); // Limit for a clean look

    const getIcon = (status: string) => {
        switch (status) {
            case 'COMPLETED': return <CheckCircle2 className="text-emerald-500" size={16} />;
            case 'FAILED': return <XCircle className="text-red-500" size={16} />;
            case 'IN_PROGRESS': return <Loader2 className="animate-spin text-indigo-400" size={16} />;
            default: return <Clock className="text-zinc-500" size={16} />;
        }
    };

    if (instanceJobs.length === 0) {
        return <p className="text-sm text-center text-zinc-500 mt-4">No audit events found.</p>;
    }

    return (
        <div className="space-y-4">
            {instanceJobs.map(job => (
                <div key={job.id} className="flex items-center gap-3">
                    <div className="flex-shrink-0">{getIcon(job.status)}</div>
                    <div className="flex-grow">
                        <p className="text-sm font-medium text-zinc-300 capitalize">{job.type.replace(/_/g, ' ').toLowerCase()}</p>
                        <p className="text-xs text-zinc-500">{timeAgo(job.created_at)}</p>
                    </div>
                </div>
            ))}
        </div>
    );
};

const DataProtectionCard = ({ instance }: { instance: InstanceMetric }) => {
    return (
        <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4 h-full">
            <h3 className="font-semibold text-zinc-100 flex items-center gap-2"><Shield size={16}/> Data Protection</h3>
            {instance.backup_info ? (
                <div className="mt-4 space-y-3 text-sm">
                    <div className="flex justify-between items-center text-emerald-400">
                        <span className="font-bold">Active</span>
                        <CheckCircle2 size={16}/>
                    </div>
                    <div className="flex justify-between items-center">
                        <span className="text-zinc-400">Frequency</span>
                        <span className="font-mono">{instance.backup_info.schedule ?? 'Not scheduled'}</span>
                    </div>
                     <div className="flex justify-between items-center">
                        <span className="text-zinc-400">Retention</span>
                        <span className="font-mono">{instance.backup_info.retention ?? '7 days (Default)'}</span>
                    </div>
                     <div className="flex justify-between items-center">
                        <span className="text-zinc-400">Last Backup</span>
                        <span className="font-mono">{instance.backup_info.last_run ? timeAgo(instance.backup_info.last_run) : 'Never run'}</span>
                    </div>
                </div>
            ) : (
                <div className="text-center text-sm text-zinc-500 mt-6">
                    <p>Not Configured</p>
                    <button className="mt-2 text-indigo-400 hover:text-indigo-300 text-xs">Configure Policy</button>
                </div>
            )}
        </div>
    );
};


const MetricCard = ({ title, value, subValue, icon, children, timeRange, onTimeRangeChange, hasData }: any) => (
    <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4 h-64 flex flex-col">
        <div className="flex justify-between items-center">
            <h3 className="font-semibold text-zinc-100 flex items-center gap-2">{icon} {title}</h3>
            <div className="flex gap-1">
                {['1H', '24H', '7D'].map(range => (
                    <button key={range} onClick={() => onTimeRangeChange(range)} className={`px-2 py-0.5 text-xs rounded-md transition-colors ${timeRange === range ? 'bg-zinc-700 text-zinc-100' : 'bg-transparent text-zinc-400 hover:bg-zinc-800'}`}>{range}</button>
                ))}
            </div>
        </div>
        <div className="mt-3">
            <span className="text-2xl font-bold text-zinc-100">{value}</span>
            {subValue && <span className="text-sm text-zinc-400 ml-2">{subValue}</span>}
        </div>
        <div className="flex-grow mt-4">
            {hasData ? (
                children
            ) : (
                <div className="flex items-center justify-center h-full text-zinc-500 text-sm">
                    Waiting for metrics...
                </div>
            )}
        </div>
    </div>
);


const MetricChartCard = ({ title, value, subValue, icon, children, timeRange, onTimeRangeChange, hasData }: any) => (
    <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4 h-64 flex flex-col">
        <div className="flex justify-between items-center">
            <h3 className="font-semibold text-zinc-100 flex items-center gap-2">{icon} {title}</h3>
            <div className="flex gap-1">
                {['1H', '24H', '7D'].map(range => (
                    <button key={range} onClick={() => onTimeRangeChange(range)} className={`px-2 py-0.5 text-xs rounded-md transition-colors ${timeRange === range ? 'bg-zinc-700 text-zinc-100' : 'bg-transparent text-zinc-400 hover:bg-zinc-800'}`}>{range}</button>
                ))}
            </div>
        </div>
        <div className="mt-3">
            <span className="text-2xl font-bold text-zinc-100">{value}</span>
            {subValue && <span className="text-sm text-zinc-400 ml-2">{subValue}</span>}
        </div>
        <div className="flex-grow mt-4">
            {hasData ? children : <div className="flex items-center justify-center h-full text-zinc-500 text-sm">Waiting for metrics...</div>}
        </div>
    </div>
);

const PerformanceMetrics = ({ history, timeRange, onTimeRangeChange, cpuUsage, memPercent, memUsage, memTotal }: any) => {
    const chartData = history.map((h: MetricHistory) => ({
        time: new Date(h.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        cpu: h.cpu_usage,
        memory: h.memory_usage ? h.memory_usage / 1024 : 0, // In MB
    }));

    return (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <MetricChartCard title="Memory Usage" value={`${memPercent}%`} subValue={`${memUsage} / ${memTotal}`} icon={<MemoryStick size={16}/>} timeRange={timeRange} onTimeRangeChange={onTimeRangeChange} hasData={chartData.length > 0}>
                <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={chartData} margin={{ top: 5, right: 20, left: -10, bottom: 0 }}>
                        <Tooltip contentStyle={{ backgroundColor: '#18181b', border: '1px solid #3f3f46', fontSize: '12px' }} labelStyle={{ color: '#a1a1aa' }}/>
                        <defs><linearGradient id="colorMemory" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#8884d8" stopOpacity={0.4}/><stop offset="95%" stopColor="#8884d8" stopOpacity={0}/></linearGradient></defs>
                        <Area type="monotone" dataKey="memory" stroke="#8884d8" fillOpacity={1} fill="url(#colorMemory)" strokeWidth={2}/>
                        <YAxis stroke="#52525b" fontSize={12} unit="MB"/>
                        <XAxis dataKey="time" stroke="#52525b" fontSize={12} />
                    </AreaChart>
                </ResponsiveContainer>
            </MetricChartCard>

            <MetricChartCard title="CPU Usage" value={`~${cpuUsage}%`} icon={<Cpu size={16}/>} timeRange={timeRange} onTimeRangeChange={onTimeRangeChange} hasData={chartData.length > 0}>
                <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={chartData} margin={{ top: 5, right: 20, left: -10, bottom: 0 }}>
                        <Tooltip contentStyle={{ backgroundColor: '#18181b', border: '1px solid #3f3f46', fontSize: '12px' }} labelStyle={{ color: '#a1a1aa' }}/>
                        <defs><linearGradient id="colorCpu" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#82ca9d" stopOpacity={0.4}/><stop offset="95%" stopColor="#82ca9d" stopOpacity={0}/></linearGradient></defs>
                        <Area type="monotone" dataKey="cpu" stroke="#82ca9d" fillOpacity={1} fill="url(#colorCpu)" strokeWidth={2}/>
                        <YAxis stroke="#52525b" fontSize={12} unit="%"/>
                        <XAxis dataKey="time" stroke="#52525b" fontSize={12} />
                    </AreaChart>
                </ResponsiveContainer>
            </MetricChartCard>
        </div>
    );
};

export default function InstanceDetailPage() {
    const params = useParams();
    const router = useRouter();
    const instanceName = params.name as string;

    const [instance, setInstance] = useState<InstanceMetric | null>(null);
    const [jobs, setJobs] = useState<Job[]>([]);
    const [history, setHistory] = useState<MetricHistory[]>([]);
    const [token, setToken] = useState<string | null>(null);
    const [isTerminalOpen, setTerminalOpen] = useState(false);
    const [timeRange, setTimeRange] = useState('1H');

    useEffect(() => {
        const storedToken = localStorage.getItem('axion_token');
        if (!storedToken) router.push('/login');
        else setToken(storedToken);
    }, [router]);

    // Combined fetch logic
    useEffect(() => {
        if (!token || !instanceName) return;

        const fetchData = async () => {
            try {
                const [instRes, jobsRes, historyRes] = await Promise.all([
                    fetch(`${window.location.protocol}//${window.location.hostname}:8500/instances/${instanceName}`, { headers: { 'Authorization': `Bearer ${token}` } }),
                    fetch(`${window.location.protocol}//${window.location.hostname}:8500/jobs`, { headers: { 'Authorization': `Bearer ${token}` } }),
                    fetch(`${window.location.protocol}//${window.location.hostname}:8500/instances/${instanceName}/metrics?range=${timeRange}`, { headers: { 'Authorization': `Bearer ${token}` } }),
                ]);

                if (instRes.status === 401) { router.push('/login'); return; }
                
                if (instRes.ok) setInstance(await instRes.json());
                if (jobsRes.ok) {
                    const jobsData = await jobsRes.json();
                    setJobs(Array.isArray(jobsData) ? jobsData : []);
                }
                if (historyRes.ok) {
                    const historyData = await historyRes.json();
                    setHistory(Array.isArray(historyData) ? historyData : []);
                }

            } catch (err) {
                console.error("Failed to fetch instance details:", err);
                toast.error("Failed to load instance data.");
            }
        };

        fetchData();

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.hostname;
        const wsUrl = `${protocol}//${host}:8500/ws/telemetry?token=${token}`;
        const ws = new WebSocket(wsUrl);

        ws.onmessage = (event) => {
            try {
                const rawData = JSON.parse(event.data);
                if (rawData && rawData.type === 'instance_metrics') {
                    const updatedInstance = rawData.data.find((inst: InstanceMetric) => inst.name === instanceName);
                    if (updatedInstance) {
                        setInstance(prevInstance => ({ ...prevInstance, ...updatedInstance }));
                    }
                }
            } catch (err) {
                console.error('WS parsing error:', err);
            }
        };

        return () => {
            ws.close();
        };
    }, [token, instanceName, router, timeRange]);
    
    const handlePowerAction = (action: 'start' | 'stop' | 'restart') => {
        if (!token) return;
        toast.promise(
            fetch(`${window.location.protocol}//${window.location.hostname}:8500/instances/${instanceName}/state`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
                body: JSON.stringify({ action }),
            }).then(res => { if (!res.ok) throw new Error(); }),
            {
                loading: `Requesting to ${action} instance...`,
                success: `Instance ${action} request queued.`,
                error: `Failed to ${action} instance.`,
            }
        );
    };

    const copyToClipboard = (text: string) => {
        if (!text || text === 'N/A') return;
        navigator.clipboard.writeText(text);
        toast.success("IP Address copied to clipboard!");
    };

    if (!instance) {
        return <div className="flex h-screen w-full items-center justify-center bg-zinc-950"><Loader2 className="animate-spin text-zinc-500" size={32} /></div>;
    }

    const isRunning = instance.status?.toLowerCase() === 'running';
    const ipAddress = instance.state?.network?.eth0?.addresses?.find(a => a.family === 'inet')?.address || 'N/A';
    
    const memUsage = formatBytes(instance.state?.memory?.usage);
    const memTotal = formatBytes(instance.state?.memory?.total);
    const diskUsage = formatBytes(instance.state?.root_device?.usage);
    const diskTotal = formatBytes(instance.state?.root_device?.total);
    const memPercent = (instance.state?.memory?.usage && instance.state?.memory?.total) ? ((instance.state.memory.usage / instance.state.memory.total) * 100).toFixed(0) : 0;
    const cpuUsage = instance.state?.cpu?.usage ? instance.state.cpu.usage.toFixed(0) : 0;

    const chartData = history.map(h => ({
        time: new Date(h.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        cpu: h.cpu_usage,
        memory: h.memory_usage ? h.memory_usage / 1024 : 0, // In MB
    }));

    return (
        <div className="min-h-screen bg-zinc-950 text-zinc-300 font-sans p-4 sm:p-6">
            <Toaster position="bottom-right" theme="dark" richColors />
            {isTerminalOpen && <WebTerminal instanceName={instanceName} onClose={() => setTerminalOpen(false)} />}
            
            <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4 mb-6">
                <div className="flex flex-col sm:flex-row justify-between sm:items-center">
                    <div className="flex items-center gap-3">
                        <h1 className="text-2xl font-bold text-zinc-100">{instance.name}</h1>
                        <span className={`flex items-center gap-2 px-2 py-0.5 rounded-full text-xs ${isRunning ? 'bg-emerald-900/50 text-emerald-300' : 'bg-red-900/50 text-red-300'}`}>
                            <span className={`h-2 w-2 rounded-full ${isRunning ? 'bg-emerald-500' : 'bg-red-500'}`}></span>
                            {instance.status}
                        </span>
                    </div>
                    <div className="flex items-center gap-2 text-sm text-zinc-400 mt-3 sm:mt-0">
                        {isRunning ? (
                            <>
                                <button onClick={() => handlePowerAction('stop')} className="px-3 py-1.5 text-sm rounded-md flex items-center gap-2 bg-red-900/50 border border-red-700/40 hover:bg-red-900/80 text-red-300 transition-colors"><Square size={14} /> Stop</button>
                                <button onClick={() => handlePowerAction('restart')} className="px-3 py-1.5 text-sm rounded-md flex items-center gap-2 bg-zinc-800 border border-zinc-700 hover:bg-zinc-700/80 text-zinc-200 transition-colors"><RefreshCw size={14} /> Restart</button>
                            </>
                        ) : (
                             <button onClick={() => handlePowerAction('start')} className="px-3 py-1.5 text-sm rounded-md flex items-center gap-2 bg-emerald-900/50 border border-emerald-700/40 hover:bg-emerald-900/80 text-emerald-300 transition-colors"><Play size={14} /> Start</button>
                        )}
                        <button onClick={() => setTerminalOpen(true)} disabled={!isRunning} className="px-3 py-1.5 text-sm rounded-md flex items-center gap-2 bg-zinc-800 border border-zinc-700 hover:bg-zinc-700/80 text-zinc-200 transition-colors disabled:opacity-50"><SquareTerminal size={14} /> Console</button>
                    </div>
                </div>
            </div>

            <main className="space-y-6">
                <div className="col-span-12">
                   <PerformanceMetrics history={history} timeRange={timeRange} onTimeRangeChange={setTimeRange} cpuUsage={cpuUsage} memPercent={memPercent} memUsage={memUsage} memTotal={memTotal} />
                </div>
                
                <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                    <InstanceInfoCard 
                        node={instance.location || 'N/A'}
                        ipAddress={ipAddress}
                        vcpu={instance.config?.['limits.cpu'] ? parseInt(instance.config['limits.cpu']) : 0}
                        ram={instance.config?.['limits.memory'] || 'N/A'}
                        disk={`${diskUsage} / ${diskTotal}`}
                    />
                    <DataProtectionCard instance={instance} />
                    <AuditLog jobs={jobs} instanceName={instanceName} />
                </div>
            </main>
        </div>
    );
}
