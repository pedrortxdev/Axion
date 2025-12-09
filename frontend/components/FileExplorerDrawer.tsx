'use client';

import React, { useState, useEffect, useRef } from 'react';
import { X, Folder, File, ArrowLeft, Upload, Trash2, FilePlus, FolderPlus, Download, Loader2, Edit } from 'lucide-react';
import { toast } from 'sonner';
import FileEditorModal from './FileEditorModal';

interface FileEntry {
  name: string;
  type: string; // "file" or "directory"
}

interface FileExplorerDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  instanceName: string | null;
}

export default function FileExplorerDrawer({ isOpen, onClose, instanceName }: FileExplorerDrawerProps) {
  const [currentPath, setCurrentPath] = useState('/root');
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  
  // Editor State
  const [editorOpen, setEditorOpen] = useState(false);
  const [editorPath, setEditorPath] = useState('');
  const [editorContent, setEditorContent] = useState('');

  // Upload Ref
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen && instanceName) {
      fetchFiles(currentPath);
    }
  }, [isOpen, instanceName, currentPath]);

  const fetchFiles = async (path: string) => {
    if (!instanceName) return;
    setLoading(true);
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const token = localStorage.getItem('axion_token');

      // Note: ListFiles endpoint returns []string (names of files in directory)
      const res = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/files/list?path=${path}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });

      if (res.ok) {
        const data: FileEntry[] = await res.json();
        const sortedEntries = data.sort((a, b) => {
          if (a.type === 'directory' && b.type !== 'directory') {
            return -1;
          }
          if (a.type !== 'directory' && b.type === 'directory') {
            return 1;
          }
          return a.name.localeCompare(b.name);
        });
        setEntries(sortedEntries);
      } else {
        toast.error("Failed to list directory");
        // If it failed, it might be a file we tried to list as a dir.
        // But our logic should prevent that.
      }
    } catch (err) {
      toast.error("Network Error");
    } finally {
      setLoading(false);
    }
  };

  const handleEntryClick = async (entry: FileEntry) => {
    const fullPath = currentPath === '/' ? `/${entry.name}` : `${currentPath}/${entry.name}`;
    
    setLoading(true);
    try {
        if (entry.type === 'directory') {
            setCurrentPath(fullPath); // This will trigger useEffect to fetch and update entries
        } else {
            handleOpenFile(fullPath);
        }
    } catch (e) {
        toast.error("Error processing entry");
    } finally {
        setLoading(false);
    }
  };

  const handleOpenFile = async (path: string) => {
    // Check extension for editability
    const editableExts = ['txt', 'md', 'js', 'ts', 'go', 'py', 'html', 'css', 'json', 'yaml', 'yml', 'conf', 'sh', 'env', 'log', 'ini', 'xml', 'sql'];
    const ext = path.split('.').pop()?.toLowerCase();

    // If no extension or editable extension, try to open in editor
    if (!ext || editableExts.includes(ext)) {
        try {
            const protocol = window.location.protocol;
            const host = window.location.hostname;
            const port = '8500';
            const token = localStorage.getItem('axion_token');
            
            // Note: If file is huge, backend returns attachment disposition.
            // Client fetch might handle it or we check header.
            const res = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/files/content?path=${path}`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            
            if (res.ok) {
                const disposition = res.headers.get("Content-Disposition");
                if (disposition && disposition.includes("attachment")) {
                    toast.info("File too large for editor, downloading...");
                    handleDownload(path);
                    return;
                }

                const text = await res.text();
                setEditorPath(path);
                setEditorContent(text);
                setEditorOpen(true);
            } else {
                toast.error("Failed to read file");
            }
        } catch (e) {
            toast.error("Network Error");
        }
    } else {
        // Binary or unknown -> Download
        handleDownload(path);
    }
  };

  const handleDownload = (path: string) => {
    const protocol = window.location.protocol;
    const host = window.location.hostname;
    const port = '8500';
    const token = localStorage.getItem('axion_token');
    
    // Direct link with token
    const url = `${protocol}//${host}:${port}/instances/${instanceName}/files/content?path=${path}&token=${token}`;
    window.open(url, '_blank');
  };

  const handleUp = () => {
    if (currentPath === '/') return;
    const parent = currentPath.substring(0, currentPath.lastIndexOf('/')) || '/';
    setCurrentPath(parent);
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || !e.target.files[0] || !instanceName) return;
    const file = e.target.files[0];
    const targetPath = currentPath === '/' ? `/${file.name}` : `${currentPath}/${file.name}`;

    const formData = new FormData();
    formData.append('file', file);

    const toastId = toast.loading("Uploading...");

    try {
        const protocol = window.location.protocol;
        const host = window.location.hostname;
        const port = '8500';
        const token = localStorage.getItem('axion_token');

        const res = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/files?path=${targetPath}`, {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${token}` },
            body: formData
        });

        if (res.ok) {
            toast.success("Upload Completed", { id: toastId });
            fetchFiles(currentPath); // Refresh
        } else {
            toast.error("Upload Failed", { id: toastId });
        }
    } catch (e) {
        toast.error("Network Error", { id: toastId });
    }
  };

  const handleDelete = async (entry: string) => {
    if (!window.confirm(`Delete ${entry}?`)) return;
    const targetPath = currentPath === '/' ? `/${entry}` : `${currentPath}/${entry}`;

    try {
        const protocol = window.location.protocol;
        const host = window.location.hostname;
        const port = '8500';
        const token = localStorage.getItem('axion_token');

        const res = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/files?path=${targetPath}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });

        if (res.ok) {
            toast.success("Deleted");
            fetchFiles(currentPath);
        } else {
            toast.error("Delete Failed");
        }
    } catch (e) {
        toast.error("Network Error");
    }
  };

  return (
    <>
      {/* Editor Modal */}
      {instanceName && (
        <FileEditorModal 
            isOpen={editorOpen} 
            onClose={() => setEditorOpen(false)} 
            instanceName={instanceName} 
            filePath={editorPath}
            initialContent={editorContent}
        />
      )}

      {/* Drawer */}
      <div 
        className={`fixed inset-0 bg-black/60 backdrop-blur-[2px] z-40 transition-opacity duration-300 ${
          isOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'
        }`}
        onClick={onClose}
      />

      <div 
        className={`
          fixed top-0 right-0 h-full w-full sm:w-[600px] bg-zinc-950 border-l border-zinc-800 shadow-2xl z-50 transform transition-transform duration-300 ease-out flex flex-col
          ${isOpen ? 'translate-x-0' : 'translate-x-full'}
        `}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-5 border-b border-zinc-900 bg-zinc-950/50 backdrop-blur-md">
          <div>
            <h2 className="text-lg font-semibold text-zinc-100 flex items-center gap-2">
              <Folder size={18} className="text-indigo-500" />
              File System
            </h2>
            <p className="text-xs text-zinc-500 mt-0.5 font-mono">{currentPath}</p>
          </div>
          <button onClick={onClose} className="p-2 text-zinc-500 hover:text-zinc-100 hover:bg-zinc-900 rounded-lg transition-colors">
            <X size={20} />
          </button>
        </div>

        {/* Toolbar */}
        <div className="p-3 border-b border-zinc-900 bg-zinc-900/20 flex items-center gap-2">
            <button 
                onClick={handleUp} 
                disabled={currentPath === '/'}
                className="p-2 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800 rounded disabled:opacity-30"
                title="Go Up"
            >
                <ArrowLeft size={16} />
            </button>
            <div className="h-6 w-px bg-zinc-800 mx-1"></div>
            <button 
                onClick={() => fileInputRef.current?.click()}
                className="flex items-center gap-2 px-3 py-1.5 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs font-medium rounded transition-colors"
            >
                <Upload size={14} /> Upload
            </button>
            <input type="file" className="hidden" ref={fileInputRef} onChange={handleUpload} />
            
            <div className="ml-auto flex items-center gap-2">
                <button className="p-2 text-zinc-500 hover:text-zinc-200 rounded hover:bg-zinc-800" title="New File (Mock)">
                    <FilePlus size={16} />
                </button>
                <button className="p-2 text-zinc-500 hover:text-zinc-200 rounded hover:bg-zinc-800" title="New Folder (Mock)">
                    <FolderPlus size={16} />
                </button>
            </div>
        </div>

        {/* List */}
        <div className="flex-1 overflow-y-auto p-2">
            {loading ? (
                <div className="flex justify-center py-10"><Loader2 className="animate-spin text-zinc-600" /></div>
            ) : entries.length === 0 ? (
                <div className="text-center py-10 text-zinc-600">Empty directory</div>
            ) : (
                <div className="space-y-1">
                    {entries.map(entry => {
                        // Heuristic visual only: if extension, file icon.
                        // Ideally backend would return type.
                        const isLikelyFile = entry.type === 'file';
                        
                        return (
                            <div 
                                key={entry.name} 
                                className="flex items-center justify-between p-2 rounded hover:bg-zinc-900 cursor-pointer group"
                                onClick={() => handleEntryClick(entry)}
                            >
                                <div className="flex items-center gap-3">
                                    {entry.type === 'file' ? <File size={16} className="text-zinc-500" /> : <Folder size={16} className="text-indigo-400" />}
                                    <span className="text-sm text-zinc-200">{entry.name}</span>
                                </div>
                                
                                <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity" onClick={(e) => e.stopPropagation()}>
                                    <button onClick={() => handleDownload(`${currentPath}/${entry.name}`)} className="p-1.5 text-zinc-500 hover:text-blue-400 rounded hover:bg-blue-500/10">
                                        <Download size={14} />
                                    </button>
                                    <button onClick={() => handleDelete(entry.name)} className="p-1.5 text-zinc-500 hover:text-red-400 rounded hover:bg-red-500/10">
                                        <Trash2 size={14} />
                                    </button>
                                </div>
                            </div>
                        );
                    })}
                </div>
            )}
        </div>
      </div>
    </>
  );
}