'use client';

import React, { useState, useEffect } from 'react';
import { X, Server, Box, Zap, FileCode, Code, Loader2, Monitor, User, Key, Network, FileUp } from 'lucide-react';
import { toast } from 'sonner';

interface Template {
  id: string;
  name: string;
  icon: string;
  description: string;
  min_cpu: number;
  min_ram_mb: number;
}

interface CreateInstanceModalProps {
  isOpen: boolean;
  onClose: () => void;
  token: string | null;
  initialType?: 'container' | 'virtual-machine';
}

// Validated Cloud-Init Templates (legacy, for direct YAML selection)
const LEGACY_TEMPLATES = [
  {
    id: 'none',
    label: 'Clean OS',
    description: 'Empty configuration',
    yaml: '',
  },
  {
    id: 'docker',
    label: 'Docker Host',
    description: 'Pre-installed Docker & Compose',
    yaml: `
packages:
  - docker.io
  - docker-compose
runcmd:
  - systemctl enable --now docker
  - usermod -aG docker ubuntu`,
  },
  {
    id: 'web',
    label: 'Web Server',
    description: 'Nginx with default page',
    yaml: `
packages:
  - nginx
  - curl
write_files:
  - path: /var/www/html/index.html
    content: |
      <h1>Axion Instance Deployed!</h1>
      <p>Cloud-init worked successfully.</p>`,
  },
];

const CONTAINER_IMAGES = [
  { value: 'ubuntu/24.04', label: 'Ubuntu 24.04 LTS' },
  { value: 'ubuntu/22.04', label: 'Ubuntu 22.04 LTS' },
  { value: 'alpine/3.19', label: 'Alpine 3.19' },
];

const VM_IMAGES = [
  { value: 'ubuntu/24.04', label: 'Ubuntu 24.04 LTS (VM)' },
  { value: 'debian/12', label: 'Debian 12 (VM)' },
];

