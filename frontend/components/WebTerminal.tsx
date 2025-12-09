'use client';

import React, { useEffect, useRef } from 'react';
import { X } from 'lucide-react';
import '@xterm/xterm/css/xterm.css';

interface WebTerminalProps {
  instanceName: string;
  onClose: () => void;
}

export default function WebTerminal({ instanceName, onClose }: WebTerminalProps) {
  const terminalRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  // Refs para as instâncias do xterm e fit addon para limpeza
  const xtermInstance = useRef<any>(null); 
  const fitAddonInstance = useRef<any>(null);

  useEffect(() => {
    let term: any;
    let fitAddon: any;
    let socket: WebSocket;

    const initTerminal = async () => {
      // Importação dinâmica para evitar SSR issues
      const { Terminal } = await import('@xterm/xterm');
      const { FitAddon } = await import('@xterm/addon-fit');

      term = new Terminal({
        cursorBlink: true,
        theme: {
          background: '#09090b', // zinc-950
          foreground: '#e4e4e7', // zinc-200
          cursor: '#6366f1', // indigo-500
        },
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        fontSize: 14,
      });

      fitAddon = new FitAddon();
      term.loadAddon(fitAddon);

      if (terminalRef.current) {
        term.open(terminalRef.current);
        fitAddon.fit();
      }

      xtermInstance.current = term;
      fitAddonInstance.current = fitAddon;

      // WebSocket Connection
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const host = window.location.hostname;
      const port = '8500';
      const token = localStorage.getItem('axion_token');
      
      const wsUrl = `${protocol}//${host}:${port}/ws/terminal/${instanceName}?token=${token}`;
      socket = new WebSocket(wsUrl);
      wsRef.current = socket;

      socket.onopen = () => {
        term.write(`\r\n\x1b[32m✔ Connected to ${instanceName}\x1b[0m\r\n`);
        term.focus();
      };

      socket.onmessage = (event) => {
        // Se for Blob/ArrayBuffer, converte (mas xterm lida bem com string)
        if (event.data instanceof Blob) {
            const reader = new FileReader();
            reader.onload = () => {
                term.write(reader.result as string);
            };
            reader.readAsText(event.data);
        } else {
            term.write(event.data);
        }
      };

      socket.onclose = () => {
        term.write('\r\n\x1b[31m✖ Connection closed\x1b[0m\r\n');
      };

      socket.onerror = () => {
        term.write('\r\n\x1b[31m✖ Connection error\x1b[0m\r\n');
      };

      // Terminal -> Socket
      term.onData((data: string) => {
        if (socket.readyState === WebSocket.OPEN) {
          socket.send(data);
        }
      });

      // Handle Resize
      const handleResize = () => {
        fitAddon.fit();
        // Opcional: enviar resize para backend se suportado
      };
      window.addEventListener('resize', handleResize);
      
      return () => {
        window.removeEventListener('resize', handleResize);
      };
    };

    initTerminal();

    return () => {
      // Cleanup
      if (socket) socket.close();
      if (xtermInstance.current) xtermInstance.current.dispose();
    };
  }, [instanceName]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 animate-in fade-in duration-200">
      <div className="w-full max-w-5xl bg-zinc-950 border border-zinc-800 rounded-xl shadow-2xl overflow-hidden flex flex-col h-[600px]">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-900 bg-zinc-900/50">
          <div className="flex items-center gap-2">
            <div className="flex gap-1.5">
              <div className="w-3 h-3 rounded-full bg-red-500/20 border border-red-500/50"></div>
              <div className="w-3 h-3 rounded-full bg-yellow-500/20 border border-yellow-500/50"></div>
              <div className="w-3 h-3 rounded-full bg-green-500/20 border border-green-500/50"></div>
            </div>
            <span className="ml-3 text-sm font-mono text-zinc-400">root@{instanceName}:~</span>
          </div>
          <button 
            onClick={onClose}
            className="p-1.5 text-zinc-500 hover:text-zinc-200 hover:bg-zinc-800 rounded transition-colors"
          >
            <X size={16} />
          </button>
        </div>

        {/* Terminal Area */}
        <div className="flex-1 p-1 bg-[#09090b] overflow-hidden relative">
            <div ref={terminalRef} className="h-full w-full" />
        </div>
      </div>
    </div>
  );
}
