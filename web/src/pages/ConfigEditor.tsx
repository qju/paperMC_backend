import React, { useEffect, useState, useMemo } from 'react';
import { Settings, Save, RefreshCw, AlertTriangle, CheckCircle, XCircle, Search, Sliders, FileCode, Check, Shield, Globe, Cpu, Radio, ListPlus, Zap, Terminal, Copy, Sparkles, ExternalLink, Info, CheckCheck } from 'lucide-react';
import type { FlagsStatusResponse, FlagPresetInfo } from '../types';

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

interface PropertyDefinition {
    key: string;
    label: string;
    description: string;
    type: 'boolean' | 'number' | 'string' | 'select';
    options?: string[];
    category: 'general' | 'gameplay' | 'security' | 'performance' | 'rcon' | 'other';
}

const RAM_OPTIONS = ['2G', '4G', '6G', '8G', '10G', '12G', '16G', '24G', '32G'];

function parseRamToMB(ramStr: string): number {
    const clean = (ramStr || '').trim().toUpperCase();
    if (clean.endsWith('G')) {
        return (parseFloat(clean.slice(0, -1)) || 0) * 1024;
    }
    if (clean.endsWith('M')) {
        return parseFloat(clean.slice(0, -1)) || 0;
    }
    return (parseFloat(clean) || 0) * 1024;
}

const FLAG_EXPLANATIONS: Record<string, string> = {
    '-XX:+UseG1GC': 'Garbage-First (G1) collector: balances throughput with low pause times.',
    '-XX:+ParallelRefProcEnabled': 'Enables parallel reference processing to speed up weak reference cleanups.',
    '-XX:MaxGCPauseMillis=200': 'Target max GC pause time (200ms) to ensure smooth 20 TPS server ticks.',
    '-XX:+UnlockExperimentalVMOptions': 'Enables advanced low-overhead JVM tuning parameters.',
    '-XX:+DisableExplicitGC': 'Blocks plugins from invoking full-world System.gc() freeze cycles.',
    '-XX:+AlwaysPreTouch': 'Pre-allocates physical memory on launch to prevent page-fault latency during gameplay.',
    '-XX:G1NewSizePercent=30': 'Allocates at least 30% heap to young generation for fast object collection.',
    '-XX:G1NewSizePercent=40': 'Allocates at least 40% heap to young generation for 12GB+ setups.',
    '-XX:G1MaxNewSizePercent=40': 'Caps young gen at 40% to guarantee predictable GC pause boundaries.',
    '-XX:G1MaxNewSizePercent=50': 'Caps young gen at 50% for 12GB+ large heap workloads.',
    '-XX:G1ReservePercent=20': 'Reserves 20% emergency buffer to prevent evacuation failures and lag spikes.',
    '-XX:G1ReservePercent=15': 'Reserves 15% buffer for 12GB+ memory allocations.',
    '-XX:InitiatingHeapOccupancyPercent=15': 'Triggers concurrent marking when old gen reaches 15% occupancy.',
    '-XX:InitiatingHeapOccupancyPercent=20': 'Triggers concurrent marking at 20% for 12GB+ memory allocations.',
    '-XX:G1MixedGCLiveThresholdPercent=90': 'Recycles old gen memory regions that contain up to 90% garbage.',
    '-XX:G1RSetUpdatingPauseTimePercent=5': 'Limits remembered set update overhead to at most 5% of pause time.',
    '-XX:SurvivorRatio=32': 'Maximizes Eden space efficiency for transient entities and chunk packets.',
    '-XX:+PerfDisableSharedMem': 'Disables JVM hsperfdata to prevent disk IO wait blocking Java threads.',
    '-XX:MaxTenuringThreshold=1': 'Promotes surviving objects quickly to minimize young-gen copy overhead.',
    '-Dusing.aikars.flags=https://mcflags.emc.gs': 'Notifies PaperMC engine that Aikar flags are active to suppress warnings.',
    '-Daikars.new.flags=true': 'Flags telemetry confirmation for PaperMC watchdog.',
};

const KNOWN_PROPERTIES: PropertyDefinition[] = [
    // General
    { key: 'motd', label: 'Message of the Day (MOTD)', description: 'The server description shown in the Minecraft multiplayer server list.', type: 'string', category: 'general' },
    { key: 'server-port', label: 'Server Port', description: 'Port Minecraft listens on for incoming player connections (default: 25565).', type: 'number', category: 'general' },
    { key: 'server-ip', label: 'Server IP', description: 'Network IP to bind the server to (leave blank for all interfaces).', type: 'string', category: 'general' },
    { key: 'max-players', label: 'Max Players', description: 'Maximum number of concurrent players allowed on the server.', type: 'number', category: 'general' },
    { key: 'online-mode', label: 'Online Mode', description: 'Authenticates players against Mojang session servers. Disable only for offline/testing.', type: 'boolean', category: 'general' },
    { key: 'enable-status', label: 'Enable Server List Ping', description: 'Allows the server to appear as online in the multiplayer server list.', type: 'boolean', category: 'general' },
    { key: 'hide-online-players', label: 'Hide Online Players', description: 'Prevents the server list ping from displaying the online player count and list.', type: 'boolean', category: 'general' },

    // Gameplay
    { key: 'gamemode', label: 'Default Gamemode', description: 'Default game mode for new players joining the world.', type: 'select', options: ['survival', 'creative', 'adventure', 'spectator'], category: 'gameplay' },
    { key: 'force-gamemode', label: 'Force Gamemode', description: 'Forces players to join in the default gamemode on reconnect.', type: 'boolean', category: 'gameplay' },
    { key: 'difficulty', label: 'Difficulty', description: 'World difficulty setting affecting mob spawning and damage.', type: 'select', options: ['peaceful', 'easy', 'normal', 'hard'], category: 'gameplay' },
    { key: 'hardcore', label: 'Hardcore Mode', description: 'Locks difficulty to hard and bans players permanently upon death.', type: 'boolean', category: 'gameplay' },
    { key: 'pvp', label: 'Player vs Player (PvP)', description: 'Allows players to attack and deal damage to one another.', type: 'boolean', category: 'gameplay' },
    { key: 'allow-flight', label: 'Allow Flight', description: 'Allows players to fly in survival mode without kicking for flying.', type: 'boolean', category: 'gameplay' },
    { key: 'allow-nether', label: 'Allow Nether', description: 'Enables portal travel to the Nether dimension.', type: 'boolean', category: 'gameplay' },
    { key: 'spawn-monsters', label: 'Spawn Monsters', description: 'Allows hostile mobs (zombies, creepers, skeletons) to spawn.', type: 'boolean', category: 'gameplay' },
    { key: 'spawn-animals', label: 'Spawn Animals', description: 'Allows passive friendly animals (cows, sheep, pigs) to spawn.', type: 'boolean', category: 'gameplay' },
    { key: 'spawn-npcs', label: 'Spawn NPCs', description: 'Allows villagers to spawn in villages.', type: 'boolean', category: 'gameplay' },
    { key: 'generate-structures', label: 'Generate Structures', description: 'Generates structures like villages, strongholds, and temples.', type: 'boolean', category: 'gameplay' },

    // Security & Network
    { key: 'white-list', label: 'Enable Whitelist', description: 'Only players registered on the server whitelist can connect.', type: 'boolean', category: 'security' },
    { key: 'enforce-whitelist', label: 'Enforce Whitelist', description: 'Kicks connected non-whitelisted players immediately when whitelist is reloaded.', type: 'boolean', category: 'security' },
    { key: 'enforce-secure-profile', label: 'Enforce Secure Profile', description: 'Requires cryptographic chat signatures from players.', type: 'boolean', category: 'security' },
    { key: 'prevent-proxy-connections', label: 'Prevent Proxy Connections', description: 'Blocks connections originating from known VPN or proxy services.', type: 'boolean', category: 'security' },
    { key: 'network-compression-threshold', label: 'Network Compression Threshold', description: 'Packet size threshold for gzip network compression (default: 256).', type: 'number', category: 'security' },
    { key: 'rate-limit', label: 'Packet Rate Limit', description: 'Maximum number of packets allowed per second per client before kicking.', type: 'number', category: 'security' },

    // Performance & World
    { key: 'view-distance', label: 'View Distance (Chunks)', description: 'Render distance sent to clients in chunks (range: 3-32).', type: 'number', category: 'performance' },
    { key: 'simulation-distance', label: 'Simulation Distance (Chunks)', description: 'Radius in chunks where entities and block ticks are simulated.', type: 'number', category: 'performance' },
    { key: 'max-tick-time', label: 'Max Tick Time (Watchdog)', description: 'Maximum ms a single tick may take before watchdog halts server (-1 to disable).', type: 'number', category: 'performance' },
    { key: 'entity-broadcast-range-percentage', label: 'Entity Broadcast Range %', description: 'Controls percentage distance for entity tracking updates (default: 100).', type: 'number', category: 'performance' },
    { key: 'sync-chunk-writes', label: 'Sync Chunk Writes', description: 'Forces synchronous disk chunk writes to prevent data loss on sudden power cutoff.', type: 'boolean', category: 'performance' },
    { key: 'spawn-protection', label: 'Spawn Protection Radius', description: 'Radius around world spawn where non-ops cannot build or destroy blocks.', type: 'number', category: 'performance' },

    // RCON
    { key: 'enable-rcon', label: 'Enable Remote Console (RCON)', description: 'Allows remote console administration over TCP.', type: 'boolean', category: 'rcon' },
    { key: 'rcon.port', label: 'RCON Port', description: 'Listening port for RCON connection (default: 25575).', type: 'number', category: 'rcon' },
    { key: 'rcon.password', label: 'RCON Password', description: 'Secret authentication password for remote RCON access.', type: 'string', category: 'rcon' },
];

