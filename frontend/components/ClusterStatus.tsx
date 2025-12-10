import React, { useState, useEffect } from 'react';

interface ClusterMember {
  name: string;
  status: string;
  address: string;
  roles: string[];
}

interface ClusterStatusProps {
  token?: string;
}

const ClusterStatus: React.FC<ClusterStatusProps> = ({ token }) => {
  const [members, setMembers] = useState<ClusterMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;

    const fetchClusterMembers = async () => {
      try {
        const response = await fetch('/cluster/members', {
          headers: {
            'Authorization': `Bearer ${token}`,
          },
        });

        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();
        setMembers(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch cluster members');
        console.error('Error fetching cluster members:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchClusterMembers();
  }, [token]);

  if (loading) {
    return (
      <div className="mb-6">
        <h2 className="text-xl font-semibold text-zinc-200 mb-3">Cluster Status</h2>
        <div className="flex justify-center items-center p-6 bg-zinc-900/30 border border-zinc-800 rounded-xl">
          <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-indigo-500"></div>
          <span className="ml-3 text-zinc-500">Loading cluster status...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="mb-6">
        <h2 className="text-xl font-semibold text-zinc-200 mb-3">Cluster Status</h2>
        <div className="p-4 bg-red-900/30 border border-red-700 rounded-xl">
          <p className="text-red-400">Error: {error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="mb-6">
      <h2 className="text-xl font-semibold text-zinc-200 mb-3">Cluster Status</h2>
      <div className="flex flex-wrap gap-3">
        {members.map((member) => (
          <div
            key={member.name}
            className="bg-zinc-900/30 backdrop-blur-sm border border-zinc-800 rounded-xl p-4 min-w-[180px] flex flex-col"
          >
            <div className="flex items-center justify-between mb-3">
              <h3 className="font-semibold text-zinc-200 truncate">{member.name}</h3>
              <div className={`w-3 h-3 rounded-full ${member.status.toLowerCase() === 'online' ? 'bg-emerald-500' : 'bg-red-500'}`}
                   title={member.status}>
              </div>
            </div>
            <div className="text-xs text-zinc-500 mb-3 truncate" title={member.address}>
              {member.address}
            </div>
            <div className="mt-auto pt-3 border-t border-zinc-800/50">
              <div className="flex flex-wrap gap-1">
                {member.roles && member.roles.length > 0 ? (
                  member.roles.map((role, index) => (
                    <span
                      key={index}
                      className="px-2 py-1 text-[10px] rounded bg-indigo-900/30 text-indigo-400 border border-indigo-700/50 uppercase tracking-wider font-medium"
                    >
                      {role}
                    </span>
                  ))
                ) : (
                  <span className="text-xs text-zinc-600 italic">No roles</span>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

export default ClusterStatus;