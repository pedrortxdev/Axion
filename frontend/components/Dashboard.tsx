'use client';

import React, { useEffect, useState, useRef } from 'react';
import { AreaChart, Area, ResponsiveContainer, Tooltip } from 'recharts';
import { Server, Cpu, Zap, Activity, Play, Square, RefreshCw, Loader2, Settings, Save, History, Wifi, WifiOff, Plus, SquareTerminal, Trash2, AlertTriangle, HardDrive, Network, FolderOpen, MoreVertical, X, FileText, Globe } from 'lucide-react';
import { toast, Toaster } from 'sonner';
import { useRouter } from 'next/navigation';
import ActivityDrawer from './ActivityDrawer';
import CreateInstanceModal from './CreateInstanceModal';
import SnapshotDrawer from './SnapshotDrawer';
import NetworkDrawer from './NetworkDrawer';
import FileExplorerDrawer from './FileExplorerDrawer';
import WebTerminal from './WebTerminal';
import HostStatsCard from './HostStatsCard';
import ClusterStatus from './ClusterStatus';
import LogDrawer from './LogDrawer';
import NetworkManagerModal from './NetworkManagerModal';
import { Job, InstanceMetric } from '@/types';

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

const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const formatMemToMB = (bytes: number) => {
  return parseFloat((bytes / (1024 * 1024)).toFixed(1));
};

const parseMemoryString = (memStr?: string): number => {
  if (!memStr) return 128; 
  const match = memStr.match(/^(\d+)(MB|GB)?$/i);
  if (!match) return 128;
  const value = parseInt(match[1], 10);
  const unit = match[2]?.toUpperCase();
  if (unit === 'GB') return value * 1024;
  return value;
};

const parseCpuString = (cpuStr?: string): number => {
  if (!cpuStr) return 1;
  return parseInt(cpuStr, 10) || 1;
};

// --- Component ---

