'use client';

import React from 'react';
import ClusterStatus from '@/components/ClusterStatus';
import HostStatsCard from '@/components/HostStatsCard';
import { useState, useEffect } from 'react';

export default function OverviewPage() {
  const [stats, setStats] = useState<any>(null);

  return (
    <div className="max-w-7xl mx-auto">
      <header className="mb-8">
        <h1 className="text-2xl font-bold text-zinc-100">Dashboard Overview</h1>
        <p className="text-zinc-500">Monitor your infrastructure at a glance</p>
      </header>

      <div className="space-y-8">
        <HostStatsCard data={stats} />
        <ClusterStatus />
      </div>
    </div>
  );
}