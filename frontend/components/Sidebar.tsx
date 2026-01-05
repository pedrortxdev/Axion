'use client';

import React from 'react';
import { LayoutDashboard, Box, Network, CalendarClock, Settings, LogOut, FileUp } from 'lucide-react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';

const Sidebar = () => {
  const pathname = usePathname();
  const router = useRouter();

  const [logoUrl, setLogoUrl] = React.useState<string | null>(null);
  const [hidePoweredBy, setHidePoweredBy] = React.useState(false);

  React.useEffect(() => {
    const token = localStorage.getItem('axion_token');
    if (!token) return;

    const protocol = window.location.protocol;
    const hostname = window.location.hostname;
    fetch(`${protocol}//${hostname}:8500/branding/settings`, {
      headers: { 'Authorization': `Bearer ${token}` }
    })
      .then(res => res.json())
      .then(data => {
        if (data.logo_url) setLogoUrl(data.logo_url);
        if (data.hide_powered_by !== undefined) setHidePoweredBy(data.hide_powered_by);
      })
      .catch(() => { });
  }, []);

  const menuItems = [
    {
      href: '/',
      label: 'Overview',
      icon: LayoutDashboard
    },
    {
      href: '/instances',
      label: 'Instances',
      icon: Box
    },
    {
      href: '/networks',
      label: 'Networks',
      icon: Network
    },
    {
      href: '/automation',
      label: 'Automation',
      icon: CalendarClock
    },
    {
      href: '/settings/branding',
      label: 'Settings',
      icon: Settings
    },
  ];

  const handleLogout = () => {
    localStorage.removeItem('axion_token');
    router.push('/login');
  };

  const getLogoSrc = () => {
    if (!logoUrl) return null;
    if (logoUrl.startsWith('http')) return logoUrl;
    const protocol = window.location.protocol;
    const hostname = window.location.hostname;
    return `${protocol}//${hostname}:8500${logoUrl}`;
  };

  const logoSrc = getLogoSrc();

  return (
    <aside className="w-64 h-screen bg-zinc-900 border-r border-zinc-800 flex flex-col">
      {/* Logo */}
      <div className="p-6 border-b border-zinc-800">
        <div className="flex items-center gap-3">
          {logoSrc ? (
            <img src={logoSrc} alt="Logo" className="h-8 object-contain" />
          ) : (
            <div className="h-8 w-8 bg-zinc-100 rounded-lg flex items-center justify-center text-zinc-950 font-bold shadow-lg shadow-zinc-100/10">
              A
            </div>
          )}
          <div>
            <h1 className="text-lg font-semibold text-zinc-100">
              {logoSrc ? '' : 'Axion'}
            </h1>
            {!logoSrc && <p className="text-xs text-zinc-500">Control Plane v1.0</p>}
          </div>
        </div>
      </div>

      {/* Menu */}
      <nav className="flex-1 py-6 px-4">
        <ul className="space-y-1">
          {menuItems.map((item) => {
            const Icon = item.icon;
            const isActive = pathname === item.href;

            return (
              <li key={item.href}>
                <Link
                  href={item.href}
                  className={`flex items-center gap-3 px-4 py-3 rounded-lg text-sm transition-colors ${isActive
                    ? 'bg-zinc-800/50 text-white'
                    : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/30'
                    }`}
                >
                  <Icon size={18} />
                  <span>{item.label}</span>
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>

      {/* Footer */}
      <div className="p-4 border-t border-zinc-800 space-y-2">
        {!hidePoweredBy && (
          <div className="text-center pb-2">
            <p className="text-[10px] text-zinc-600 uppercase tracking-wider font-medium">Powered by Axion</p>
          </div>
        )}
        <button
          onClick={handleLogout}
          className="flex items-center gap-3 w-full px-4 py-3 text-sm text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/30 rounded-lg transition-colors"
        >
          <LogOut size={18} />
          <span>Logout</span>
        </button>
      </div>
    </aside>
  );

};

export default Sidebar;