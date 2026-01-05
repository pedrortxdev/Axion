'use client';

import React, { useEffect, useState, useRef } from 'react';
import { toast, Toaster } from 'sonner';
import { useRouter } from 'next/navigation';
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
  const storedToken = typeof window !== 'undefined' ? localStorage.getItem('axion_token') : null;
  const [token] = useState<string | null>(storedToken);
  const [prevMetrics, setPrevMetrics] = useState<Record<string, InstanceMetric>>({});

  // UI State
  const [showSettings, setShowSettings] = useState<Record<string, boolean>>({});
  const [resourceInputs, setResourceInputs] = useState<Record<string, ResourceState>>({});

  // Menu State
  const [menuOpen, setMenuOpen] = useState<string | null>(null);

  // Refs
  const draggingRef = useRef<Record<string, boolean>>({});

  const router = useRouter();


  // --- Auth Check ---
  useEffect(() => {
    if (!token) {
      router.push('/login');
    }
  }, [router, token]);

  const handlePowerAction = async (name: string, action: 'start' | 'stop' | 'restart') => {
    if (!token) return;
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const response = await fetch(`${protocol}//${host}:${port}/api/v1/instances/${name}/state`, {
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

      const response = await fetch(`${protocol}//${host}:${port}/api/v1/instances/${name}/limits`, {
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
      const response = await fetch(`${protocol}//${host}:${port}/api/v1/instances/${name}`, {
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