export default function Dashboard() {
  // State
  const [metrics, setMetrics] = useState<InstanceMetric[]>([]);
  const [history, setHistory] = useState<Record<string, InstanceHistory>>({});
  const [jobs, setJobs] = useState<Job[]>([]);
  const [connected, setConnected] = useState(false);
  const [lastTelemetryTime, setLastTelemetryTime] = useState<string>('-');
  const [token, setToken] = useState<string | null>(null);
  const [hostStats, setHostStats] = useState<any>(null); // We'll use any to match the HostStats interface
  const [prevMetrics, setPrevMetrics] = useState<Record<string, InstanceMetric>>({});
  
  // UI State
  const [isActivityOpen, setIsActivityOpen] = useState(false);
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [snapshotInstance, setSnapshotInstance] = useState<string | null>(null);
  const [networkInstance, setNetworkInstance] = useState<string | null>(null);
  const [fileInstance, setFileInstance] = useState<string | null>(null);
  const [terminalInstance, setTerminalInstance] = useState<string | null>(null);
  const [logInstance, setLogInstance] = useState<string | null>(null);
  const [showNetworkManager, setShowNetworkManager] = useState(false);
  const [showSettings, setShowSettings] = useState<Record<string, boolean>>({});
  const [resourceInputs, setResourceInputs] = useState<Record<string, ResourceState>>({});

  // Menu State
  const [menuOpen, setMenuOpen] = useState<string | null>(null);
  
  // Refs
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const draggingRef = useRef<Record<string, boolean>>({});
  
  const router = useRouter();

  // --- Auth Check ---
  useEffect(() => {
    const storedToken = localStorage.getItem('axion_token');
    if (!storedToken) {
        router.push('/login');
    } else {
        setToken(storedToken);
    }
  }, [router]);

  // --- Initial Data Fetch ---
  useEffect(() => {
    if (!token) return;

    const fetchJobs = async () => {
      try {
        const protocol = window.location.protocol;
        const host = window.location.hostname;
        const port = '8500';
        const res = await fetch(`${protocol}//${host}:${port}/jobs`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        
        if (res.status === 401) {
            localStorage.removeItem('axion_token');
            router.push('/login');
            return;
        }

        if (res.ok) {
          const data = await res.json();
          setJobs(data || []);
        }
      } catch (err) {
        console.error("Failed to fetch jobs:", err);
      }
    };
    fetchJobs();
  }, [token, router]);

  // --- WebSocket Logic ---
  const connect = () => {
    if (!token) return;
    if (wsRef.current?.readyState === WebSocket.OPEN || wsRef.current?.readyState === WebSocket.CONNECTING) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.hostname;
    const wsUrl = `${protocol}//${host}:8500/ws/telemetry?token=${token}`;

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
    };

    ws.onmessage = (event) => {
      try {
        const rawData = JSON.parse(event.data);
        if (Array.isArray(rawData)) {
            handleTelemetry(rawData);
            return;
        }
        if (rawData.type === 'job_update') {
            handleJobUpdate(rawData.payload as Job);
        }
        if (rawData.type === 'host_telemetry') {
            setHostStats(rawData.data);
        }
      } catch (err) {
        console.error('WS parsing error:', err);
      }
    };

    ws.onclose = () => {
      setConnected(false);
      wsRef.current = null;
      scheduleReconnect();
    };

    ws.onerror = () => ws.close();
  };

  const scheduleReconnect = () => {
    if (!reconnectTimeoutRef.current) {
      reconnectTimeoutRef.current = setTimeout(() => {
        reconnectTimeoutRef.current = null;
        connect();
      }, 2000);
    }
  };

  useEffect(() => {
    if (token) connect();
    return () => {
      if (wsRef.current) wsRef.current.close();
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current);
    };
  }, [token]); 

  // --- Message Handlers ---

  const handleTelemetry = (data: InstanceMetric[]) => {
    const now = new Date();
    const timeStr = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    setLastTelemetryTime(timeStr);
    
    setMetrics(prevMetricsList => {
      const newPrevMetrics: Record<string, InstanceMetric> = {};
      prevMetricsList.forEach(m => {
        newPrevMetrics[m.name] = m;
      });
      setPrevMetrics(newPrevMetrics);
      return data;
    });

    setResourceInputs(prev => {
        const next = { ...prev };
        let changed = false;
        data.forEach(inst => {
            const isDragging = draggingRef.current[inst.name];
            const currentLocal = next[inst.name];
            if (!isDragging && (!currentLocal || !currentLocal.isDirty)) {
                const serverMem = parseMemoryString(inst.config["limits.memory"]);
                const serverCpu = parseCpuString(inst.config["limits.cpu"]);
                if (!currentLocal || currentLocal.memoryInput !== serverMem || currentLocal.cpuInput !== serverCpu) {
                    next[inst.name] = { memoryInput: serverMem, cpuInput: serverCpu, isDirty: false };
                    changed = true;
                }
            }
        });
        return changed ? next : prev;
    });

    setHistory(prevHistory => {
      const newHistory = { ...prevHistory };
      data.forEach(instance => {
        if (!newHistory[instance.name]) newHistory[instance.name] = { name: instance.name, data: [] };
        const currentData = [...newHistory[instance.name].data];
        currentData.push({ time: timeStr, memoryMB: formatMemToMB(instance.memory_usage_bytes) });
        if (currentData.length > 60) currentData.shift();
        newHistory[instance.name] = { ...newHistory[instance.name], data: currentData };
      });
      return newHistory;
    });
  };

  const handleJobUpdate = (job: Job) => {
    setJobs(prevJobs => {
        const index = prevJobs.findIndex(j => j.id === job.id);
        if (index !== -1) {
            const updated = [...prevJobs];
            updated[index] = job;
            return updated;
        }
        return [job, ...prevJobs];
    });

    if (job.status === 'COMPLETED') {
        toast.success(
            <div className="flex flex-col gap-1">
                <span className="font-semibold">Operation Completed</span>
                <span className="text-xs text-zinc-400">{job.type} on {job.target} finished successfully.</span>
            </div>
        );
    } else if (job.status === 'FAILED') {
        toast.error(
            <div className="flex flex-col gap-1">
                <span className="font-semibold">Operation Failed</span>
                <span className="text-xs text-zinc-400">{job.error}</span>
            </div>
        );
    }
  };

  // --- Actions ---

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
    } catch (error) {
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
    } catch (error) {
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
    } catch (error) {
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

  // --- Render ---

  if (!token) return null; 

  const getCpuUsagePercent = (inst: InstanceMetric, prevMetrics: Record<string, InstanceMetric>, lastTelemetryTime: string) => {
    const prevInst = prevMetrics[inst.name];
    if (!prevInst) return 0;

    const cores = parseCpuString(inst.config["limits.cpu"]);
    const timeDelta = (new Date().getTime() - new Date(lastTelemetryTime).getTime()) / 1000;
    if (timeDelta === 0) return 0;

    const cpuSecondsDelta = inst.cpu_usage_seconds - prevInst.cpu_usage_seconds;
    return (cpuSecondsDelta / (timeDelta * cores)) * 100;
  };

  const getMemUsagePercent = (inst: InstanceMetric) => {
    const limitBytes = parseMemoryString(inst.config["limits.memory"]) * 1024 * 1024;
    if (limitBytes === 0) return 0;
    return (inst.memory_usage_bytes / limitBytes) * 100;
  };

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-200 font-sans antialiased selection:bg-indigo-500/30">
      <Toaster position="top-right" theme="dark" closeButton />
    </div>
  );
}
