'use client';

import React, { useEffect, useState, useCallback } from 'react';
import { ShieldCheck, Server, Globe, Lock, Plus } from 'lucide-react';
import { Toaster, toast } from 'sonner';
import CreateNetworkModal from '../../../components/CreateNetworkModal';

interface NetworkPool {
  id: string;
  name: string;
  cidr: string;
  gateway: string;
  is_public: boolean;
  total_ips: number;
  used_ips: number;
  usage_percent: number;
}

export default function NetworksPage() {
  const [pools, setPools] = useState<NetworkPool[]>([]);
  const [loading, setLoading] = useState(true);
  const [isModalOpen, setIsModalOpen] = useState(false);

  // Define fetch logic as reusable function
  const fetchNetworks = useCallback(async () => {
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const token = localStorage.getItem('axion_token');
      if (!token) return;

      const res = await fetch(`${protocol}//${host}:${port}/api/v1/networks`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });

      if (res.ok) {
        const data = await res.json();
        setPools(data);
      } else {
        // Only toast if not loading first time to avoid noisy auth errors
        // toast.error("Failed to refresh networks");
      }
    } catch (err) {
      console.error(err);
      toast.error("Network error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchNetworks();
  }, [fetchNetworks]);

  return (
    <div className="p-6 space-y-8">
      <Toaster position="top-right" theme="dark" />

      <CreateNetworkModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        onSuccess={fetchNetworks}
      />

      <div className="flex justify-between items-center">
        <div>
          <h2 className="text-2xl font-bold text-zinc-100">Network Pools (IPAM)</h2>
          <p className="text-zinc-500">Manage IP address allocation pools.</p>
        </div>
        <button
          onClick={() => setIsModalOpen(true)}
          className="flex items-center gap-2 bg-indigo-600 hover:bg-indigo-500 text-white px-4 py-2 rounded-lg font-medium transition-colors shadow-lg shadow-indigo-900/20"
        >
          <Plus size={18} />
          Create Pool
        </button>
      </div>

      {loading ? (
        <div className="text-zinc-500">Loading networks...</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {pools.map(pool => (
            <div key={pool.id} className="bg-zinc-900/50 border border-zinc-800 rounded-xl p-6 shadow-lg backdrop-blur-sm hover:border-zinc-700 transition-all">

              {/* Header */}
              <div className="flex justify-between items-start mb-6">
                <div>
                  <h3 className="font-semibold text-lg text-zinc-100 flex items-center gap-2">
                    {pool.name}
                  </h3>
                  <div className="flex items-center gap-2 mt-1">
                    <span className="font-mono text-zinc-400 text-sm bg-zinc-950 px-2 py-0.5 rounded border border-zinc-800">
                      {pool.cidr}
                    </span>
                  </div>
                </div>
                {pool.is_public ? (
                  <span className="flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-bold bg-purple-500/10 text-purple-400 border border-purple-500/20">
                    <Globe size={12} /> PUBLIC
                  </span>
                ) : (
                  <span className="flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                    <Lock size={12} /> PRIVATE
                  </span>
                )}
              </div>

              {/* Progress Bar */}
              <div className="mb-6">
                <div className="flex justify-between text-xs font-medium text-zinc-400 mb-2">
                  <span>Usage: {pool.used_ips} / {pool.total_ips} IPs</span>
                  <span className={pool.usage_percent > 90 ? 'text-red-400' : 'text-zinc-400'}>
                    {pool.usage_percent.toFixed(1)}%
                  </span>
                </div>
                <div className="w-full bg-zinc-800 rounded-full h-2 overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-all duration-500 ${pool.usage_percent > 90 ? 'bg-red-500' :
                      pool.usage_percent > 60 ? 'bg-amber-500' : 'bg-indigo-500'
                      }`}
                    style={{ width: `${Math.max(2, pool.usage_percent)}%` }}
                  ></div>
                </div>
              </div>

              {/* Details */}
              <div className="grid grid-cols-2 gap-4 pt-4 border-t border-zinc-800/50">
                <div>
                  <span className="block text-[10px] uppercase tracking-wider text-zinc-500 font-semibold mb-1">Gateway</span>
                  <div className="text-sm font-mono text-zinc-300">{pool.gateway}</div>
                </div>
                <div className="text-right">
                  <span className="block text-[10px] uppercase tracking-wider text-zinc-500 font-semibold mb-1">Type</span>
                  <div className="text-sm text-zinc-300 flex justify-end items-center gap-1.5">
                    {pool.is_public ? <Server size={14} /> : <ShieldCheck size={14} />}
                    {pool.is_public ? 'Direct Bridge' : 'AxHV NAT'}
                  </div>
                </div>
              </div>

            </div>
          ))}
        </div>
      )}
    </div>
  );
}