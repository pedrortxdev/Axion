'use client';

import React, { useState, useEffect } from 'react';
import { X, Server, Cpu, Zap, Box, Loader2, Code, FileCode } from 'lucide-react';
import { toast } from 'sonner';

interface CreateInstanceModalProps {
  isOpen: boolean;
  onClose: () => void;
  token: string | null;
}

const IMAGES = [
  { value: 'ubuntu/22.04', label: 'Ubuntu 22.04 LTS' },
  { value: 'debian/12', label: 'Debian 12 Bookworm' },
  { value: 'alpine/3.18', label: 'Alpine 3.18' },
];

const TEMPLATES = [
  {
    id: 'none',
    label: 'Empty OS',
    description: 'Clean installation with no extra packages.',
    yaml: ''
  },
  {
    id: 'docker',
    label: 'Docker Host',
    description: 'Ready-to-use Docker environment.',
    yaml: `#cloud-config
package_update: true
packages:
  - docker.io
  - docker-compose
runcmd:
  - systemctl enable --now docker
  - usermod -aG docker ubuntu
`
  },
  {
    id: 'web',
    label: 'Web Server (Nginx)',
    description: 'Nginx installed and configured.',
    yaml: `#cloud-config
package_update: true
packages:
  - nginx
  - curl
write_files:
  - path: /var/www/html/index.html
    content: |
      <h1>Welcome to Axion Cloud</h1>
      <p>Deployed via Cloud-Init</p>
runcmd:
  - systemctl enable --now nginx
`
  },
  {
    id: 'dev',
    label: 'Dev Environment',
    description: 'Git, Zsh, Htop, Build Essentials.',
    yaml: `#cloud-config
package_update: true
packages:
  - git
  - zsh
  - htop
  - curl
  - build-essential
  - vim
`
  }
];

