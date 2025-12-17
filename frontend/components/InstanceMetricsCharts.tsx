'use client';

import React, { useState, useEffect } from 'react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts';
import { Loader2, ServerCrash } from 'lucide-react';
import { toast } from 'sonner';

interface Metric {
  timestamp: string;
  cpu_percent: number;
  memory_usage: number;
}

interface InstanceMetricsChartsProps {
  instanceName: string;
  token: string | null;
}

const formatBytes = (bytes: number) => {
  if (bytes === 0 || isNaN(bytes)) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  // Proteção para não estourar o array se o número for absurdamente grande ou pequeno
  const size = sizes[i] || sizes[sizes.length - 1]; 
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + size;
};

export default function InstanceMetricsCharts({ instanceName, token }: InstanceMetricsChartsProps) {
  const [metrics, setMetrics] = useState<Metric[]>([]);
  const [range, setRange] = useState('1h');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!instanceName || !token) return;

    const fetchMetrics = async () => {
      setLoading(true);
      setError(null);
      try {
        const protocol = window.location.protocol;
        const host = window.location.hostname;
        const port = '8500';
        const res = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/metrics/history?range=${range}`, {
          headers: { 'Authorization': `Bearer ${token}` }
        });

        if (!res.ok) {
          throw new Error('Failed to fetch metrics data');
        }

        const data = await res.json();
        setMetrics(data);
      } catch (err) {
        setError('Could not load metrics. Please try again later.');
        toast.error('Failed to load metrics history.');
        console.error(err);
      } finally {
        setLoading(false);
      }
    };

    fetchMetrics();
  }, [instanceName, range, token]);

  interface CustomTooltipProps {
    active?: boolean;
    payload?: Array<{ value: number; name: string; dataKey: string; }>;
    label?: string;
  }

  // CORREÇÃO AQUI: Tooltip agora é dinâmico baseado no dataKey
  const CustomTooltip = ({ active, payload, label }: CustomTooltipProps) => {
    if (active && payload && payload.length) {
      const data = payload[0];
      const isMemory = data.dataKey === 'memory_usage';
      const value = data.value;

      return (
        <div className="bg-zinc-900/80 backdrop-blur-sm border border-zinc-700 rounded-lg p-3 text-sm shadow-lg">
          <p className="label text-zinc-400 mb-1">{`${new Date(label!).toLocaleString()}`}</p>
          <p className={`font-medium ${isMemory ? 'text-indigo-400' : 'text-cyan-400'}`}>
            {isMemory ? 'Memory: ' : 'CPU: '}
            {isMemory ? formatBytes(value) : `${value !== undefined && value !== null ? value.toFixed(2) : '0.00'} %`}
          </p>
        </div>
      );
    }
    return null;
  };

  const renderChart = (title: string, dataKey: "memory_usage" | "cpu_percent", color: string) => (
    <div className="bg-zinc-900/50 p-4 rounded-lg border border-zinc-800" style={{ height: '300px' }}>
      <h4 className="text-sm font-medium text-zinc-300 mb-4">{title}</h4>
      {loading ? (
        <div className="flex items-center justify-center h-full text-zinc-500">
          <Loader2 size={24} className="animate-spin" />
        </div>
      ) : error ? (
          <div className="flex items-center justify-center h-full text-red-400/80">
               <ServerCrash size={24} /> <span className="ml-2">Error loading data</span>
          </div>
      ) : metrics.length > 0 ? (
        <div style={{ width: '100%', height: 300, minHeight: 300 }}>
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={metrics}>
              <defs>
                <linearGradient id={`gradient-${dataKey}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor={color} stopOpacity={0.3} />
                  <stop offset="95%" stopColor={color} stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" strokeOpacity={0.1} />
              <XAxis 
                dataKey="timestamp" 
                tickFormatter={(ts) => new Date(ts).toLocaleTimeString()}
                stroke="#9ca3af"
                fontSize={12}
                tickLine={false}
                axisLine={false}
                minTickGap={30}
              />
              <YAxis 
                tickFormatter={(value) => dataKey === 'memory_usage' ? formatBytes(value) : `${value.toFixed(0)}%`}
                stroke="#9ca3af"
                fontSize={12}
                tickLine={false}
                axisLine={false}
                width={60}
              />
              <Tooltip content={<CustomTooltip />} />
              <Area type="monotone" dataKey={dataKey} stroke={color} strokeWidth={2} fill={`url(#gradient-${dataKey})`} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <div className="flex items-center justify-center h-full text-zinc-500">
          Coletando dados...
        </div>
      )}
    </div>
  );
   
  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h3 className="text-lg font-semibold text-zinc-100">Performance Metrics</h3>
        <div className="bg-zinc-900/50 p-1 rounded-lg inline-flex border border-zinc-800/50">
          {['1h', '24h', '7d'].map((r) => (
            <button
              key={r}
              onClick={() => setRange(r)}
              className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${range === r ? 'bg-zinc-700 text-white' : 'text-zinc-400 hover:text-zinc-200'}`}
            >
              {r.toUpperCase()}
            </button>
          ))}
        </div>
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
         {renderChart("Memory Usage", "memory_usage", "#818cf8")}
         {renderChart("CPU Usage (%)", "cpu_percent", "#22d3ee")}
      </div>
    </div>
  );
}
