'use client';

import React, { useState, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { toast, Toaster } from 'sonner';
import { Play, Square, RefreshCw, SquareTerminal, HardDrive, Network, FolderOpen, ShieldCheck, Settings, Trash2, Box, Monitor, Server, Wifi, Cpu, Loader2 } from 'lucide-react';
import * as Tabs from '@radix-ui/react-tabs';

import { InstanceMetric, Job } from '@/types';
import InstanceMetricsCharts from '@/components/InstanceMetricsCharts';
import SnapshotDrawer from '@/components/SnapshotDrawer';
import FileExplorerDrawer from '@/components/FileExplorerDrawer';
import WebTerminal from '@/components/WebTerminal';
import NetworkDrawer from '@/components/NetworkDrawer';
import BackupPolicyModal from '@/components/BackupPolicyModal';
import BackupHealthCard from '@/components/BackupHealthCard';

interface Address {
  family: string;
  address: string;
  netmask: string;
  scope: string;
}

const formatIp = (inst?: InstanceMetric) => {
    if (!inst?.state?.network) return 'N/A';
    const eth0 = inst.state.network.eth0;
    if (eth0?.addresses) {
        const ip = eth0.addresses.find((a: Address) => a.family === 'inet');
        return ip ? ip.address : 'N/A';
    }
    return 'N/A';
};

const AuditLog = ({ jobs, instanceName }: { jobs: Job[], instanceName: string }) => {
    const instanceJobs = jobs
        .filter(j => j.target === instanceName)
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());

    if (instanceJobs.length === 0) {
        return <p className="text-sm text-zinc-500 mt-4">No recent activity found for this instance.</p>;
    }

    return (
        <div className="mt-6 space-y-4">
            {instanceJobs.map(job => (
                 <div key={job.id} className="flex items-start gap-3">
                    <div className="flex-shrink-0 mt-1.5 h-2.5 w-2.5 rounded-full bg-zinc-700"></div>
                    <div className="flex-grow">
                        <p className="text-sm text-zinc-300">{job.type.replace(/_/g, ' ')}</p>
                        <p className="text-xs text-zinc-500">
                            {new Date(job.created_at).toLocaleString()} - Status: <span className={`font-medium ${
                                job.status === 'COMPLETED' ? 'text-emerald-400' :
                                job.status === 'FAILED' ? 'text-red-400' :
                                job.status === 'IN_PROGRESS' ? 'text-indigo-400' : 'text-zinc-400'
                            }`}>{job.status}</span>
                        </p>
                         {job.status === 'FAILED' && <p className="text-xs text-red-400/80 mt-1">Error: {job.error}</p>}
                    </div>
                </div>
            ))}
        </div>
    );
};


