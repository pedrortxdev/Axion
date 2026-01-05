import { useState, useEffect, useRef } from 'react';

// Função auxiliar para formatar Bytes
const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

export const useVmStats = (vmId: string) => {
    const [metrics, setMetrics] = useState<any>(null);
    const [networkSpeed, setNetworkSpeed] = useState({ down: '0 B/s', up: '0 B/s' });

    // Guarda o estado anterior para calcular a diferença
    const prevMetrics = useRef<any>(null);
    const lastUpdate = useRef<number>(Date.now());

    useEffect(() => {
        const fetchMetrics = async () => {
            try {
                const protocol = window.location.protocol;
                const host = window.location.hostname;
                const port = '8500';
                const token = localStorage.getItem('axion_token');
                if (!token) return;
                const res = await fetch(`${protocol}//${host}:${port}/api/v1/instances/${vmId}/metrics`, {
                    headers: {
                        'Authorization': `Bearer ${token}`
                    }
                });
                if (!res.ok) return;
                const data = await res.json();

                const now = Date.now();
                const timeDiffSeconds = (now - lastUpdate.current) / 1000;

                if (prevMetrics.current && timeDiffSeconds > 0) {
                    // Cálculo de Delta (Bytes por Segundo)
                    // AxHV returns cumulative bytes
                    const deltaRx = data.netRxBytes - prevMetrics.current.netRxBytes;
                    const deltaTx = data.netTxBytes - prevMetrics.current.netTxBytes;

                    const rxSpeed = Math.max(0, deltaRx / timeDiffSeconds);
                    const txSpeed = Math.max(0, deltaTx / timeDiffSeconds);

                    setNetworkSpeed({
                        down: `${formatBytes(rxSpeed)}/s`,
                        up: `${formatBytes(txSpeed)}/s`
                    });
                }

                prevMetrics.current = data;
                lastUpdate.current = now;
                setMetrics(data);
            } catch (error) {
                console.error("Failed to fetch metrics", error);
            }
        };

        // Polling a cada 5 segundos (Low cost pro server)
        const interval = setInterval(fetchMetrics, 5000);
        fetchMetrics(); // Primeira chamada imediata

        return () => clearInterval(interval);
    }, [vmId]);

    return { metrics, networkSpeed };
};
