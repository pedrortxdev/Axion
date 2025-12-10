'use client';

import React, { useState, useEffect } from 'react';
import { X, RotateCw, Copy, Loader2 } from 'lucide-react';
import { toast } from 'sonner';

interface LogDrawerProps {
  instanceName: string | null;
  isOpen: boolean;
  onClose: () => void;
  token: string | null;
}

export default function LogDrawer({ instanceName, isOpen, onClose, token }: LogDrawerProps) {
  const [log, setLog] = useState<string>('');
  const [isLoading, setIsLoading] = useState<boolean>(true);

  const fetchLogs = async () => {
    if (!instanceName || !token) return;

    setIsLoading(true);
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      
      const response = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/logs`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to fetch logs');
      }

      const data = await response.json();
      setLog(data.log);
    } catch (error) {
      toast.error('Failed to load logs', {
        description: error instanceof Error ? error.message : 'Unknown error'
      });
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen && instanceName) {
      fetchLogs();
    }
  }, [isOpen, instanceName, token]);

  const copyToClipboard = () => {
    navigator.clipboard.writeText(log);
    toast.success('Logs copied to clipboard');
  };

  if (!isOpen || !instanceName) return null;

  return (
    <>
      <div
        className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 transition-opacity animate-in fade-in duration-200"
        onClick={onClose}
      />

      <div className="fixed right-0 top-0 h-full w-full max-w-2xl z-50 transform transition-transform duration-300 ease-in-out">
        <div className="h-full flex flex-col bg-zinc-950 border border-zinc-800 rounded-l-xl shadow-2xl pointer-events-auto animate-in slide-in-from-right duration-200">
          
          {/* Header */}
          <div className="flex items-center justify-between p-4 border-b border-zinc-900 flex-shrink-0">
            <div>
              <h2 className="text-lg font-semibold text-zinc-100 flex items-center gap-2">
                System Logs: {instanceName}
              </h2>
              <p className="text-xs text-zinc-500">Boot/Console logs for debugging</p>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={copyToClipboard}
                disabled={!log}
                className="p-2 text-zinc-500 hover:text-zinc-200 hover:bg-zinc-900 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                title="Copy logs to clipboard"
              >
                <Copy size={16} />
              </button>
              <button
                type="button"
                onClick={fetchLogs}
                disabled={isLoading}
                className="p-2 text-zinc-500 hover:text-zinc-200 hover:bg-zinc-900 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                title="Refresh logs"
              >
                {isLoading ? <Loader2 size={16} className="animate-spin" /> : <RotateCw size={16} />}
              </button>
              <button
                onClick={onClose}
                className="p-2 text-zinc-500 hover:text-zinc-200 hover:bg-zinc-900 rounded-lg transition-colors"
              >
                <X size={16} />
              </button>
            </div>
          </div>

          {/* Log Content */}
          <div className="flex-1 overflow-hidden p-2">
            {isLoading ? (
              <div className="h-full flex items-center justify-center">
                <div className="flex items-center gap-2 text-zinc-500">
                  <Loader2 size={16} className="animate-spin" />
                  Loading logs...
                </div>
              </div>
            ) : log ? (
              <div className="h-full w-full bg-black rounded-lg p-4 overflow-auto font-mono">
                <pre className="text-green-400 text-xs whitespace-pre-wrap break-words">
                  {log}
                </pre>
              </div>
            ) : (
              <div className="h-full w-full bg-black rounded-lg p-4 flex items-center justify-center text-zinc-600">
                No logs available for {instanceName}
              </div>
            )}
          </div>

        </div>
      </div>
    </>
  );
}