'use client';

import React, { useEffect, useState, useRef } from 'react';
import { AreaChart, Area, ResponsiveContainer, Tooltip } from 'recharts';
import { Server, Cpu, Zap, Play, Square, RefreshCw, Loader2, Settings, Save, HardDrive, Network, FolderOpen, MoreVertical, X, SquareTerminal, Trash2, Box, Monitor, Search, ShieldCheck } from 'lucide-react';
import { toast, Toaster } from 'sonner';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import ActivityDrawer from '@/components/ActivityDrawer';
import CreateInstanceModal from '@/components/CreateInstanceModal';
import SnapshotDrawer from '@/components/SnapshotDrawer';
import NetworkDrawer from '@/components/NetworkDrawer';
import FileExplorerDrawer from '@/components/FileExplorerDrawer';
import WebTerminal from '@/components/WebTerminal';
import BackupPolicyModal from '@/components/BackupPolicyModal';
import { Job, InstanceMetric } from '@/types';
import { formatBytes } from '@/lib/utils'; // Import formatBytes

// --- Types Local ---

interface InstanceHistory {
  name: string;
  data: { time: string; memoryMB: number }[];
}

interface ResourceState {
  memoryInput: number;
  cpuInput: number;
  isDirty: boolean;
}

// --- Helper Functions ---


// --- Component ---