export default function ConfigEditor() {
    const [properties, setProperties] = useState<Record<string, string>>({});
    const [originalProperties, setOriginalProperties] = useState<Record<string, string>>({});
    const [rawText, setRawText] = useState<string>('');
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [activeTab, setActiveTab] = useState<'visual' | 'all' | 'raw' | 'flags'>('visual');
    const [activeCategory, setActiveCategory] = useState<'general' | 'gameplay' | 'security' | 'performance' | 'rcon' | 'other'>('general');
    const [searchQuery, setSearchQuery] = useState('');
    const [toasts, setToasts] = useState<Toast[]>([]);

    // Java & Smart Flags State
    const [flagsStatus, setFlagsStatus] = useState<FlagsStatusResponse | null>(null);
    const [flagPresets, setFlagPresets] = useState<FlagPresetInfo[]>([]);
    const [selectedRam, setSelectedRam] = useState('8G');
    const [selectedPreset, setSelectedPreset] = useState('aikar');
    const [customFlagsText, setCustomFlagsText] = useState('');
    const [loadingFlags, setLoadingFlags] = useState(false);
    const [savingFlags, setSavingFlags] = useState(false);
    const [copiedCommand, setCopiedCommand] = useState(false);

    const showToast = (message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, message, type }]);
        setTimeout(() => {
            setToasts(prev => prev.filter(t => t.id !== id));
        }, 3500);
    };

    const fetchConfig = async () => {
        setLoading(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/config', {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                const data = await res.json();
                setProperties(data);
                setOriginalProperties(data);

                // Build raw text format
                const lines = Object.entries(data).map(([k, v]) => `${k}=${v}`);
                setRawText(lines.join('\n'));
            } else {
                showToast("Failed to load server.properties", 'error');
            }
        } catch (err) {
            console.error("Failed to load config", err);
            showToast("Network error loading server.properties", 'error');
        } finally {
            setLoading(false);
        }
    };

    const fetchFlags = async () => {
        setLoadingFlags(true);
        const token = localStorage.getItem('token');
        try {
            const [flagsRes, presetsRes] = await Promise.all([
                fetch('/api/flags', {
                    headers: { 'Authorization': `Bearer ${token}` }
                }),
                fetch('/api/flags/presets', {
                    headers: { 'Authorization': `Bearer ${token}` }
                })
            ]);

            if (flagsRes.ok) {
                const ct = flagsRes.headers.get('content-type') || '';
                if (ct.includes('application/json')) {
                    const data: FlagsStatusResponse = await flagsRes.json();
                    setFlagsStatus(data);
                    if (data.configured) {
                        setSelectedRam(data.configured.ram || '8G');
                        setSelectedPreset(data.configured.preset || 'aikar');
                        setCustomFlagsText(data.configured.custom_flags || '');
                    }
                } else {
                    console.warn("Received non-JSON response from /api/flags. The backend server might need a restart.");
                }
            } else {
                showToast("Failed to load Java & Smart Flags", 'error');
            }

            if (presetsRes.ok) {
                const ct = presetsRes.headers.get('content-type') || '';
                if (ct.includes('application/json')) {
                    const presetsData: FlagPresetInfo[] = await presetsRes.json();
                    setFlagPresets(presetsData);
                }
            }
        } catch (err) {
            console.error("Failed to load flags", err);
            showToast("Network error loading Java & Smart Flags", 'error');
        } finally {
            setLoadingFlags(false);
        }
    };

    useEffect(() => {
        fetchConfig();
        fetchFlags();
    }, []);

    // Check if there are unsaved changes
    const hasUnsavedChanges = useMemo(() => {
        if (activeTab === 'raw') {
            const currentLines = Object.entries(originalProperties).map(([k, v]) => `${k}=${v}`).join('\n');
            return rawText.trim() !== currentLines.trim();
        }
        const currentKeys = Object.keys(properties);
        const origKeys = Object.keys(originalProperties);
        if (currentKeys.length !== origKeys.length) return true;
        for (const key of currentKeys) {
            if (properties[key] !== originalProperties[key]) return true;
        }
        return false;
    }, [properties, originalProperties, rawText, activeTab]);

    const handlePropertyChange = (key: string, value: string) => {
        setProperties(prev => ({
            ...prev,
            [key]: value
        }));
    };

    const handleSave = async () => {
        setSaving(true);
        const token = localStorage.getItem('token');

        let payload: Record<string, string> = { ...properties };

        if (activeTab === 'raw') {
            // Parse raw text into dictionary
            const parsed: Record<string, string> = {};
            const lines = rawText.split('\n');
            for (const line of lines) {
                const clean = line.trim();
                if (!clean || clean.startsWith('#')) continue;
                const parts = clean.split('=');
                if (parts.length >= 2) {
                    const key = parts[0].trim();
                    const val = parts.slice(1).join('=').trim();
                    if (key) parsed[key] = val;
                }
            }
            payload = parsed;
        }

        try {
            const res = await fetch('/config', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(payload)
            });

            if (res.ok) {
                showToast("server.properties saved successfully!", 'success');
                setOriginalProperties(payload);
                setProperties(payload);
                const lines = Object.entries(payload).map(([k, v]) => `${k}=${v}`);
                setRawText(lines.join('\n'));
            } else {
                const errData = await res.json().catch(() => ({}));
                showToast(errData.error || "Failed to save configuration", 'error');
            }
        } catch (err) {
            console.error("Save config error", err);
            showToast("Network error saving configuration", 'error');
        } finally {
            setSaving(false);
        }
    };

    const fetchPresets = async (ramToQuery?: string) => {
        const token = localStorage.getItem('token');
        const r = (ramToQuery || selectedRam || '8G').trim();
        try {
            const res = await fetch(`/api/flags/presets?ram=${encodeURIComponent(r)}`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                const data: FlagPresetInfo[] = await res.json();
                setFlagPresets(data);
            }
        } catch (err) {
            console.error("Failed to load presets", err);
        }
    };

    const handleRamChange = (newRam: string) => {
        setSelectedRam(newRam);
        fetchPresets(newRam);
    };

    const hasUnsavedFlagsChanges = useMemo(() => {
        if (!flagsStatus || !flagsStatus.configured) return false;
        const origRam = flagsStatus.configured.ram || '8G';
        const origPreset = flagsStatus.configured.preset || 'aikar';
        const origCustom = flagsStatus.configured.custom_flags || '';

        return (
            selectedRam.trim() !== origRam.trim() ||
            selectedPreset !== origPreset ||
            customFlagsText.trim() !== origCustom.trim()
        );
    }, [flagsStatus, selectedRam, selectedPreset, customFlagsText]);

    const effectiveFlags = useMemo(() => {
        const ram = selectedRam.trim() || '8G';
        const ramArgs = [`-Xms${ram}`, `-Xmx${ram}`];
        if (selectedPreset === 'none') {
            return ramArgs;
        }
        if (selectedPreset === 'minimal') {
            return [...ramArgs, '-XX:+UseG1GC'];
        }
        if (selectedPreset === 'custom') {
            const customList = customFlagsText.trim().split(/\s+/).filter(Boolean);
            return [...ramArgs, ...customList];
        }
        // aikar
        const isHigh = parseRamToMB(ram) >= 12 * 1024;
        return [
            ...ramArgs,
            '-XX:+UseG1GC',
            '-XX:+ParallelRefProcEnabled',
            '-XX:MaxGCPauseMillis=200',
            '-XX:+UnlockExperimentalVMOptions',
            '-XX:+DisableExplicitGC',
            '-XX:+AlwaysPreTouch',
            isHigh ? '-XX:G1NewSizePercent=40' : '-XX:G1NewSizePercent=30',
            isHigh ? '-XX:G1MaxNewSizePercent=50' : '-XX:G1MaxNewSizePercent=40',
            isHigh ? '-XX:G1ReservePercent=15' : '-XX:G1ReservePercent=20',
            isHigh ? '-XX:InitiatingHeapOccupancyPercent=20' : '-XX:InitiatingHeapOccupancyPercent=15',
            '-XX:G1MixedGCLiveThresholdPercent=90',
            '-XX:G1RSetUpdatingPauseTimePercent=5',
            '-XX:SurvivorRatio=32',
            '-XX:+PerfDisableSharedMem',
            '-XX:MaxTenuringThreshold=1',
            '-Dusing.aikars.flags=https://mcflags.emc.gs',
            '-Daikars.new.flags=true',
        ];
    }, [selectedRam, selectedPreset, customFlagsText]);

    const handleSaveFlags = async () => {
        setSavingFlags(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/flags', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    ram: selectedRam.trim() || '8G',
                    preset: selectedPreset,
                    custom_flags: customFlagsText.trim()
                })
            });

            if (res.ok) {
                const data: FlagsStatusResponse = await res.json();
                setFlagsStatus(data);
                setSelectedRam(data.configured.ram);
                setSelectedPreset(data.configured.preset);
                setCustomFlagsText(data.configured.custom_flags || '');
                showToast("Java & Smart Flags updated successfully!", 'success');
            } else {
                const errData = await res.json().catch(() => ({}));
                showToast(errData.error || "Failed to save flags configuration", 'error');
            }
        } catch (err) {
            console.error("Save flags error", err);
            showToast("Network error saving flags configuration", 'error');
        } finally {
            setSavingFlags(false);
        }
    };

    const handleCopyCommand = () => {
        const fullCmd = `java ${effectiveFlags.join(' ')} -jar paper.jar --nogui`;
        navigator.clipboard.writeText(fullCmd);
        setCopiedCommand(true);
        setTimeout(() => setCopiedCommand(false), 2500);
        showToast("Java launch command copied to clipboard!", 'success');
    };

    // Filtered properties for the "All Properties" tab
    const filteredAllProperties = useMemo(() => {
        const query = searchQuery.toLowerCase().trim();
        const entries = Object.entries(properties);
        if (!query) return entries;
        return entries.filter(([k, v]) => k.toLowerCase().includes(query) || v.toLowerCase().includes(query));
    }, [properties, searchQuery]);

    // Compute custom / extra properties not in KNOWN_PROPERTIES
    const otherProperties = useMemo(() => {
        const knownKeys = new Set(KNOWN_PROPERTIES.map(p => p.key));
        return Object.keys(properties)
            .filter(k => !knownKeys.has(k))
            .map(k => ({
                key: k,
                label: k,
                description: 'Custom or unclassified server property.',
                type: (properties[k] === 'true' || properties[k] === 'false' ? 'boolean' : !isNaN(Number(properties[k])) && properties[k] !== '' ? 'number' : 'string') as PropertyDefinition['type'],
                category: 'other' as const
            }));
    }, [properties]);

    const categorizedProperties = useMemo(() => {
        if (activeCategory === 'other') return otherProperties;
        return KNOWN_PROPERTIES.filter(p => p.category === activeCategory);
    }, [activeCategory, otherProperties]);

    const categoryCounts = useMemo(() => {
        const counts: Record<string, number> = {
            general: 0,
            gameplay: 0,
            security: 0,
            performance: 0,
            rcon: 0,
            other: otherProperties.length
        };
        for (const p of KNOWN_PROPERTIES) {
            if (p.key in properties) {
                counts[p.category] = (counts[p.category] || 0) + 1;
            }
        }
        return counts;
    }, [properties, otherProperties]);

    return (
        <div className="space-y-6 relative w-full pb-16">
            {/* TOAST NOTIFICATIONS */}
            <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-2">
                {toasts.map(toast => (
                    <div
                        key={toast.id}
                        className={`
                            flex items-center gap-3 px-4 py-3 rounded-lg shadow-2xl border backdrop-blur-md animate-slide-in
                            ${toast.type === 'success'
                                ? 'bg-green-950/90 border-green-500/50 text-green-100'
                                : 'bg-red-950/90 border-red-500/50 text-red-100'}
                        `}
                    >
                        {toast.type === 'success' ? <CheckCircle size={18} /> : <XCircle size={18} />}
                        <span className="font-mono text-sm">{toast.message}</span>
                    </div>
                ))}
            </div>

            {/* HEADER & ACTION BAR */}
            <div className="bg-black/60 border border-white/10 rounded-xl p-5 backdrop-blur-md flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div>
                    <h1 className="text-3xl font-pixel text-mc-diamond flex items-center gap-3">
                        <Settings className="text-mc-diamond" size={28} /> Server Configuration
                    </h1>
                    <p className="text-white/50 font-mono text-sm mt-1">
                        {activeTab === 'flags' ? (
                            <span>Fine-tune JVM memory heap, garbage collector flags, and Aikar optimizations</span>
                        ) : (
                            <span>Visual controls and raw editor for <code className="text-mc-gold font-mono">server.properties</code> ({Object.keys(properties).length} properties loaded)</span>
                        )}
                    </p>
                </div>

                <div className="flex items-center gap-3 self-end md:self-center">
                    <button
                        onClick={activeTab === 'flags' ? fetchFlags : fetchConfig}
                        disabled={loading || saving || loadingFlags || savingFlags}
                        className="p-2.5 bg-white/5 hover:bg-white/10 rounded-lg transition-colors border border-white/10 text-white/80"
                        title={activeTab === 'flags' ? "Reload Flags from Database" : "Reload from Disk"}
                    >
                        <RefreshCw size={18} className={(activeTab === 'flags' ? loadingFlags : loading) ? "animate-spin" : ""} />
                    </button>
                    <button
                        onClick={activeTab === 'flags' ? handleSaveFlags : handleSave}
                        disabled={activeTab === 'flags' ? (savingFlags || !hasUnsavedFlagsChanges) : (saving || !hasUnsavedChanges)}
                        className={`
                            px-5 py-2.5 rounded-lg font-mono font-bold text-sm flex items-center gap-2 transition-all shadow-lg border
                            ${(activeTab === 'flags' ? hasUnsavedFlagsChanges : hasUnsavedChanges)
                                ? 'bg-green-600 hover:bg-green-500 text-white border-green-400/50 shadow-green-900/40 cursor-pointer animate-pulse'
                                : 'bg-white/10 text-white/40 border-transparent cursor-not-allowed'}
                        `}
                    >
                        {(activeTab === 'flags' ? savingFlags : saving) ? <RefreshCw size={16} className="animate-spin" /> : <Save size={16} />}
                        {(activeTab === 'flags' ? savingFlags : saving) ? 'Saving...' : (activeTab === 'flags' ? hasUnsavedFlagsChanges : hasUnsavedChanges) ? 'Save Changes' : 'Saved'}
                    </button>
                </div>
            </div>

            {/* RESTART NOTICE BANNER */}
            {activeTab === 'flags' ? (
                flagsStatus?.restart_required ? (
                    <div className="bg-amber-950/50 border border-amber-500/50 p-4 rounded-xl flex items-start gap-3 backdrop-blur-md animate-pulse">
                        <AlertTriangle className="text-amber-400 shrink-0 mt-0.5" size={20} />
                        <div className="text-xs font-mono text-amber-200 leading-relaxed">
                            <strong className="text-amber-300 font-bold uppercase tracking-wider block mb-0.5">JVM Restart Pending:</strong>
                            Configured flags differ from currently running Java process flags. Restart the server from the Console tab to boot with the new JVM memory and optimization flags.
                        </div>
                    </div>
                ) : (
                    <div className="bg-cyan-950/30 border border-cyan-500/30 p-4 rounded-xl flex items-start gap-3 backdrop-blur-md">
                        <Sparkles className="text-mc-diamond shrink-0 mt-0.5" size={20} />
                        <div className="text-xs font-mono text-cyan-200/90 leading-relaxed">
                            <strong className="text-mc-diamond font-bold uppercase tracking-wider block mb-0.5">Aikar's Flag Optimizer:</strong>
                            Automatically optimizes garbage collector pause times, young generation boundaries, and concurrent marking thresholds to eliminate GC lag spikes on PaperMC.
                        </div>
                    </div>
                )
            ) : (
                <div className="bg-amber-950/40 border border-amber-500/30 p-4 rounded-xl flex items-start gap-3 backdrop-blur-md">
                    <AlertTriangle className="text-amber-400 shrink-0 mt-0.5" size={20} />
                    <div className="text-xs font-mono text-amber-200/90 leading-relaxed">
                        <strong className="text-amber-300 font-bold uppercase tracking-wider block mb-0.5">Minecraft Engine Notice:</strong>
                        Changes made to <span className="text-white font-bold">server.properties</span> take effect when the server restarts. After saving, restart the server from the Console to apply them.
                    </div>
                </div>
            )}

            {/* TABS SELECTOR */}
            <div className="flex items-center gap-2 border-b border-white/10 pb-3 pt-1 overflow-x-auto custom-scrollbar shrink-0">
                <button
                    onClick={() => setActiveTab('visual')}
                    className={`flex items-center gap-2 px-4 py-2.5 rounded-lg font-mono text-sm font-bold transition-all shrink-0 ${
                        activeTab === 'visual'
                            ? 'bg-mc-diamond/20 text-mc-diamond border border-mc-diamond/30 shadow-[0_0_12px_rgba(85,255,255,0.15)]'
                            : 'text-white/60 hover:text-white hover:bg-white/5 border border-transparent'
                    }`}
                >
                    <Sliders size={16} /> Categorized Settings
                </button>
                <button
                    onClick={() => setActiveTab('all')}
                    className={`flex items-center gap-2 px-4 py-2.5 rounded-lg font-mono text-sm font-bold transition-all shrink-0 ${
                        activeTab === 'all'
                            ? 'bg-mc-diamond/20 text-mc-diamond border border-mc-diamond/30 shadow-[0_0_12px_rgba(85,255,255,0.15)]'
                            : 'text-white/60 hover:text-white hover:bg-white/5 border border-transparent'
                    }`}
                >
                    <Search size={16} /> All Properties ({Object.keys(properties).length})
                </button>
                <button
                    onClick={() => setActiveTab('raw')}
                    className={`flex items-center gap-2 px-4 py-2.5 rounded-lg font-mono text-sm font-bold transition-all shrink-0 ${
                        activeTab === 'raw'
                            ? 'bg-mc-diamond/20 text-mc-diamond border border-mc-diamond/30 shadow-[0_0_12px_rgba(85,255,255,0.15)]'
                            : 'text-white/60 hover:text-white hover:bg-white/5 border border-transparent'
                    }`}
                >
                    <FileCode size={16} /> Raw File Editor
                </button>
                <button
                    onClick={() => setActiveTab('flags')}
                    className={`flex items-center gap-2 px-4 py-2.5 rounded-lg font-mono text-sm font-bold transition-all shrink-0 ${
                        activeTab === 'flags'
                            ? 'bg-mc-diamond/20 text-mc-diamond border border-mc-diamond/30 shadow-[0_0_12px_rgba(85,255,255,0.15)]'
                            : 'text-white/60 hover:text-white hover:bg-white/5 border border-transparent'
                    }`}
                >
                    <Zap size={16} className={activeTab === 'flags' ? "text-mc-gold animate-pulse" : "text-mc-gold/70"} />
                    <span>Java & Smart Flags</span>
                    {flagsStatus?.restart_required && (
                        <span className="w-2 h-2 rounded-full bg-amber-400 animate-ping ml-1" title="Restart required to apply changes" />
                    )}
                </button>
            </div>

            {/* TAB CONTENT */}
            {activeTab === 'visual' && (
                <div className="space-y-6">
                    {/* CATEGORY SELECTOR */}
                    <div className="flex flex-wrap gap-2">
                        <CategoryButton
                            active={activeCategory === 'general'}
                            onClick={() => setActiveCategory('general')}
                            icon={<Globe size={16} />}
                            label="General & Server"
                            count={categoryCounts.general}
                        />
                        <CategoryButton
                            active={activeCategory === 'gameplay'}
                            onClick={() => setActiveCategory('gameplay')}
                            icon={<Sliders size={16} />}
                            label="Gameplay & World"
                            count={categoryCounts.gameplay}
                        />
                        <CategoryButton
                            active={activeCategory === 'security'}
                            onClick={() => setActiveCategory('security')}
                            icon={<Shield size={16} />}
                            label="Security & Whitelist"
                            count={categoryCounts.security}
                        />
                        <CategoryButton
                            active={activeCategory === 'performance'}
                            onClick={() => setActiveCategory('performance')}
                            icon={<Cpu size={16} />}
                            label="Performance & Ticks"
                            count={categoryCounts.performance}
                        />
                        <CategoryButton
                            active={activeCategory === 'rcon'}
                            onClick={() => setActiveCategory('rcon')}
                            icon={<Radio size={16} />}
                            label="RCON Console"
                            count={categoryCounts.rcon}
                        />
                        {otherProperties.length > 0 && (
                            <CategoryButton
                                active={activeCategory === 'other'}
                                onClick={() => setActiveCategory('other')}
                                icon={<ListPlus size={16} />}
                                label="Custom / Other"
                                count={categoryCounts.other}
                            />
                        )}
                    </div>

                    {/* CATEGORY CONTROLS */}
                    <div className="bg-black/60 border border-white/10 rounded-xl p-6 backdrop-blur-md">
                        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                            {categorizedProperties.map(prop => (
                                <PropertyField
                                    key={prop.key}
                                    def={prop}
                                    value={properties[prop.key] ?? ''}
                                    onChange={(val) => handlePropertyChange(prop.key, val)}
                                />
                            ))}
                        </div>
                    </div>
                </div>
            )}

            {activeTab === 'all' && (
                <div className="space-y-4">
                    {/* SEARCH INPUT */}
                    <div className="relative">
                        <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-white/40" size={18} />
                        <input
                            type="text"
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            placeholder="Filter properties by key or value (e.g. motd, difficulty, port)..."
                            className="w-full bg-black/60 border border-white/10 rounded-xl pl-12 pr-4 py-3 text-white font-mono text-sm focus:border-mc-diamond focus:outline-none placeholder-white/30 backdrop-blur-md"
                        />
                    </div>

                    {/* TABLE OF ALL PROPERTIES */}
                    <div className="bg-black/60 border border-white/10 rounded-xl overflow-hidden backdrop-blur-md">
                        <div className="divide-y divide-white/5">
                            {filteredAllProperties.length > 0 ? (
                                filteredAllProperties.map(([key, val]) => (
                                    <div key={key} className="p-4 flex flex-col md:flex-row md:items-center justify-between gap-3 hover:bg-white/5 transition-colors">
                                        <div className="font-mono text-sm">
                                            <span className="text-mc-diamond font-bold">{key}</span>
                                        </div>
                                        <div className="w-full md:w-80">
                                            {val === 'true' || val === 'false' ? (
                                                <div className="flex items-center gap-2">
                                                    <button
                                                        type="button"
                                                        onClick={() => handlePropertyChange(key, val === 'true' ? 'false' : 'true')}
                                                        className={`px-3 py-1.5 rounded font-mono text-xs font-bold transition-all border ${
                                                            val === 'true'
                                                                ? 'bg-green-600 text-white border-green-500 shadow-[0_0_8px_rgba(34,197,94,0.3)]'
                                                                : 'bg-red-950/80 text-red-300 border-red-500/30'
                                                        }`}
                                                    >
                                                        {val === 'true' ? 'TRUE (Enabled)' : 'FALSE (Disabled)'}
                                                    </button>
                                                </div>
                                            ) : (
                                                <input
                                                    type="text"
                                                    value={val}
                                                    onChange={(e) => handlePropertyChange(key, e.target.value)}
                                                    className="w-full bg-black/80 border border-white/20 rounded-lg p-2.5 text-white font-mono text-xs focus:border-mc-diamond focus:outline-none"
                                                />
                                            )}
                                        </div>
                                    </div>
                                ))
                            ) : (
                                <div className="p-8 text-center text-white/30 font-mono text-sm">
                                    No properties matching query &quot;{searchQuery}&quot;
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            )}

            {activeTab === 'raw' && (
                <div className="bg-black/60 border border-white/10 rounded-xl p-6 backdrop-blur-md flex flex-col space-y-4">
                    <div className="flex justify-between items-center text-xs font-mono text-white/50">
                        <span>Direct File Editor (Preserves custom comments and formatting)</span>
                        <span>{rawText.split('\n').length} lines</span>
                    </div>
                    <textarea
                        value={rawText}
                        onChange={(e) => setRawText(e.target.value)}
                        rows={24}
                        className="w-full bg-black/90 border border-white/20 rounded-xl p-4 font-mono text-xs text-white leading-relaxed focus:border-mc-diamond focus:outline-none custom-scrollbar"
                        placeholder="# server.properties"
                        spellCheck={false}
                    />
                </div>
            )}

            {/* TAB: JAVA & SMART FLAGS */}
            {activeTab === 'flags' && (
                <div className="space-y-6">
                    {/* SECTION 1: RAM ALLOCATION */}
                    <div className="bg-black/60 border border-white/10 rounded-xl p-6 backdrop-blur-md space-y-5">
                        <div className="flex flex-col md:flex-row md:items-center justify-between gap-3 border-b border-white/10 pb-4">
                            <div>
                                <h2 className="text-xl font-mono font-bold text-white flex items-center gap-2.5">
                                    <Cpu className="text-mc-diamond" size={22} />
                                    <span>1. Heap RAM Allocation</span>
                                </h2>
                                <p className="text-xs font-mono text-white/50 mt-1 leading-relaxed">
                                    Configures initial (<code className="text-mc-diamond">-Xms</code>) and maximum (<code className="text-mc-diamond">-Xmx</code>) heap memory. Matching values eliminates GC pauses from dynamic memory reallocation.
                                </p>
                            </div>
                            <div className="px-3 py-1.5 rounded-lg bg-white/5 border border-white/10 font-mono text-xs text-white/70 self-start md:self-auto">
                                Current: <strong className="text-mc-diamond font-bold">{selectedRam.trim() || '8G'}</strong>
                            </div>
                        </div>

                        {/* Quick RAM Buttons */}
                        <div className="space-y-3">
                            <label className="block text-xs font-mono font-bold uppercase tracking-wider text-white/70">
                                Quick Selection
                            </label>
                            <div className="flex flex-wrap gap-2">
                                {RAM_OPTIONS.map(ram => (
                                    <button
                                        key={ram}
                                        type="button"
                                        onClick={() => handleRamChange(ram)}
                                        className={`px-4 py-2 rounded-lg font-mono text-xs font-bold transition-all border ${
                                            selectedRam.trim().toUpperCase() === ram
                                                ? 'bg-mc-diamond text-black border-mc-diamond shadow-[0_0_12px_rgba(85,255,255,0.4)] scale-105'
                                                : 'bg-white/5 text-white/70 hover:text-white hover:bg-white/10 border-white/10'
                                        }`}
                                    >
                                        {ram}
                                    </button>
                                ))}
                            </div>
                        </div>

                        {/* Custom RAM input */}
                        <div className="space-y-2 pt-2">
                            <label className="block text-xs font-mono font-bold uppercase tracking-wider text-white/70">
                                Custom RAM Limit
                            </label>
                            <div className="flex items-center gap-3 max-w-sm">
                                <input
                                    type="text"
                                    value={selectedRam}
                                    onChange={(e) => handleRamChange(e.target.value)}
                                    placeholder="e.g. 8G, 10240M"
                                    className="w-full bg-black/80 border border-white/20 rounded-lg p-2.5 text-white font-mono text-sm focus:border-mc-diamond focus:outline-none"
                                />
                                <span className="text-xs font-mono text-white/40 whitespace-nowrap">
                                    (~{(parseRamToMB(selectedRam) / 1024).toFixed(1)} GB)
                                </span>
                            </div>
                            <p className="text-[11px] font-mono text-white/40">
                                Standard format: number followed by <code className="text-white/60">G</code> (Gigabytes) or <code className="text-white/60">M</code> (Megabytes). Example: <code className="text-mc-gold">8G</code> or <code className="text-mc-gold">8192M</code>.
                            </p>
                        </div>

                        {/* Adaptive G1GC tuning indicator */}
                        {selectedPreset === 'aikar' && (
                            <div className={`p-4 rounded-xl border backdrop-blur-md flex items-start gap-3 transition-all ${
                                parseRamToMB(selectedRam) >= 12 * 1024
                                    ? 'bg-amber-950/30 border-amber-500/40 text-amber-200'
                                    : 'bg-cyan-950/30 border-cyan-500/30 text-cyan-200'
                            }`}>
                                <Zap className={parseRamToMB(selectedRam) >= 12 * 1024 ? "text-amber-400 shrink-0 mt-0.5" : "text-mc-diamond shrink-0 mt-0.5"} size={18} />
                                <div className="text-xs font-mono leading-relaxed">
                                    {parseRamToMB(selectedRam) >= 12 * 1024 ? (
                                        <>
                                            <strong className="text-amber-300 font-bold block mb-0.5">High Memory Tuning Active (&ge; 12GB):</strong>
                                            Aikar optimizer dynamically adapts parameters: <code className="text-amber-200 font-bold">-XX:G1NewSizePercent=40</code>, <code className="text-amber-200 font-bold">-XX:G1MaxNewSizePercent=50</code>, <code className="text-amber-200 font-bold">-XX:G1ReservePercent=15</code>, and <code className="text-amber-200 font-bold">-XX:InitiatingHeapOccupancyPercent=20</code> for large heap allocation efficiency.
                                        </>
                                    ) : (
                                        <>
                                            <strong className="text-cyan-300 font-bold block mb-0.5">Standard Memory Tuning Active (&lt; 12GB):</strong>
                                            Aikar optimizer dynamically adapts parameters: <code className="text-cyan-200 font-bold">-XX:G1NewSizePercent=30</code>, <code className="text-cyan-200 font-bold">-XX:G1MaxNewSizePercent=40</code>, <code className="text-cyan-200 font-bold">-XX:G1ReservePercent=20</code>, and <code className="text-cyan-200 font-bold">-XX:InitiatingHeapOccupancyPercent=15</code> to maintain sub-200ms garbage collection pauses.
                                        </>
                                    )}
                                </div>
                            </div>
                        )}
                    </div>

                    {/* SECTION 2: PRESET SELECTION */}
                    <div className="bg-black/60 border border-white/10 rounded-xl p-6 backdrop-blur-md space-y-5">
                        <div className="flex flex-col md:flex-row md:items-center justify-between gap-3 border-b border-white/10 pb-4">
                            <div>
                                <h2 className="text-xl font-mono font-bold text-white flex items-center gap-2.5">
                                    <Sparkles className="text-mc-gold" size={22} />
                                    <span>2. Optimization Preset</span>
                                </h2>
                                <p className="text-xs font-mono text-white/50 mt-1">
                                    Select an optimization profile designed for your server&apos;s workload ({flagPresets.length || 4} presets loaded).
                                </p>
                            </div>
                            <a
                                href="https://docs.papermc.io/paper/aikars-flags"
                                target="_blank"
                                rel="noreferrer"
                                className="text-xs font-mono text-mc-diamond/80 hover:text-mc-diamond flex items-center gap-1.5 transition-colors self-start md:self-auto"
                            >
                                <ExternalLink size={14} /> PaperMC Documentation
                            </a>
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            {/* Preset: Aikar */}
                            <div
                                onClick={() => setSelectedPreset('aikar')}
                                className={`p-5 rounded-xl border cursor-pointer transition-all relative space-y-3 ${
                                    selectedPreset === 'aikar'
                                        ? 'bg-mc-gold/10 border-mc-gold shadow-[0_0_15px_rgba(255,170,0,0.2)]'
                                        : 'bg-black/40 border-white/10 hover:border-white/20 hover:bg-white/5'
                                }`}
                            >
                                <div className="flex justify-between items-start">
                                    <div className="flex items-center gap-2">
                                        <Zap className={selectedPreset === 'aikar' ? "text-mc-gold" : "text-white/60"} size={20} />
                                        <span className="font-mono font-bold text-sm text-white">Aikar&apos;s Flags</span>
                                    </div>
                                    <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-green-500/20 text-green-400 border border-green-500/40">
                                        RECOMMENDED
                                    </span>
                                </div>
                                <p className="text-xs font-mono text-white/60 leading-relaxed">
                                    Tuned specifically by Aikar for PaperMC. Eliminates GC pause latency, stabilizes TPS, manages survivor ratio, and disables disruptive explicit GC calls.
                                </p>
                                <div className="text-[11px] font-mono text-mc-gold/80 flex items-center gap-1">
                                    <Check size={14} /> 200ms target pause time &bull; G1GC &bull; String deduplication
                                </div>
                            </div>

                            {/* Preset: Minimal */}
                            <div
                                onClick={() => setSelectedPreset('minimal')}
                                className={`p-5 rounded-xl border cursor-pointer transition-all relative space-y-3 ${
                                    selectedPreset === 'minimal'
                                        ? 'bg-mc-diamond/10 border-mc-diamond shadow-[0_0_15px_rgba(85,255,255,0.15)]'
                                        : 'bg-black/40 border-white/10 hover:border-white/20 hover:bg-white/5'
                                }`}
                            >
                                <div className="flex items-center gap-2">
                                    <Cpu className={selectedPreset === 'minimal' ? "text-mc-diamond" : "text-white/60"} size={20} />
                                    <span className="font-mono font-bold text-sm text-white">Minimal Flags</span>
                                </div>
                                <p className="text-xs font-mono text-white/60 leading-relaxed">
                                    Activates modern <code className="text-mc-diamond">-XX:+UseG1GC</code> while keeping all other garbage collector and memory settings at JVM defaults.
                                </p>
                                <div className="text-[11px] font-mono text-mc-diamond/70 flex items-center gap-1">
                                    <Check size={14} /> Basic G1GC collector with standard runtime parameters
                                </div>
                            </div>

                            {/* Preset: None */}
                            <div
                                onClick={() => setSelectedPreset('none')}
                                className={`p-5 rounded-xl border cursor-pointer transition-all relative space-y-3 ${
                                    selectedPreset === 'none'
                                        ? 'bg-purple-500/10 border-purple-500 shadow-[0_0_15px_rgba(168,85,247,0.2)]'
                                        : 'bg-black/40 border-white/10 hover:border-white/20 hover:bg-white/5'
                                }`}
                            >
                                <div className="flex items-center gap-2">
                                    <Sliders className={selectedPreset === 'none' ? "text-purple-400" : "text-white/60"} size={20} />
                                    <span className="font-mono font-bold text-sm text-white">None (RAM Only)</span>
                                </div>
                                <p className="text-xs font-mono text-white/60 leading-relaxed">
                                    Applies only <code className="text-purple-300">-Xms</code> and <code className="text-purple-300">-Xmx</code> limits without specifying any garbage collection algorithms or tuning flags.
                                </p>
                                <div className="text-[11px] font-mono text-purple-300/70 flex items-center gap-1">
                                    <Check size={14} /> Vanilla Java defaults
                                </div>
                            </div>

                            {/* Preset: Custom */}
                            <div
                                onClick={() => setSelectedPreset('custom')}
                                className={`p-5 rounded-xl border cursor-pointer transition-all relative space-y-3 ${
                                    selectedPreset === 'custom'
                                        ? 'bg-amber-500/10 border-amber-500 shadow-[0_0_15px_rgba(245,158,11,0.2)]'
                                        : 'bg-black/40 border-white/10 hover:border-white/20 hover:bg-white/5'
                                }`}
                            >
                                <div className="flex items-center gap-2">
                                    <Terminal className={selectedPreset === 'custom' ? "text-amber-400" : "text-white/60"} size={20} />
                                    <span className="font-mono font-bold text-sm text-white">Custom Flags</span>
                                </div>
                                <p className="text-xs font-mono text-white/60 leading-relaxed">
                                    Enter your own custom JVM flags, experimental GC options (such as Generational ZGC), or Java system properties.
                                </p>
                                <div className="text-[11px] font-mono text-amber-400/80 flex items-center gap-1">
                                    <Check size={14} /> User-defined arguments
                                </div>
                            </div>
                        </div>

                        {/* SECTION 3: CUSTOM FLAGS TEXTAREA */}
                        {selectedPreset === 'custom' && (
                            <div className="pt-4 border-t border-white/10 space-y-3">
                                <div className="flex justify-between items-center">
                                    <label className="block text-xs font-mono font-bold uppercase tracking-wider text-amber-300">
                                        Custom JVM Arguments
                                    </label>
                                    <span className="text-xs font-mono text-white/40">
                                        {customFlagsText.trim().split(/\s+/).filter(Boolean).length} flags entered
                                    </span>
                                </div>
                                <textarea
                                    value={customFlagsText}
                                    onChange={(e) => setCustomFlagsText(e.target.value)}
                                    rows={4}
                                    placeholder="-XX:+UseZGC -XX:+ZGenerational -XX:+AlwaysPreTouch"
                                    className="w-full bg-black/90 border border-white/20 rounded-xl p-3 font-mono text-xs text-white leading-relaxed focus:border-amber-400 focus:outline-none custom-scrollbar"
                                    spellCheck={false}
                                />
                                <p className="text-[11px] font-mono text-white/40">
                                    Separate arguments by spaces or newlines. Heap arguments (<code className="text-mc-diamond">-Xms</code> and <code className="text-mc-diamond">-Xmx</code>) are automatically injected from Section 1.
                                </p>
                            </div>
                        )}
                    </div>

                    {/* SECTION 4: LIVE JAVA COMMAND PREVIEW */}
                    <div className="bg-black/80 border border-mc-diamond/30 rounded-xl p-6 backdrop-blur-md space-y-4 shadow-[0_0_20px_rgba(85,255,255,0.05)]">
                        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-white/10 pb-3">
                            <div className="flex items-center gap-2.5">
                                <Terminal className="text-mc-diamond" size={20} />
                                <h3 className="font-mono font-bold text-sm text-white">
                                    Launch Command Preview
                                </h3>
                            </div>
                            <button
                                type="button"
                                onClick={handleCopyCommand}
                                className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/40 text-mc-diamond font-mono text-xs font-bold transition-all self-start sm:self-auto"
                            >
                                {copiedCommand ? <CheckCheck size={14} /> : <Copy size={14} />}
                                <span>{copiedCommand ? 'Copied!' : 'Copy Command'}</span>
                            </button>
                        </div>

                        {/* Terminal Box */}
                        <div className="bg-black/95 border border-white/10 rounded-xl p-4 font-mono text-xs overflow-x-auto custom-scrollbar">
                            <span className="text-mc-gold font-bold">java </span>
                            {effectiveFlags.map((flag, idx) => {
                                const isMem = flag.startsWith('-Xm');
                                const isAikarSpec = flag.startsWith('-XX:G1') || flag.startsWith('-XX:Initiating');
                                return (
                                    <span
                                        key={idx}
                                        className={`mr-1.5 ${
                                            isMem
                                                ? 'text-mc-diamond font-bold'
                                                : isAikarSpec
                                                ? 'text-amber-300 font-semibold'
                                                : 'text-green-300'
                                        }`}
                                    >
                                        {flag}
                                    </span>
                                );
                            })}
                            <span className="text-white/80 font-bold"> -jar paper.jar --nogui</span>
                        </div>

                        {/* Breakdown of Flags */}
                        <div className="space-y-3 pt-2">
                            <div className="flex items-center justify-between">
                                <span className="text-xs font-mono font-bold text-white/70 uppercase tracking-wider">
                                    Flag Breakdown ({effectiveFlags.length} arguments)
                                </span>
                            </div>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                                {effectiveFlags.map((flag, idx) => (
                                    <div
                                        key={idx}
                                        className="bg-black/40 border border-white/5 hover:border-white/15 rounded-lg p-2.5 flex items-start gap-2.5 transition-colors"
                                    >
                                        <Info size={14} className="text-mc-diamond shrink-0 mt-0.5" />
                                        <div className="min-w-0">
                                            <code className="text-xs font-mono text-mc-diamond font-bold block truncate">
                                                {flag}
                                            </code>
                                            <p className="text-[11px] font-mono text-white/50 mt-0.5 leading-snug">
                                                {FLAG_EXPLANATIONS[flag] || (flag.startsWith('-Xm') ? `Heap allocation limit: set to ${selectedRam}` : 'Configured JVM launch argument.')}
                                            </p>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </div>
                    </div>

                    {/* SECTION 5: CURRENT ACTIVE PROCESS STATUS */}
                    <div className="bg-black/60 border border-white/10 rounded-xl p-6 backdrop-blur-md space-y-4">
                        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                            <div className="flex items-center gap-2.5">
                                <Radio className={flagsStatus?.server_running ? "text-green-400 animate-pulse" : "text-white/40"} size={20} />
                                <h3 className="font-mono font-bold text-sm text-white">
                                    Active Process Status: {flagsStatus?.server_running ? (
                                        <span className="text-green-400">Online</span>
                                    ) : (
                                        <span className="text-white/40">Offline</span>
                                    )}
                                </h3>
                            </div>
                            {flagsStatus?.server_running ? (
                                flagsStatus.restart_required ? (
                                    <span className="px-3 py-1 rounded-full text-xs font-mono font-bold bg-amber-500/20 text-amber-300 border border-amber-500/40">
                                        Restart Required to Sync
                                    </span>
                                ) : (
                                    <span className="px-3 py-1 rounded-full text-xs font-mono font-bold bg-green-500/20 text-green-300 border border-green-500/40">
                                        Running with Up-to-Date Flags
                                    </span>
                                )
                            ) : (
                                <span className="px-3 py-1 rounded-full text-xs font-mono text-white/50 bg-white/5 border border-white/10">
                                    Will apply on next start
                                </span>
                            )}
                        </div>

                        {flagsStatus?.server_running && flagsStatus.active_args && flagsStatus.active_args.length > 0 && (
                            <div className="space-y-2">
                                <label className="block text-xs font-mono font-bold text-white/60 uppercase tracking-wider">
                                    Currently Running JVM Flags:
                                </label>
                                <div className="bg-black/90 border border-white/10 rounded-lg p-3 font-mono text-xs text-white/70 overflow-x-auto custom-scrollbar">
                                    java {flagsStatus.active_args.join(' ')} -jar paper.jar --nogui
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
}

function CategoryButton({ active, onClick, icon, label, count }: { active: boolean; onClick: () => void; icon: React.ReactNode; label: string; count?: number }) {
    return (
        <button
            onClick={onClick}
            className={`flex items-center gap-2 px-4 py-2.5 rounded-lg font-mono text-xs uppercase tracking-wider font-bold transition-all border shrink-0 ${
                active
                    ? 'bg-mc-gold/20 text-mc-gold border-mc-gold/40 shadow-[0_0_10px_rgba(255,170,0,0.2)]'
                    : 'bg-white/5 text-white/60 hover:text-white hover:bg-white/10 border-white/5'
            }`}
        >
            {icon}
            <span>{label}</span>
            {count !== undefined && (
                <span className="ml-1 px-1.5 py-0.5 rounded text-[10px] bg-black/40 text-white/70 border border-white/10">
                    {count}
                </span>
            )}
        </button>
    );
}

function PropertyField({ def, value, onChange }: { def: PropertyDefinition; value: string; onChange: (val: string) => void }) {
    const isBool = def.type === 'boolean' || value === 'true' || value === 'false';
    const isTrue = value === 'true';

    return (
        <div className="bg-black/40 border border-white/5 hover:border-white/15 p-4 rounded-xl space-y-2 transition-all">
            <div className="flex justify-between items-start gap-2">
                <div>
                    <label className="font-mono text-sm font-bold text-white block">{def.label}</label>
                    <code className="text-[11px] font-mono text-mc-diamond/70">{def.key}</code>
                </div>
                {isBool && (
                    <button
                        type="button"
                        onClick={() => onChange(isTrue ? 'false' : 'true')}
                        className={`px-3 py-1.5 rounded font-mono text-xs font-bold transition-all border flex items-center gap-1.5 ${
                            isTrue
                                ? 'bg-green-600 text-white border-green-500 shadow-[0_0_8px_rgba(34,197,94,0.3)]'
                                : 'bg-red-950/80 text-red-300 border-red-500/30'
                        }`}
                    >
                        {isTrue && <Check size={12} />}
                        {isTrue ? 'ENABLED' : 'DISABLED'}
                    </button>
                )}
            </div>

            <p className="text-xs font-mono text-white/50 leading-relaxed">{def.description}</p>

            {!isBool && def.type === 'select' && (
                <select
                    value={value}
                    onChange={(e) => onChange(e.target.value)}
                    className="w-full bg-black/80 border border-white/20 rounded-lg p-2.5 text-white font-mono text-xs focus:border-mc-diamond focus:outline-none"
                >
                    {def.options?.map(opt => (
                        <option key={opt} value={opt}>{opt}</option>
                    ))}
                </select>
            )}

            {!isBool && def.type !== 'select' && (
                <input
                    type={def.type === 'number' ? 'number' : 'text'}
                    value={value}
                    onChange={(e) => onChange(e.target.value)}
                    placeholder={`e.g. ${def.key}`}
                    className="w-full bg-black/80 border border-white/20 rounded-lg p-2.5 text-white font-mono text-xs focus:border-mc-diamond focus:outline-none placeholder-white/20"
                />
            )}
        </div>
    );
}
