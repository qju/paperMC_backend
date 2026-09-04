import React, { useEffect, useState, useMemo, useCallback } from 'react';
import {
    Puzzle, RefreshCw, Upload, Download, Trash2, CheckCircle2,
    XCircle, AlertTriangle, Search, ExternalLink, Smartphone,
    Globe, Play, Pause, ChevronRight, Check, ArrowUpRight
} from 'lucide-react';
import type { PluginInfo, BedrockBridgeStatus, ModrinthHit } from '../types';

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

export default function Plugins() {
    const [activeTab, setActiveTab] = useState<'bedrock' | 'installed' | 'market'>('bedrock');
    const [pluginsList, setPluginsList] = useState<PluginInfo[]>([]);
    const [bedrockStatus, setBedrockStatus] = useState<BedrockBridgeStatus | null>(null);
    const [loading, setLoading] = useState(true);
    const [refreshing, setRefreshing] = useState(false);
    const [toasts, setToasts] = useState<Toast[]>([]);

    // Filter & Search
    const [searchInstalled, setSearchInstalled] = useState('');

    // Marketplace state
    const [marketQuery, setMarketQuery] = useState('');
    const [marketResults, setMarketResults] = useState<ModrinthHit[]>([]);
    const [marketLoading, setMarketLoading] = useState(false);
    const [installingProject, setInstallingProject] = useState<string | null>(null);

    // Bedrock Bridge update state
    const [updatingGeyser, setUpdatingGeyser] = useState(false);
    const [updatingFloodgate, setUpdatingFloodgate] = useState(false);
    const [updatingBoth, setUpdatingBoth] = useState(false);

    // Upload state
    const [isUploadModalOpen, setIsUploadModalOpen] = useState(false);
    const [uploading, setUploading] = useState(false);
    const [selectedFile, setSelectedFile] = useState<File | null>(null);

    // Delete state
    const [deleteTarget, setDeleteTarget] = useState<PluginInfo | null>(null);
    const [deleting, setDeleting] = useState(false);

    const showToast = useCallback((message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, message, type }]);
        setTimeout(() => {
            setToasts(prev => prev.filter(t => t.id !== id));
        }, 3500);
    }, []);

    const fetchPlugins = useCallback(async () => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/plugins', {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                const data = await res.json();
                setPluginsList(data.plugins || []);
            } else {
                showToast('Failed to load plugins list', 'error');
            }
        } catch (err) {
            console.error('Fetch plugins error:', err);
            showToast('Network error fetching plugins', 'error');
        }
    }, [showToast]);

    const fetchBedrockStatus = useCallback(async () => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/plugins/geyser/status', {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                const data: BedrockBridgeStatus = await res.json();
                setBedrockStatus(data);
            } else {
                showToast('Failed to check Geyser & Floodgate status', 'error');
            }
        } catch (err) {
            console.error('Fetch Geyser status error:', err);
            showToast('Network error checking Geyser status', 'error');
        }
    }, [showToast]);

    const refreshData = async () => {
        setRefreshing(true);
        await Promise.all([fetchPlugins(), fetchBedrockStatus()]);
        setRefreshing(false);
    };

    useEffect(() => {
        const init = async () => {
            setLoading(true);
            await Promise.all([fetchPlugins(), fetchBedrockStatus()]);
            setLoading(false);
        };
        init();
    }, [fetchPlugins, fetchBedrockStatus]);

    // Search marketplace
    const handleSearchMarket = useCallback(async (q: string) => {
        setMarketLoading(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch(`/api/plugins/market/search?query=${encodeURIComponent(q)}&limit=24`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                const data = await res.json();
                setMarketResults(data.hits || []);
            } else {
                showToast('Failed to search Modrinth marketplace', 'error');
            }
        } catch (err) {
            console.error('Marketplace search error:', err);
            showToast('Network error querying Modrinth', 'error');
        } finally {
            setMarketLoading(false);
        }
    }, [showToast]);

    useEffect(() => {
        if (activeTab === 'market' && marketResults.length === 0) {
            handleSearchMarket('');
        }
    }, [activeTab, marketResults.length, handleSearchMarket]);

    const handleTogglePlugin = async (filename: string) => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/plugins/toggle', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ filename })
            });

            if (res.ok) {
                const data = await res.json();
                showToast(`Plugin toggled to ${data.new_filename}`, 'success');
                await Promise.all([fetchPlugins(), fetchBedrockStatus()]);
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to toggle plugin', 'error');
            }
        } catch (err) {
            console.error('Toggle error:', err);
            showToast('Network error toggling plugin', 'error');
        }
    };

    const handleDeletePlugin = async () => {
        if (!deleteTarget) return;
        setDeleting(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch(`/api/plugins?filename=${encodeURIComponent(deleteTarget.filename)}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${token}` }
            });

            if (res.ok) {
                showToast(`Plugin ${deleteTarget.filename} deleted`, 'success');
                setDeleteTarget(null);
                await Promise.all([fetchPlugins(), fetchBedrockStatus()]);
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to delete plugin', 'error');
            }
        } catch (err) {
            console.error('Delete error:', err);
            showToast('Network error deleting plugin', 'error');
        } finally {
            setDeleting(false);
        }
    };

    const handleUploadPlugin = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!selectedFile) {
            showToast('Please select a .jar file to upload', 'error');
            return;
        }

        setUploading(true);
        const token = localStorage.getItem('token');
        const formData = new FormData();
        formData.append('file', selectedFile);

        try {
            const res = await fetch('/api/plugins/upload', {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}` },
                body: formData
            });

            if (res.ok) {
                const created: PluginInfo = await res.json();
                showToast(`Plugin '${created.name}' installed successfully!`, 'success');
                setIsUploadModalOpen(false);
                setSelectedFile(null);
                await Promise.all([fetchPlugins(), fetchBedrockStatus()]);
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to upload plugin', 'error');
            }
        } catch (err) {
            console.error('Upload error:', err);
            showToast('Network error uploading plugin', 'error');
        } finally {
            setUploading(false);
        }
    };

    const handleUpdateBedrock = async (target: 'geyser' | 'floodgate' | 'both') => {
        if (target === 'geyser') setUpdatingGeyser(true);
        if (target === 'floodgate') setUpdatingFloodgate(true);
        if (target === 'both') setUpdatingBoth(true);

        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/plugins/geyser/update', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ target })
            });

            if (res.ok) {
                showToast(`Successfully updated ${target === 'both' ? 'Geyser & Floodgate' : target}!`, 'success');
                await Promise.all([fetchPlugins(), fetchBedrockStatus()]);
            } else {
                const err = await res.json();
                showToast(err.error || `Failed to update ${target}`, 'error');
            }
        } catch (err) {
            console.error('Update Bedrock error:', err);
            showToast('Network error updating Bedrock bridge', 'error');
        } finally {
            setUpdatingGeyser(false);
            setUpdatingFloodgate(false);
            setUpdatingBoth(false);
        }
    };

    const handleInstallMarketPlugin = async (projectID: string, title: string) => {
        setInstallingProject(projectID);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/plugins/market/install', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ project_id: projectID })
            });

            if (res.ok) {
                showToast(`Plugin '${title}' installed successfully!`, 'success');
                await Promise.all([fetchPlugins(), fetchBedrockStatus()]);
            } else {
                const err = await res.json();
                showToast(err.error || `Failed to install ${title}`, 'error');
            }
        } catch (err) {
            console.error('Install error:', err);
            showToast('Network error installing plugin', 'error');
        } finally {
            setInstallingProject(null);
        }
    };

    // Filtered installed plugins
    const filteredInstalled = useMemo(() => {
        if (!searchInstalled.trim()) return pluginsList;
        const q = searchInstalled.toLowerCase();
        return pluginsList.filter(p =>
            p.name.toLowerCase().includes(q) ||
            p.filename.toLowerCase().includes(q) ||
            (p.description && p.description.toLowerCase().includes(q)) ||
            (p.authors && p.authors.some(a => a.toLowerCase().includes(q)))
        );
    }, [pluginsList, searchInstalled]);

    const activePluginsCount = pluginsList.filter(p => p.is_enabled).length;

    return (
        <div className="space-y-6 max-w-7xl mx-auto w-full pb-12">
            {/* TOAST ALERTS */}
            <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
                {toasts.map(t => (
                    <div
                        key={t.id}
                        className={`flex items-center gap-3 px-4 py-3 rounded-md shadow-2xl border backdrop-blur-md text-sm font-mono animate-fade-in ${
                            t.type === 'success'
                                ? 'bg-mc-darkgreen/90 text-green-200 border-green-500/40'
                                : 'bg-red-950/90 text-red-200 border-red-500/40'
                        }`}
                    >
                        {t.type === 'success' ? <CheckCircle2 size={18} /> : <XCircle size={18} />}
                        {t.message}
                    </div>
                ))}
            </div>

            {/* HEADER */}
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-white/10 pb-4">
                <div>
                    <h1 className="text-2xl md:text-3xl font-pixel text-white flex items-center gap-3">
                        <Puzzle className="text-mc-diamond" size={28} />
                        Plugin Manager & Bedrock Bridge
                    </h1>
                    <p className="text-sm text-white/50 font-mono mt-1">
                        Paper/Spigot plugin control, Modrinth marketplace, and dedicated Geyser & Floodgate Bedrock compatibility monitor.
                    </p>
                </div>

                <div className="flex items-center gap-3">
                    <button
                        onClick={refreshData}
                        disabled={refreshing}
                        className="flex items-center gap-2 px-3 py-2 rounded bg-white/5 hover:bg-white/10 border border-white/10 text-white/80 transition-colors text-sm font-mono"
                        title="Refresh plugins and Geyser status"
                    >
                        <RefreshCw size={16} className={refreshing ? 'animate-spin' : ''} />
                        Refresh
                    </button>
                    <button
                        onClick={() => setIsUploadModalOpen(true)}
                        className="flex items-center gap-2 px-4 py-2 rounded bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/50 text-mc-diamond font-mono text-sm transition-colors shadow-lg shadow-mc-diamond/10"
                    >
                        <Upload size={16} />
                        Upload Plugin (.jar)
                    </button>
                </div>
            </div>

            {/* TAB SWITCHER */}
            <div className="flex items-center gap-2 border-b border-white/10 pb-2 font-mono text-sm">
                <button
                    onClick={() => setActiveTab('bedrock')}
                    className={`flex items-center gap-2 px-4 py-2 rounded transition-colors ${
                        activeTab === 'bedrock'
                            ? 'bg-white/10 text-white border border-white/20'
                            : 'text-white/60 hover:text-white hover:bg-white/5'
                    }`}
                >
                    <Smartphone size={16} className={activeTab === 'bedrock' ? 'text-emerald-400' : ''} />
                    Bedrock Bridge (Geyser)
                    {bedrockStatus?.overall_status === 'update_available' && (
                        <span className="ml-1.5 px-2 py-0.2 text-[10px] rounded-full bg-amber-500/20 text-amber-300 border border-amber-500/40 animate-pulse">
                            Update Available
                        </span>
                    )}
                </button>

                <button
                    onClick={() => setActiveTab('installed')}
                    className={`flex items-center gap-2 px-4 py-2 rounded transition-colors ${
                        activeTab === 'installed'
                            ? 'bg-white/10 text-white border border-white/20'
                            : 'text-white/60 hover:text-white hover:bg-white/5'
                    }`}
                >
                    <Puzzle size={16} className={activeTab === 'installed' ? 'text-mc-diamond' : ''} />
                    Installed Plugins
                    <span className="ml-1.5 px-2 py-0.5 text-xs rounded-full bg-white/10 text-white/80">
                        {activePluginsCount}/{pluginsList.length}
                    </span>
                </button>

                <button
                    onClick={() => setActiveTab('market')}
                    className={`flex items-center gap-2 px-4 py-2 rounded transition-colors ${
                        activeTab === 'market'
                            ? 'bg-white/10 text-white border border-white/20'
                            : 'text-white/60 hover:text-white hover:bg-white/5'
                    }`}
                >
                    <Globe size={16} className={activeTab === 'market' ? 'text-mc-diamond' : ''} />
                    Modrinth Marketplace
                </button>
            </div>

            {/* TAB 1: BEDROCK BRIDGE (GEYSER & FLOODGATE) */}
            {activeTab === 'bedrock' && (
                <div className="space-y-6">
                    {/* BEDROCK COMPATIBILITY SPOTLIGHT BANNER */}
                    <div className="bg-gradient-to-r from-emerald-950/40 via-black/60 to-zinc-950 border border-emerald-500/30 rounded-lg p-6 shadow-xl relative overflow-hidden">
                        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-6 relative z-10">
                            <div className="space-y-2">
                                <div className="flex items-center gap-2.5">
                                    <div className="p-2 rounded bg-emerald-500/20 border border-emerald-500/40 text-emerald-300">
                                        <Smartphone size={24} />
                                    </div>
                                    <div>
                                        <h2 className="text-xl font-mono font-bold text-white flex items-center gap-2">
                                            Minecraft Bedrock Client Compatibility
                                        </h2>
                                        <div className="flex items-center gap-2 mt-0.5">
                                            <span className="text-xs font-mono text-emerald-400 font-semibold bg-emerald-500/10 border border-emerald-500/30 px-2 py-0.5 rounded">
                                                {bedrockStatus?.geyser.supported_bedrock || 'Bedrock v1.21.x / v26.x (Latest)'}
                                            </span>
                                            <span className="text-xs font-mono text-white/50">
                                                Latest Supported: <strong className="text-white">{bedrockStatus?.geyser.latest_bedrock_ver || 'Latest'}</strong>
                                            </span>
                                        </div>
                                    </div>
                                </div>
                                <p className="text-xs text-white/60 font-mono max-w-2xl leading-relaxed">
                                    Geyser allows Minecraft: Bedrock Edition players (iOS, Android, Xbox, PlayStation, Switch, Windows 10/11) to seamlessly join your Paper Java server without installing mods.
                                </p>
                            </div>

                            <div className="flex items-center gap-3">
                                <button
                                    onClick={() => handleUpdateBedrock('both')}
                                    disabled={updatingBoth}
                                    className="flex items-center gap-2 px-5 py-2.5 rounded bg-emerald-600 hover:bg-emerald-500 text-white font-mono text-sm font-semibold shadow-lg shadow-emerald-950/50 transition-colors disabled:opacity-50"
                                >
                                    {updatingBoth ? (
                                        <RefreshCw size={16} className="animate-spin" />
                                    ) : (
                                        <Download size={16} />
                                    )}
                                    {updatingBoth ? 'Updating Both...' : '1-Click Update Geyser & Floodgate'}
                                </button>
                            </div>
                        </div>
                    </div>

                    {/* TWO CARDS: GEYSER & FLOODGATE */}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                        {/* GEYSER CARD */}
                        <div className="bg-black/50 border border-white/15 rounded-lg p-6 flex flex-col justify-between space-y-4 shadow-lg">
                            <div className="space-y-4">
                                <div className="flex items-start justify-between">
                                    <div className="flex items-center gap-3">
                                        <div className="w-10 h-10 rounded bg-emerald-500/20 border border-emerald-500/40 flex items-center justify-center font-pixel text-emerald-400 text-xl">
                                            G
                                        </div>
                                        <div>
                                            <h3 className="text-lg font-mono font-bold text-white flex items-center gap-2">
                                                Geyser-Spigot
                                                <a
                                                    href="https://geysermc.org"
                                                    target="_blank"
                                                    rel="noreferrer"
                                                    className="text-white/40 hover:text-white"
                                                    title="GeyserMC Official Website"
                                                >
                                                    <ExternalLink size={14} />
                                                </a>
                                            </h3>
                                            <p className="text-xs text-white/50 font-mono">Bedrock translation proxy</p>
                                        </div>
                                    </div>

                                    {/* Status Badge */}
                                    {bedrockStatus?.geyser.installed ? (
                                        <span className={`px-2.5 py-1 rounded text-xs font-mono font-bold border flex items-center gap-1.5 ${
                                            bedrockStatus.geyser.is_enabled
                                                ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40'
                                                : 'bg-amber-500/20 text-amber-300 border-amber-500/40'
                                        }`}>
                                            <Check size={12} />
                                            {bedrockStatus.geyser.is_enabled ? 'INSTALLED & ACTIVE' : 'DISABLED'}
                                        </span>
                                    ) : (
                                        <span className="px-2.5 py-1 rounded text-xs font-mono font-bold bg-white/5 text-white/40 border border-white/10">
                                            NOT INSTALLED
                                        </span>
                                    )}
                                </div>

                                {/* VERSION & UPSTREAM METRICS */}
                                <div className="bg-white/5 border border-white/5 rounded p-4 font-mono text-xs space-y-2">
                                    <div className="flex items-center justify-between">
                                        <span className="text-white/50">Installed Version:</span>
                                        <span className="text-white font-semibold">
                                            {bedrockStatus?.geyser.installed ? (
                                                bedrockStatus.geyser.installed_build ? (
                                                    `Build #${bedrockStatus.geyser.installed_build}`
                                                ) : (
                                                    bedrockStatus.geyser.installed_version || 'Detected'
                                                )
                                            ) : (
                                                'None'
                                            )}
                                        </span>
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <span className="text-white/50">Latest Upstream:</span>
                                        <span className="text-mc-diamond font-bold">
                                            {bedrockStatus?.geyser.latest_build ? `Build #${bedrockStatus.geyser.latest_build}` : 'Checking...'}
                                        </span>
                                    </div>
                                    <div className="flex items-center justify-between pt-1 border-t border-white/5">
                                        <span className="text-white/50">Bedrock Compatibility:</span>
                                        <span className="text-emerald-400 font-medium">
                                            {bedrockStatus?.geyser.supported_bedrock || 'v1.21.x'}
                                        </span>
                                    </div>
                                </div>

                                {/* RECENT COMMITS / CHANGELOG */}
                                {bedrockStatus?.geyser.recent_changes && bedrockStatus.geyser.recent_changes.length > 0 && (
                                    <div className="space-y-1.5 font-mono text-xs">
                                        <span className="text-white/40 uppercase text-[11px]">Recent Upstream Changes</span>
                                        <div className="bg-black/60 border border-white/10 rounded p-2.5 space-y-1 text-white/70 max-h-24 overflow-y-auto custom-scrollbar text-[11px]">
                                            {bedrockStatus.geyser.recent_changes.map((ch, idx) => (
                                                <div key={idx} className="flex items-start gap-1.5">
                                                    <ChevronRight size={12} className="text-emerald-400 shrink-0 mt-0.5" />
                                                    <span className="truncate">{ch}</span>
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                )}
                            </div>

                            {/* UPDATE BUTTON */}
                            <div className="pt-3 border-t border-white/10 flex items-center justify-between">
                                <span className="text-xs font-mono text-white/40">
                                    Official GeyserMC v2 Build
                                </span>
                                <button
                                    onClick={() => handleUpdateBedrock('geyser')}
                                    disabled={updatingGeyser}
                                    className="flex items-center gap-1.5 px-4 py-2 rounded bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/50 text-mc-diamond font-mono text-xs transition-colors"
                                >
                                    {updatingGeyser ? (
                                        <RefreshCw size={13} className="animate-spin" />
                                    ) : (
                                        <Download size={13} />
                                    )}
                                    {updatingGeyser
                                        ? 'Updating...'
                                        : bedrockStatus?.geyser.installed
                                            ? bedrockStatus.geyser.update_available
                                                ? 'Update Geyser'
                                                : 'Reinstall Latest'
                                            : 'Install Geyser'}
                                </button>
                            </div>
                        </div>

                        {/* FLOODGATE CARD */}
                        <div className="bg-black/50 border border-white/15 rounded-lg p-6 flex flex-col justify-between space-y-4 shadow-lg">
                            <div className="space-y-4">
                                <div className="flex items-start justify-between">
                                    <div className="flex items-center gap-3">
                                        <div className="w-10 h-10 rounded bg-cyan-500/20 border border-cyan-500/40 flex items-center justify-center font-pixel text-cyan-400 text-xl">
                                            F
                                        </div>
                                        <div>
                                            <h3 className="text-lg font-mono font-bold text-white flex items-center gap-2">
                                                Floodgate-Spigot
                                                <a
                                                    href="https://geysermc.org/wiki/floodgate/"
                                                    target="_blank"
                                                    rel="noreferrer"
                                                    className="text-white/40 hover:text-white"
                                                    title="Floodgate Documentation"
                                                >
                                                    <ExternalLink size={14} />
                                                </a>
                                            </h3>
                                            <p className="text-xs text-white/50 font-mono">Passwordless Bedrock auth & skins</p>
                                        </div>
                                    </div>

                                    {/* Status Badge */}
                                    {bedrockStatus?.floodgate.installed ? (
                                        <span className={`px-2.5 py-1 rounded text-xs font-mono font-bold border flex items-center gap-1.5 ${
                                            bedrockStatus.floodgate.is_enabled
                                                ? 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40'
                                                : 'bg-amber-500/20 text-amber-300 border-amber-500/40'
                                        }`}>
                                            <Check size={12} />
                                            {bedrockStatus.floodgate.is_enabled ? 'INSTALLED & ACTIVE' : 'DISABLED'}
                                        </span>
                                    ) : (
                                        <span className="px-2.5 py-1 rounded text-xs font-mono font-bold bg-white/5 text-white/40 border border-white/10">
                                            NOT INSTALLED
                                        </span>
                                    )}
                                </div>

                                {/* VERSION & UPSTREAM METRICS */}
                                <div className="bg-white/5 border border-white/5 rounded p-4 font-mono text-xs space-y-2">
                                    <div className="flex items-center justify-between">
                                        <span className="text-white/50">Installed Version:</span>
                                        <span className="text-white font-semibold">
                                            {bedrockStatus?.floodgate.installed ? (
                                                bedrockStatus.floodgate.installed_build ? (
                                                    `Build #${bedrockStatus.floodgate.installed_build}`
                                                ) : (
                                                    bedrockStatus.floodgate.installed_version || 'Detected'
                                                )
                                            ) : (
                                                'None'
                                            )}
                                        </span>
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <span className="text-white/50">Latest Upstream:</span>
                                        <span className="text-cyan-400 font-bold">
                                            {bedrockStatus?.floodgate.latest_build ? `Build #${bedrockStatus.floodgate.latest_build}` : 'Checking...'}
                                        </span>
                                    </div>
                                    <div className="flex items-center justify-between pt-1 border-t border-white/5">
                                        <span className="text-white/50">Authentication:</span>
                                        <span className="text-cyan-300 font-medium">
                                            Xbox Live Auth & Skin Relay
                                        </span>
                                    </div>
                                </div>

                                {/* RECENT COMMITS */}
                                {bedrockStatus?.floodgate.recent_changes && bedrockStatus.floodgate.recent_changes.length > 0 && (
                                    <div className="space-y-1.5 font-mono text-xs">
                                        <span className="text-white/40 uppercase text-[11px]">Recent Upstream Changes</span>
                                        <div className="bg-black/60 border border-white/10 rounded p-2.5 space-y-1 text-white/70 max-h-24 overflow-y-auto custom-scrollbar text-[11px]">
                                            {bedrockStatus.floodgate.recent_changes.map((ch, idx) => (
                                                <div key={idx} className="flex items-start gap-1.5">
                                                    <ChevronRight size={12} className="text-cyan-400 shrink-0 mt-0.5" />
                                                    <span className="truncate">{ch}</span>
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                )}
                            </div>

                            {/* UPDATE BUTTON */}
                            <div className="pt-3 border-t border-white/10 flex items-center justify-between">
                                <span className="text-xs font-mono text-white/40">
                                    Official GeyserMC v2 Build
                                </span>
                                <button
                                    onClick={() => handleUpdateBedrock('floodgate')}
                                    disabled={updatingFloodgate}
                                    className="flex items-center gap-1.5 px-4 py-2 rounded bg-cyan-500/20 hover:bg-cyan-500/30 border border-cyan-500/50 text-cyan-300 font-mono text-xs transition-colors"
                                >
                                    {updatingFloodgate ? (
                                        <RefreshCw size={13} className="animate-spin" />
                                    ) : (
                                        <Download size={13} />
                                    )}
                                    {updatingFloodgate
                                        ? 'Updating...'
                                        : bedrockStatus?.floodgate.installed
                                            ? bedrockStatus.floodgate.update_available
                                                ? 'Update Floodgate'
                                                : 'Reinstall Latest'
                                            : 'Install Floodgate'}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* TAB 2: INSTALLED PLUGINS */}
            {activeTab === 'installed' && (
                <div className="space-y-4">
                    {/* SEARCH & CONTROLS */}
                    <div className="bg-black/40 border border-white/10 rounded-lg p-4 flex flex-col md:flex-row md:items-center justify-between gap-4 font-mono text-xs">
                        <div className="relative flex-1 max-w-md">
                            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-white/40" />
                            <input
                                type="text"
                                placeholder="Search installed plugins by name, author, or description..."
                                value={searchInstalled}
                                onChange={(e) => setSearchInstalled(e.target.value)}
                                className="w-full bg-black/60 border border-white/15 rounded pl-9 pr-3 py-2 text-white placeholder-white/40 focus:outline-none focus:border-mc-diamond text-xs"
                            />
                        </div>

                        <div className="flex items-center gap-3 text-white/60">
                            <span>
                                Active: <strong className="text-emerald-400">{activePluginsCount}</strong>
                            </span>
                            <span>
                                Disabled: <strong className="text-amber-400">{pluginsList.length - activePluginsCount}</strong>
                            </span>
                            <span>
                                Total: <strong className="text-white">{pluginsList.length}</strong>
                            </span>
                        </div>
                    </div>

                    {/* PLUGINS LIST */}
                    {loading ? (
                        <div className="text-center py-16 text-white/40 font-mono flex items-center justify-center gap-3">
                            <RefreshCw className="animate-spin" size={20} />
                            Scanning plugins folder...
                        </div>
                    ) : filteredInstalled.length === 0 ? (
                        <div className="bg-black/40 border border-white/10 rounded-lg p-12 text-center font-mono">
                            <Puzzle className="mx-auto text-white/20 mb-4" size={48} />
                            <h3 className="text-lg text-white/80 mb-2">No Installed Plugins Found</h3>
                            <p className="text-sm text-white/50 max-w-md mx-auto mb-6">
                                Upload custom plugin jars or browse the Modrinth marketplace to install plugins directly to your server.
                            </p>
                            <button
                                onClick={() => setIsUploadModalOpen(true)}
                                className="inline-flex items-center gap-2 px-4 py-2 rounded bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/50 text-mc-diamond text-xs transition-colors"
                            >
                                <Upload size={14} />
                                Upload Plugin
                            </button>
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            {filteredInstalled.map(plugin => {
                                const isEnabled = plugin.is_enabled;
                                return (
                                    <div
                                        key={plugin.filename}
                                        className={`bg-black/50 border rounded-lg p-5 transition-all flex flex-col justify-between ${
                                            isEnabled
                                                ? 'border-white/15 hover:border-white/30 shadow-lg'
                                                : 'border-white/5 opacity-70 bg-black/30'
                                        }`}
                                    >
                                        <div>
                                            <div className="flex items-start justify-between gap-3 mb-2">
                                                <div>
                                                    <h3 className="font-mono font-bold text-white text-base flex items-center gap-2">
                                                        {plugin.name}
                                                        <span className="text-xs px-2 py-0.5 rounded bg-white/10 text-mc-diamond font-normal">
                                                            v{plugin.version}
                                                        </span>
                                                    </h3>
                                                    {plugin.authors && plugin.authors.length > 0 && (
                                                        <p className="text-xs text-white/40 font-mono mt-0.5">
                                                            by {plugin.authors.join(', ')}
                                                        </p>
                                                    )}
                                                </div>

                                                {/* TOGGLE SWITCH */}
                                                <button
                                                    onClick={() => handleTogglePlugin(plugin.filename)}
                                                    className={`px-3 py-1 rounded-full text-xs font-mono font-semibold flex items-center gap-1.5 transition-colors border ${
                                                        isEnabled
                                                            ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40 hover:bg-emerald-500/30'
                                                            : 'bg-white/5 text-white/40 border-white/10 hover:bg-white/10 hover:text-white/60'
                                                    }`}
                                                    title={isEnabled ? "Click to Disable (.jar.disabled)" : "Click to Enable (.jar)"}
                                                >
                                                    {isEnabled ? <Play size={12} /> : <Pause size={12} />}
                                                    {isEnabled ? 'ACTIVE' : 'DISABLED'}
                                                </button>
                                            </div>

                                            {plugin.description && (
                                                <p className="text-xs text-white/70 font-mono my-2 line-clamp-2 leading-relaxed">
                                                    {plugin.description}
                                                </p>
                                            )}

                                            <div className="flex flex-wrap items-center gap-2 mt-3 text-[11px] font-mono text-white/50">
                                                <span className="bg-white/5 px-2 py-0.5 rounded">
                                                    {plugin.formatted_size}
                                                </span>
                                                <span className="bg-white/5 px-2 py-0.5 rounded truncate max-w-[220px]" title={plugin.filename}>
                                                    {plugin.filename}
                                                </span>
                                                {plugin.api_version && (
                                                    <span className="bg-white/5 px-2 py-0.5 rounded text-white/60">
                                                        API: {plugin.api_version}
                                                    </span>
                                                )}
                                            </div>
                                        </div>

                                        <div className="flex items-center justify-between pt-3 border-t border-white/10 mt-3 font-mono text-xs">
                                            <span className="text-white/40 text-[11px]">
                                                Updated {new Date(plugin.mod_time).toLocaleDateString()}
                                            </span>

                                            <button
                                                onClick={() => setDeleteTarget(plugin)}
                                                className="p-1.5 rounded bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20 transition-colors"
                                                title="Delete Plugin"
                                            >
                                                <Trash2 size={14} />
                                            </button>
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                </div>
            )}

            {/* TAB 3: MODRINTH MARKETPLACE */}
            {activeTab === 'market' && (
                <div className="space-y-4">
                    {/* SEARCH INPUT */}
                    <div className="bg-black/40 border border-white/10 rounded-lg p-4 font-mono">
                        <form
                            onSubmit={(e) => {
                                e.preventDefault();
                                handleSearchMarket(marketQuery);
                            }}
                            className="flex items-center gap-3"
                        >
                            <div className="relative flex-1">
                                <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-white/40" />
                                <input
                                    type="text"
                                    placeholder="Search Modrinth for Paper & Spigot plugins (e.g. spark, luckperms, vault, coreprotect)..."
                                    value={marketQuery}
                                    onChange={(e) => setMarketQuery(e.target.value)}
                                    className="w-full bg-black/60 border border-white/15 rounded pl-9 pr-3 py-2 text-white placeholder-white/40 focus:outline-none focus:border-mc-diamond text-xs font-mono"
                                />
                            </div>
                            <button
                                type="submit"
                                disabled={marketLoading}
                                className="flex items-center gap-2 px-4 py-2 rounded bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/50 text-mc-diamond text-xs font-mono transition-colors"
                            >
                                {marketLoading ? <RefreshCw size={14} className="animate-spin" /> : <Search size={14} />}
                                Search
                            </button>
                        </form>
                    </div>

                    {/* MARKETPLACE RESULTS */}
                    {marketLoading ? (
                        <div className="text-center py-16 text-white/40 font-mono flex items-center justify-center gap-3">
                            <RefreshCw className="animate-spin" size={20} />
                            Searching Modrinth...
                        </div>
                    ) : marketResults.length === 0 ? (
                        <div className="bg-black/40 border border-white/10 rounded-lg p-12 text-center font-mono">
                            <Globe className="mx-auto text-white/20 mb-3" size={40} />
                            <p className="text-sm text-white/60">No plugins found on Modrinth matching "{marketQuery}".</p>
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                            {marketResults.map(hit => {
                                const isInstalling = installingProject === hit.project_id;
                                return (
                                    <div
                                        key={hit.project_id}
                                        className="bg-black/50 border border-white/15 rounded-lg p-5 hover:border-white/30 transition-all flex flex-col justify-between shadow-lg font-mono"
                                    >
                                        <div>
                                            <div className="flex items-start gap-3 mb-3">
                                                {hit.icon_url ? (
                                                    <img
                                                        src={hit.icon_url}
                                                        alt={hit.title}
                                                        className="w-12 h-12 rounded object-cover border border-white/10 shrink-0 bg-white/5"
                                                    />
                                                ) : (
                                                    <div className="w-12 h-12 rounded bg-white/5 border border-white/10 flex items-center justify-center text-white/40 shrink-0">
                                                        <Puzzle size={20} />
                                                    </div>
                                                )}
                                                <div className="min-w-0 flex-1">
                                                    <h3 className="font-bold text-white text-sm truncate flex items-center gap-1.5">
                                                        {hit.title}
                                                        <a
                                                            href={`https://modrinth.com/plugin/${hit.slug}`}
                                                            target="_blank"
                                                            rel="noreferrer"
                                                            className="text-white/30 hover:text-white"
                                                            title="View on Modrinth"
                                                        >
                                                            <ArrowUpRight size={13} />
                                                        </a>
                                                    </h3>
                                                    <p className="text-xs text-white/40 truncate">
                                                        by {hit.author}
                                                    </p>
                                                </div>
                                            </div>

                                            <p className="text-xs text-white/70 line-clamp-2 mb-3 leading-relaxed">
                                                {hit.description}
                                            </p>

                                            {hit.categories && hit.categories.length > 0 && (
                                                <div className="flex flex-wrap gap-1 mb-3">
                                                    {hit.categories.slice(0, 3).map((cat, idx) => (
                                                        <span key={idx} className="px-1.5 py-0.5 rounded text-[10px] bg-white/5 text-white/50">
                                                            {cat}
                                                        </span>
                                                    ))}
                                                </div>
                                            )}
                                        </div>

                                        <div className="flex items-center justify-between pt-3 border-t border-white/10 text-xs">
                                            <span className="text-white/40 text-[11px]">
                                                {hit.downloads.toLocaleString()} downloads
                                            </span>

                                            <button
                                                onClick={() => handleInstallMarketPlugin(hit.project_id, hit.title)}
                                                disabled={isInstalling}
                                                className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/50 text-mc-diamond text-xs transition-colors"
                                            >
                                                {isInstalling ? (
                                                    <RefreshCw size={12} className="animate-spin" />
                                                ) : (
                                                    <Download size={12} />
                                                )}
                                                {isInstalling ? 'Installing...' : 'Install'}
                                            </button>
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                </div>
            )}

            {/* UPLOAD PLUGIN MODAL */}
            {isUploadModalOpen && (
                <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4">
                    <div className="bg-zinc-950 border border-white/20 rounded-lg max-w-md w-full p-6 shadow-2xl space-y-4 font-mono text-sm">
                        <div className="flex items-center justify-between border-b border-white/10 pb-3">
                            <h3 className="text-lg font-bold text-white flex items-center gap-2">
                                <Upload className="text-mc-diamond" size={20} />
                                Upload Plugin
                            </h3>
                            <button
                                onClick={() => setIsUploadModalOpen(false)}
                                className="text-white/40 hover:text-white"
                            >
                                <XCircle size={20} />
                            </button>
                        </div>

                        <form onSubmit={handleUploadPlugin} className="space-y-4">
                            <div className="border-2 border-dashed border-white/20 rounded-lg p-6 text-center hover:border-mc-diamond/50 transition-colors">
                                <Upload className="mx-auto text-white/40 mb-2" size={32} />
                                <label className="block text-xs text-white/80 cursor-pointer mb-2">
                                    Click to select or drag a Minecraft plugin <code>.jar</code> file here
                                </label>
                                <input
                                    type="file"
                                    accept=".jar"
                                    onChange={(e) => {
                                        if (e.target.files && e.target.files[0]) {
                                            setSelectedFile(e.target.files[0]);
                                        }
                                    }}
                                    className="block w-full text-xs text-white/50 file:mr-4 file:py-2 file:px-4 file:rounded file:border-0 file:text-xs file:font-mono file:bg-mc-diamond/20 file:text-mc-diamond hover:file:bg-mc-diamond/30 cursor-pointer"
                                />
                                {selectedFile && (
                                    <div className="mt-3 text-xs text-emerald-400 font-bold truncate">
                                        Selected: {selectedFile.name} ({(selectedFile.size / 1024).toFixed(1)} KB)
                                    </div>
                                )}
                            </div>

                            <p className="text-[11px] text-white/40">
                                File must be a valid Spigot or Paper plugin jar archive containing <code>plugin.yml</code> or <code>paper-plugin.yml</code>.
                            </p>

                            <div className="flex justify-end gap-3 pt-3 border-t border-white/10">
                                <button
                                    type="button"
                                    onClick={() => setIsUploadModalOpen(false)}
                                    className="px-4 py-2 rounded bg-white/5 hover:bg-white/10 text-white/70 text-xs transition-colors"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={uploading || !selectedFile}
                                    className="flex items-center gap-2 px-4 py-2 rounded bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/50 text-mc-diamond text-xs transition-colors disabled:opacity-50"
                                >
                                    {uploading && <RefreshCw size={14} className="animate-spin" />}
                                    Upload & Install
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* DELETE CONFIRMATION MODAL */}
            {deleteTarget && (
                <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4">
                    <div className="bg-zinc-950 border border-red-500/30 rounded-lg max-w-md w-full p-6 shadow-2xl space-y-4 font-mono text-sm">
                        <div className="flex items-center gap-3 text-red-400">
                            <AlertTriangle size={24} />
                            <h3 className="text-lg font-bold text-white">Delete Plugin</h3>
                        </div>
                        <p className="text-white/70 text-xs leading-relaxed">
                            Are you sure you want to permanently delete{' '}
                            <span className="text-white font-bold">"{deleteTarget.filename}"</span>?
                            The server will need to be restarted for this change to take effect.
                        </p>
                        <div className="flex justify-end gap-3 pt-2">
                            <button
                                onClick={() => setDeleteTarget(null)}
                                className="px-4 py-2 rounded bg-white/5 hover:bg-white/10 text-white/70 text-xs transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleDeletePlugin}
                                disabled={deleting}
                                className="flex items-center gap-2 px-4 py-2 rounded bg-red-500/20 hover:bg-red-500/30 border border-red-500/50 text-red-300 text-xs transition-colors"
                            >
                                {deleting && <RefreshCw size={14} className="animate-spin" />}
                                Delete Plugin
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
