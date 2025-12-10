'use client';

import React, { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Sidebar from '@/components/Sidebar';

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();

  useEffect(() => {
    const token = localStorage.getItem('axion_token');
    if (!token) {
      router.push('/login');
    }
  }, [router]);

  // If not authenticated, we don't render anything (redirect will happen)
  // This is a simple approach; you might want to show a loading state instead
  const token = localStorage.getItem('axion_token');
  if (!token) {
    return null;
  }

  return (
    <div className="flex h-screen bg-zinc-950 text-white">
      <Sidebar />
      <main className="flex-1 overflow-auto p-8">
        {children}
      </main>
    </div>
  );
}