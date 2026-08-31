import { useEffect, useState, useMemo } from 'react';
import { Shield, ShieldAlert, Trash2, UserPlus, UserCheck, UserX, AlertTriangle, Play, RefreshCw, Ban, XCircle, CheckCircle, Search, ChevronLeft, ChevronRight } from 'lucide-react';
import type { Player, RejectedPlayer, UnifiedPlayer } from '../types';

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

    // Search & Filter State
    const [searchQuery, setSearchQuery] = useState('');
    const [activeFilter, setActiveFilter] = useState<'all' | 'online' | 'whitelisted' | 'banned' | 'ops'>('all');

    // Pagination State
    const [currentPage, setCurrentPage] = useState(1);
    const [pageSize, setPageSize] = useState<number>(25);

    // Direct Add Player State
    const [newPlayerName, setNewPlayerName] = useState('');

    // UI State
    const [loading, setLoading] = useState(true);
    const [refreshTrigger, setRefreshTrigger] = useState(0);
    const [toasts, setToasts] = useState<Toast[]>([]);

    const showToast = (message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, message, type }]);
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
            } finally {
                setLoading(false);
            }
        };
        fetchData();
    }, [refreshTrigger]);

    const refresh = () => setRefreshTrigger(prev => prev + 1);

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

            let message = "";
            const text = await res.text();
            try {
                const data = JSON.parse(text);
                message = data.status || text;
            } catch {
                message = text;
            }

            if (res.ok) {
                showToast(message || "Action Successful", 'success');
                refresh();
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

    // --- UNIFIED LIST & FILTERING ---
    const allNames = useMemo(() => new Set<string>([
        ...whitelist.map(p => p.name),
        ...banned.map(p => p.name),
        ...ops.map(p => p.name),
        ...onlinePlayers.map(p => p.name),
        ...rejected.map(p => p.username)
    ]), [whitelist, banned, ops, onlinePlayers, rejected]);

    const unifiedList: UnifiedPlayer[] = useMemo(() => {
        return Array.from(allNames).map(name => {
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
    }, [allNames, whitelist, banned, ops, rejected, onlinePlayers]);

    // Filtered by Search & Tab
    const filteredPlayers = useMemo(() => {
        return unifiedList.filter(player => {
            // Tab filter
            if (activeFilter === 'online' && !player.isOnline) return false;
            if (activeFilter === 'whitelisted' && !player.isWhitelisted) return false;
            if (activeFilter === 'banned' && !player.isBanned) return false;
            if (activeFilter === 'ops' && !player.isOp) return false;

            // Search query
            if (searchQuery.trim()) {
                const query = searchQuery.toLowerCase();
                const matchName = player.name.toLowerCase().includes(query);
                const matchUUID = player.uuid ? player.uuid.toLowerCase().includes(query) : false;
                if (!matchName && !matchUUID) return false;
            }
            return true;
        });
    }, [unifiedList, activeFilter, searchQuery]);

    // Pagination calculations
    const totalPages = Math.ceil(filteredPlayers.length / pageSize) || 1;
    const paginatedPlayers = useMemo(() => {
        const start = (currentPage - 1) * pageSize;
        return filteredPlayers.slice(start, start + pageSize);
    }, [filteredPlayers, currentPage, pageSize]);

    // Reset page to 1 when filters change
    useEffect(() => {
        setCurrentPage(1);
    }, [searchQuery, activeFilter, pageSize]);

    return (
        <div className="space-y-6 relative min-h-[500px]">
            {/* TOAST CONTAINER */}
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
                    <p className="text-white/50 font-mono text-sm">Manage whitelist access, bans, and server operators</p>
                </div>
                <button onClick={refresh} className="p-2 bg-white/5 hover:bg-white/10 rounded-full transition-colors">
                    <RefreshCw size={20} className={loading ? "animate-spin" : ""} />
                </button>
            </div>

            {/* BLOCKED CONNECTION ATTEMPTS */}
            {rejected.length > 0 && (
                <div className="bg-red-900/20 border border-red-500/30 rounded-xl p-4 mb-6">
                    <h3 className="text-red-400 font-pixel text-lg mb-3 flex items-center gap-2">
                        <AlertTriangle size={20} /> Blocked Connection Attempts ({rejected.length})
                    </h3>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3 max-h-48 overflow-y-auto pr-1">
                        {rejected.map(p => (
                            <div key={p.username} className="bg-black/40 p-3 rounded-lg flex items-center justify-between border border-red-500/20">
                                <div>
                                    <div className="font-mono text-white text-sm font-bold">{p.username}</div>
                                    <div className="text-xs text-red-300">{p.count} attempts • {new Date(p.last_seen).toLocaleTimeString()}</div>
                                </div>
                                <div className="flex gap-2">
                                    <button
                                        onClick={() => handleWhitelist(p.username, true)}
                                        title="Whitelist Player"
                                        className="p-2 bg-green-600/20 hover:bg-green-600/40 text-green-400 rounded-lg transition-colors"
                                    >
                                        <UserPlus size={16} />
                                    </button>
                                    <button
                                        onClick={() => handleDismissRejected(p.username)}
                                        title="Dismiss"
                                        className="p-2 bg-white/5 hover:bg-white/10 text-white/50 rounded-lg transition-colors"
                                    >
                                        <Trash2 size={16} />
                                    </button>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* CONTROLS BAR: ADD PLAYER, SEARCH, TABS */}
            <div className="bg-black/60 border border-white/10 rounded-xl p-4 space-y-4 backdrop-blur-md">
                <div className="flex flex-col md:flex-row justify-between gap-4">
                    {/* Add Player Input */}
                    <form
                        onSubmit={(e) => {
                            e.preventDefault();
                            if (newPlayerName.trim()) {
                                handleWhitelist(newPlayerName.trim(), true);
                                setNewPlayerName('');
                            }
                        }}
                        className="flex gap-2"
                    >
                        <input
                            type="text"
                            value={newPlayerName}
                            onChange={(e) => setNewPlayerName(e.target.value)}
                            placeholder="Add Player Username..."
                            className="bg-black/50 border border-white/20 rounded-lg px-3 py-2 text-white font-mono text-sm focus:border-mc-diamond focus:outline-none w-64"
                        />
                        <button
                            type="submit"
                            disabled={!newPlayerName.trim()}
                            className="bg-green-600 hover:bg-green-500 disabled:opacity-50 text-white font-mono text-sm font-bold px-4 py-2 rounded-lg flex items-center gap-1.5 transition-colors"
                        >
                            <UserPlus size={16} /> Add Whitelist
                        </button>
                    </form>

                    {/* Search Input */}
                    <div className="relative flex-1 max-w-md">
                        <Search size={16} className="absolute left-3 top-3 text-white/40" />
                        <input
                            type="text"
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            placeholder="Search by player name or UUID..."
                            className="w-full bg-black/50 border border-white/20 rounded-lg pl-9 pr-4 py-2 text-white font-mono text-sm focus:border-mc-diamond focus:outline-none"
                        />
                    </div>
                </div>

                {/* Filter Tabs & Page Size Selector */}
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3 pt-2 border-t border-white/5">
                    <div className="flex flex-wrap gap-2">
                        {[
                            { key: 'all', label: `All (${unifiedList.length})` },
                            { key: 'online', label: `Online (${onlinePlayers.length})` },
                            { key: 'whitelisted', label: `Whitelisted (${whitelist.length})` },
                            { key: 'ops', label: `Operators (${ops.length})` },
                            { key: 'banned', label: `Banned (${banned.length})` },
                        ].map(tab => (
                            <button
                                key={tab.key}
                                onClick={() => setActiveFilter(tab.key as any)}
                                className={`px-3 py-1.5 rounded-lg text-xs font-mono transition-colors ${
                                    activeFilter === tab.key
                                        ? 'bg-mc-diamond/20 text-mc-diamond border border-mc-diamond/40 font-bold'
                                        : 'bg-white/5 text-white/60 hover:bg-white/10 hover:text-white'
                                }`}
                            >
                                {tab.label}
                            </button>
                        ))}
                    </div>

                    <div className="flex items-center gap-2 font-mono text-xs text-white/50 self-end sm:self-center">
                        <span>Show:</span>
                        <select
                            value={pageSize}
                            onChange={(e) => setPageSize(Number(e.target.value))}
                            className="bg-black/50 border border-white/20 rounded px-2 py-1 text-white font-mono focus:outline-none"
                        >
                            <option value={10}>10</option>
                            <option value={25}>25</option>
                            <option value={50}>50</option>
                            <option value={100}>100</option>
                        </select>
                        <span>per page</span>
                    </div>
                </div>
            </div>

            {/* MAIN PLAYER TABLE WITH SCROLLBAR */}
            <div className="bg-black/60 border border-white/10 rounded-xl overflow-hidden backdrop-blur-md">
                <div className="max-h-[550px] overflow-y-auto">
                    <table className="w-full text-left border-collapse">
                        <thead className="sticky top-0 bg-black/90 backdrop-blur-md border-b border-white/10 z-10">
                            <tr className="text-white/50 text-xs uppercase tracking-wider font-mono">
                                <th className="p-4">Player</th>
                                <th className="p-4">Status</th>
                                <th className="p-4">Access</th>
                                <th className="p-4 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/5">
                            {paginatedPlayers.length > 0 ? (
                                paginatedPlayers.map(player => (
                                    <tr key={player.name} className="hover:bg-white/5 transition-colors group">
                                        <td className="p-4">
                                            <div className="flex items-center gap-3">
                                                <img
                                                    src={`https://api.mineatar.io/face/${player.uuid || player.name}`}
                                                    alt={player.name}
                                                    className="w-8 h-8 rounded bg-black/50 shrink-0"
                                                />
                                                <div>
                                                    <div className="font-mono text-white flex items-center gap-2 font-bold text-sm">
                                                        {player.name}
                                                        {player.isOp && <Shield size={14} className="text-mc-gold" />}
                                                    </div>
                                                    {player.uuid && <div className="text-[11px] text-white/30 font-mono">{player.uuid}</div>}
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
                                            <div className="flex items-center justify-end gap-2">
                                                <button
                                                    onClick={() => handleWhitelist(player.name, !player.isWhitelisted)}
                                                    title={player.isWhitelisted ? "Remove from Whitelist" : "Whitelist"}
                                                    className={`p-2 rounded-lg transition-colors ${player.isWhitelisted ? 'bg-red-500/10 text-red-400 hover:bg-red-500/20' : 'bg-green-500/10 text-green-400 hover:bg-green-500/20'}`}
                                                >
                                                    {player.isWhitelisted ? <UserX size={18} /> : <UserPlus size={18} />}
                                                </button>
                                                <button
                                                    onClick={() => handleOp(player.name, !player.isOp)}
                                                    title={player.isOp ? "De-Op" : "Make Operator"}
                                                    className={`p-2 rounded-lg transition-colors ${player.isOp ? 'bg-yellow-500/10 text-yellow-400 hover:bg-yellow-500/20' : 'bg-white/5 text-white/50 hover:bg-white/10'}`}
                                                >
                                                    {player.isOp ? <ShieldAlert size={18} /> : <Shield size={18} />}
                                                </button>
                                                <button
                                                    onClick={() => handleBan(player.name, !player.isBanned)}
                                                    title={player.isBanned ? "Unban" : "Ban Player"}
                                                    className={`p-2 rounded-lg transition-colors ${player.isBanned ? 'bg-green-500/10 text-green-400 hover:bg-green-500/20' : 'bg-red-900/20 text-red-400 hover:bg-red-900/40'}`}
                                                >
                                                    {player.isBanned ? <Play size={18} /> : <Ban size={18} />}
                                                </button>
                                            </div>
                                        </td>
                                    </tr>
                                ))
                            ) : (
                                <tr>
                                    <td colSpan={4} className="p-8 text-center text-white/30 font-mono text-sm">
                                        No players found matching current search/filter.
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>

                {/* PAGINATION FOOTER */}
                <div className="p-4 border-t border-white/10 flex flex-col sm:flex-row justify-between items-center gap-3 font-mono text-xs text-white/60">
                    <div>
                        Showing {filteredPlayers.length > 0 ? (currentPage - 1) * pageSize + 1 : 0} to {Math.min(currentPage * pageSize, filteredPlayers.length)} of {filteredPlayers.length} players
                    </div>

                    <div className="flex items-center gap-2">
                        <button
                            onClick={() => setCurrentPage(prev => Math.max(prev - 1, 1))}
                            disabled={currentPage <= 1}
                            className="p-2 bg-white/5 hover:bg-white/10 disabled:opacity-30 disabled:cursor-not-allowed rounded-lg transition-colors text-white"
                        >
                            <ChevronLeft size={16} />
                        </button>
                        <span className="px-3 py-1 bg-black/40 rounded border border-white/10 text-white">
                            Page {currentPage} of {totalPages}
                        </span>
                        <button
                            onClick={() => setCurrentPage(prev => Math.min(prev + 1, totalPages))}
                            disabled={currentPage >= totalPages}
                            className="p-2 bg-white/5 hover:bg-white/10 disabled:opacity-30 disabled:cursor-not-allowed rounded-lg transition-colors text-white"
                        >
                            <ChevronRight size={16} />
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}
