'use client';

import React, { useState, useEffect } from 'react';
import { toast } from 'sonner';
import { X, ShieldCheck, Save, ChevronDown } from 'lucide-react';
import { InstanceMetric } from '@/types';

interface BackupPolicyModalProps {
  isOpen: boolean;
  onClose: () => void;
  instance: InstanceMetric | null;
  token: string | null;
}

export default function BackupPolicyModal({ isOpen, onClose, instance, token }: BackupPolicyModalProps) {
  const [enabled, setEnabled] = useState(false);
  const [schedule, setSchedule] = useState('@daily');
  const [retention, setRetention] = useState(7);
  const [customSchedule, setCustomSchedule] = useState('');

  useEffect(() => {
    if (isOpen && instance) {
      const backupInfo = instance.backup_info;
      if (!backupInfo) return; // Add this check

      // Update all state together to avoid cascading renders
      setEnabled(backupInfo.enabled);
      setRetention(backupInfo.retention || 7);

      const isStandardSchedule = ['@daily', '@hourly', '@weekly'].includes(backupInfo.schedule);
      if (isStandardSchedule) {
        setSchedule(backupInfo.schedule);
        setCustomSchedule('');
      } else if (backupInfo.schedule) {
        setSchedule('custom');
        setCustomSchedule(backupInfo.schedule);
      } else {
        setSchedule('@daily');
        setCustomSchedule('');
      }
    }
  }, [isOpen, instance]);

  const handleSave = async () => {
    if (!instance || !token) return;

    const finalSchedule = schedule === 'custom' ? customSchedule : schedule;

    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const response = await fetch(`${protocol}//${host}:${port}/instances/${instance.name}/backups/config`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          enabled,
          schedule: finalSchedule,
          retention,
        }),
      });

      if (response.ok) {
        toast.success('Backup policy updated successfully!');
        onClose();
      } else {
        const error = await response.json();
        toast.error('Failed to update policy', { description: error.error });
      }
    } catch {
      toast.error('Network error while updating policy.');
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-40 flex justify-center items-center animate-in fade-in duration-200">
      <div className="relative bg-zinc-900/80 border border-zinc-800 rounded-2xl w-full max-w-md m-4 shadow-2xl shadow-black/40">
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-zinc-800">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-indigo-500/10 border border-indigo-500/20 rounded-lg text-indigo-400">
              <ShieldCheck size={20} />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-zinc-100">Automatic Backup Policy</h2>
              <p className="text-sm text-zinc-400">for {instance?.name}</p>
            </div>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-full text-zinc-500 hover:text-zinc-200 hover:bg-zinc-800 transition-colors">
            <X size={18} />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6">
          {/* Enable Toggle */}
          <div className="flex items-center justify-between p-4 bg-zinc-950/40 border border-zinc-800 rounded-lg">
            <label htmlFor="enable-backups" className="font-medium text-zinc-200">
              Enable Auto-Backups
            </label>
            <button
              id="enable-backups"
              onClick={() => setEnabled(!enabled)}
              className={`relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-zinc-900 ${
                enabled ? 'bg-indigo-600' : 'bg-zinc-700'
              }`}
            >
              <span
                className={`inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
                  enabled ? 'translate-x-5' : 'translate-x-0'
                }`}
              />
            </button>
          </div>

          {/* Frequency Select */}
          <div>
            <label htmlFor="frequency" className="block text-sm font-medium text-zinc-400 mb-2">
              Frequency
            </label>
            <div className="relative">
              <select
                id="frequency"
                value={schedule}
                onChange={(e) => setSchedule(e.target.value)}
                className="w-full bg-zinc-800/50 border border-zinc-700 rounded-md px-3 py-2.5 text-zinc-200 appearance-none focus:outline-none focus:ring-2 focus:ring-indigo-500 transition-colors"
                disabled={!enabled}
              >
                <option value="@daily">Daily (at midnight)</option>
                <option value="@hourly">Hourly</option>
                <option value="@weekly">Weekly</option>
                <option value="custom">Custom (Cron Expression)</option>
              </select>
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2 text-zinc-400">
                <ChevronDown size={20} />
              </div>
            </div>
            {schedule === 'custom' && (
              <input
                type="text"
                value={customSchedule}
                onChange={(e) => setCustomSchedule(e.target.value)}
                placeholder="e.g., 0 3 * * *"
                className="mt-2 w-full bg-zinc-950/60 border border-zinc-700 rounded-md px-3 py-2 text-zinc-200 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                disabled={!enabled}
              />
            )}
          </div>

          {/* Retention Policy */}
          <div>
            <label htmlFor="retention" className="block text-sm font-medium text-zinc-400 mb-2">
              Retention Policy
            </label>
            <div className="relative">
              <input
                id="retention"
                type="number"
                value={retention}
                onChange={(e) => setRetention(parseInt(e.target.value, 10) || 0)}
                min="1"
                className="w-full bg-zinc-800/50 border border-zinc-700 rounded-md pl-3 pr-16 py-2.5 text-zinc-200 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                disabled={!enabled}
              />
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-4 text-zinc-400 text-sm">
                backups
              </div>
            </div>
             <p className="text-xs text-zinc-500 mt-2">Keep the last X number of automatic backups.</p>
          </div>
        </div>

        {/* Footer */}
        <div className="p-5 bg-zinc-950/50 border-t border-zinc-800 flex justify-end gap-3 rounded-b-2xl">
          <button
            onClick={onClose}
            className="px-5 py-2 text-sm font-medium text-zinc-300 bg-zinc-800/50 hover:bg-zinc-700/50 border border-zinc-700 rounded-md transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            className="flex items-center gap-2 px-5 py-2 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-500 rounded-md transition-colors shadow-lg shadow-indigo-900/20 disabled:bg-zinc-700 disabled:text-zinc-400 disabled:cursor-not-allowed"
          >
            <Save size={16} />
            Save Policy
          </button>
        </div>
      </div>
    </div>
  );
}