export default function InstancesPage() {
  // State
  const [metrics, setMetrics] = useState<InstanceMetric[]>([]);
  const [history, setHistory] = useState<Record<string, InstanceHistory>>({});
  const [jobs, setJobs] = useState<Job[]>([]);
  const storedToken = typeof window !== 'undefined' ? localStorage.getItem('axion_token') : null;
  const [token] = useState<string | null>(storedToken);

  // UI State
  const [isActivityOpen, setIsActivityOpen] = useState(false);
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [createModalType, setCreateModalType] = useState<'container' | 'virtual-machine'>('container');
  const [snapshotInstance, setSnapshotInstance] = useState<string | null>(null);
  const [networkInstance, setNetworkInstance] = useState<InstanceMetric | null>(null);
  const [fileInstance, setFileInstance] = useState<string | null>(null);
  const [terminalInstance, setTerminalInstance] = useState<string | null>(null);
  const [backupPolicyInstance, setBackupPolicyInstance] = useState<InstanceMetric | null>(null);
  const [showSettings, setShowSettings] = useState<Record<string, boolean>>({});
  const [resourceInputs, setResourceInputs] = useState<Record<string, ResourceState>>({});
  
  // Filter State
  const [filterType, setFilterType] = useState<'all' | 'container' | 'virtual-machine'>('all');

  // Menu State
  const [menuOpen, setMenuOpen] = useState<string | null>(null);

  // Refs
  const draggingRef = useRef<Record<string, boolean>>({});

  const router = useRouter();
  const wsRef = useRef<WebSocket | null>(null);


  // --- Auth Check ---
  useEffect(() => {
    if (!token) {
        router.push('/login');
    }
  }, [router, token]);

  // --- WebSocket Telementry ---
  useEffect(() => {
    if (!token) {
        console.warn("WebSocket connection skipped: no token provided.");
        return;
    }

    // Define the WebSocket URL with the hardcoded port 8500
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.hostname;
    const wsUrl = `${protocol}//${host}:8500/ws/telemetry?token=${token}`;

    let reconnectTimeout: NodeJS.Timeout;

    const connect = () => {
      // Prevent multiple connections
      if (wsRef.current && wsRef.current.readyState < 2) { // <2 means CONNECTING or OPEN
          return;
      }

      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        console.log('WebSocket connected to :8500');
      };

      ws.onmessage = (event) => {
        try {
          const rawData = JSON.parse(event.data);
          
          if (rawData && rawData.type === 'instance_metrics') {
            setMetrics(rawData.data);
          } else if (rawData && rawData.type === 'instance_history') {
            setHistory(rawData.data);
          } else if (rawData && rawData.type === 'jobs_update') {
            setJobs(rawData.data);
          } else if (Array.isArray(rawData)) { // Fallback for initial full list
            setMetrics(rawData);
          }

        } catch (err) {
          console.error('WS parsing error:', err);
        }
      };

      ws.onerror = (event) => {
        // Use console.warn for less intrusive error logging
        console.warn('WebSocket error:', event);
        // The onclose event will be fired next, which will handle reconnection.
        ws.close();
      };
      
      ws.onclose = () => {
        console.log('WebSocket disconnected. Attempting to reconnect in 5 seconds...');
        // Clear any existing timeout to avoid multiple reconnection loops
        clearTimeout(reconnectTimeout);
        // Set a timeout to reconnect
        reconnectTimeout = setTimeout(connect, 5000);
      };
    };

    connect();

    // Cleanup on component unmount
    return () => {
      clearTimeout(reconnectTimeout); // Clear the timeout on unmount
      if (wsRef.current) {
        // Remove the onclose listener before closing to prevent reconnection attempts on manual close
        wsRef.current.onclose = null; 
        wsRef.current.close();
      }
    };
  }, [token]);

  const handlePowerAction = async (name: string, action: 'start' | 'stop' | 'restart') => {
    if (!token) return;
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const response = await fetch(`${protocol}//${host}:${port}/instances/${name}/state`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({ action }),
      });

      if (response.status === 401) {
          router.push('/login');
          return;
      }

      if (response.ok) {
        toast.info("Request Queued");
      } else {
        const error = await response.json();
        toast.error("Request Rejected", { description: error.error });
      }
    } catch {
        toast.error("Network Error");
    }
  };

  const handleApplyLimits = async (name: string) => {
    const inputs = resourceInputs[name];
    if (!inputs || !token) return;

    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const payload = { memory: `${inputs.memoryInput}MB`, cpu: `${inputs.cpuInput}` };

      const response = await fetch(`${protocol}//${host}:${port}/instances/${name}/limits`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(payload),
      });

      if (response.status === 401) {
          router.push('/login');
          return;
      }

      if (response.ok) {
        toast.info("Config Update Queued");
        setResourceInputs(prev => ({ ...prev, [name]: { ...prev[name], isDirty: false } }));
        setShowSettings(prev => ({ ...prev, [name]: false })); // Close settings on success
      } else {
        const error = await response.json();
        toast.error("Update Rejected", { description: error.error });
      }
    } catch {
        toast.error("Network Error");
    }
  };

  const handleDeleteInstance = async (name: string) => {
    if (!token) return;
    if (!window.confirm(`Are you sure you want to DELETE instance "${name}"?`)) return;

    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const response = await fetch(`${protocol}//${host}:${port}/instances/${name}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      });

      if (response.ok) {
        toast.info("Deletion Queued");
        setShowSettings(prev => ({ ...prev, [name]: false }));
      } else {
        toast.error("Deletion Rejected");
      }
    } catch {
        toast.error("Network Error");
    }
  };

  const handleSliderChange = (name: string, type: 'memory' | 'cpu', value: number) => {
    setResourceInputs(prev => ({
        ...prev,
        [name]: { ...prev[name], [type === 'memory' ? 'memoryInput' : 'cpuInput']: value, isDirty: true }
    }));
  };

  const isInstanceBusy = (name: string) => {
    return jobs?.some(j => j.target === name && j.status === 'IN_PROGRESS');
  };

  // Close menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
        if (menuOpen && !(event.target as Element).closest('.menu-trigger')) {
            setMenuOpen(null);
        }
    };
    document.addEventListener('click', handleClickOutside);
    return () => document.removeEventListener('click', handleClickOutside);
  }, [menuOpen]);

  // --- Filtering Logic ---
  const filteredMetrics = metrics.filter(inst => {
    if (filterType === 'all') return true;
    return inst.type === filterType;
  });

  // --- Render ---

  if (!token) return null;

  return (
    <div>
      <Toaster position="top-right" theme="dark" closeButton />

      <ActivityDrawer isOpen={isActivityOpen} onClose={() => setIsActivityOpen(false)} jobs={jobs || []} />
      <CreateInstanceModal isOpen={isCreateModalOpen} onClose={() => setIsCreateModalOpen(false)} token={token} initialType={createModalType} />
      <SnapshotDrawer isOpen={!!snapshotInstance} onClose={() => setSnapshotInstance(null)} instanceName={snapshotInstance} />
      <NetworkDrawer isOpen={!!networkInstance} onClose={() => setNetworkInstance(null)} instance={networkInstance} />
      <FileExplorerDrawer isOpen={!!fileInstance} onClose={() => setFileInstance(null)} instanceName={fileInstance} />
      {terminalInstance && <WebTerminal instanceName={terminalInstance} onClose={() => setTerminalInstance(null)} />}
      <BackupPolicyModal isOpen={!!backupPolicyInstance} onClose={() => setBackupPolicyInstance(null)} instance={backupPolicyInstance} token={token} />

      <header className="mb-8">
        <div className="flex justify-between items-center mb-6">
          <div>
            <h1 className="text-2xl font-bold text-zinc-100">Instances</h1>
            <p className="text-zinc-500">Manage your containers and virtual machines</p>
          </div>
          <div className="flex items-center gap-3">
            <button onClick={() => setIsActivityOpen(true)} className="group flex items-center gap-2 px-3 py-1.5 bg-zinc-900 hover:bg-zinc-800 border border-zinc-800 rounded-md text-sm text-zinc-400 hover:text-zinc-200 transition-all hover:border-zinc-700">
              <HardDrive size={16} className="group-hover:text-indigo-400 transition-colors" /><span>Activity</span>
              {jobs?.some(j => j.status === 'IN_PROGRESS') && (
                  <span className="flex h-2 w-2 relative ml-1"><span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75"></span><span className="relative inline-flex rounded-full h-2 w-2 bg-indigo-500"></span></span>
              )}
            </button>
            <div className="flex items-center gap-2">
                <button onClick={() => { setCreateModalType('container'); setIsCreateModalOpen(true); }} className="flex items-center gap-2 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-md text-sm font-medium shadow-lg shadow-indigo-900/20 transition-all hover:scale-[1.02]">
                    <Box size={16} /><span>New Container</span>
                </button>
                <button onClick={() => { setCreateModalType('virtual-machine'); setIsCreateModalOpen(true); }} className="flex items-center gap-2 px-3 py-1.5 bg-purple-600 hover:bg-purple-500 text-white rounded-md text-sm font-medium shadow-lg shadow-purple-900/20 transition-all hover:scale-[1.02]">
                    <Monitor size={16} /><span>New VM</span>
                </button>
            </div>
          </div>
        </div>

        {/* Filter Tabs */}
        <div className="flex items-center">
            <div className="bg-zinc-900/50 p-1 rounded-lg inline-flex border border-zinc-800/50">
                <button
                    onClick={() => setFilterType('all')}
                    className={`px-4 py-1.5 text-sm font-medium rounded-md transition-all ${filterType === 'all' ? 'bg-zinc-800 text-white shadow-sm' : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/30'}`}
                >
                    All
                </button>
                <button
                    onClick={() => setFilterType('container')}
                    className={`px-4 py-1.5 text-sm font-medium rounded-md transition-all flex items-center gap-2 ${filterType === 'container' ? 'bg-zinc-800 text-white shadow-sm' : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/30'}`}
                >
                    <Box size={14} /> Containers
                </button>
                <button
                    onClick={() => setFilterType('virtual-machine')}
                    className={`px-4 py-1.5 text-sm font-medium rounded-md transition-all flex items-center gap-2 ${filterType === 'virtual-machine' ? 'bg-zinc-800 text-white shadow-sm' : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/30'}`}
                >
                    <Monitor size={14} /> Virtual Machines
                </button>
            </div>
        </div>
      </header>

      {/* Grid */}
      {filteredMetrics.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-24 text-zinc-500">
              <div className="bg-zinc-900/50 p-4 rounded-full mb-4 ring-1 ring-zinc-800">
                  <Search size={32} strokeWidth={1.5} className="text-zinc-600" />
              </div>
              <h3 className="text-lg font-medium text-zinc-300">No instances found</h3>
              <p className="text-sm">There are no {filterType === 'all' ? 'instances' : filterType === 'container' ? 'containers' : 'virtual machines'} in this view.</p>
          </div>
      ) : (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filteredMetrics.map((inst) => {
          const isRunning = inst.status && typeof inst.status === 'string' ? inst.status.toLowerCase() === 'running' : false;
          const histData = history[inst.name]?.data || [];
          const isBusy = isInstanceBusy(inst.name);
          const isSettingsOpen = showSettings[inst.name];
          const isMenuOpen = menuOpen === inst.name;
          const inputs = resourceInputs[inst.name] || { memoryInput: 128, cpuInput: 1, isDirty: false };

          return (
            <div key={inst.name} className={`group bg-zinc-900/30 backdrop-blur-sm border border-zinc-800 rounded-xl p-5 transition-all duration-300 flex flex-col relative overflow-visible ${isSettingsOpen ? 'ring-1 ring-zinc-700 shadow-xl shadow-black/50' : 'hover:border-zinc-700 hover:shadow-lg hover:shadow-black/20'}`}>
              {/* Status Stripe */}
              <div className={`absolute top-0 left-0 w-full h-0.5 rounded-t-xl ${isRunning ? 'bg-emerald-500/50' : 'bg-zinc-800'}`}></div>

              {/* Header */}
              <div className="flex justify-between items-start mb-6">
                <div className="flex items-center gap-3">
                  <div className={`p-2.5 rounded-lg border ${isRunning ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-500' : 'bg-zinc-800/50 border-zinc-700/50 text-zinc-500'}`}>
                    {inst.type === 'virtual-machine' ? <Monitor size={18} strokeWidth={1.5} /> : <Box size={18} strokeWidth={1.5} />}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <Link href={`/instances/${inst.name}`} className="font-bold text-sm text-zinc-200 tracking-tight hover:underline decoration-zinc-500 underline-offset-4">
                        {inst.name}
                      </Link>
                      {inst.location && inst.location !== 'none' && inst.location.trim() !== '' && (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 bg-zinc-800 border border-zinc-700 rounded text-xs text-zinc-300 font-medium">
                          <Server size={10} strokeWidth={1.5} />
                          {inst.location}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-1.5 mt-0.5">
                      <span className={`h-1.5 w-1.5 rounded-full ${isRunning ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.4)]' : 'bg-zinc-600'}`}></span>
                      <span className="text-[10px] font-medium text-zinc-500 uppercase tracking-wider">{inst.status}</span>
                    </div>
                  </div>
                </div>

                {/* Controls */}
                <div className="flex items-center gap-1 relative">
                  {isBusy ? (
                    <div className="px-2 py-1 bg-indigo-500/10 border border-indigo-500/20 rounded text-xs text-indigo-400 flex items-center gap-2">
                      <Loader2 size={12} className="animate-spin" />
                      <span>Working...</span>
                    </div>
                  ) : (
                    <>
                      {isRunning && (
                          <button onClick={() => setTerminalInstance(inst.name)} className="p-2 text-zinc-500 hover:text-zinc-200 hover:bg-zinc-800/50 rounded-md transition-colors" title="Terminal">
                              <SquareTerminal size={16} />
                          </button>
                      )}

                      {isRunning ? (
                        <>
                          <button onClick={() => handlePowerAction(inst.name, 'restart')} className="p-2 text-zinc-500 hover:text-blue-400 hover:bg-blue-500/10 rounded-md transition-colors"><RefreshCw size={16} /></button>
                          <button onClick={() => handlePowerAction(inst.name, 'stop')} className="p-2 text-zinc-500 hover:text-red-400 hover:bg-red-500/10 rounded-md transition-colors"><Square size={16} fill="currentColor" className="opacity-80" /></button>
                        </>
                      ) : (
                        <button onClick={() => handlePowerAction(inst.name, 'start')} className="p-2 text-zinc-500 hover:text-emerald-400 hover:bg-emerald-500/10 rounded-md transition-colors"><Play size={16} fill="currentColor" className="opacity-80" /></button>
                      )}

                      {/* Menu Trigger */}
                      <button
                          className={`p-2 rounded-md transition-colors menu-trigger ${isMenuOpen ? 'bg-zinc-800 text-zinc-200' : 'text-zinc-500 hover:text-zinc-200 hover:bg-zinc-800/50'}`}
                          onClick={(e) => { e.stopPropagation(); setMenuOpen(isMenuOpen ? null : inst.name); }}
                      >
                          <MoreVertical size={16} />
                      </button>

                      {/* Dropdown Menu */}
                      {isMenuOpen && (
                          <div className="absolute top-full right-0 mt-2 w-48 bg-zinc-900 border border-zinc-800 rounded-lg shadow-xl z-20 py-1 animate-in fade-in zoom-in-95 duration-100 origin-top-right">
                              <button onClick={() => { setFileInstance(inst.name); setMenuOpen(null); }} className="w-full text-left px-4 py-2 text-sm text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100 flex items-center gap-2">
                                  <FolderOpen size={14} /> Files
                              </button>
                              <button onClick={() => { setNetworkInstance(inst); setMenuOpen(null); }} className="w-full text-left px-4 py-2 text-sm text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100 flex items-center gap-2">
                                  <Network size={14} /> Network
                              </button>
                              <button onClick={() => { setSnapshotInstance(inst.name); setMenuOpen(null); }} className="w-full text-left px-4 py-2 text-sm text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100 flex items-center gap-2">
                                  <HardDrive size={14} /> Backups
                              </button>
                              <button onClick={() => { setBackupPolicyInstance(inst); setMenuOpen(null); }} className="w-full text-left px-4 py-2 text-sm text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100 flex items-center gap-2">
                                  <ShieldCheck size={14} /> Backup Settings
                              </button>
                              <div className="h-px bg-zinc-800 my-1"></div>
                              <button onClick={() => { setShowSettings(prev => ({ ...prev, [inst.name]: true })); setMenuOpen(null); }} className="w-full text-left px-4 py-2 text-sm text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100 flex items-center gap-2">
                                  <Settings size={14} /> Settings
                              </button>
                              <button onClick={() => { handleDeleteInstance(inst.name); setMenuOpen(null); }} className="w-full text-left px-4 py-2 text-sm text-red-400 hover:bg-red-500/10 hover:text-red-300 flex items-center gap-2">
                                  <Trash2 size={14} /> Delete
                              </button>
                          </div>
                      )}
                    </>
                  )}
                </div>
              </div>

              {/* Settings Panel (Moved inside) */}
              {isSettingsOpen && (
                  <div className="mb-6 p-4 bg-black/40 rounded-lg border border-zinc-800 space-y-5 animate-in fade-in slide-in-from-top-2 duration-200">
                      <div className="flex justify-between items-center pb-2 border-b border-zinc-800/50">
                          <span className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">Resource Limits</span>
                          <button onClick={() => setShowSettings(prev => ({ ...prev, [inst.name]: false }))} className="text-zinc-500 hover:text-zinc-300"><X size={14}/></button>
                      </div>
                      <div>
                          <div className="flex justify-between items-center mb-2"><span className="text-xs font-medium text-zinc-400">RAM Limit</span><span className="text-xs font-mono text-indigo-400">{inputs.memoryInput} MB</span></div>
                          <input type="range" min="128" max="4096" step="128" value={inputs.memoryInput} onMouseDown={() => draggingRef.current[inst.name] = true} onMouseUp={() => draggingRef.current[inst.name] = false} onChange={(e) => handleSliderChange(inst.name, 'memory', parseInt(e.target.value))} className="w-full h-1.5 bg-zinc-800 rounded-lg appearance-none cursor-pointer accent-indigo-500 hover:accent-indigo-400" />
                      </div>
                      <div>
                          <div className="flex justify-between items-center mb-2"><span className="text-xs font-medium text-zinc-400">vCPU Limit</span><span className="text-xs font-mono text-indigo-400">{inputs.cpuInput} Cores</span></div>
                          <input type="range" min="1" max="8" step="1" value={inputs.cpuInput} onMouseDown={() => draggingRef.current[inst.name] = true} onMouseUp={() => draggingRef.current[inst.name] = false} onChange={(e) => handleSliderChange(inst.name, 'cpu', parseInt(e.target.value))} className="w-full h-1.5 bg-zinc-800 rounded-lg appearance-none cursor-pointer accent-indigo-500 hover:accent-indigo-400" />
                      </div>
                      <div className="flex justify-end pt-1">
                          {inputs.isDirty && (
                              <button onClick={() => handleApplyLimits(inst.name)} className="flex items-center gap-2 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium rounded-md shadow-lg shadow-indigo-500/20 transition-all active:scale-95">
                                  <Save size={12} /> Apply Changes
                              </button>
                          )}
                      </div>
                  </div>
              )}

              {/* Metrics */}
              <div className="grid grid-cols-2 gap-3 mb-6">
                <div className="bg-zinc-950/30 p-3 rounded-lg border border-zinc-800/50">
                  <div className="flex items-center gap-2 mb-1 text-zinc-500"><Zap size={12} strokeWidth={1.5} /><span className="text-[10px] uppercase tracking-wider font-semibold">CPU Time</span></div>
                  <p className="text-sm font-mono text-zinc-200">{inst.cpu_usage_seconds}s</p>
                </div>
                <div className="bg-zinc-900/30 p-3 rounded-lg border border-zinc-800/50">
                  <div className="flex items-center gap-2 mb-1 text-zinc-500"><Cpu size={12} strokeWidth={1.5} /><span className="text-[10px] uppercase tracking-wider font-semibold">Memory</span></div>
                  <p className="text-sm font-mono text-zinc-200">{formatBytes(inst.memory_usage_bytes)}</p>
                </div>
              </div>

              {/* Chart */}
              <div className="h-20 w-full -mx-2 mt-auto opacity-80 hover:opacity-100 transition-opacity">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={histData}>
                    <defs><linearGradient id={`gradient-${inst.name}`} x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="#6366f1" stopOpacity={0.2}/><stop offset="100%" stopColor="#6366f1" stopOpacity={0}/></linearGradient></defs>
                    <Tooltip contentStyle={{ backgroundColor: '#09090b', borderColor: '#27272a', color: '#e4e4e7', borderRadius: '8px', fontSize: '12px', padding: '6px 10px', boxShadow: '0 4px 12px rgba(0,0,0,0.5)' }} itemStyle={{ color: '#818cf8', padding: 0 }} labelStyle={{ display: 'none' }} cursor={{ stroke: '#3f3f46', strokeWidth: 1 }} formatter={(value: number) => [`${value} MB`, '']} />
                    <Area type="monotone" dataKey="memoryMB" stroke="#6366f1" strokeWidth={1.5} fill={`url(#gradient-${inst.name})`} isAnimationActive={false} />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>
          );
        })}
      </div>
      )}
    </div>
  );
}