'use client';

import React, { useState, useEffect, useRef } from 'react';
import Editor from '@monaco-editor/react';
import { X, Save, Loader2, Download } from 'lucide-react';
import { toast } from 'sonner';

interface FileEditorModalProps {
  isOpen: boolean;
  onClose: () => void;
  instanceName: string;
  filePath: string;
  initialContent: string;
}

export default function FileEditorModal({ isOpen, onClose, instanceName, filePath, initialContent }: FileEditorModalProps) {
  const [content, setContent] = useState(initialContent);
  const contentRef = useRef(content);
  contentRef.current = content;

  const [saving, setSaving] = useState(false);

  // Update content when file changes (important if reusing modal)
  useEffect(() => {
    console.log('Resetting content to initial:', initialContent.length);
    setContent(initialContent);
  }, [initialContent]);

  if (!isOpen) return null;

  const getLanguage = (path: string) => {
    const ext = path.split('.').pop()?.toLowerCase();
    switch (ext) {
      case 'js': return 'javascript';
      case 'ts': return 'typescript';
      case 'py': return 'python';
      case 'go': return 'go';
      case 'html': return 'html';
      case 'css': return 'css';
      case 'json': return 'json';
      case 'md': return 'markdown';
      case 'sh': return 'shell';
      case 'yaml': case 'yml': return 'yaml';
      case 'xml': return 'xml';
      case 'sql': return 'sql';
      default: return 'plaintext';
    }
  };

  const handleSave = async () => {
    setSaving(true);
    const currentContent = contentRef.current;
    try {
      const protocol = window.location.protocol;
      const host = window.location.hostname;
      const port = '8500';
      const token = localStorage.getItem('axion_token');

      // Create a Blob from currentContent to simulate file upload
      console.log('Content length:', currentContent?.length);
      const blob = new Blob([currentContent || ''], { type: 'text/plain' });
      console.log('Blob size:', blob.size);
      const formData = new FormData();
      formData.append('file', blob);

      const res = await fetch(`${protocol}//${host}:${port}/instances/${instanceName}/files?path=${filePath}`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`
        },
        body: formData
      });

      if (res.ok) {
        toast.success(`Saved ${filePath}`);
      } else {
        const err = await res.json();
        toast.error("Save Failed", { description: err.error });
      }
    } catch (error) {
      toast.error("Network Error");
    } finally {
      setSaving(false);
    }
  };

  const handleDownload = () => {
    const protocol = window.location.protocol;
    const host = window.location.hostname;
    const port = '8500';
    const token = localStorage.getItem('axion_token');
    
    const url = `${protocol}//${host}:${port}/instances/${instanceName}/files/content?path=${filePath}&token=${token}`;
    window.open(url, '_blank');
  };

  const handleEditorDidMount = (editor: any, monaco: any) => {
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
      handleSave();
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 animate-in fade-in duration-200">
      <div className="w-full h-full max-w-[95%] max-h-[90%] bg-[#1e1e1e] border border-zinc-800 rounded-xl shadow-2xl overflow-hidden flex flex-col">
        
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 bg-[#252526] border-b border-zinc-800">
          <div className="flex items-center gap-3">
            <span className="text-sm font-mono text-zinc-300">{filePath}</span>
            <span className="text-xs text-zinc-500 bg-zinc-800 px-2 py-0.5 rounded">{getLanguage(filePath)}</span>
          </div>
          <div className="flex items-center gap-2">
            <button 
              onClick={handleDownload}
              className="p-1.5 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-700 rounded transition-colors"
              title="Download File"
            >
              <Download size={18} />
            </button>
            <div className="h-4 w-px bg-zinc-700 mx-1"></div>
            <button 
              onClick={handleSave}
              disabled={saving}
              className="flex items-center gap-2 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-xs font-medium rounded transition-colors"
            >
              {saving ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
              Save
            </button>
            <button 
              onClick={onClose}
              className="p-1.5 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-700 rounded transition-colors ml-2"
            >
              <X size={18} />
            </button>
          </div>
        </div>

        {/* Editor */}
        <div className="flex-1 relative">
          <Editor
            height="100%"
            defaultLanguage={getLanguage(filePath)}
            value={content}
            theme="vs-dark"
            onChange={(val) => {
                console.log('Editor changed, new len:', val?.length);
                setContent(val || '');
            }}
            onMount={handleEditorDidMount}
            options={{
              minimap: { enabled: true },
              fontSize: 14,
              wordWrap: 'on', // AQUI está a mudança!
              wrappingIndent: 'same', // E AQUI!
              padding: { top: 16 },
              scrollBeyondLastLine: false,
            }}
          />
        </div>
      </div>
    </div>
  );
}
