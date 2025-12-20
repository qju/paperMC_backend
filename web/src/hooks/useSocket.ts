import { useEffect, useRef, useState } from 'react';

type LogMessage = {
    type: 'log' | 'error';
    data: string;
};

export function useSocket() {
    const [isConnected, setIsConnected] = useState(false);
    const [logs, setLogs] = useState<string[]>([]);

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

        // 4. Handle Event
        ws.onopen = () => setIsConnected(true);
        ws.onclose = () => setIsConnected(false);

        // THE IMPORTANT PART: Receving data
        ws.onmessage = (event) => {
            try {
                const msg: LogMessage = JSON.parse(event.data);
                if (msg.type === 'log') {
                    // Functoin State Update:
                    // "Tate the previous list, add the new line at the end"
                    setLogs((prev) => [...prev, msg.data]);
                }
            } catch (err) {
                console.error("WS Parse Error", err);
            }
        };

        // 5. Cleanup; if the user leave the page, kill connection
        return () => {
            ws.close();
        };

    }, []); // [] means "Run this onece when the component mounts"

    // Helper function to send data BACK to server
    const sendCommand = (cmd: string) => {
        // Only send if the connection is actually Open
        if (socketRef.current && socketRef.current.readyState == WebSocket.OPEN) {
            socketRef.current.send(JSON.stringify({ type: 'command', data: cmd }));
        }
    };

    return { isConnected, logs, sendCommand };
}

