'use client';

import React from 'react';
import { ShieldCheck, ShieldAlert, Clock, Calendar, Settings } from 'lucide-react';
import { formatDistanceToNow, format } from 'date-fns';
import { InstanceMetric } from '@/types';

interface BackupHealthCardProps {
  instance: InstanceMetric;
  onConfigureBackup: () => void;
}

const BackupHealthCard: React.FC<BackupHealthCardProps> = ({ 
  instance, 
  onConfigureBackup 
}) => {
  const { backup_info } = instance;

  // If backup is not enabled, show not protected state
  if (!backup_info?.enabled) {
    return (
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
        <div className="flex items-center gap-2 mb-4">
          <ShieldAlert className="text-zinc-500" size={20} />
          <h3 className="font-semibold text-zinc-100">Data Protection</h3>
        </div>
        <div className="text-center py-6">
          <p className="text-zinc-400 mb-4">This instance is not protected by automatic backups.</p>
          <button
            onClick={onConfigureBackup}
            className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-md flex items-center gap-2 mx-auto transition-colors"
          >
            <Settings size={16} />
            Configure Backup
          </button>
        </div>
      </div>
    );
  }

  // Calculate status badge color based on last_status
  const getStatusBadgeClass = () => {
    if (!backup_info.last_status) {
      return 'bg-zinc-700 text-zinc-300'; // Not run yet
    }

    switch (backup_info.last_status.toLowerCase()) {
      case 'completed':
        return 'bg-emerald-500/20 text-emerald-400';
      case 'failed':
        return 'bg-red-500/20 text-red-400';
      case 'in_progress':
        return 'bg-blue-500/20 text-blue-400';
      case 'canceled':
        return 'bg-yellow-500/20 text-yellow-400';
      default:
        return 'bg-zinc-700 text-zinc-300';
    }
  };

  // Format schedule display
  const formatSchedule = (schedule: string) => {
    if (schedule === '@daily') return 'Daily (@daily)';
    if (schedule === '@weekly') return 'Weekly (@weekly)';
    if (schedule === '@monthly') return 'Monthly (@monthly)';
    return schedule; // Show raw cron string if not a standard alias
  };

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <div className="flex items-center gap-2 mb-4">
        <ShieldCheck className="text-emerald-400" size={20} />
        <h3 className="font-semibold text-zinc-100">Data Protection</h3>
      </div>
      
      <div className="space-y-3">
        {/* Status */}
        <div className="flex items-center justify-between">
          <span className="text-sm text-zinc-400">Status</span>
          <span className={`px-2 py-1 rounded-full text-xs font-medium ${getStatusBadgeClass()}`}>
            {backup_info.last_status || 'Never Run'}
          </span>
        </div>

        {/* Schedule */}
        <div className="flex items-center justify-between">
          <span className="text-sm text-zinc-400">Schedule</span>
          <span className="text-sm text-zinc-300">{formatSchedule(backup_info.schedule)}</span>
        </div>

        {/* Last Backup */}
        {backup_info.last_run && (
          <div className="flex items-center justify-between">
            <span className="text-sm text-zinc-400">Last Backup</span>
            <span className="text-sm text-zinc-300 flex items-center gap-1">
              <Clock size={12} />
              {formatDistanceToNow(new Date(backup_info.last_run), { addSuffix: true })}
            </span>
          </div>
        )}

        {/* Next Backup */}
        {backup_info.next_run && (
          <div className="flex items-center justify-between">
            <span className="text-sm text-zinc-400">Next Backup</span>
            <span className="text-sm text-zinc-300 flex items-center gap-1">
              <Calendar size={12} />
              in {formatDistanceToNow(new Date(backup_info.next_run))}
            </span>
          </div>
        )}
      </div>
    </div>
  );
};

export default BackupHealthCard;