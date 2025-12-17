import { useState, useEffect, useRef } from 'react';
import { useSocket } from '../hooks/useSocket';
import { Send, Power, PowerOff } from 'lucide-react';
import { LogLine } from '../components/LogLine';

export default function Console() {
    // 1. Hook into our WebSocket logic
    const { isConnected, logs, sendCommand } = useSocket();

    const [input, setInput] = useState('');
    const bottomRef = useRef<HTMLDivElement>(null);

    // 2. Auto-scroll to bottom whenever new logs arrive
    useEffect(() => {
        bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, [logs]);

    // 3. Handle sending commands
    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!input.trim()) return;

        sendCommand(input); // <--- Uses your hook
        setInput('');
    };

    // 4. Handle Start/Stop (Simple API calls)
    const handlePower = async (action: 'start' | 'stop') => {
        const token = localStorage.getItem('token');
        try {
            await fetch(`/${action}`, {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}` }
            });
        } catch (err) {
            console.error("Power action failed", err);
        }
    };

    return (
        <div className="flex flex-col h-full gap-4">
            {/* --- HEADER: STATUS & POWER --- */}
            <div className="flex justify-between items-center bg-black/40 p-4 border border-white/10 rounded-lg backdrop-blur-sm">

                {/* Connection Status Indicator */}
                <div className="flex items-center gap-3">
                    <div className={`w-3 h-3 rounded-full transition-colors duration-500 ${isConnected ? 'bg-mc-green shadow-[0_0_10px_#55aa55]' : 'bg-mc-red'}`} />
                    <span className="font-pixel text-xl text-white tracking-wide">
                        {isConnected ? 'LIVE CONNECTION' : 'DISCONNECTED'}
                    </span>
                </div>

                {/* Power Buttons */}
                <div className="flex gap-2">
                    <button
                        onClick={() => handlePower('start')}
                        className="flex items-center gap-2 bg-mc-green/10 border border-mc-green/50 text-mc-green hover:bg-mc-green/30 px-4 py-2 rounded font-mono text-sm transition-all"
                    >
                        <Power size={16} /> START
                    </button>
                    <button
                        onClick={() => handlePower('stop')}
                        className="flex items-center gap-2 bg-mc-red/10 border border-mc-red/50 text-mc-red hover:bg-mc-red/30 px-4 py-2 rounded font-mono text-sm transition-all"
                    >
                        <PowerOff size={16} /> STOP
                    </button>
                </div>
            </div>

            {/* --- LOG WINDOW --- */}
            <div className="flex-1 bg-black/80 border border-white/10 rounded-lg p-4 font-mono text-sm overflow-y-auto custom-scrollbar shadow-inner">
                {logs.length === 0 && (
                    <div className="h-full flex flex-col items-center justify-center text-white/30 space-y-2">
                        <div className="animate-pulse">Waiting for server logs...</div>
                    </div>
                )}

                {logs.map((line, index) => (
                    <LogLine key={index} content={line} />
                    //    <div key={index} className="break-words leading-relaxed font-mono text-sm">
                    //        {/* Basic Syntax Highlighting */}
                    //        {line.includes("ERROR") || line.includes("Exception") ? (
                    //            <span className="text-red-400">{line}</span>
                    //        ) : line.includes("WARN") ? (
                    //            <span className="text-mc-gold">{line}</span>
                    //        ) : line.includes("INFO") ? (
                    //            <span className="text-blue-300">{line}</span>
                    //        ) : (
                    //            <LogLine content={line} />
                    //       )}
                    //    </div>
                ))}
                {/* Invisible element to scroll to */}
                <div ref={bottomRef} />
            </div>

            {/* --- COMMAND INPUT --- */}
            <form onSubmit={handleSubmit} className="flex gap-0 shadow-lg">
                <div className="bg-black/60 border border-white/10 border-r-0 rounded-l-lg px-4 flex items-center text-mc-diamond font-bold font-pixel text-xl">
                    &gt;_
                </div>
                <input
                    type="text"
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    placeholder="Type a command (e.g., 'op playername')..."
                    className="flex-1 bg-black/60 border border-white/10 border-l-0 text-white font-mono focus:bg-black/80 focus:border-white/30 transition-colors p-4 outline-none"
                />
                <button
                    type="submit"
                    className="bg-white/10 border border-white/10 border-l-0 rounded-r-lg px-6 hover:bg-white/20 text-white transition-colors"
                >
                    <Send size={20} />
                </button>
            </form>
        </div>
    );
}