export default function CreateInstanceModal({ isOpen, onClose, token }: CreateInstanceModalProps) {
  const [name, setName] = useState('');
  const [image, setImage] = useState(IMAGES[0].value);
  const [cpu, setCpu] = useState(1);
  const [memory, setMemory] = useState(512);
  const [templateId, setTemplateId] = useState('none');
  const [userData, setUserData] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [isLoading, setIsLoading] = useState(false);

  // Update user data when template changes
  useEffect(() => {
    const tmpl = TEMPLATES.find(t => t.id === templateId);
    if (tmpl) {
      setUserData(tmpl.yaml);
    }
  }, [templateId]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token) return;

    if (!name.trim() || name.includes(' ')) {
      toast.error('Invalid Name', { description: 'Name cannot be empty or contain spaces.' });
      return;
    }

    setIsLoading(true);

    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const payload = {
        name: name.trim(),
        image: image,
        limits: {
          "limits.cpu": cpu.toString(),
          "limits.memory": `${memory}MB`
        },
        user_data: userData // Send cloud-init
      };

      const response = await fetch(`${protocol}//${host}:${port}/instances`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(payload),
      });

      if (response.ok) {
        toast.success('Provisioning Started', { description: `Creating ${name} with ${TEMPLATES.find(t => t.id === templateId)?.label}...` });
        onClose();
        // Reset form
        setName('');
        setMemory(512);
        setCpu(1);
        setTemplateId('none');
        setUserData('');
        setShowAdvanced(false);
      } else {
        const error = await response.json();
        toast.error('Creation Failed', { description: error.error || 'Unknown error' });
      }
    } catch (error) {
      toast.error('Network Error', { description: 'Could not reach control plane.' });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <>
      <div 
        className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 transition-opacity animate-in fade-in duration-200"
        onClick={onClose}
      />

      <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none">
        <div className="bg-zinc-950 border border-zinc-800 rounded-xl w-full max-w-2xl shadow-2xl pointer-events-auto animate-in zoom-in-95 duration-200 flex flex-col max-h-[90vh]">
          
          {/* Header */}
          <div className="flex items-center justify-between p-6 border-b border-zinc-900 flex-shrink-0">
            <div>
              <h2 className="text-xl font-semibold text-zinc-100 flex items-center gap-2">
                <Box className="text-indigo-500" size={20} />
                New Instance Wizard
              </h2>
              <p className="text-sm text-zinc-500 mt-1">Configure container resources and initialization.</p>
            </div>
            <button 
              onClick={onClose}
              className="p-2 text-zinc-500 hover:text-zinc-200 hover:bg-zinc-900 rounded-lg transition-colors"
            >
              <X size={20} />
            </button>
          </div>

          {/* Scrollable Content */}
          <div className="flex-1 overflow-y-auto p-6 space-y-8">
            
            {/* Basics */}
            <div className="grid grid-cols-2 gap-6">
              <div>
                <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">Name</label>
                <input 
                  type="text" 
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. app-server-01"
                  className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all placeholder:text-zinc-600"
                  autoFocus
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">OS Image</label>
                <div className="relative">
                  <select 
                    value={image}
                    onChange={(e) => setImage(e.target.value)}
                    className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2.5 appearance-none focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all cursor-pointer"
                  >
                    {IMAGES.map(img => (
                      <option key={img.value} value={img.value}>{img.label}</option>
                    ))}
                  </select>
                  <div className="absolute right-4 top-1/2 -translate-y-1/2 pointer-events-none text-zinc-500">
                    <Server size={16} />
                  </div>
                </div>
              </div>
            </div>

            {/* Resources */}
            <div className="space-y-6">
              <h3 className="text-sm font-medium text-zinc-300 flex items-center gap-2">
                <Zap size={16} className="text-zinc-500" /> Resources
              </h3>
              
              <div className="bg-zinc-900/30 p-4 rounded-xl border border-zinc-800/50 space-y-5">
                {/* CPU */}
                <div>
                    <div className="flex justify-between items-center mb-2">
                    <span className="text-xs text-zinc-400">vCPU Allocation</span>
                    <span className="text-xs font-mono text-indigo-400 bg-indigo-500/10 px-2 py-0.5 rounded border border-indigo-500/20">{cpu} Cores</span>
                    </div>
                    <input type="range" min="1" max="4" step="1" value={cpu} onChange={(e) => setCpu(parseInt(e.target.value))} className="w-full h-1.5 bg-zinc-800 rounded-lg appearance-none cursor-pointer accent-indigo-500 hover:accent-indigo-400" />
                </div>

                {/* RAM */}
                <div>
                    <div className="flex justify-between items-center mb-2">
                    <span className="text-xs text-zinc-400">Memory Limit</span>
                    <span className="text-xs font-mono text-indigo-400 bg-indigo-500/10 px-2 py-0.5 rounded border border-indigo-500/20">{memory} MB</span>
                    </div>
                    <input type="range" min="128" max="4096" step="128" value={memory} onChange={(e) => setMemory(parseInt(e.target.value))} className="w-full h-1.5 bg-zinc-800 rounded-lg appearance-none cursor-pointer accent-indigo-500 hover:accent-indigo-400" />
                </div>
              </div>
            </div>

            {/* Initialization Templates */}
            <div className="space-y-4">
                <h3 className="text-sm font-medium text-zinc-300 flex items-center gap-2">
                    <FileCode size={16} className="text-zinc-500" /> Initialization (Cloud-Init)
                </h3>
                
                <div className="grid grid-cols-2 gap-3">
                    {TEMPLATES.map(tmpl => (
                        <button
                            key={tmpl.id}
                            type="button"
                            onClick={() => setTemplateId(tmpl.id)}
                            className={`
                                text-left p-3 rounded-lg border transition-all relative overflow-hidden
                                ${templateId === tmpl.id 
                                    ? 'bg-indigo-600/10 border-indigo-500/50 ring-1 ring-indigo-500/20' 
                                    : 'bg-zinc-900/30 border-zinc-800 hover:bg-zinc-900 hover:border-zinc-700'
                                }
                            `}
                        >
                            <div className="flex justify-between items-start mb-1">
                                <span className={`text-sm font-medium ${templateId === tmpl.id ? 'text-indigo-400' : 'text-zinc-300'}`}>
                                    {tmpl.label}
                                </span>
                                {templateId === tmpl.id && <div className="h-2 w-2 rounded-full bg-indigo-500 shadow-[0_0_8px_rgba(99,102,241,0.6)]"></div>}
                            </div>
                            <p className="text-[11px] text-zinc-500 leading-tight">{tmpl.description}</p>
                        </button>
                    ))}
                </div>

                {/* Advanced Editor */}
                <div>
                    <button 
                        type="button"
                        onClick={() => setShowAdvanced(!showAdvanced)}
                        className="text-xs text-zinc-500 hover:text-indigo-400 flex items-center gap-1 transition-colors"
                    >
                        <Code size={12} />
                        {showAdvanced ? 'Hide Configuration' : 'Edit Cloud-Config YAML'}
                    </button>
                    
                    {showAdvanced && (
                        <div className="mt-2 animate-in fade-in slide-in-from-top-2 duration-200">
                            <textarea 
                                value={userData}
                                onChange={(e) => { setUserData(e.target.value); setTemplateId('custom'); }}
                                className="w-full h-48 bg-black/50 border border-zinc-800 rounded-lg p-3 font-mono text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/50 resize-y"
                                placeholder="#cloud-config..."
                            />
                            <p className="text-[10px] text-zinc-600 mt-1">
                                Valid Cloud-Init YAML required. Used for package installation and system setup.
                            </p>
                        </div>
                    )}
                </div>
            </div>

          </div>

          {/* Footer Actions */}
          <div className="p-6 border-t border-zinc-900 bg-zinc-950 flex-shrink-0 flex items-center justify-end gap-3">
            <button 
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-zinc-400 hover:text-zinc-200 hover:bg-zinc-900 rounded-lg transition-colors"
            >
              Cancel
            </button>
            <button 
              type="submit"
              onClick={handleSubmit}
              disabled={isLoading}
              className="px-6 py-2 text-sm font-medium bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg shadow-lg shadow-indigo-500/20 transition-all flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isLoading && <Loader2 size={16} className="animate-spin" />}
              {isLoading ? 'Provisioning...' : 'Create Instance'}
            </button>
          </div>

        </div>
      </div>
    </>
  );
}