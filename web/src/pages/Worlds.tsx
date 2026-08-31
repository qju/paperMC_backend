import { useEffect, useState } from 'react';
import { RefreshCw, Play, Plus, Globe, CheckCircle, XCircle, Copy, Trash2, HardDrive, Tag, Layers, Clock, ShieldAlert } from 'lucide-react';

interface WorldInfo {
    name: string;
    disk_path: string;
    size_bytes: number;
    formatted_size: string;
    is_active: boolean;
    format: string;
    dimensions: string[];
    last_played: string;
    minecraft_version?: string;
    game_mode?: string;
    difficulty?: string;
    hardcore: boolean;
}

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

export default function Worlds() {
    const [activeWorldName, setActiveWorldName] = useState<string>('');
    const [worlds, setWorlds] = useState<WorldInfo[]>([]);
    const [newWorldName, setNewWorldName] = useState('');
    const [newWorldSeed, setNewWorldSeed] = useState('');

    const [cloneSource, setCloneSource] = useState<string | null>(null);
    const [cloneTargetName, setCloneTargetName] = useState('');
    const [cloning, setCloning] = useState(false);

    const [loading, setLoading] = useState(true);
    const [refreshTrigger, setRefreshTrigger] = useState(0);
    const [toasts, setToasts] = useState<Toast[]>([]);

    const showToast = (message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, message, type }]);
        setTimeout(() => {
            setToasts(prev => prev.filter(t => t.id !== id));
        }, 3500);
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
                    setActiveWorldName(data.active_world);
                    setWorlds(data.worlds || []);
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
        if (!confirm(`Are you sure you want to switch to "${worldName}"? If running, the server will flush data and restart automatically.`)) {
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

            const data = await res.json();
            if (res.ok) {
                showToast(data.status || "Active world updated", 'success');
                setNewWorldName('');
                setNewWorldSeed('');
                refresh();
            } else {
                showToast(data.error || "Failed to switch world", 'error');
            }
        } catch (err) {
            console.error("API Call Failed: ", err);
            showToast("Network Error: Could not reach server", 'error');
        }
    };

    const handleDuplicate = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!cloneSource || !cloneTargetName.trim()) return;

        setCloning(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/worlds/duplicate', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    source_world: cloneSource,
                    target_world: cloneTargetName.trim()
                })
            });

            const data = await res.json();
            if (res.ok) {
                showToast(`World duplicated as "${cloneTargetName.trim()}"`, 'success');
                setCloneSource(null);
                setCloneTargetName('');
                refresh();
            } else {
                showToast(data.error || "Duplication failed", 'error');
            }
        } catch (err) {
            console.error("Duplication error: ", err);
            showToast("Network error during duplication", 'error');
        } finally {
            setCloning(false);
        }
    };

    const handleDelete = async (worldName: string) => {
        if (!confirm(`DANGER: Are you sure you want to permanently delete world "${worldName}"? This action cannot be undone.`)) {
            return;
        }

        const token = localStorage.getItem('token');
        try {
            const res = await fetch(`/api/worlds?name=${encodeURIComponent(worldName)}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${token}` }
            });

            const data = await res.json();
            if (res.ok) {
                showToast(`World "${worldName}" deleted`, 'success');
                refresh();
            } else {
                showToast(data.error || "Delete failed", 'error');
            }
        } catch (err) {
            console.error("Delete failed: ", err);
            showToast("Network error while deleting world", 'error');
        }
    };

    const handleCreateWorld = (e: React.FormEvent) => {
        e.preventDefault();
        if (!newWorldName.trim()) return;
        handleSwitchWorld(newWorldName.trim(), newWorldSeed.trim());
    };

    const activeWorld = worlds.find(w => w.is_active || w.name === activeWorldName);
    const inactiveWorlds = worlds.filter(w => !w.is_active && w.name !== activeWorldName);

    const formatDate = (isoString: string) => {
        if (!isoString) return 'Unknown';
        const d = new Date(isoString);
        return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
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

            {/* DUPLICATE MODAL */}
            {cloneSource && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
                    <div className="bg-black/90 border border-white/20 rounded-xl p-6 w-full max-w-md space-y-4 shadow-2xl">
                        <div className="flex justify-between items-center">
                            <h3 className="text-xl font-pixel text-mc-gold flex items-center gap-2">
                                <Copy size={20} /> Duplicate World
                            </h3>
                            <button onClick={() => setCloneSource(null)} className="text-white/50 hover:text-white">
                                <XCircle size={20} />
                            </button>
                        </div>
                        <p className="text-xs font-mono text-white/60">
                            Safely clone all chunk data and dimensions for <strong className="text-white">{cloneSource}</strong>.
                        </p>
                        <form onSubmit={handleDuplicate} className="space-y-4">
                            <div>
                                <label className="block text-xs uppercase font-mono text-white/50 mb-1">
                                    New World Directory Name
                                </label>
                                <input
                                    type="text"
                                    value={cloneTargetName}
                                    onChange={(e) => setCloneTargetName(e.target.value)}
                                    placeholder="e.g. survival_backup"
                                    required
                                    pattern="^[a-zA-Z0-9_\-]+$"
                                    className="w-full bg-black/60 border border-white/20 rounded p-3 text-white font-mono focus:border-mc-gold focus:outline-none"
                                />
                            </div>
                            <div className="flex justify-end gap-3 pt-2">
                                <button
                                    type="button"
                                    onClick={() => setCloneSource(null)}
                                    className="px-4 py-2 rounded font-mono text-sm bg-white/10 hover:bg-white/20 text-white"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={cloning || !cloneTargetName.trim()}
                                    className="px-4 py-2 rounded font-mono font-bold text-sm bg-green-600 hover:bg-green-500 text-white flex items-center gap-2"
                                >
                                    {cloning ? <RefreshCw size={16} className="animate-spin" /> : <Copy size={16} />}
                                    {cloning ? 'Cloning...' : 'Duplicate'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* Header */}
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-3xl font-pixel text-mc-diamond">World Management</h1>
                    <p className="text-white/50 font-mono text-sm">Inspect disk diagnostics, dimensions, version metadata, and switch worlds safely</p>
                </div>
                <button onClick={refresh} className="p-2 bg-white/5 hover:bg-white/10 rounded-full transition-colors">
                    <RefreshCw size={20} className={loading ? "animate-spin" : ""} />
                </button>
            </div>

            {/* ACTIVE WORLD SPOTLIGHT CARD */}
            {activeWorld && (
                <div className="bg-green-950/30 border-2 border-green-500/40 rounded-xl p-6 backdrop-blur-md shadow-xl relative overflow-hidden">
                    <div className="absolute top-0 right-0 px-4 py-1.5 bg-green-500/20 border-b border-l border-green-500/40 rounded-bl-lg font-mono text-xs font-bold text-green-400 flex items-center gap-1.5">
                        <span className="w-2 h-2 rounded-full bg-green-400 animate-pulse"></span> ACTIVE WORLD
                    </div>

                    <div className="flex flex-col lg:flex-row justify-between gap-6">
                        <div className="space-y-4">
                            <div className="flex items-center gap-3">
                                <Globe size={28} className="text-green-400" />
                                <div>
                                    <h2 className="text-2xl font-pixel text-white">{activeWorld.name}</h2>
                                    <p className="text-xs text-white/40 font-mono flex items-center gap-1 mt-0.5">
                                        <HardDrive size={13} /> {activeWorld.disk_path}
                                    </p>
                                </div>
                            </div>

                            {/* Stat Chips */}
                            <div className="flex flex-wrap gap-2 pt-2">
                                <span className="px-2.5 py-1 rounded bg-black/40 border border-white/10 text-xs font-mono text-mc-diamond flex items-center gap-1">
                                    <HardDrive size={13} /> {activeWorld.formatted_size}
                                </span>
                                <span className="px-2.5 py-1 rounded bg-black/40 border border-white/10 text-xs font-mono text-mc-gold flex items-center gap-1">
                                    <Tag size={13} /> {activeWorld.minecraft_version ? `MC ${activeWorld.minecraft_version}` : 'Version Unknown'}
                                </span>
                                <span className="px-2.5 py-1 rounded bg-black/40 border border-white/10 text-xs font-mono text-white/70 flex items-center gap-1">
                                    <Layers size={13} /> {activeWorld.format}
                                </span>
                                <span className="px-2.5 py-1 rounded bg-black/40 border border-white/10 text-xs font-mono text-white/70">
                                    Mode: {activeWorld.game_mode} ({activeWorld.difficulty})
                                </span>
                                {activeWorld.hardcore && (
                                    <span className="px-2.5 py-1 rounded bg-red-950/60 border border-red-500/40 text-xs font-mono text-red-300 flex items-center gap-1">
                                        <ShieldAlert size={13} /> Hardcore
                                    </span>
                                )}
                            </div>

                            {/* Dimensions */}
                            <div className="flex items-center gap-2 pt-1 font-mono text-xs text-white/50">
                                <span>Dimensions:</span>
                                {activeWorld.dimensions?.map(dim => (
                                    <span key={dim} className="px-2 py-0.5 rounded bg-white/10 text-white/80 uppercase text-[10px] tracking-wider">
                                        {dim.replace('the_', '')}
                                    </span>
                                ))}
                            </div>
                        </div>

                        {/* Action buttons */}
                        <div className="flex flex-row lg:flex-col justify-end items-end gap-3 self-end lg:self-center">
                            <button
                                onClick={() => {
                                    setCloneSource(activeWorld.name);
                                    setCloneTargetName(`${activeWorld.name}_backup`);
                                }}
                                className="px-4 py-2.5 bg-black/50 hover:bg-black/70 border border-white/20 rounded-lg text-white text-xs font-mono font-bold flex items-center gap-2 transition-colors"
                            >
                                <Copy size={16} className="text-mc-gold" /> Duplicate / Backup
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Create World Form */}
            <div className="bg-black/60 border border-white/10 rounded-xl p-6 backdrop-blur-md">
                <h2 className="text-xl font-pixel text-white mb-4 flex items-center gap-2">
                    <Plus size={24} className="text-green-400" /> Create & Switch to New World
                </h2>
                <form onSubmit={handleCreateWorld} className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <input
                        type="text"
                        value={newWorldName}
                        onChange={(e) => setNewWorldName(e.target.value)}
                        placeholder="World Name (e.g. survival_s2)"
                        className="bg-black/50 border border-white/20 rounded-lg px-4 py-3 text-white font-mono text-sm focus:outline-none focus:border-green-500 transition-colors"
                        required
                    />
                    <input
                        type="text"
                        value={newWorldSeed}
                        onChange={(e) => setNewWorldSeed(e.target.value)}
                        placeholder="Seed (Optional)"
                        className="bg-black/50 border border-white/20 rounded-lg px-4 py-3 text-white font-mono text-sm focus:outline-none focus:border-green-500 transition-colors"
                    />
                    <button
                        type="submit"
                        className="bg-green-600 hover:bg-green-500 text-white font-mono font-bold py-3 px-6 rounded-lg transition-colors flex items-center justify-center gap-2 shadow-lg"
                        disabled={!newWorldName.trim()}
                    >
                        <Play size={18} /> Create & Switch
                    </button>
                </form>
                <p className="text-white/40 text-xs mt-2 font-mono">
                    The server will generate the 26.1+ world structure on startup with all dimensions.
                </p>
            </div>

            {/* Inactive Worlds List */}
            <div className="bg-black/60 border border-white/10 rounded-xl overflow-hidden backdrop-blur-md">
                <div className="p-4 border-b border-white/10 flex justify-between items-center">
                    <h3 className="font-pixel text-lg text-white">Other Available Worlds ({inactiveWorlds.length})</h3>
                </div>

                <div className="divide-y divide-white/5">
                    {inactiveWorlds.length > 0 ? (
                        inactiveWorlds.map(world => (
                            <div key={world.name} className="p-5 hover:bg-white/5 transition-colors flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
                                <div className="space-y-2">
                                    <div className="flex items-center gap-3">
                                        <Globe size={20} className="text-white/40" />
                                        <span className="font-mono text-base font-bold text-white/90">{world.name}</span>
                                        <span className="text-[11px] font-mono text-mc-diamond bg-black/40 px-2 py-0.5 rounded border border-white/5">
                                            {world.formatted_size}
                                        </span>
                                        {world.minecraft_version && (
                                            <span className="text-[11px] font-mono text-mc-gold bg-black/40 px-2 py-0.5 rounded border border-white/5">
                                                MC {world.minecraft_version}
                                            </span>
                                        )}
                                        <span className="text-[11px] font-mono text-white/40 bg-black/40 px-2 py-0.5 rounded border border-white/5">
                                            {world.format}
                                        </span>
                                    </div>
                                    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs font-mono text-white/40">
                                        <span>Path: {world.disk_path}</span>
                                        <span>Mode: {world.game_mode} ({world.difficulty})</span>
                                        <span className="flex items-center gap-1">
                                            <Clock size={12} /> Last Played: {formatDate(world.last_played)}
                                        </span>
                                    </div>
                                </div>

                                <div className="flex items-center gap-2 self-end md:self-center shrink-0">
                                    <button
                                        onClick={() => {
                                            setCloneSource(world.name);
                                            setCloneTargetName(`${world.name}_copy`);
                                        }}
                                        title="Duplicate World"
                                        className="p-2 bg-white/5 hover:bg-white/10 text-mc-gold rounded-lg transition-colors"
                                    >
                                        <Copy size={16} />
                                    </button>
                                    <button
                                        onClick={() => handleDelete(world.name)}
                                        title="Delete World"
                                        className="p-2 bg-red-950/40 hover:bg-red-900/60 text-red-400 rounded-lg transition-colors border border-red-500/20"
                                    >
                                        <Trash2 size={16} />
                                    </button>
                                    <button
                                        onClick={() => handleSwitchWorld(world.name)}
                                        className="px-4 py-2 bg-blue-600/20 hover:bg-blue-600/40 text-blue-300 border border-blue-500/30 rounded-lg font-mono text-xs font-bold flex items-center gap-1.5 transition-colors"
                                    >
                                        <Play size={14} /> Switch
                                    </button>
                                </div>
                            </div>
                        ))
                    ) : (
                        <div className="p-8 text-center text-white/30 font-mono text-sm">
                            No other world directories found in server directory.
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
