'use client';

import React, { useEffect, useState } from 'react';
import { CalendarClock, Trash2, Plus, X, Loader2, AlertTriangle, Clock } from 'lucide-react';
import { toast } from 'sonner';
import { Schedule, JobType } from '@/types';

interface AutomationDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  instanceName: string | null;
}

export default function AutomationDrawer({ isOpen, onClose, instanceName }: AutomationDrawerProps) {
  const [schedules, setSchedules] = useState<Schedule[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  
  // Form State
  const [frequency, setFrequency] = useState('@daily');
  const [customCron, setCustomCron] = useState('* * * * *');
  const [actionType, setActionType] = useState<JobType | 'restart'>('create_snapshot');

  const token = typeof window !== 'undefined' ? localStorage.getItem('axion_token') : null;

  useEffect(() => {
    if (isOpen && instanceName && token) {
      fetchSchedules();
    }
  }, [isOpen, instanceName, token]);

  const fetchSchedules = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const res = await fetch(`${protocol}//${host}:${port}/schedules`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (res.ok) {
        const data = await res.json();
        // Filter client-side as per plan
        const filtered = (data || []).filter((s: Schedule) => s.target === instanceName);
        setSchedules(filtered);
      }
    } catch (err) {
      toast.error('Failed to fetch schedules');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!token || !instanceName) return;

    setCreating(true);
    try {
      const cronExpr = frequency === 'custom' ? customCron : frequency;
      
      let type: JobType;
      let payloadObj: any = {};

      if (actionType === 'restart') {
        type = 'state_change';
        payloadObj = { action: 'restart' };
      } else {
        type = actionType as JobType; // create_snapshot
        if (type === 'create_snapshot') {
          payloadObj = { snapshot_name: `auto-${Date.now()}` };
        }
      }

      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      
      const res = await fetch(`${protocol}//${host}:${port}/schedules`, {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}` 
        },
        body: JSON.stringify({
          cron: cronExpr,
          type: type,
          target: instanceName,
          payload: JSON.stringify(payloadObj)
        })
      });

      if (res.ok) {
        toast.success('Schedule created');
        fetchSchedules();
        // Reset defaults
        setFrequency('@daily');
        setActionType('create_snapshot');
      } else {
        const err = await res.json();
        toast.error('Failed to create schedule', { description: err.error });
      }
    } catch (err) {
      toast.error('Network error');
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!token) return;
    if (!confirm('Are you sure you want to delete this schedule?')) return;

    try {
        const protocol = window.location.protocol;
        const host = window.location.hostname;
        const port = '8500';
        const res = await fetch(`${protocol}//${host}:${port}/schedules/${id}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        
        if (res.ok) {
            toast.success('Schedule deleted');
            setSchedules(prev => prev.filter(s => s.id !== id));
        } else {
            toast.error('Failed to delete');
        }
    } catch (err) {
        toast.error('Network error');
    }
  };

  const getActionLabel = (type: string, payload: string) => {
    if (type === 'state_change') {
        if (payload.includes('restart')) return 'Auto Restart';
        if (payload.includes('stop')) return 'Auto Stop';
        if (payload.includes('start')) return 'Auto Start';
    }
    if (type === 'create_snapshot') return 'Auto Backup';
    return type;
  };

  return (
    <>
      {/* Backdrop */}
      {isOpen && (
        <div 
          className="fixed inset-0 bg-black/60 backdrop-blur-sm z-40 transition-opacity"
          onClick={onClose}
        />
      )}

      {/* Drawer */}
      <div className={`fixed inset-y-0 right-0 w-full md:w-[480px] bg-zinc-950 border-l border-zinc-900 shadow-2xl z-50 transform transition-transform duration-300 ease-in-out ${isOpen ? 'translate-x-0' : 'translate-x-full'}`}>
        <div className="h-full flex flex-col">
          
          {/* Header */}
          <div className="flex items-center justify-between p-6 border-b border-zinc-900 bg-zinc-950/50 backdrop-blur-md">
            <div>
              <h2 className="text-xl font-semibold text-zinc-100 flex items-center gap-2">
                <CalendarClock className="text-indigo-500" />
                Automation
              </h2>
              <p className="text-sm text-zinc-500 mt-1">Manage scheduled tasks for <span className="text-indigo-400 font-mono">{instanceName}</span></p>
            </div>
            <button onClick={onClose} className="p-2 text-zinc-500 hover:text-zinc-300 hover:bg-zinc-900 rounded-lg transition-colors">
              <X size={20} />
            </button>
          </div>

          {/* Content */}
          <div className="flex-1 overflow-y-auto p-6 space-y-8">
            
            {/* Create New */}
            <div className="bg-zinc-900/50 border border-zinc-800 rounded-xl p-5 space-y-4">
                <div className="flex items-center gap-2 text-sm font-semibold text-zinc-300">
                    <Plus size={16} className="text-indigo-500"/> New Schedule
                </div>
                
                <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1.5">
                        <label className="text-xs font-medium text-zinc-500">Frequency</label>
                        <select 
                            value={frequency} 
                            onChange={(e) => setFrequency(e.target.value)}
                            className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-1 focus:ring-indigo-500/50"
                        >
                            <option value="@hourly">Hourly (@hourly)</option>
                            <option value="@daily">Daily (@daily)</option>
                            <option value="@weekly">Weekly (@weekly)</option>
                            <option value="custom">Custom Cron</option>
                        </select>
                    </div>

                    <div className="space-y-1.5">
                        <label className="text-xs font-medium text-zinc-500">Action</label>
                        <select 
                            value={actionType} 
                            onChange={(e) => setActionType(e.target.value as any)}
                            className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-1 focus:ring-indigo-500/50"
                        >
                            <option value="create_snapshot">Backup (Snapshot)</option>
                            <option value="restart">Restart Instance</option>
                        </select>
                    </div>
                </div>

                {frequency === 'custom' && (
                    <div className="space-y-1.5 animate-in fade-in slide-in-from-top-1">
                        <label className="text-xs font-medium text-zinc-500">Cron Expression</label>
                        <input 
                            type="text" 
                            value={customCron}
                            onChange={(e) => setCustomCron(e.target.value)}
                            placeholder="* * * * *"
                            className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-3 py-2 text-sm text-zinc-200 font-mono focus:outline-none focus:ring-1 focus:ring-indigo-500/50"
                        />
                        <p className="text-[10px] text-zinc-600">Format: minute hour dom month dow</p>
                    </div>
                )}

                <button 
                    onClick={handleCreate}
                    disabled={creating}
                    className="w-full flex items-center justify-center gap-2 bg-indigo-600 hover:bg-indigo-500 text-white py-2 rounded-md text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                    {creating ? <Loader2 size={16} className="animate-spin" /> : <Clock size={16} />}
                    Schedule Task
                </button>
            </div>

            {/* List */}
            <div className="space-y-3">
                <h3 className="text-sm font-semibold text-zinc-400 uppercase tracking-wider">Active Schedules</h3>
                
                {loading ? (
                    <div className="flex justify-center py-8"><Loader2 className="animate-spin text-zinc-700" /></div>
                ) : schedules.length === 0 ? (
                    <div className="text-center py-8 border border-dashed border-zinc-800 rounded-xl">
                        <CalendarClock className="mx-auto h-8 w-8 text-zinc-800 mb-2" />
                        <p className="text-zinc-500 text-sm">No active schedules found</p>
                    </div>
                ) : (
                    <div className="grid gap-3">
                        {schedules.map(sched => (
                            <div key={sched.id} className="group bg-zinc-900/30 border border-zinc-800 hover:border-zinc-700 rounded-lg p-4 flex items-center justify-between transition-all">
                                <div>
                                    <div className="flex items-center gap-2">
                                        <span className="text-sm font-medium text-zinc-200">{getActionLabel(sched.type, sched.payload)}</span>
                                        <span className="px-2 py-0.5 rounded-full bg-zinc-800 text-[10px] font-mono text-zinc-400">{sched.cron_expr}</span>
                                    </div>
                                    <p className="text-xs text-zinc-600 mt-1 font-mono">ID: {sched.id.substring(0,8)}</p>
                                </div>
                                <button 
                                    onClick={() => handleDelete(sched.id)}
                                    className="p-2 text-zinc-600 hover:text-red-400 hover:bg-red-500/10 rounded-md transition-colors opacity-0 group-hover:opacity-100"
                                >
                                    <Trash2 size={16} />
                                </button>
                            </div>
                        ))}
                    </div>
                )}
            </div>

          </div>
        </div>
      </div>
    </>
  );
}