export default function CreateInstanceModal({ isOpen, onClose, token, initialType = 'container' }: CreateInstanceModalProps) {
  const [type, setType] = useState<'container' | 'virtual-machine'>(initialType);
  const [name, setName] = useState('');
  const [image, setImage] = useState(initialType === 'container' ? CONTAINER_IMAGES[0].value : VM_IMAGES[0].value);
  const [cpu, setCpu] = useState(1);
  const [memory, setMemory] = useState(512);
  const [disk, setDisk] = useState(10); // Default 10GB
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [templateId, setTemplateId] = useState('none');
  const [userData, setUserData] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<'config' | 'templates'>('config');
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loadingTemplates, setLoadingTemplates] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<Template | null>(null);
  const [installMethod, setInstallMethod] = useState<'template' | 'iso'>('template');
  const [isoImages, setIsoImages] = useState<IsoImage[]>([]);
  const [selectedIso, setSelectedIso] = useState<string>('');

  interface IsoImage {
    name: string;
    size: number;
  }

  // Update defaults when initialType changes or modal opens
  useEffect(() => {
    if (isOpen) {
        setType(initialType);
        setImage(initialType === 'container' ? CONTAINER_IMAGES[0].value : VM_IMAGES[0].value);
        // Reset fields
        setName('');
        setCpu(1);
        setMemory(initialType === 'container' ? 256 : 1024); // VMs need more RAM usually
        setUsername('admin');
        setPassword('');
        setTemplateId('none');
        setUserData('');
        setShowAdvanced(false);
        setSelectedTemplate(null);
        setInstallMethod('template');
        setSelectedIso('');
    }
  }, [isOpen, initialType]);
  
  const fetchTemplates = React.useCallback(async () => {
    if (!token) return;

    setLoadingTemplates(true);
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const response = await fetch(`${protocol}//${host}:${port}/templates`, {
        headers: {
          'Authorization': `Bearer ${token}`
        },
      });

      if (response.ok) {
        const data = await response.json();
        setTemplates(data);
      } else {
        toast.error('Failed to load templates', { description: 'Could not fetch available templates' });
      }
    } catch (error) {
      toast.error('Network Error', { description: 'Could not reach control plane' });
    } finally {
      setLoadingTemplates(false);
    }
  }, [token]);

  const fetchIsoImages = React.useCallback(async () => {
    if (!token) return;
    try {
        const protocol = window.location.protocol;
        const host = window.location.hostname;
        const port = '8500';
        const response = await fetch(`${protocol}//${host}:${port}/isos`, {
            headers: {
                'Authorization': `Bearer ${token}`
            },
        });
        if (response.ok) {
            const data = await response.json();
            setIsoImages(data || []);
            if (data && data.length > 0) {
              setSelectedIso(data[0].name);
            }
        } else {
            toast.error('Failed to load ISOs', { description: 'Could not fetch available ISO images' });
        }
    } catch (error) {
        toast.error('Network Error', { description: 'Could not reach control plane for ISOs' });
    }
}, [token]);


  // Fetch templates when modal opens
  useEffect(() => {
    if (isOpen) {
      if (activeTab === 'templates') {
        fetchTemplates();
      }
      fetchIsoImages();
    }
  }, [isOpen, activeTab, fetchTemplates, fetchIsoImages]);

  const generateUserData = React.useCallback(() => {
    // Use selected template if one is selected, otherwise use legacy templates
    let templateYaml = '';

    if (selectedTemplate) {
      // For new template system, we'll handle the cloud config in the backend
      templateYaml = '';
    } else {
      // Use legacy templates
      templateYaml = LEGACY_TEMPLATES.find(t => t.id === templateId)?.yaml || '';
    }

    const yamlLines = ['#cloud-config'];

    // 1. User Configuration (VM Only)
    if (type === 'virtual-machine' && username && password) {
      yamlLines.push('');
      yamlLines.push('users:');
      yamlLines.push(`  - name: ${username}`);
      yamlLines.push(`    passwd: ${password}`);
      yamlLines.push('    groups: [sudo, users, admin]');
      yamlLines.push('    lock_passwd: false');
      yamlLines.push('    shell: /bin/bash');

      // Force password set
      yamlLines.push('');
      yamlLines.push('chpasswd:');
      yamlLines.push('  list: |');
      yamlLines.push(`    ${username}:${password}`);
      yamlLines.push('  expire: False');
    }

    // 2. Package updates
    yamlLines.push('');
    yamlLines.push('package_update: true');
    yamlLines.push('packages:');

    if (type === 'virtual-machine') {
      yamlLines.push('  - qemu-guest-agent');
    }

    // Add packages from legacy template
    if (!selectedTemplate) {
      const templateLines = templateYaml.split('\n');
      let inPackagesSection = false;
      for (const line of templateLines) {
        if (line.trim() === 'packages:') {
          inPackagesSection = true;
          continue;
        }

        if (inPackagesSection && line.trim().startsWith('  - ')) {
          yamlLines.push(line);
        } else if (inPackagesSection && !line.trim().startsWith('  - ') && line.trim() !== '') {
          inPackagesSection = false;
          break;
        }
      }

      // Add remaining template content
      let processingRest = false;
      for (const line of templateLines) {
        if (line.trim() === 'packages:') {
          processingRest = true;
          continue;
        }
        if (processingRest && !line.trim().startsWith('  - ')) {
          yamlLines.push(line);
        }
      }
    }

    setUserData(yamlLines.join('\n'));
  }, [type, templateId, username, password, selectedTemplate]);

  // Update user data when relevant fields change
  useEffect(() => {
    generateUserData();
  }, [generateUserData]);

  if (!isOpen) return null;

  const handleTemplateSelect = (template: Template) => {
    setSelectedTemplate(template);
    setTemplateId('none'); // Reset legacy template selection

    // Auto-fill based on template requirements
    if (template.min_cpu > cpu) {
      setCpu(template.min_cpu);
    }
    if (template.min_ram_mb > memory) {
      setMemory(template.min_ram_mb);
    }

    // Set default image based on template (all templates use Ubuntu 24.04)
    setImage('ubuntu/24.04');
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token) return;

    if (!name.trim() || name.includes(' ')) {
      toast.error('Invalid Name', { description: 'Name cannot be empty or contain spaces.' });
      return;
    }

    // Specific validation for VM
    if (type === 'virtual-machine') {
        if (!username.trim()) {
            toast.error('Invalid Username', { description: 'Username is required for VMs.' });
            return;
        }
        if (!password.trim()) {
            toast.error('Invalid Password', { description: 'Password is required for VMs.' });
            return;
        }
    }

    setIsLoading(true);

    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';

      // Define payload with proper type
      let payload: any = {
        name: name.trim(),
        type: type,
        limits: {
          "limits.cpu": cpu.toString(),
          "limits.memory": `${memory}MB`
        },
      };

      if (type === 'virtual-machine' && installMethod === 'iso') {
        payload.iso_image = selectedIso;
        payload.limits["limits.rootfs"] = `${disk}GB`;
        // No user_data or template_id for ISO installs
      } else {
        payload.image = image;
        // Only add user_data if we're not using a template or using legacy templates
        if (!selectedTemplate) {
          payload.user_data = userData;
        } else {
          // If using a new template, add the template_id
          payload.template_id = selectedTemplate.id;
        }
      }

      const response = await fetch(`${protocol}//${host}:${port}/instances`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(payload),
      });

      if (response.ok) {
        toast.success('Provisioning Started', { description: `Creating ${name} as ${type}...` });
        onClose();
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

  const imagesList = type === 'container' ? CONTAINER_IMAGES : VM_IMAGES;

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
                {type === 'container' ? <Box className="text-indigo-500" size={20} /> : <Monitor className="text-purple-500" size={20} />}
                New {type === 'container' ? 'Container' : 'Virtual Machine'}
              </h2>
              <p className="text-sm text-zinc-500 mt-1">
                {type === 'container' ? 'Lightweight, system container.' : 'Full virtual machine with dedicated kernel.'}
              </p>
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
            {/* Tab Navigation */}
            <div className="flex border-b border-zinc-800">
              <button
                onClick={() => setActiveTab('config')}
                className={`px-4 py-2 text-sm font-medium transition-colors ${
                  activeTab === 'config'
                    ? 'text-zinc-200 border-b-2 border-indigo-500'
                    : 'text-zinc-500 hover:text-zinc-300'
                }`}
              >
                Manual Configuration
              </button>
              <button
                onClick={() => setActiveTab('templates')}
                className={`px-4 py-2 text-sm font-medium transition-colors ${
                  activeTab === 'templates'
                    ? 'text-zinc-200 border-b-2 border-indigo-500'
                    : 'text-zinc-500 hover:text-zinc-300'
                }`}
              >
                App Templates
              </button>
            </div>

            {/* Content based on active tab */}
            {activeTab === 'config' ? (
              <>
                {/* Basic Configuration */}
                <div className="space-y-4">
                  <h3 className="text-sm font-medium text-zinc-300 flex items-center gap-2">
                    <Server size={16} className="text-zinc-500" /> Configuration
                  </h3>

                  {type === 'virtual-machine' && (
                    <div className="flex items-center gap-2 bg-zinc-900/50 border border-zinc-800 rounded-lg p-1 w-max">
                        <button 
                            onClick={() => setInstallMethod('template')}
                            className={`px-3 py-1.5 text-xs rounded-md transition-colors ${installMethod === 'template' ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:text-zinc-200'}`}
                        >
                            From Template
                        </button>
                        <button 
                            onClick={() => setInstallMethod('iso')}
                            className={`px-3 py-1.5 text-xs rounded-md transition-colors ${installMethod === 'iso' ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:text-zinc-200'}`}
                        >
                            From ISO
                        </button>
                    </div>
                  )}

                  <div className="grid grid-cols-2 gap-6">
                    <div>
                      <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">Name</label>
                      <input
                        type="text"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        placeholder={type === 'container' ? "my-container-01" : "my-vm-01"}
                        className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all placeholder:text-zinc-600"
                        autoFocus
                      />
                    </div>

                    {installMethod === 'iso' ? (
                        <div>
                            <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">ISO Image</label>
                            <div className="relative">
                                <select
                                    value={selectedIso}
                                    onChange={(e) => setSelectedIso(e.target.value)}
                                    className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2.5 appearance-none focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all cursor-pointer"
                                    disabled={isoImages.length === 0}
                                >
                                    {isoImages.length > 0 ? (
                                        isoImages.map(iso => (
                                            <option key={iso.name} value={iso.name}>{iso.name}</option>
                                        ))
                                    ) : (
                                        <option>No ISOs available</option>
                                    )}
                                </select>
                                <div className="absolute right-4 top-1/2 -translate-y-1/2 pointer-events-none text-zinc-500">
                                    <FileUp size={16} />
                                </div>
                            </div>
                        </div>
                    ) : (
                        <div>
                            <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">Image</label>
                            <div className="relative">
                                <select
                                    value={image}
                                    onChange={(e) => setImage(e.target.value)}
                                    className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2.5 appearance-none focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all cursor-pointer"
                                >
                                    {imagesList.map(img => (
                                        <option key={img.value} value={img.value}>{img.label}</option>
                                    ))}
                                </select>
                                <div className="absolute right-4 top-1/2 -translate-y-1/2 pointer-events-none text-zinc-500">
                                    <Server size={16} />
                                </div>
                            </div>
                        </div>
                    )}
                  </div>
                </div>

                {/* Resources */}
                <div className="space-y-4">
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
                      <input
                        type="range"
                        min="1"
                        max="8"
                        step="1"
                        value={cpu}
                        onChange={(e) => setCpu(parseInt(e.target.value))}
                        className="w-full h-1.5 bg-zinc-800 rounded-lg appearance-none cursor-pointer accent-indigo-500 hover:accent-indigo-400"
                      />
                    </div>

                    {/* RAM */}
                    <div>
                      <div className="flex justify-between items-center mb-2">
                        <span className="text-xs text-zinc-400">Memory Limit</span>
                        <span className="text-xs font-mono text-indigo-400 bg-indigo-500/10 px-2 py-0.5 rounded border border-indigo-500/20">{memory} MB</span>
                      </div>
                      <input
                        type="range"
                        min="256"
                        max="8192"
                        step="256"
                        value={memory}
                        onChange={(e) => setMemory(parseInt(e.target.value))}
                        className="w-full h-1.5 bg-zinc-800 rounded-lg appearance-none cursor-pointer accent-indigo-500 hover:accent-indigo-400"
                      />
                    </div>

                    {/* Disk */}
                    {type === 'virtual-machine' && (
                        <div>
                            <div className="flex justify-between items-center mb-2">
                                <span className="text-xs text-zinc-400">Disk Size</span>
                                <span className="text-xs font-mono text-indigo-400 bg-indigo-500/10 px-2 py-0.5 rounded border border-indigo-500/20">{disk} GB</span>
                            </div>
                            <input
                                type="range"
                                min="10"
                                max="100"
                                step="5"
                                value={disk}
                                onChange={(e) => setDisk(parseInt(e.target.value))}
                                className="w-full h-1.5 bg-zinc-800 rounded-lg appearance-none cursor-pointer accent-indigo-500 hover:accent-indigo-400"
                            />
                        </div>
                    )}
                  </div>
                </div>

                {/* Access Configuration (Conditional) */}
                <div className="space-y-4">
                  <h3 className="text-sm font-medium text-zinc-300 flex items-center gap-2">
                    <User size={16} className="text-zinc-500" /> Access & Network
                  </h3>

                  <div className="space-y-4">
                    {type === 'virtual-machine' ? (
                        <div className="grid grid-cols-2 gap-4">
                          <div>
                            <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">Username</label>
                            <div className="relative">
                              <input
                                type="text"
                                value={username}
                                onChange={(e) => setUsername(e.target.value)}
                                placeholder="admin"
                                className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all placeholder:text-zinc-600 pl-10"
                              />
                              <div className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500">
                                <User size={16} />
                              </div>
                            </div>
                          </div>

                          <div>
                            <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">Password</label>
                            <div className="relative">
                              <input
                                type="password"
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                placeholder="••••••••"
                                className="w-full bg-zinc-900/50 border border-zinc-800 text-zinc-200 rounded-lg px-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 transition-all placeholder:text-zinc-600 pl-10"
                              />
                              <div className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500">
                                <Key size={16} />
                              </div>
                            </div>
                          </div>
                        </div>
                    ) : (
                        <div className="p-3 bg-zinc-900/30 border border-zinc-800 rounded-lg flex items-center gap-3">
                            <User size={16} className="text-zinc-500" />
                            <span className="text-sm text-zinc-400">Authentication is disabled for Containers (System Container).</span>
                        </div>
                    )}

                    <div>
                      <label className="block text-xs font-medium text-zinc-400 mb-1.5 uppercase tracking-wider">Network</label>
                      <div className="inline-flex items-center px-3 py-1.5 bg-zinc-900/50 border border-zinc-800 rounded-lg text-xs text-zinc-400">
                        <Network size={12} className="mr-1.5" />
                        DHCP (Managed by LXD)
                      </div>
                    </div>
                  </div>
                </div>

                {/* Templates */}
                {installMethod === 'template' && (
                    <div className="space-y-4">
                    <h3 className="text-sm font-medium text-zinc-300 flex items-center gap-2">
                        <FileCode size={16} className="text-zinc-500" /> Cloud-Init Template
                    </h3>

                    <div className="space-y-4">
                        <div>
                        <label className="block text-xs font-medium text-zinc-400 mb-1.5">Select Template</label>
                        <div className="grid grid-cols-2 gap-3">
                            {LEGACY_TEMPLATES.map(tmpl => (
                            <button
                                key={tmpl.id}
                                type="button"
                                onClick={() => {
                                setTemplateId(tmpl.id);
                                setSelectedTemplate(null); // Reset new template selection
                                }}
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
                        </div>

                        {/* YAML Editor */}
                        <div>
                        <div className="flex justify-between items-center mb-2">
                            <label className="text-xs font-medium text-zinc-400">Resulting YAML (Preview)</label>
                            <button
                            type="button"
                            onClick={() => setShowAdvanced(!showAdvanced)}
                            className="text-xs text-zinc-500 hover:text-indigo-400 flex items-center gap-1 transition-colors"
                            >
                            <Code size={12} />
                            {showAdvanced ? 'Hide Editor' : 'Edit YAML'}
                            </button>
                        </div>

                        {showAdvanced ? (
                            <textarea
                            value={userData}
                            onChange={(e) => setUserData(e.target.value)}
                            className="w-full h-48 bg-black/50 border border-zinc-800 rounded-lg p-3 font-mono text-xs text-zinc-300 focus:outline-none focus:border-indigo-500/50 resize-y"
                            placeholder="#cloud-config..."
                            />
                        ) : (
                            <div className="w-full h-48 bg-black/50 border border-zinc-800 rounded-lg p-3 font-mono text-xs text-zinc-300 overflow-y-auto whitespace-pre-wrap overflow-x-auto">
                            {userData || <span className="text-zinc-600 italic">Select a template or enable editor to view YAML</span>}
                            </div>
                        )}
                        </div>
                    </div>
                    </div>
                )}
              </>
            ) : (
              // Templates Tab
              <div className="space-y-4">
                <h3 className="text-sm font-medium text-zinc-300 flex items-center gap-2">
                  <FileCode size={16} className="text-zinc-500" /> App Templates
                </h3>

                {loadingTemplates ? (
                  <div className="flex justify-center items-center h-64">
                    <Loader2 size={24} className="animate-spin text-indigo-500" />
                  </div>
                ) : (
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {templates.map((template) => (
                      <div
                        key={template.id}
                        onClick={() => handleTemplateSelect(template)}
                        className={`
                          p-4 rounded-xl border cursor-pointer transition-all
                          ${selectedTemplate?.id === template.id
                            ? 'bg-indigo-600/10 border-indigo-500 ring-1 ring-indigo-500/20'
                            : 'bg-zinc-900/30 border-zinc-800 hover:bg-zinc-900 hover:border-zinc-700'
                          }
                        `}
                      >
                        <div className="flex items-start gap-3">
                          <div className="text-2xl">{template.icon}</div>
                          <div className="flex-1">
                            <div className="flex justify-between items-start mb-1">
                              <h4 className={`font-medium ${selectedTemplate?.id === template.id ? 'text-indigo-400' : 'text-zinc-200'}`}>
                                {template.name}
                              </h4>
                              {selectedTemplate?.id === template.id && (
                                <div className="h-2 w-2 rounded-full bg-indigo-500 shadow-[0_0_8px_rgba(99,102,241,0.6)]"></div>
                              )}
                            </div>
                            <p className="text-sm text-zinc-400 mb-2">{template.description}</p>
                            <div className="flex items-center gap-2 text-xs text-zinc-500">
                              <span>Min: {template.min_cpu} CPU</span>
                              <span>•</span>
                              <span>{template.min_ram_mb} MB RAM</span>
                            </div>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                {selectedTemplate && (
                  <div className="mt-4 p-4 bg-zinc-900/30 border border-zinc-800 rounded-lg">
                    <div className="flex justify-between items-center mb-2">
                      <h4 className="font-medium text-zinc-200">Selected Template</h4>
                      <button
                        onClick={() => setSelectedTemplate(null)}
                        className="text-xs text-zinc-500 hover:text-zinc-300"
                      >
                        Clear selection
                      </button>
                    </div>
                    <div className="flex items-center gap-3">
                      <div className="text-2xl">{selectedTemplate.icon}</div>
                      <div>
                        <div className="text-sm font-medium text-zinc-200">{selectedTemplate.name}</div>
                        <div className="text-xs text-zinc-400">{selectedTemplate.description}</div>
                        <div className="flex items-center gap-2 mt-1 text-xs text-zinc-500">
                          <span>Min: {selectedTemplate.min_cpu} CPU</span>
                          <span>•</span>
                          <span>{selectedTemplate.min_ram_mb} MB RAM</span>
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}

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
              className={`px-6 py-2 text-sm font-medium text-white rounded-lg shadow-lg transition-all flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed ${type === 'container' ? 'bg-indigo-600 hover:bg-indigo-500 shadow-indigo-500/20' : 'bg-purple-600 hover:bg-purple-500 shadow-purple-500/20'}`}
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