export default function InstanceDetailPage() {
    const params = useParams();
    const router = useRouter();
    const instanceName = params.name as string;

    const [instance, setInstance] = useState<InstanceMetric | null>(null);
    const [jobs, setJobs] = useState<Job[]>([]);
    const [token, setToken] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);

    // Drawer/Modal States
    const [isSnapshotDrawerOpen, setSnapshotDrawerOpen] = useState(false);
    const [isFileExplorerOpen, setFileExplorerOpen] = useState(false);
    const [isTerminalOpen, setTerminalOpen] = useState(false);
    const [isNetworkDrawerOpen, setNetworkDrawerOpen] = useState(false);
    const [isBackupModalOpen, setIsBackupModalOpen] = useState(false);

    useEffect(() => {
        const storedToken = localStorage.getItem('axion_token');
        if (!storedToken) {
            router.push('/login');
        } else {
            setToken(storedToken);
        }
    }, [router]);

    useEffect(() => {
        if (!token || !instanceName) return;

        const fetchData = async () => {
            setLoading(true);
            try {
                // Fetch the specific instance with backup info using the new API endpoint
                const [instRes, jobsRes] = await Promise.all([
                    fetch(`${window.location.protocol}//${window.location.hostname}:8500/instances/${instanceName}`, { headers: { 'Authorization': `Bearer ${token}` } }),
                    fetch(`${window.location.protocol}//${window.location.hostname}:8500/jobs`, { headers: { 'Authorization': `Bearer ${token}` } }),
                ]);

                if (instRes.status === 401 || jobsRes.status === 401) {
                    router.push('/login');
                    return;
                }

                if (instRes.ok) {
                    const instanceData = await instRes.json();
                    setInstance(instanceData);
                } else {
                    toast.error(`Failed to load instance: ${instRes.status}`);
                }

                setJobs(await jobsRes.json() || []);

            } catch (err) {
                console.error("Failed to fetch instance details:", err);
                toast.error("Failed to load instance data.");
            } finally {
                setLoading(false);
            }
        };

        fetchData();
    }, [token, instanceName, router]);
    
    const handlePowerAction = async (action: 'start' | 'stop' | 'restart') => {
        if (!token) return;
        try {
            const response = await fetch(`${window.location.protocol}//${window.location.hostname}:8500/instances/${instanceName}/state`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
                body: JSON.stringify({ action }),
            });
            if (response.ok) toast.info(`Instance ${action} request queued.`);
            else toast.error(`Failed to ${action} instance.`);
        } catch (error) {
            toast.error("Network Error");
        }
    };


    if (loading) {
        return <div className="flex justify-center items-center h-screen"><Loader2 className="animate-spin" size={32} /></div>;
    }
    
    if (!instance) {
         return <div className="flex justify-center items-center h-screen text-red-500">Instance not found.</div>;
    }

    const isRunning = instance.status.toLowerCase() === 'running';

    return (
        <div className="max-w-7xl mx-auto p-4 sm:p-6 lg:p-8">
            <Toaster position="top-right" theme="dark" />
            <SnapshotDrawer isOpen={isSnapshotDrawerOpen} onClose={() => setSnapshotDrawerOpen(false)} instanceName={instanceName} />
            <FileExplorerDrawer isOpen={isFileExplorerOpen} onClose={() => setFileExplorerOpen(false)} instanceName={instanceName} />
            <NetworkDrawer isOpen={isNetworkDrawerOpen} onClose={() => setNetworkDrawerOpen(false)} instance={instance} />
            <BackupPolicyModal isOpen={isBackupModalOpen} onClose={() => setIsBackupModalOpen(false)} instanceName={instanceName} token={token} />
            {isTerminalOpen && <WebTerminal instanceName={instanceName} onClose={() => setTerminalOpen(false)} />}


            {/* Header */}
            <header className="mb-8">
                <div className="flex flex-col md:flex-row justify-between md:items-center gap-4">
                    <div>
                        <h1 className="text-3xl font-bold text-zinc-100 flex items-center gap-3">
                           {instance.type === 'virtual-machine' ? <Monitor size={28} /> : <Box size={28} />}
                           {instance.name}
                        </h1>
                         <div className="flex items-center gap-4 mt-2 text-sm text-zinc-400">
                            <span className={`flex items-center gap-1.5 px-2 py-1 rounded-full text-xs font-medium ${isRunning ? 'bg-emerald-500/10 text-emerald-400' : 'bg-zinc-700 text-zinc-300'}`}>
                               <span className={`h-2 w-2 rounded-full ${isRunning ? 'bg-emerald-500' : 'bg-zinc-500'}`}></span>
                               {instance.status}
                            </span>
                             <span className="flex items-center gap-2"><Wifi size={14}/> {formatIp(instance)}</span>
                             <span className="flex items-center gap-2"><Server size={14}/>{instance.location}</span>
                        </div>
                    </div>
                     <div className="flex items-center gap-2">
                        {isRunning && <button onClick={() => setTerminalOpen(true)} className="px-4 py-2 text-sm rounded-md flex items-center gap-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 transition-colors"><SquareTerminal size={14} /> Terminal</button>}
                        {isRunning ? (
                            <>
                                <button onClick={() => handlePowerAction('restart')} className="px-4 py-2 text-sm rounded-md flex items-center gap-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 transition-colors"><RefreshCw size={14} /> Restart</button>
                                <button onClick={() => handlePowerAction('stop')} className="px-4 py-2 text-sm rounded-md flex items-center gap-2 bg-red-600/20 hover:bg-red-600/30 text-red-400 transition-colors"><Square size={14} /> Stop</button>
                            </>
                        ) : (
                             <button onClick={() => handlePowerAction('start')} className="px-4 py-2 text-sm rounded-md flex items-center gap-2 bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-400 transition-colors"><Play size={14} /> Start</button>
                        )}
                    </div>
                </div>
            </header>
            
            {/* Tabs */}
             <Tabs.Root defaultValue="overview" className="w-full">
                <Tabs.List className="flex border-b border-zinc-800">
                    <Tabs.Trigger value="overview" className="px-4 py-2 text-sm font-medium text-zinc-400 data-[state=active]:text-white data-[state=active]:shadow-[inset_0_-1px_0_0,0_1px_0_0] data-[state=active]:shadow-current hover:text-white transition-colors">Overview</Tabs.Trigger>
                    <Tabs.Trigger value="console" className="px-4 py-2 text-sm font-medium text-zinc-400 data-[state=active]:text-white data-[state=active]:shadow-[inset_0_-1px_0_0,0_1px_0_0] data-[state=active]:shadow-current hover:text-white transition-colors">Console</Tabs.Trigger>
                    <Tabs.Trigger value="files" className="px-4 py-2 text-sm font-medium text-zinc-400 data-[state=active]:text-white data-[state=active]:shadow-[inset_0_-1px_0_0,0_1px_0_0] data-[state=active]:shadow-current hover:text-white transition-colors">Files</Tabs.Trigger>
                    <Tabs.Trigger value="backups" className="px-4 py-2 text-sm font-medium text-zinc-400 data-[state=active]:text-white data-[state=active]:shadow-[inset_0_-1px_0_0,0_1px_0_0] data-[state=active]:shadow-current hover:text-white transition-colors">Backups</Tabs.Trigger>
                    <Tabs.Trigger value="networking" className="px-4 py-2 text-sm font-medium text-zinc-400 data-[state=active]:text-white data-[state=active]:shadow-[inset_0_-1px_0_0,0_1px_0_0] data-[state=active]:shadow-current hover:text-white transition-colors">Networking</Tabs.Trigger>
                </Tabs.List>
                
                <Tabs.Content value="overview" className="py-6 focus:outline-none">
                    <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-6">
                        <div className="lg:col-span-2 xl:col-span-2">
                            <InstanceMetricsCharts instanceName={instanceName} token={token} />
                        </div>
                        <div className="space-y-6">
                            {/* Backup Health Card */}
                            <BackupHealthCard
                                instance={instance}
                                onConfigureBackup={() => setIsBackupModalOpen(true)}
                            />

                            <div>
                                <h3 className="text-lg font-semibold text-zinc-100">Audit Log</h3>
                                <AuditLog jobs={jobs} instanceName={instanceName} />
                            </div>
                        </div>
                    </div>
                </Tabs.Content>
                <Tabs.Content value="console" className="py-6 focus:outline-none text-center text-zinc-500">
                    <p>Inline terminal is not yet available.</p>
                    <button onClick={() => setTerminalOpen(true)} className="mt-4 px-4 py-2 text-sm rounded-md flex items-center gap-2 bg-indigo-600 hover:bg-indigo-500 text-white transition-colors mx-auto"><SquareTerminal size={14} /> Open Terminal in New Window</button>
                </Tabs.Content>
                <Tabs.Content value="files" className="py-6 focus:outline-none text-center text-zinc-500">
                     <p>Inline file management is not yet available.</p>
                     <button onClick={() => setFileExplorerOpen(true)} className="mt-4 px-4 py-2 text-sm rounded-md flex items-center gap-2 bg-indigo-600 hover:bg-indigo-500 text-white transition-colors mx-auto"><FolderOpen size={14} /> Open File Explorer</button>
                </Tabs.Content>
                <Tabs.Content value="backups" className="py-6 focus:outline-none text-center text-zinc-500">
                    <div className="flex flex-col items-center gap-4">
                        <p>Configure and manage instance snapshots and backup policies.</p>
                        <div className="flex gap-4">
                            <button onClick={() => setSnapshotDrawerOpen(true)} className="px-4 py-2 text-sm rounded-md flex items-center gap-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 transition-colors"><HardDrive size={14} /> Manage Snapshots</button>
                            <button onClick={() => setIsBackupModalOpen(true)} className="px-4 py-2 text-sm rounded-md flex items-center gap-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 transition-colors"><ShieldCheck size={14} /> Automatic Backups</button>
                        </div>
                    </div>
                </Tabs.Content>
                 <Tabs.Content value="networking" className="py-6 focus:outline-none text-center text-zinc-500">
                    <p>Configure network interfaces and port forwarding.</p>
                    <button onClick={() => setNetworkDrawerOpen(true)} className="mt-4 px-4 py-2 text-sm rounded-md flex items-center gap-2 bg-indigo-600 hover:bg-indigo-500 text-white transition-colors mx-auto"><Network size={14} /> Open Network Manager</button>
                </Tabs.Content>
            </Tabs.Root>
        </div>
    );
}
