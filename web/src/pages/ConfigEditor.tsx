import React, { useEffect, useState, useMemo } from 'react';
import { Settings, Save, RefreshCw, AlertTriangle, CheckCircle, XCircle, Search, Sliders, FileCode, Check, Shield, Globe, Cpu, Radio } from 'lucide-react';

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
    category: 'general' | 'gameplay' | 'security' | 'performance' | 'rcon';
}

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
    const [activeTab, setActiveTab] = useState<'visual' | 'raw' | 'all'>('visual');
    const [activeCategory, setActiveCategory] = useState<'general' | 'gameplay' | 'security' | 'performance' | 'rcon'>('general');
    const [searchQuery, setSearchQuery] = useState('');
    const [toasts, setToasts] = useState<Toast[]>([]);

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

    useEffect(() => {
        fetchConfig();
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

    // Filtered properties for the "All Properties" tab
    const filteredAllProperties = useMemo(() => {
        const query = searchQuery.toLowerCase().trim();
        const entries = Object.entries(properties);
        if (!query) return entries;
        return entries.filter(([k, v]) => k.toLowerCase().includes(query) || v.toLowerCase().includes(query));
    }, [properties, searchQuery]);

    const categorizedProperties = useMemo(() => {
        return KNOWN_PROPERTIES.filter(p => p.category === activeCategory);
    }, [activeCategory]);

    return (
        <div className="space-y-6 relative min-h-[600px] flex flex-col">
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

            {/* HEADER */}
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div>
                    <h1 className="text-3xl font-pixel text-mc-diamond flex items-center gap-3">
                        <Settings className="text-mc-diamond" size={28} /> Server Configuration
                    </h1>
                    <p className="text-white/50 font-mono text-sm mt-1">
                        Visual and raw configuration editor for <code className="text-mc-gold font-mono">server.properties</code>
                    </p>
                </div>

                <div className="flex items-center gap-3">
                    <button
                        onClick={fetchConfig}
                        disabled={loading || saving}
                        className="p-2.5 bg-white/5 hover:bg-white/10 rounded-lg transition-colors border border-white/10 text-white/80"
                        title="Reload from Disk"
                    >
                        <RefreshCw size={18} className={loading ? "animate-spin" : ""} />
                    </button>
                    <button
                        onClick={handleSave}
                        disabled={saving || !hasUnsavedChanges}
                        className={`
                            px-5 py-2.5 rounded-lg font-mono font-bold text-sm flex items-center gap-2 transition-all shadow-lg border
                            ${hasUnsavedChanges
                                ? 'bg-green-600 hover:bg-green-500 text-white border-green-400/50 shadow-green-900/40 cursor-pointer animate-pulse'
                                : 'bg-white/10 text-white/40 border-transparent cursor-not-allowed'}
                        `}
                    >
                        {saving ? <RefreshCw size={16} className="animate-spin" /> : <Save size={16} />}
                        {saving ? 'Saving...' : hasUnsavedChanges ? 'Save Changes' : 'Saved'}
                    </button>
                </div>
            </div>

            {/* RESTART NOTICE BANNER */}
            <div className="bg-amber-950/40 border border-amber-500/30 p-4 rounded-xl flex items-start gap-3 backdrop-blur-md">
                <AlertTriangle className="text-amber-400 shrink-0 mt-0.5" size={20} />
                <div className="text-xs font-mono text-amber-200/90 leading-relaxed">
                    <strong className="text-amber-300 font-bold uppercase tracking-wider block mb-0.5">Minecraft Engine Notice:</strong>
                    Changes made to <span className="text-white font-bold">server.properties</span> are loaded when Minecraft boots. After saving your changes, restart the server from the Console to apply them.
                </div>
            </div>

            {/* TABS SELECTOR */}
            <div className="flex items-center gap-2 border-b border-white/10 pb-2">
                <button
                    onClick={() => setActiveTab('visual')}
                    className={`flex items-center gap-2 px-4 py-2 rounded-lg font-mono text-sm font-bold transition-all ${
                        activeTab === 'visual'
                            ? 'bg-mc-diamond/20 text-mc-diamond border border-mc-diamond/30 shadow-[0_0_12px_rgba(85,255,255,0.15)]'
                            : 'text-white/60 hover:text-white hover:bg-white/5 border border-transparent'
                    }`}
                >
                    <Sliders size={16} /> Categorized Settings
                </button>
                <button
                    onClick={() => setActiveTab('all')}
                    className={`flex items-center gap-2 px-4 py-2 rounded-lg font-mono text-sm font-bold transition-all ${
                        activeTab === 'all'
                            ? 'bg-mc-diamond/20 text-mc-diamond border border-mc-diamond/30 shadow-[0_0_12px_rgba(85,255,255,0.15)]'
                            : 'text-white/60 hover:text-white hover:bg-white/5 border border-transparent'
                    }`}
                >
                    <Search size={16} /> All Properties ({Object.keys(properties).length})
                </button>
                <button
                    onClick={() => setActiveTab('raw')}
                    className={`flex items-center gap-2 px-4 py-2 rounded-lg font-mono text-sm font-bold transition-all ${
                        activeTab === 'raw'
                            ? 'bg-mc-diamond/20 text-mc-diamond border border-mc-diamond/30 shadow-[0_0_12px_rgba(85,255,255,0.15)]'
                            : 'text-white/60 hover:text-white hover:bg-white/5 border border-transparent'
                    }`}
                >
                    <FileCode size={16} /> Raw File Editor
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
                        />
                        <CategoryButton
                            active={activeCategory === 'gameplay'}
                            onClick={() => setActiveCategory('gameplay')}
                            icon={<Sliders size={16} />}
                            label="Gameplay & World"
                        />
                        <CategoryButton
                            active={activeCategory === 'security'}
                            onClick={() => setActiveCategory('security')}
                            icon={<Shield size={16} />}
                            label="Security & Whitelist"
                        />
                        <CategoryButton
                            active={activeCategory === 'performance'}
                            onClick={() => setActiveCategory('performance')}
                            icon={<Cpu size={16} />}
                            label="Performance & Ticks"
                        />
                        <CategoryButton
                            active={activeCategory === 'rcon'}
                            onClick={() => setActiveCategory('rcon')}
                            icon={<Radio size={16} />}
                            label="RCON Console"
                        />
                    </div>

                    {/* CATEGORY CONTROLS */}
                    <div className="bg-black/60 border border-white/10 rounded-xl p-6 backdrop-blur-md space-y-6">
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
                            placeholder="Filter properties by key or value (e.g. motd, difficulty, pvp)..."
                            className="w-full bg-black/60 border border-white/10 rounded-xl pl-12 pr-4 py-3 text-white font-mono text-sm focus:border-mc-diamond focus:outline-none placeholder-white/30 backdrop-blur-md"
                        />
                    </div>

                    {/* TABLE OF ALL PROPERTIES */}
                    <div className="bg-black/60 border border-white/10 rounded-xl overflow-hidden backdrop-blur-md">
                        <div className="max-h-[600px] overflow-y-auto divide-y divide-white/5 custom-scrollbar">
                            {filteredAllProperties.map(([key, val]) => (
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
                                                            ? 'bg-green-600 text-white border-green-500'
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
                                                className="w-full bg-black/80 border border-white/20 rounded p-2 text-white font-mono text-xs focus:border-mc-diamond focus:outline-none"
                                            />
                                        )}
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>
                </div>
            )}

            {activeTab === 'raw' && (
                <div className="bg-black/60 border border-white/10 rounded-xl p-6 backdrop-blur-md flex-1 flex flex-col space-y-4">
                    <div className="flex justify-between items-center text-xs font-mono text-white/50">
                        <span>Direct Editor Mode (Preserves custom comments & key groupings)</span>
                        <span>{rawText.split('\n').length} lines</span>
                    </div>
                    <textarea
                        value={rawText}
                        onChange={(e) => setRawText(e.target.value)}
                        rows={22}
                        className="w-full flex-1 bg-black/90 border border-white/20 rounded-xl p-4 font-mono text-xs text-white leading-relaxed focus:border-mc-diamond focus:outline-none custom-scrollbar"
                        placeholder="# server.properties"
                        spellCheck={false}
                    />
                </div>
            )}
        </div>
    );
}

function CategoryButton({ active, onClick, icon, label }: { active: boolean; onClick: () => void; icon: React.ReactNode; label: string }) {
    return (
        <button
            onClick={onClick}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg font-mono text-xs uppercase tracking-wider font-bold transition-all border ${
                active
                    ? 'bg-mc-gold/20 text-mc-gold border-mc-gold/40 shadow-[0_0_10px_rgba(255,170,0,0.2)]'
                    : 'bg-white/5 text-white/60 hover:text-white hover:bg-white/10 border-white/5'
            }`}
        >
            {icon}
            <span>{label}</span>
        </button>
    );
}

function PropertyField({ def, value, onChange }: { def: PropertyDefinition; value: string; onChange: (val: string) => void }) {
    const isBool = def.type === 'boolean';
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
