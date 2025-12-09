'use client';

import React, { useEffect, useState } from 'react';
import { X, RotateCcw, Trash2, Camera, Plus, Loader2, Clock, HardDrive } from 'lucide-react';
import { format } from 'date-fns';
import { toast } from 'sonner';

interface Snapshot {
  name: string;
  created_at: string;
  stateful: boolean;
}

interface SnapshotDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  instanceName: string | null;
}

export default function SnapshotDrawer({ isOpen, onClose, instanceName }: SnapshotDrawerProps) {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newSnapName, setNewSnapName] = useState('');

  // Generate default name when opening
  useEffect(() => {
    if (isOpen) {
      const now = new Date();
      const defaultName = `snap-${now.toISOString().slice(0, 10).replace(/-/g, '')}-${now.getHours()}${now.getMinutes()}`;
      setNewSnapName(defaultName);
      fetchSnapshots();
    }
  }, [isOpen, instanceName]);

  const fetchSnapshots = async () => {
    if (!instanceName) return;
    setLoading(true);
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const token = localStorage.getItem('axion_token');

      const res = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/snapshots`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });

      if (res.ok) {
        const data = await res.json();
        // LXD returns snapshots with full names like "container/snap0", we want just the snapshot name
        const cleanSnaps = data.map((s: any) => ({
            ...s,
            name: s.name.split('/').pop() || s.name
        }));
        // Sort by newest
        cleanSnaps.sort((a: Snapshot, b: Snapshot) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
        setSnapshots(cleanSnaps);
      }
    } catch (err) {
      toast.error("Failed to load snapshots");
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!instanceName || !newSnapName) return;
    setCreating(true);
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const token = localStorage.getItem('axion_token');

      const res = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/snapshots`, {
        method: 'POST',
        headers: { 
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}` 
        },
        body: JSON.stringify({ name: newSnapName })
      });

      if (res.ok) {
        toast.info("Snapshot Creation Queued");
        onClose(); // Close drawer to let user see activity log
      } else {
        const err = await res.json();
        toast.error("Failed to create snapshot", { description: err.error });
      }
    } catch (err) {
      toast.error("Network Error");
    } finally {
      setCreating(false);
    }
  };

  const handleRestore = async (snapName: string) => {
    if (!instanceName) return;
    if (!window.confirm(`Are you sure you want to restore "${snapName}"? Current state will be lost.`)) return;

    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const token = localStorage.getItem('axion_token');

      const res = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/snapshots/${snapName}/restore`, {
        method: 'POST',
        headers: { 
            'Authorization': `Bearer ${token}` 
        }
      });

      if (res.ok) {
        toast.info("Restore Job Queued", { description: "Instance will stop if running." });
        onClose();
      } else {
        toast.error("Restore Request Failed");
      }
    } catch (err) {
      toast.error("Network Error");
    }
  };

  const handleDelete = async (snapName: string) => {
    if (!instanceName) return;
    if (!window.confirm(`Delete snapshot "${snapName}" permanently?`)) return;

    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const token = localStorage.getItem('axion_token');

      const res = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/snapshots/${snapName}`, {
        method: 'DELETE',
        headers: { 
            'Authorization': `Bearer ${token}` 
        }
      });

      if (res.ok) {
        toast.info("Deletion Queued");
        // Optimistic update
        setSnapshots(prev => prev.filter(s => s.name !== snapName));
      } else {
        toast.error("Delete Request Failed");
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
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-zinc-900 bg-zinc-950/50 backdrop-blur-md">
          <div>
            <h2 className="text-lg font-semibold text-zinc-100 flex items-center gap-2">
              <HardDrive size={18} className="text-indigo-500" />
              Backups & Snapshots
            </h2>
            <p className="text-xs text-zinc-500 mt-0.5">Manage restore points for {instanceName}</p>
          </div>
          <button onClick={onClose} className="p-2 text-zinc-500 hover:text-zinc-100 hover:bg-zinc-900 rounded-lg transition-colors">
            <X size={20} />
          </button>
        </div>

        {/* Create Section */}
        <div className="p-5 border-b border-zinc-900 bg-zinc-900/20">
            <div className="flex gap-2">
                <div className="flex-1">
                    <label className="block text-[10px] font-medium text-zinc-500 uppercase tracking-wider mb-1.5">New Snapshot Name</label>
                    <input 
                        type="text" 
                        value={newSnapName}
                        onChange={(e) => setNewSnapName(e.target.value)}
                        className="w-full bg-zinc-900 border border-zinc-800 text-zinc-200 rounded-md px-3 py-2 text-sm focus:outline-none focus:border-indigo-500/50"
                    />
                </div>
                <div className="flex items-end">
                    <button 
                        onClick={handleCreate}
                        disabled={creating || !newSnapName}
                        className="flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium rounded-md transition-colors shadow-lg shadow-indigo-900/20"
                    >
                        {creating ? <Loader2 size={16} className="animate-spin" /> : <Plus size={16} />}
                        Create
                    </button>
                </div>
            </div>
        </div>

        {/* List */}
        <div className="flex-1 overflow-y-auto p-5">
            {loading ? (
                <div className="flex justify-center py-10"><Loader2 className="animate-spin text-zinc-600" /></div>
            ) : snapshots.length === 0 ? (
                <div className="text-center py-10 text-zinc-600 border border-dashed border-zinc-800 rounded-lg">
                    <Camera size={24} className="mx-auto mb-2 opacity-20" />
                    <p className="text-sm">No snapshots available.</p>
                </div>
            ) : (
                <div className="space-y-3">
                    {snapshots.map(snap => (
                        <div key={snap.name} className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-4 flex items-center justify-between group hover:border-zinc-700 transition-colors">
                            <div>
                                <div className="flex items-center gap-2 mb-1">
                                    <h3 className="text-sm font-medium text-zinc-200">{snap.name}</h3>
                                    <span className="text-[10px] px-1.5 py-0.5 bg-zinc-800 rounded text-zinc-500">
                                        {formatBytes(0)} {/* LXD often doesn't return size easily via API here, placeholder */}
                                    </span>
                                </div>
                                <div className="flex items-center gap-1.5 text-xs text-zinc-500">
                                    <Clock size={12} />
                                    {format(new Date(snap.created_at), "MMM d, yyyy HH:mm")}
                                </div>
                            </div>
                            
                            <div className="flex items-center gap-1 opacity-60 group-hover:opacity-100 transition-opacity">
                                <button 
                                    onClick={() => handleRestore(snap.name)}
                                    className="p-2 text-zinc-400 hover:text-blue-400 hover:bg-blue-500/10 rounded-md transition-colors"
                                    title="Restore"
                                >
                                    <RotateCcw size={16} />
                                </button>
                                <button 
                                    onClick={() => handleDelete(snap.name)}
                                    className="p-2 text-zinc-400 hover:text-red-400 hover:bg-red-500/10 rounded-md transition-colors"
                                    title="Delete"
                                >
                                    <Trash2 size={16} />
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
      </div>
    </>
  );
}

// Dummy helper for size if needed, usually snap size is complex to get
const formatBytes = (bytes: number) => {
    // Placeholder logic
    return "Stateful"; 
};
