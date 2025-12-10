'use client';

import React, { useState, useEffect } from 'react';
import { X, Plus, Trash2, Loader2 } from 'lucide-react';
import { toast } from 'sonner';

interface Network {
  name: string;
  description: string;
  config: {
    'ipv4.address': string;
  };
  managed: boolean;
  type: string;
}

interface NetworkManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
  token: string | null;
}

export default function NetworkManagerModal({ isOpen, onClose, token }: NetworkManagerModalProps) {
  const [networks, setNetworks] = useState<Network[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isCreating, setIsCreating] = useState<boolean>(false);
  const [newNetworkName, setNewNetworkName] = useState<string>('');
  const [newNetworkDescription, setNewNetworkDescription] = useState<string>('');
  const [newNetworkSubnet, setNewNetworkSubnet] = useState<string>('10.100.0.1/24');

  const fetchNetworks = async () => {
    if (!token) return;

    setIsLoading(true);
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      
      const response = await fetch(`${protocol}//${host}:${port}/networks`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to fetch networks');
      }

      const data = await response.json();
      setNetworks(data);
    } catch (error) {
      toast.error('Failed to load networks', {
        description: error instanceof Error ? error.message : 'Unknown error'
      });
    } finally {
      setIsLoading(false);
    }
  };

  const createNetwork = async () => {
    if (!token) return;
    if (!newNetworkName || !newNetworkSubnet) {
      toast.error('Network name and subnet are required');
      return;
    }

    setIsCreating(true);
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      
      const response = await fetch(`${protocol}//${host}:${port}/networks`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          name: newNetworkName,
          description: newNetworkDescription,
          subnet: newNetworkSubnet
        }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to create network');
      }

      toast.success('Network created successfully');
      setNewNetworkName('');
      setNewNetworkDescription('');
      setNewNetworkSubnet('10.100.0.1/24');
      fetchNetworks(); // Refresh the list
    } catch (error) {
      toast.error('Failed to create network', {
        description: error instanceof Error ? error.message : 'Unknown error'
      });
    } finally {
      setIsCreating(false);
    }
  };

  const deleteNetwork = async (name: string) => {
    if (!token) return;
    if (!window.confirm(`Are you sure you want to delete network "${name}"? This action cannot be undone.`)) return;

    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      
      const response = await fetch(`${protocol}//${host}:${port}/networks/${name}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${token}`
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to delete network');
      }

      toast.success('Network deleted successfully');
      fetchNetworks(); // Refresh the list
    } catch (error) {
      toast.error('Failed to delete network', {
        description: error instanceof Error ? error.message : 'Unknown error'
      });
    }
  };

  useEffect(() => {
    if (isOpen) {
      fetchNetworks();
    }
  }, [isOpen, token]);

  if (!isOpen) return null;

  return (
    <>
      <div
        className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 transition-opacity animate-in fade-in duration-200"
        onClick={onClose}
      />

      <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none">
        <div className="bg-zinc-950 border border-zinc-800 rounded-xl w-full max-w-3xl shadow-2xl pointer-events-auto animate-in zoom-in-95 duration-200 flex flex-col max-h-[90vh]">

          {/* Header */}
          <div className="flex items-center justify-between p-6 border-b border-zinc-900 flex-shrink-0">
            <div>
              <h2 className="text-xl font-semibold text-zinc-100 flex items-center gap-2">
                Network Manager
              </h2>
              <p className="text-sm text-zinc-500 mt-1">Manage bridge networks for instances</p>
            </div>
            <button
              onClick={onClose}
              className="p-2 text-zinc-500 hover:text-zinc-200 hover:bg-zinc-900 rounded-lg transition-colors"
            >
              <X size={20} />
            </button>
          </div>

          {/* Content */}
          <div className="flex-1 overflow-y-auto p-6 space-y-6">
            
            {/* Create Form */}
            <div className="bg-zinc-900/30 p-4 rounded-xl border border-zinc-800/50">
              <h3 className="text-sm font-medium text-zinc-300 mb-3 flex items-center gap-2">
                <Plus size={16} className="text-zinc-500" /> Create Network
              </h3>
              
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div>
                  <label className="block text-xs font-medium text-zinc-400 mb-1 uppercase tracking-wider">Name</label>
                  <input
                    type="text"
                    value={newNetworkName}
                    onChange={(e) => setNewNetworkName(e.target.value)}
                    placeholder="e.g. dev-net"
                    className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all"
                  />
                </div>
                
                <div>
                  <label className="block text-xs font-medium text-zinc-400 mb-1 uppercase tracking-wider">Subnet</label>
                  <input
                    type="text"
                    value={newNetworkSubnet}
                    onChange={(e) => setNewNetworkSubnet(e.target.value)}
                    placeholder="10.x.x.1/24"
                    className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all"
                  />
                </div>
                
                <div className="md:col-span-3">
                  <label className="block text-xs font-medium text-zinc-400 mb-1 uppercase tracking-wider">Description (Optional)</label>
                  <input
                    type="text"
                    value={newNetworkDescription}
                    onChange={(e) => setNewNetworkDescription(e.target.value)}
                    placeholder="Description of the network"
                    className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all"
                  />
                </div>
              </div>
              
              <div className="mt-4 flex justify-end">
                <button
                  type="button"
                  onClick={createNetwork}
                  disabled={isCreating || !newNetworkName || !newNetworkSubnet}
                  className="px-4 py-2 text-sm font-medium bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg shadow-lg shadow-indigo-500/20 transition-all flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {isCreating && <Loader2 size={16} className="animate-spin" />}
                  {isCreating ? 'Creating...' : 'Create Bridge'}
                </button>
              </div>
            </div>

            {/* Networks List */}
            <div>
              <h3 className="text-sm font-medium text-zinc-300 mb-3">Existing Networks</h3>
              
              {isLoading ? (
                <div className="flex items-center justify-center py-8">
                  <div className="flex items-center gap-2 text-zinc-500">
                    <Loader2 size={16} className="animate-spin" />
                    Loading networks...
                  </div>
                </div>
              ) : networks.length === 0 ? (
                <div className="text-center py-6 text-zinc-600">
                  No networks found. Create one above.
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm text-left text-zinc-400">
                    <thead className="text-xs text-zinc-300 uppercase bg-zinc-900/50 border-b border-zinc-800">
                      <tr>
                        <th className="px-4 py-3">Name</th>
                        <th className="px-4 py-3">Subnet</th>
                        <th className="px-4 py-3">Description</th>
                        <th className="px-4 py-3">Actions</th>
                      </tr>
                    </thead>
                    <tbody className="border-b border-zinc-800">
                      {networks.map((network) => (
                        <tr key={network.name} className="bg-transparent border-b border-zinc-800/50 hover:bg-zinc-900/20">
                          <td className="px-4 py-3 font-medium text-zinc-200">{network.name}</td>
                          <td className="px-4 py-3">
                            <span className="px-2 py-1 bg-zinc-800/50 text-zinc-300 rounded text-xs font-mono">
                              {network.config['ipv4.address']}
                            </span>
                          </td>
                          <td className="px-4 py-3 text-zinc-400">{network.description || '-'}</td>
                          <td className="px-4 py-3">
                            <button
                              type="button"
                              onClick={() => deleteNetwork(network.name)}
                              className="p-2 text-red-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-colors"
                              title="Delete network"
                            >
                              <Trash2 size={16} />
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
            
          </div>

          {/* Footer Actions */}
          <div className="p-6 border-t border-zinc-900 bg-zinc-950 flex-shrink-0 flex items-center justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-zinc-400 hover:text-zinc-200 hover:bg-zinc-900 rounded-lg transition-colors"
            >
              Close
            </button>
          </div>

        </div>
      </div>
    </>
  );
}