import { useEffect, useState } from 'react';
import { Shield, ShieldAlert, Trash2, UserPlus, UserCheck, UserX, AlertTriangle, Play, RefreshCw, Ban, XCircle, CheckCircle } from 'lucide-react';
import type { Player, RejectedPlayer, UnifiedPlayer } from '../types';

// 1. Toast Interface
interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

export default function Players() {
    // Data State
    const [whitelist, setWhitelist] = useState<Player[]>([]);
    const [banned, setBanned] = useState<Player[]>([]);
    const [ops, setOps] = useState<Player[]>([]);
    const [rejected, setRejected] = useState<RejectedPlayer[]>([]);
    const [onlinePlayers, setOnlinePlayers] = useState<Player[]>([]);

    // UI State
    const [loading, setLoading] = useState(true);
    const [refreshTrigger, setRefreshTrigger] = useState(0);
    const [toasts, setToasts] = useState<Toast[]>([]); // <--- NEW: Toast State

    // --- TOAST HELPER ---
    const showToast = (message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, message, type }]);

        // Auto-dismiss after 3 seconds
        setTimeout(() => {
            setToasts(prev => prev.filter(t => t.id !== id));
        }, 3000);
    };

    // --- FETCH DATA ---
    useEffect(() => {
        const fetchData = async () => {
            const token = localStorage.getItem('token');
            const headers = { 'Authorization': `Bearer ${token}` };

            try {
                const [resWhite, resBan, resOp, resRej, resStatus] = await Promise.all([
                    fetch('/api/players', { headers }),
                    fetch('/api/players/banned', { headers }),
                    fetch('/api/players/ops', { headers }),
                    fetch('/api/players/rejected', { headers }),
                    fetch('/status', { headers })
                ]);

                if (resWhite.ok) setWhitelist(await resWhite.json());
                if (resBan.ok) setBanned(await resBan.json());
                if (resOp.ok) setOps(await resOp.json());
                if (resRej.ok) setRejected(await resRej.json());
                if (resStatus.ok) {
                    const statusData = await resStatus.json();
                    setOnlinePlayers(statusData.player_list || []);
                }
            } catch (error) {
                console.error("Failed to fetch data", error);
                // We don't toast here to avoid spamming on load errors
            } finally {
                setLoading(false);
            }
        };
        fetchData();
    }, [refreshTrigger]);

    const refresh = () => setRefreshTrigger(prev => prev + 1);

    // --- ACTIONS (UPDATED) ---
    const apiCall = async (url: string, method: string, body?: Record<string, unknown>) => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch(url, {
                method,
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: body ? JSON.stringify(body) : undefined
            });

            // 1. Capture the Server Response
            let message = "";
            const text = await res.text();

            try {
                // Try to parse JSON { "status": "..." }
                const data = JSON.parse(text);
                message = data.status || text;
            } catch {
                // Fallback to plain text
                message = text;
            }

            // 2. Determine Success or Failure
            if (res.ok) {
                showToast(message || "Action Successful", 'success');
                refresh(); // Only refresh data on success
            } else {
                showToast(message || "Action Failed", 'error');
            }

        } catch (err) {
            console.error("API Call Failed: ", err);
            showToast("Network Error: Could not reach server", 'error');
        }
    };

    const handleWhitelist = (name: string, add: boolean) => {
        if (add) apiCall('/api/players', 'POST', { username: name });
        else apiCall(`/api/players?username=${name}`, 'DELETE');
    };

    const handleBan = (name: string, add: boolean) => {
        if (add) {
            const reason = prompt("Ban Reason:", "Violating rules");
            if (reason) apiCall('/api/players/banned', 'POST', { username: name, reason });
        } else {
            apiCall(`/api/players/banned?username=${name}`, 'DELETE');
        }
    };

    const handleOp = (name: string, add: boolean) => {
        const action = add ? 'add' : 'remove';
        apiCall(`/api/players/ops?action=${action}`, 'POST', { username: name });
    };

    const handleDismissRejected = (name: string) => {
        apiCall(`/api/players/rejected?username=${name}`, 'DELETE');
    };

    // --- MERGE LOGIC ---
    const allNames = new Set<string>([
        ...whitelist.map(p => p.name),
        ...banned.map(p => p.name),
        ...ops.map(p => p.name),
        ...onlinePlayers.map(p => p.name),
        ...rejected.map(p => p.username)
    ]);

    const unifiedList: UnifiedPlayer[] = Array.from(allNames).map(name => {
        const w = whitelist.find(p => p.name === name);
        const b = banned.find(p => p.name === name);
        const o = ops.find(p => p.name === name);
        const r = rejected.find(p => p.username === name);
        const onlineP = onlinePlayers.find(p => p.name === name);

        return {
            name,
            uuid: w?.uuid || onlineP?.uuid || o?.uuid || b?.uuid,
            isWhitelisted: !!w,
            isBanned: !!b,
            isOp: !!o,
            isOnline: !!onlineP,
            isRejected: !!r,
            reason: b?.reason,
            rejectionCount: r?.count
        };
    }).sort((a, b) => {
        if (a.isOnline !== b.isOnline) return a.isOnline ? -1 : 1;
        if (a.isRejected !== b.isRejected) return a.isRejected ? -1 : 1;
        return a.name.localeCompare(b.name);
    });

    return (
        <div className="space-y-6 relative min-h-[500px]">

            {/* TOAST CONTAINER (Fixed Bottom Right) */}
            <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-2">
                {toasts.map(toast => (
                    <div
                        key={toast.id}
                        className={`
                            flex items-center gap-3 px-4 py-3 rounded shadow-lg border backdrop-blur-md animate-slide-in
                            ${toast.type === 'success'
                                ? 'bg-green-900/80 border-green-500/50 text-green-100'
                                : 'bg-red-900/80 border-red-500/50 text-red-100'}
                        `}
                    >
                        {toast.type === 'success' ? <CheckCircle size={18} /> : <XCircle size={18} />}
                        <span className="font-mono text-sm">{toast.message}</span>
                    </div>
                ))}
            </div>

            {/* Header */}
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-3xl font-pixel text-mc-diamond">Player Manager</h1>
                    <p className="text-white/50 font-mono text-sm">Manage access, bans, and operators</p>
                </div>
                <button onClick={refresh} className="p-2 bg-white/5 hover:bg-white/10 rounded-full transition-colors">
                    <RefreshCw size={20} className={loading ? "animate-spin" : ""} />
                </button>
            </div>

            {/* REJECTED PLAYERS */}
            {rejected.length > 0 && (
                <div className="bg-red-900/20 border border-red-500/30 rounded-lg p-4 mb-6">
                    <h3 className="text-red-400 font-pixel text-lg mb-3 flex items-center gap-2">
                        <AlertTriangle size={20} />
                        Blocked Connection Attempts
                    </h3>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                        {rejected.map(p => (
                            <div key={p.username} className="bg-black/40 p-3 rounded flex items-center justify-between border border-red-500/20">
                                <div>
                                    <div className="font-mono text-white">{p.username}</div>
                                    <div className="text-xs text-red-300">{p.count} attempts • {new Date(p.last_seen).toLocaleTimeString()}</div>
                                </div>
                                <div className="flex gap-2">
                                    <button
                                        onClick={() => handleWhitelist(p.username, true)}
                                        title="Whitelist"
                                        className="p-2 bg-green-600/20 hover:bg-green-600/40 text-green-400 rounded transition-colors"
                                    >
                                        <UserPlus size={16} />
                                    </button>
                                    <button
                                        onClick={() => handleDismissRejected(p.username)}
                                        title="Dismiss"
                                        className="p-2 bg-white/5 hover:bg-white/10 text-white/50 rounded transition-colors"
                                    >
                                        <Trash2 size={16} />
                                    </button>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* MAIN PLAYER TABLE */}
            <div className="bg-black/60 border border-white/10 rounded-lg overflow-hidden backdrop-blur-md">
                <table className="w-full text-left border-collapse">
                    <thead>
                        <tr className="bg-white/5 text-white/50 text-xs uppercase tracking-wider font-mono">
                            <th className="p-4">Player</th>
                            <th className="p-4">Status</th>
                            <th className="p-4">Access</th>
                            <th className="p-4 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-white/5">
                        {unifiedList.map(player => (
                            <tr key={player.name} className="hover:bg-white/5 transition-colors group">
                                <td className="p-4">
                                    <div className="flex items-center gap-3">
                                        <img
                                            src={`https://api.mineatar.io/face/${player.uuid || player.name}`}
                                            alt={player.name}
                                            className="w-8 h-8 rounded bg-black/50"
                                        />
                                        <div>
                                            <div className="font-mono text-white flex items-center gap-2">
                                                {player.name}
                                                {player.isOp && <Shield size={14} className="text-mc-gold" />}
                                            </div>
                                            {player.uuid && <div className="text-xs text-white/30 font-mono">{player.uuid}</div>}
                                        </div>
                                    </div>
                                </td>
                                <td className="p-4">
                                    <div className="flex flex-col gap-1">
                                        {player.isOnline ? (
                                            <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-bold bg-green-500/20 text-green-400 border border-green-500/30 w-fit">
                                                <span className="w-1.5 h-1.5 rounded-full bg-green-400 animate-pulse"></span> ONLINE
                                            </span>
                                        ) : (
                                            <span className="text-xs text-white/30 font-mono">OFFLINE</span>
                                        )}
                                        {player.isBanned && (
                                            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-bold bg-red-500/20 text-red-400 border border-red-500/30 w-fit">
                                                BANNED
                                            </span>
                                        )}
                                    </div>
                                </td>
                                <td className="p-4">
                                    {player.isWhitelisted ? (
                                        <div className="flex items-center gap-2 text-green-400 text-sm">
                                            <UserCheck size={16} />
                                            <span>Whitelisted</span>
                                        </div>
                                    ) : (
                                        <div className="flex items-center gap-2 text-white/30 text-sm">
                                            <UserX size={16} />
                                            <span>Not Listed</span>
                                        </div>
                                    )}
                                </td>
                                <td className="p-4 text-right">
                                    <div className="flex items-center justify-end gap-2 opacity-100 sm:opacity-0 group-hover:opacity-100 transition-opacity">
                                        <button
                                            onClick={() => handleWhitelist(player.name, !player.isWhitelisted)}
                                            title={player.isWhitelisted ? "Remove from Whitelist" : "Whitelist"}
                                            className={`p-2 rounded transition-colors ${player.isWhitelisted ? 'bg-red-500/10 text-red-400 hover:bg-red-500/20' : 'bg-green-500/10 text-green-400 hover:bg-green-500/20'}`}
                                        >
                                            {player.isWhitelisted ? <UserX size={18} /> : <UserPlus size={18} />}
                                        </button>
                                        <button
                                            onClick={() => handleOp(player.name, !player.isOp)}
                                            title={player.isOp ? "De-Op" : "Make Operator"}
                                            className={`p-2 rounded transition-colors ${player.isOp ? 'bg-yellow-500/10 text-yellow-400 hover:bg-yellow-500/20' : 'bg-white/5 text-white/50 hover:bg-white/10'}`}
                                        >
                                            {player.isOp ? <ShieldAlert size={18} /> : <Shield size={18} />}
                                        </button>
                                        <button
                                            onClick={() => handleBan(player.name, !player.isBanned)}
                                            title={player.isBanned ? "Unban" : "Ban Player"}
                                            className={`p-2 rounded transition-colors ${player.isBanned ? 'bg-green-500/10 text-green-400 hover:bg-green-500/20' : 'bg-red-900/20 text-red-400 hover:bg-red-900/40'}`}
                                        >
                                            {player.isBanned ? <Play size={18} /> : <Ban size={18} />}
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
}
