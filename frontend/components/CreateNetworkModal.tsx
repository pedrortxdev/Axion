'use client';

import React, { useState } from 'react';
import { X, Network, Flag, ShieldCheck, Globe, Router } from 'lucide-react';
import { toast } from 'sonner';

interface CreateNetworkModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSuccess: () => void;
}

export default function CreateNetworkModal({ isOpen, onClose, onSuccess }: CreateNetworkModalProps) {
    const [name, setName] = useState('');
    const [cidr, setCidr] = useState('');
    const [gateway, setGateway] = useState('');
    const [isPublic, setIsPublic] = useState(false);
    const [isLoading, setIsLoading] = useState(false);

    if (!isOpen) return null;

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsLoading(true);

        try {
            const token = localStorage.getItem('axion_token');
            if (!token) return;

            const protocol = window.location.protocol;
            const host = window.location.hostname;
            const port = '8500';

            const response = await fetch(`${protocol}//${host}:${port}/api/v1/networks`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({
                    name,
                    cidr,
                    gateway,
                    is_public: isPublic
                }),
            });

            if (response.ok) {
                toast.success('Network Pool Created');
                onSuccess();
                onClose();
                // Reset form
                setName('');
                setCidr('');
                setGateway('');
                setIsPublic(false);
            } else {
                const err = await response.json();
                toast.error('Creation Failed', { description: err.error });
            }
        } catch (error) {
            console.error(error);
            toast.error('Network Error');
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <>
            <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 animate-in fade-in duration-200" onClick={onClose} />
            <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none">
                <div className="bg-zinc-950 border border-zinc-800 rounded-xl w-full max-w-lg shadow-2xl pointer-events-auto animate-in zoom-in-95 duration-200 flex flex-col">

                    {/* Header */}
                    <div className="flex items-center justify-between p-6 border-b border-zinc-900">
                        <div>
                            <h2 className="text-xl font-semibold text-zinc-100 flex items-center gap-2">
                                <Network className="text-indigo-500" size={20} />
                                Create Network Pool
                            </h2>
                            <p className="text-sm text-zinc-500 mt-1">Add a new subnet for IPAM management.</p>
                        </div>
                        <button onClick={onClose} className="p-2 text-zinc-500 hover:text-zinc-200 hover:bg-zinc-900 rounded-lg transition-colors">
                            <X size={20} />
                        </button>
                    </div>

                    <form onSubmit={handleSubmit} className="p-6 space-y-6">
                        <div>
                            <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">Pool Name</label>
                            <input
                                type="text"
                                required
                                value={name}
                                onChange={e => setName(e.target.value)}
                                placeholder="e.g. Corporate VLAN 10"
                                className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all placeholder:text-zinc-600"
                            />
                        </div>

                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">CIDR Block</label>
                                <div className="relative">
                                    <input
                                        type="text"
                                        required
                                        value={cidr}
                                        onChange={e => setCidr(e.target.value)}
                                        placeholder="10.0.0.0/24"
                                        className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all placeholder:text-zinc-600 pl-10"
                                    />
                                    <Flag size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
                                </div>
                            </div>

                            <div>
                                <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">Gateway IP</label>
                                <div className="relative">
                                    <input
                                        type="text"
                                        required
                                        value={gateway}
                                        onChange={e => setGateway(e.target.value)}
                                        placeholder="10.0.0.1"
                                        className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all placeholder:text-zinc-600 pl-10"
                                    />
                                    <Router size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
                                </div>
                            </div>
                        </div>

                        {/* Public/Private Toggle */}
                        <div className="bg-zinc-900/30 border border-zinc-800 rounded-lg p-1 flex">
                            <button
                                type="button"
                                onClick={() => setIsPublic(false)}
                                className={`flex-1 flex items-center justify-center gap-2 py-2 rounded-md text-sm font-medium transition-all ${!isPublic ? 'bg-zinc-800 text-zinc-100 shadow-sm' : 'text-zinc-500 hover:text-zinc-300'
                                    }`}
                            >
                                <ShieldCheck size={16} />
                                Private (NAT)
                            </button>
                            <button
                                type="button"
                                onClick={() => setIsPublic(true)}
                                className={`flex-1 flex items-center justify-center gap-2 py-2 rounded-md text-sm font-medium transition-all ${isPublic ? 'bg-indigo-600/10 text-indigo-400 border border-indigo-500/20 shadow-sm' : 'text-zinc-500 hover:text-zinc-300'
                                    }`}
                            >
                                <Globe size={16} />
                                Public (Bridge)
                            </button>
                        </div>

                        <button
                            type="submit"
                            disabled={isLoading}
                            className="w-full bg-indigo-600 hover:bg-indigo-500 text-white font-medium py-2.5 rounded-lg transition-colors flex justify-center items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            {isLoading ? 'Creating Pool...' : 'Create Network Pool'}
                        </button>
                    </form>

                </div>
            </div>
        </>
    );
}
