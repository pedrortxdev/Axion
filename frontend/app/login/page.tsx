'use client';

import React, { useState } from 'react';
import { useRouter } from 'next/navigation';
import { toast, Toaster } from 'sonner';
import { Lock, ArrowRight, Loader2 } from 'lucide-react';

export default function LoginPage() {
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const router = useRouter();

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);

    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const res = await fetch(`${protocol}//${host}:${port}/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      });

      if (res.ok) {
        const data = await res.json();
        localStorage.setItem('axion_token', data.token);
        toast.success('Authenticated');
        router.push('/');
      } else {
        toast.error('Invalid credentials');
      }
    } catch (error) {
      toast.error('Connection failed');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-zinc-950 flex flex-col items-center justify-center p-4">
      <Toaster position="top-center" theme="dark" />
      
      <div className="w-full max-w-md">
        <div className="text-center mb-10">
          <div className="h-12 w-12 bg-zinc-100 rounded-xl flex items-center justify-center mx-auto mb-6 shadow-2xl shadow-zinc-100/10">
            <Lock className="text-zinc-950" size={24} />
          </div>
          <h1 className="text-2xl font-bold text-zinc-100 tracking-tight">Axion Control Plane</h1>
          <p className="text-zinc-500 mt-2 text-sm">Secure access required</p>
        </div>

        <form onSubmit={handleLogin} className="space-y-4">
          <div className="relative">
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter access key..."
              className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-indigo-500/50 focus:border-indigo-500 transition-all text-center tracking-widest"
              autoFocus
            />
          </div>

          <button
            type="submit"
            disabled={isLoading || !password}
            className="w-full bg-indigo-600 hover:bg-indigo-500 disabled:bg-zinc-800 disabled:text-zinc-600 text-white font-medium rounded-lg px-4 py-3 transition-all flex items-center justify-center gap-2 group"
          >
            {isLoading ? (
              <Loader2 size={18} className="animate-spin" />
            ) : (
              <>
                <span>Authenticate</span>
                <ArrowRight size={16} className="group-hover:translate-x-0.5 transition-transform" />
              </>
            )}
          </button>
        </form>

        <p className="text-center text-xs text-zinc-700 mt-8">
          Authorized personnel only. System activity is monitored.
        </p>
      </div>
    </div>
  );
}
