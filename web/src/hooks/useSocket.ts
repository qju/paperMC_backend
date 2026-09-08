import { useEffect, useRef, useState, useCallback } from 'react';
import type { Vitals, CrashReport } from '../types';

type WSIncomingMessage =
    | { type: 'log'; data: string }
    | { type: 'error'; data: string }
    | { type: 'vitals'; data: Vitals }
    | { type: 'crash_alert'; data: CrashReport };

export function useSocket() {
    const [isConnected, setIsConnected] = useState(false);
    const [logs, setLogs] = useState<string[]>([]);
    const [liveVitals, setLiveVitals] = useState<Vitals | null>(null);
    const [crashAlert, setCrashAlert] = useState<CrashReport | null>(null);

    // We use ref because we need to talk to the *same* useSocket
    // across different render of the component.
    const socketRef = useRef<WebSocket | null>(null);

    useEffect(() => {
        // 1. Security Check: Do we have a token?
        const token = localStorage.getItem('token');
        if (!token) return;

        // 2. Build the URL (Handle SSL automatically)
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws?token=${token}`;

        // 3. Open connection
        const ws = new WebSocket(wsUrl);
        socketRef.current = ws;

        // 4. Handle Events
        ws.onopen = () => setIsConnected(true);
        ws.onclose = () => setIsConnected(false);

        // 5. Receiving data
        ws.onmessage = (event) => {
            try {
                const msg: WSIncomingMessage = JSON.parse(event.data);
                if (msg.type === 'log') {
                    setLogs((prev) => [...prev, msg.data]);
                } else if (msg.type === 'vitals') {
                    setLiveVitals(msg.data);
                } else if (msg.type === 'crash_alert') {
                    setCrashAlert(msg.data);
                }
            } catch (err) {
                console.error("WS Parse Error", err);
            }
        };

        // 6. Cleanup
        return () => {
            ws.close();
        };
    }, []);

    const clearCrashAlert = useCallback(() => {
        setCrashAlert(null);
    }, []);

    // Helper function to send data BACK to server
    const sendCommand = (cmd: string) => {
        if (socketRef.current && socketRef.current.readyState === WebSocket.OPEN) {
            socketRef.current.send(JSON.stringify({ type: 'command', data: cmd }));
        }
    };

    return { isConnected, logs, liveVitals, crashAlert, clearCrashAlert, sendCommand };
}

