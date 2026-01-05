'use client';

import React, { useState } from 'react';
import { useRouter } from 'next/navigation';
import { toast, Toaster } from 'sonner';
import { Lock, ArrowRight, Loader2 } from 'lucide-react';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isRegistering, setIsRegistering] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const router = useRouter();

  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !password) {
      toast.error('Please fill in all fields');
      return;
    }

    setIsLoading(true);

    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500'; // Make sure this matches your backend port
      const endpoint = isRegistering ? '/api/v1/register' : '/api/v1/login';

      const res = await fetch(`${protocol}//${host}:${port}${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      const data = await res.json();

      if (res.ok) {
        if (isRegistering) {
          toast.success('Registration successful! Please login.');
          setIsRegistering(false);
          setPassword(''); // Clear password for safety
        } else {
          localStorage.setItem('axion_token', data.access_token);
          // Optional: Store user info if returned
          // localStorage.setItem('axion_user', JSON.stringify(data.user)); 
          toast.success('Authenticated');
          router.push('/');
        }
      } else {
        toast.error(data.error || 'Authentication failed');
      }
    } catch (err) {
      console.error(err);
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
          <h1 className="text-2xl font-bold text-zinc-100 tracking-tight">{isRegistering ? 'Create Admin Account' : 'Axion Control Plane'}</h1>
          <p className="text-zinc-500 mt-2 text-sm">{isRegistering ? 'Register the initial administrator' : 'Secure access required'}</p>
        </div>

        <form onSubmit={handleAuth} className="space-y-4">
          <div className="space-y-4">
            <div className="relative">
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="admin@admin"
                className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-indigo-500/50 focus:border-indigo-500 transition-all"
                autoFocus
                required
              />
            </div>
            <div className="relative">
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-indigo-500/50 focus:border-indigo-500 transition-all font-mono"
                required
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={isLoading || !email || !password}
            className="w-full bg-indigo-600 hover:bg-indigo-500 disabled:bg-zinc-800 disabled:text-zinc-600 text-white font-medium rounded-lg px-4 py-3 transition-all flex items-center justify-center gap-2 group"
          >
            {isLoading ? (
              <Loader2 size={18} className="animate-spin" />
            ) : (
              <>
                <span>{isRegistering ? 'Create Account' : 'Authenticate'}</span>
                <ArrowRight size={16} className="group-hover:translate-x-0.5 transition-transform" />
              </>
            )}
          </button>
        </form>

        <div className="mt-6 text-center">
          <button
            onClick={() => setIsRegistering(!isRegistering)}
            className="text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            {isRegistering ? "Already have an account? Login" : "First time? Create admin account"}
          </button>
        </div>

        <p className="text-center text-xs text-zinc-700 mt-8">
          Authorized personnel only. System activity is monitored.
        </p>
      </div>
    </div>
  );
}
