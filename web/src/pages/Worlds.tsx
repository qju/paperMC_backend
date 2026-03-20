import { useEffect, useState } from 'react';
import { RefreshCw, Play, Plus, Globe, CheckCircle, XCircle } from 'lucide-react';

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

export default function Worlds() {
    const [activeWorld, setActiveWorld] = useState<string>('');
    const [inactiveWorlds, setInactiveWorlds] = useState<string[]>([]);
    const [newWorldName, setNewWorldName] = useState('');
    const [newWorldSeed, setNewWorldSeed] = useState('');
    
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

    const refresh = () => setRefreshTrigger(prev => prev + 1);

    useEffect(() => {
        const fetchWorlds = async () => {
            const token = localStorage.getItem('token');
            try {
                const res = await fetch('/api/worlds', {
                    headers: { 'Authorization': `Bearer ${token}` }
                });
                if (res.ok) {
                    const data = await res.json();
                    setActiveWorld(data.active_world);
                    setInactiveWorlds(data.inactive_worlds || []);
                }
            } catch (error) {
                console.error("Failed to fetch worlds", error);
            } finally {
                setLoading(false);
            }
        };
        fetchWorlds();
    }, [refreshTrigger]);

    const handleSwitchWorld = async (worldName: string, seed?: string) => {
        if (!confirm(`Are you sure you want to switch to "${worldName}"? The server will be restarted if it is running.`)) {
            return;
        }

        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/worlds/active', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ world_name: worldName, seed: seed || undefined })
            });

            const text = await res.text();
            let message = "";
            try {
                const data = JSON.parse(text);
                message = data.status || text;
            } catch {
                message = text;
            }

            if (res.ok) {
                showToast(message || "World switched successfully", 'success');
                setNewWorldName('');
                setNewWorldSeed('');
                refresh();
            } else {
                showToast(message || "Failed to switch world", 'error');
            }
        } catch (err) {
            console.error("API Call Failed: ", err);
            showToast("Network Error: Could not reach server", 'error');
        }
    };

    const handleCreateWorld = (e: React.FormEvent) => {
        e.preventDefault();
        if (!newWorldName.trim()) return;
        handleSwitchWorld(newWorldName.trim(), newWorldSeed.trim());
    };

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
                    <h1 className="text-3xl font-pixel text-mc-diamond">World Manager</h1>
                    <p className="text-white/50 font-mono text-sm">Switch and create new worlds</p>
                </div>
                <button onClick={refresh} className="p-2 bg-white/5 hover:bg-white/10 rounded-full transition-colors">
                    <RefreshCw size={20} className={loading ? "animate-spin" : ""} />
                </button>
            </div>

            {/* Create World Form */}
            <div className="bg-black/60 border border-white/10 rounded-lg p-6 backdrop-blur-md mb-6">
                <h2 className="text-xl font-pixel text-white mb-4 flex items-center gap-2">
                    <Plus size={24} className="text-green-400" /> Create & Switch to New World
                </h2>
                <form onSubmit={handleCreateWorld} className="flex gap-4">
                    <input
                        type="text"
                        value={newWorldName}
                        onChange={(e) => setNewWorldName(e.target.value)}
                        placeholder="New world name (e.g. survival_s2)"
                        className="flex-1 bg-black/50 border border-white/20 rounded px-4 py-2 text-white font-mono focus:outline-none focus:border-green-500 transition-colors"
                        required
                    />
                    <input
                        type="text"
                        value={newWorldSeed}
                        onChange={(e) => setNewWorldSeed(e.target.value)}
                        placeholder="Seed (Optional)"
                        className="flex-1 bg-black/50 border border-white/20 rounded px-4 py-2 text-white font-mono focus:outline-none focus:border-green-500 transition-colors"
                    />
                    <button
                        type="submit"
                        className="bg-green-600 hover:bg-green-500 text-white font-bold py-2 px-6 rounded transition-colors flex items-center gap-2"
                        disabled={!newWorldName.trim()}
                    >
                        <Play size={18} /> Create & Switch
                    </button>
                </form>
                <p className="text-white/40 text-sm mt-2 font-mono">
                    The server will generate the necessary files on the next startup.
                </p>
            </div>

            {/* Worlds List */}
            <div className="bg-black/60 border border-white/10 rounded-lg overflow-hidden backdrop-blur-md">
                <table className="w-full text-left border-collapse">
                    <thead>
                        <tr className="bg-white/5 text-white/50 text-xs uppercase tracking-wider font-mono">
                            <th className="p-4">World Name</th>
                            <th className="p-4">Status</th>
                            <th className="p-4 text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-white/5">
                        {/* Active World */}
                        {activeWorld && (
                            <tr className="bg-green-900/10">
                                <td className="p-4">
                                    <div className="flex items-center gap-3 font-mono text-green-400">
                                        <Globe size={20} />
                                        {activeWorld}
                                    </div>
                                </td>
                                <td className="p-4">
                                    <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-bold bg-green-500/20 text-green-400 border border-green-500/30">
                                        <span className="w-1.5 h-1.5 rounded-full bg-green-400 animate-pulse"></span> ACTIVE
                                    </span>
                                </td>
                                <td className="p-4 text-right">
                                    <span className="text-white/30 text-sm font-mono italic">Currently playing</span>
                                </td>
                            </tr>
                        )}

                        {/* Inactive Worlds */}
                        {inactiveWorlds.length > 0 ? (
                            inactiveWorlds.map(world => (
                                <tr key={world} className="hover:bg-white/5 transition-colors group">
                                    <td className="p-4">
                                        <div className="flex items-center gap-3 font-mono text-white/70">
                                            <Globe size={20} className="text-white/30" />
                                            {world}
                                        </div>
                                    </td>
                                    <td className="p-4">
                                        <span className="text-xs text-white/30 font-mono">INACTIVE</span>
                                    </td>
                                    <td className="p-4 text-right">
                                        <button
                                            onClick={() => handleSwitchWorld(world)}
                                            className="opacity-0 group-hover:opacity-100 transition-opacity bg-blue-600/20 hover:bg-blue-600/40 text-blue-400 font-bold py-1 px-4 rounded flex items-center gap-2 ml-auto"
                                        >
                                            <Play size={14} /> Switch
                                        </button>
                                    </td>
                                </tr>
                            ))
                        ) : (
                            <tr>
                                <td colSpan={3} className="p-8 text-center text-white/30 font-mono">
                                    No other worlds found.
                                </td>
                            </tr>
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    );
}
