'use client';

import React from 'react';
import { X, CheckCircle2, AlertCircle, Clock, Loader2, Play, Settings2, Terminal } from 'lucide-react';
import { Job, JobStatus } from '@/types';
import { formatDistanceToNow } from 'date-fns';

interface ActivityDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  jobs: Job[];
}

export default function ActivityDrawer({ isOpen, onClose, jobs }: ActivityDrawerProps) {
  return (
    <>
      {/* Backdrop */}
      <div 
        className={`fixed inset-0 bg-black/60 backdrop-blur-[2px] z-40 transition-opacity duration-300 ${
          isOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'
        }`}
        onClick={onClose}
      />

      {/* Drawer Panel */}
      <div 
        className={`
          fixed top-0 right-0 h-full w-full sm:w-[400px] bg-zinc-950 border-l border-zinc-800 shadow-2xl z-50 transform transition-transform duration-300 ease-out
          ${isOpen ? 'translate-x-0' : 'translate-x-full'}
        `}
      >
        <div className="flex flex-col h-full">
          {/* Header */}
          <div className="flex items-center justify-between p-5 border-b border-zinc-900 bg-zinc-950/50 backdrop-blur-md">
            <div>
              <h2 className="text-lg font-semibold text-zinc-100 flex items-center gap-2">
                <Terminal size={18} className="text-zinc-500" />
                Activity Log
              </h2>
              <p className="text-xs text-zinc-500 mt-0.5">Real-time operations history</p>
            </div>
            <button 
              onClick={onClose}
              className="p-2 text-zinc-500 hover:text-zinc-100 hover:bg-zinc-900 rounded-lg transition-colors"
            >
              <X size={20} />
            </button>
          </div>

          {/* List */}
          <div className="flex-1 overflow-y-auto p-4 space-y-3">
            {jobs.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-40 text-zinc-600 space-y-2">
                <Clock size={24} className="opacity-20" />
                <p className="text-sm">No activity recorded yet.</p>
              </div>
            ) : (
              jobs.map((job) => (
                <JobCard key={job.id} job={job} />
              ))
            )}
          </div>
        </div>
      </div>
    </>
  );
}

function JobCard({ job }: { job: Job }) {
  const getStatusIcon = (status: JobStatus) => {
    switch (status) {
      case 'COMPLETED': return <CheckCircle2 size={16} className="text-emerald-500" />;
      case 'FAILED': return <AlertCircle size={16} className="text-red-500" />;
      case 'IN_PROGRESS': return <Loader2 size={16} className="text-indigo-500 animate-spin" />;
      default: return <Clock size={16} className="text-zinc-600" />;
    }
  };

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'state_change': return <Play size={14} />;
      case 'update_limits': return <Settings2 size={14} />;
      default: return <Terminal size={14} />;
    }
  };

  const getStatusColor = (status: JobStatus) => {
    switch (status) {
      case 'COMPLETED': return 'border-emerald-500/20 bg-emerald-500/5';
      case 'FAILED': return 'border-red-500/20 bg-red-500/5';
      case 'IN_PROGRESS': return 'border-indigo-500/20 bg-indigo-500/5';
      default: return 'border-zinc-800 bg-zinc-900/50';
    }
  };

  return (
    <div className={`border rounded-lg p-3.5 transition-all ${getStatusColor(job.status)}`}>
      <div className="flex items-start justify-between mb-2">
        <div className="flex items-center gap-2">
          {getStatusIcon(job.status)}
          <span className={`text-[10px] font-bold uppercase tracking-wider ${
            job.status === 'COMPLETED' ? 'text-emerald-500' :
            job.status === 'FAILED' ? 'text-red-500' :
            job.status === 'IN_PROGRESS' ? 'text-indigo-400' :
            'text-zinc-500'
          }`}>
            {job.status.replace('_', ' ')}
          </span>
        </div>
        <span className="text-[10px] text-zinc-500 tabular-nums">
          {job.created_at ? formatDistanceToNow(new Date(job.created_at), { addSuffix: true }) : 'Just now'}
        </span>
      </div>
      
      <div className="flex items-center gap-2 mb-1.5">
        <span className="p-1 bg-zinc-900 border border-zinc-800 rounded text-zinc-400">
            {getTypeIcon(job.type)}
        </span>
        <h3 className="text-sm font-medium text-zinc-200">
            {job.target}
        </h3>
      </div>

      <div className="flex justify-between items-end">
        <p className="text-[10px] text-zinc-600 font-mono truncate max-w-[120px]">
          ID: {job.id.split('-')[0]}...
        </p>
        {job.attempt_count > 0 && job.status !== 'COMPLETED' && (
           <span className="text-[10px] text-orange-500/80 font-medium">
             Attempt {job.attempt_count}
           </span>
        )}
      </div>
      
      {job.error && (
        <div className="mt-3 p-2 bg-red-950/20 border border-red-900/30 rounded text-[11px] text-red-400 leading-tight">
          {job.error}
        </div>
      )}
    </div>
  );
}