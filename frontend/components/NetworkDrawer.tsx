'use client';

import React, { useState } from 'react';
import { X, Network, Trash2, Plus, ArrowRight, Loader2, Globe } from 'lucide-react';
import { toast } from 'sonner';
import { InstanceMetric } from '@/types';

interface NetworkDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  instance: InstanceMetric | null;
}

export default function NetworkDrawer({ isOpen, onClose, instance }: NetworkDrawerProps) {
  const [hostPort, setHostPort] = useState('');
  const [containerPort, setContainerPort] = useState('');
  const [protocol, setProtocol] = useState('tcp');
  const [loading, setLoading] = useState(false);

  if (!instance) return null;

  const ports = Object.entries(instance.devices || {})
    .filter(([_, dev]) => dev.type === 'proxy')
    .map(([name, dev]) => {
      const listenPort = dev.listen?.split(':').pop();
      const connectPort = dev.connect?.split(':').pop();
      const proto = dev.listen?.split(':')[0] || 'tcp';
      return {
        name,
        hostPort: listenPort,
        containerPort: connectPort,
        protocol: proto
      };
    });

  const handleAddPort = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!hostPort || !containerPort) return;

    setLoading(true);
    try {
      const apiProtocol = window.location.protocol; // Renomeado para não conflitar
      const host = window.location.hostname;
      const port = '8500';
      const token = localStorage.getItem('axion_token');

      const payload = {
        host_port: parseInt(hostPort),
        container_port: parseInt(containerPort),
        protocol: protocol // Usa o state (tcp/udp)
      };

      const res = await fetch(`${apiProtocol}//${host}:${port}/instances/${instance.name}/ports`, {
        method: 'POST',
        headers: { 
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}` 
        },
        body: JSON.stringify(payload)
      });

      if (res.ok) {
        toast.info("Port Mapping Queued");
        setHostPort('');
        setContainerPort('');
      } else {
        const err = await res.json();
        toast.error("Failed to map port", { description: err.error });
      }
    } catch (err) {
      toast.error("Network Error");
    } finally {
      setLoading(false);
    }
  };

  const handleDeletePort = async (portName: string, hostPortVal: string) => {
    if (!window.confirm(`Remove port mapping ${hostPortVal}?`)) return;

    try {
      const apiProtocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const token = localStorage.getItem('axion_token');

      const res = await fetch(`${apiProtocol}//${host}:${port}/instances/${instance.name}/ports/${hostPortVal}`, {
        method: 'DELETE',
        headers: { 
            'Authorization': `Bearer ${token}` 
        }
      });

      if (res.ok) {
        toast.info("Unmap Job Queued");
      } else {
        toast.error("Failed to unmap port");
      }
    } catch (err) {
      toast.error("Network Error");
    }
  };

  return (
    <>
      <div 
        className={`fixed inset-0 bg-black/60 backdrop-blur-[2px] z-40 transition-opacity duration-300 ${
          isOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'
        }`}
        onClick={onClose}
      />

      <div 
        className={`
          fixed top-0 right-0 h-full w-full sm:w-[450px] bg-zinc-950 border-l border-zinc-800 shadow-2xl z-50 transform transition-transform duration-300 ease-out flex flex-col
          ${isOpen ? 'translate-x-0' : 'translate-x-full'}
        `}
      >
        <div className="flex items-center justify-between p-5 border-b border-zinc-900 bg-zinc-950/50 backdrop-blur-md">
          <div>
            <h2 className="text-lg font-semibold text-zinc-100 flex items-center gap-2">
              <Network size={18} className="text-indigo-500" />
              Network & Ports
            </h2>
            <p className="text-xs text-zinc-500 mt-0.5">Manage external access for {instance.name}</p>
          </div>
          <button onClick={onClose} className="p-2 text-zinc-500 hover:text-zinc-100 hover:bg-zinc-900 rounded-lg transition-colors">
            <X size={20} />
          </button>
        </div>

        <div className="p-5 border-b border-zinc-900 bg-zinc-900/20">
            <form onSubmit={handleAddPort} className="flex flex-col gap-3">
                <div className="flex gap-2">
                    <div className="flex-1">
                        <label className="block text-[10px] font-medium text-zinc-500 uppercase tracking-wider mb-1.5">Host Port</label>
                        <input 
                            type="number" 
                            min="10000" max="60000"
                            placeholder="8080"
                            value={hostPort}
                            onChange={(e) => setHostPort(e.target.value)}
                            className="w-full bg-zinc-900 border border-zinc-800 text-zinc-200 rounded-md px-3 py-2 text-sm focus:outline-none focus:border-indigo-500/50"
                        />
                    </div>
                    <div className="flex-1">
                        <label className="block text-[10px] font-medium text-zinc-500 uppercase tracking-wider mb-1.5">Container Port</label>
                        <input 
                            type="number" 
                            placeholder="80"
                            value={containerPort}
                            onChange={(e) => setContainerPort(e.target.value)}
                            className="w-full bg-zinc-900 border border-zinc-800 text-zinc-200 rounded-md px-3 py-2 text-sm focus:outline-none focus:border-indigo-500/50"
                        />
                    </div>
                </div>
                
                <div className="flex gap-2">
                    <div className="flex-1">
                         <label className="block text-[10px] font-medium text-zinc-500 uppercase tracking-wider mb-1.5">Protocol</label>
                         <select
                            value={protocol}
                            onChange={(e) => setProtocol(e.target.value)}
                            className="w-full bg-zinc-900 border border-zinc-800 text-zinc-200 rounded-md px-3 py-2 text-sm focus:outline-none focus:border-indigo-500/50"
                         >
                             <option value="tcp">TCP</option>
                             <option value="udp">UDP</option>
                         </select>
                    </div>
                    <button 
                        type="submit"
                        disabled={loading || !hostPort || !containerPort}
                        className="flex-1 flex items-center justify-center gap-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-md transition-colors mt-auto h-[38px] text-sm font-medium"
                    >
                        {loading ? <Loader2 size={16} className="animate-spin" /> : <Plus size={16} />}
                        Map Port
                    </button>
                </div>
            </form>
            <p className="text-[10px] text-zinc-600 mt-3">
                Allowed Host Range: 10000 - 60000.
            </p>
        </div>

        <div className="flex-1 overflow-y-auto p-5">
            {ports.length === 0 ? (
                <div className="text-center py-10 text-zinc-600 border border-dashed border-zinc-800 rounded-lg">
                    <Globe size={24} className="mx-auto mb-2 opacity-20" />
                    <p className="text-sm">No ports mapped.</p>
                </div>
            ) : (
                <div className="space-y-3">
                    {ports.map(port => (
                        <div key={port.name} className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-4 flex items-center justify-between group hover:border-zinc-700 transition-colors">
                            <div className="flex items-center gap-4">
                                <div>
                                    <span className="block text-[10px] text-zinc-500 uppercase">Host</span>
                                    <span className="text-sm font-mono text-indigo-400">:{port.hostPort}</span>
                                </div>
                                <div className="px-2 py-0.5 rounded bg-zinc-800 text-[10px] text-zinc-400 uppercase font-bold">
                                    {port.protocol}
                                </div>
                                <div>
                                    <span className="block text-[10px] text-zinc-500 uppercase">Container</span>
                                    <span className="text-sm font-mono text-zinc-200">:{port.containerPort}</span>
                                </div>
                            </div>
                            
                            <button 
                                onClick={() => handleDeletePort(port.name, port.hostPort!)}
                                className="p-2 text-zinc-500 hover:text-red-400 hover:bg-red-500/10 rounded-md transition-colors opacity-60 group-hover:opacity-100"
                                title="Unmap Port"
                            >
                                <Trash2 size={16} />
                            </button>
                        </div>
                    ))}
                </div>
            )}
        </div>
      </div>
    </>
  );
}