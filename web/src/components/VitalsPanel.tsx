import { useMemo } from 'react';
import { Activity, Cpu, Zap, Globe, HardDrive, Clock, ShieldCheck, AlertTriangle, Layers } from 'lucide-react';
import type { Vitals } from '../types';

interface VitalsPanelProps {
    vitals: Vitals | null;
}

export default function VitalsPanel({ vitals }: VitalsPanelProps) {
    const isOnline = vitals?.status === 'Running';
    const isStarting = vitals?.status === 'Starting';

    // Format Uptime
    const formatUptime = (seconds?: number): string => {
        if (!seconds || seconds <= 0) return '0s';
        const d = Math.floor(seconds / (3600 * 24));
        const h = Math.floor((seconds % (3600 * 24)) / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        const s = Math.floor(seconds % 60);

        if (d > 0) return `${d}d ${h}h ${m}m`;
        if (h > 0) return `${h}h ${m}m ${s}s`;
        if (m > 0) return `${m}m ${s}s`;
        return `${s}s`;
    };

    // Helper to parse RAM string (e.g. "8G" -> 8GB in bytes)
    const parseMaxRam = (ramStr?: string): number => {
        if (!ramStr) return 1024 * 1024 * 1024;
        const value = parseInt(ramStr.slice(0, -1));
        const unit = ramStr.slice(-1).toUpperCase();
        if (unit === 'G') return value * 1024 * 1024 * 1024;
        if (unit === 'M') return value * 1024 * 1024;
        return value || 1024 * 1024 * 1024;
    };

    // Format bytes
    const formatBytes = (bytes?: number, decimals = 1): string => {
        if (!bytes || bytes === 0) return '0 B';
        const k = 1024;
        const dm = decimals < 0 ? 0 : decimals;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
    };

    const maxRamBytes = parseMaxRam(vitals?.total_memory);
    const ramPercent = vitals ? Math.min((vitals.ram / maxRamBytes) * 100, 100) : 0;
    const procCpuPercent = vitals ? Math.min(vitals.cpu, 100) : 0;
    const sysCpuPercent = vitals ? Math.min(vitals.system_cpu, 100) : 0;

    // TPS calculation & health color
    const tps = isOnline ? (vitals?.tps !== undefined ? vitals.tps : 20.0) : 0;
    const mspt = isOnline ? (vitals?.mspt !== undefined ? vitals.mspt : 20.0) : 0;

    const tpsColor = useMemo(() => {
        if (!isOnline) return 'text-white/40';
        if (tps >= 19.5) return 'text-mc-green';
        if (tps >= 15.0) return 'text-mc-gold';
        return 'text-red-400';
    }, [isOnline, tps]);

    const diskUsedPct = vitals?.disk_used_pct || 0;
    const diskLow = vitals?.disk_free ? vitals.disk_free < 5 * 1024 * 1024 * 1024 : false; // < 5GB

    return (
        <div className="flex flex-col gap-4 overflow-y-auto custom-scrollbar pr-1">
            {/* PANEL TITLE */}
            <div className="flex items-center justify-between pb-1 border-b border-white/10">
                <div className="flex items-center gap-2 text-mc-gold font-pixel text-xl">
                    <Activity size={20} />
                    <span>Server Vitals</span>
                </div>
                <span className="text-[10px] font-mono text-mc-diamond/70 uppercase tracking-widest bg-mc-diamond/10 px-2 py-0.5 rounded border border-mc-diamond/20">
                    Live Push
                </span>
            </div>

            {/* STATUS & UPTIME CARD */}
            <div className="bg-black/40 border border-white/10 p-3.5 rounded-xl space-y-2 backdrop-blur-sm">
                <div className="flex justify-between items-center">
                    <div className="flex items-center gap-2">
                        <div className={`w-2.5 h-2.5 rounded-full shadow-[0_0_8px] transition-colors ${
                            isOnline
                                ? 'bg-mc-green shadow-[#55aa55] animate-pulse'
                                : isStarting
                                ? 'bg-mc-gold shadow-[#ffaa00] animate-bounce'
                                : 'bg-red-500 shadow-red-500'
                        }`} />
                        <span className={`font-mono text-base font-bold ${
                            isOnline ? 'text-mc-green' : isStarting ? 'text-mc-gold' : 'text-red-400'
                        }`}>
                            {vitals?.status || "Offline"}
                        </span>
                    </div>
                    {isOnline && (
                        <span className="flex items-center gap-1 font-mono text-xs text-white/60 bg-white/5 px-2 py-0.5 rounded border border-white/5">
                            <Clock size={11} className="text-mc-gold" />
                            {formatUptime(vitals?.uptime_seconds)}
                        </span>
                    )}
                </div>

                {/* ACTIVE WORLD */}
                <div className="flex items-center justify-between pt-1 border-t border-white/5 text-xs font-mono">
                    <span className="text-white/40 flex items-center gap-1.5">
                        <Globe size={13} className="text-mc-diamond" /> Active World
                    </span>
                    <span className="text-mc-diamond font-bold truncate max-w-[140px]">
                        {vitals?.active_world || "world"}
                    </span>
                </div>
            </div>

            {/* MINECRAFT ENGINE HEALTH: TPS & MSPT */}
            <div className="bg-black/40 border border-white/10 p-3.5 rounded-xl space-y-2 backdrop-blur-sm">
                <div className="flex justify-between items-center">
                    <div className="flex items-center gap-1.5 text-xs font-mono text-white/50 uppercase tracking-wider">
                        <ShieldCheck size={14} className="text-mc-green" /> Engine Tick Rate
                    </div>
                    <div className="flex items-center gap-2">
                        <span className={`font-mono text-base font-bold ${tpsColor}`}>
                            {isOnline ? `${tps.toFixed(1)} TPS` : '-- TPS'}
                        </span>
                    </div>
                </div>

                <div className="flex justify-between items-center text-xs font-mono">
                    <span className="text-white/40">Tick Calc Time (MSPT)</span>
                    <span className={`font-bold ${isOnline && mspt > 50 ? 'text-red-400' : 'text-white/80'}`}>
                        {isOnline ? `${mspt.toFixed(1)} ms` : '--'}
                    </span>
                </div>

                {/* TPS Sparkline */}
                {isOnline && vitals?.history && vitals.history.length > 1 && (
                    <div className="pt-1">
                        <MiniSparkline
                            data={vitals.history.map(p => p.tps !== undefined ? p.tps : 20)}
                            min={0}
                            max={20}
                            color="#55ff55"
                            height={22}
                        />
                    </div>
                )}
            </div>

            {/* MULTI-CORE CPU TRACKER */}
            <div className="bg-black/40 border border-white/10 p-3.5 rounded-xl space-y-2.5 backdrop-blur-sm">
                <div className="flex justify-between items-center">
                    <div className="flex items-center gap-1.5 text-xs font-mono text-white/50 uppercase tracking-wider">
                        <Cpu size={14} className="text-mc-gold" /> CPU Workload
                    </div>
                    <span className="text-xs font-mono font-bold text-mc-gold">
                        Process: {procCpuPercent.toFixed(1)}%
                    </span>
                </div>

                {/* Overall Process CPU Bar */}
                <div className="w-full h-2 bg-white/10 rounded-full overflow-hidden">
                    <div
                        className="h-full bg-mc-gold shadow-[0_0_8px_#ffaa00] transition-all duration-500"
                        style={{ width: `${procCpuPercent}%` }}
                    />
                </div>

                {/* Per-Core Breakdown (Core 0, Core 1, etc.) */}
                {vitals?.cpu_cores && vitals.cpu_cores.length > 0 && (
                    <div className="pt-1 space-y-1.5 border-t border-white/5">
                        <div className="flex justify-between text-[11px] font-mono text-white/40">
                            <span>Per-Core Breakdown</span>
                            <span>Host Avg: {sysCpuPercent.toFixed(1)}%</span>
                        </div>
                        <div className="grid grid-cols-2 gap-2">
                            {vitals.cpu_cores.slice(0, 4).map((coreVal, idx) => (
                                <div key={idx} className="bg-black/50 p-2 rounded border border-white/5 space-y-1">
                                    <div className="flex justify-between text-[10px] font-mono">
                                        <span className="text-white/60">Core {idx}</span>
                                        <span className="text-white font-bold">{coreVal.toFixed(0)}%</span>
                                    </div>
                                    <div className="w-full h-1.5 bg-white/10 rounded-full overflow-hidden">
                                        <div
                                            className={`h-full transition-all duration-500 ${
                                                coreVal > 85 ? 'bg-red-400' : coreVal > 60 ? 'bg-mc-gold' : 'bg-mc-diamond'
                                            }`}
                                            style={{ width: `${Math.min(coreVal, 100)}%` }}
                                        />
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>
                )}

                {/* CPU Trend Sparkline */}
                {vitals?.history && vitals.history.length > 1 && (
                    <div className="pt-1">
                        <div className="text-[10px] font-mono text-white/40 mb-1">Process CPU Trend</div>
                        <MiniSparkline
                            data={vitals.history.map(p => p.cpu || 0)}
                            min={0}
                            max={100}
                            color="#ffaa00"
                            height={22}
                        />
                    </div>
                )}
            </div>

            {/* RAM (RSS & JVM HEAP ALLOCATION) */}
            <div className="bg-black/40 border border-white/10 p-3.5 rounded-xl space-y-2 backdrop-blur-sm">
                <div className="flex justify-between items-center">
                    <div className="flex items-center gap-1.5 text-xs font-mono text-white/50 uppercase tracking-wider">
                        <Zap size={14} className="text-mc-diamond" /> RAM (RSS)
                    </div>
                    <span className="text-xs font-mono text-mc-diamond font-bold">
                        {vitals ? formatBytes(vitals.ram) : '0 GB'} / {vitals?.total_memory || '?'}
                    </span>
                </div>

                <div className="w-full h-2 bg-white/10 rounded-full overflow-hidden">
                    <div
                        className="h-full bg-mc-diamond shadow-[0_0_8px_#55ffff] transition-all duration-500"
                        style={{ width: `${ramPercent}%` }}
                    />
                </div>

                {vitals?.threads && (
                    <div className="flex justify-between items-center text-[11px] font-mono text-white/40 pt-1">
                        <span className="flex items-center gap-1">
                            <Layers size={12} /> JVM Active Threads
                        </span>
                        <span className="text-white font-bold">{vitals.threads}</span>
                    </div>
                )}

                {/* RAM Trend Sparkline */}
                {vitals?.history && vitals.history.length > 1 && (
                    <div className="pt-1">
                        <div className="text-[10px] font-mono text-white/40 mb-1">Memory Allocation Trend</div>
                        <MiniSparkline
                            data={vitals.history.map(p => p.ram || 0)}
                            min={0}
                            max={maxRamBytes}
                            color="#55ffff"
                            height={22}
                        />
                    </div>
                )}
            </div>

            {/* STORAGE & DISK HEADROOM */}
            {vitals?.disk_total && vitals.disk_total > 0 && (
                <div className="bg-black/40 border border-white/10 p-3.5 rounded-xl space-y-2 backdrop-blur-sm">
                    <div className="flex justify-between items-center">
                        <div className="flex items-center gap-1.5 text-xs font-mono text-white/50 uppercase tracking-wider">
                            <HardDrive size={14} className={diskLow ? "text-red-400" : "text-white/60"} /> Disk Space
                        </div>
                        <span className={`text-xs font-mono font-bold ${diskLow ? 'text-red-400' : 'text-white/80'}`}>
                            {formatBytes(vitals.disk_free)} Free
                        </span>
                    </div>

                    <div className="w-full h-2 bg-white/10 rounded-full overflow-hidden">
                        <div
                            className={`h-full transition-all duration-500 ${
                                diskLow || diskUsedPct > 90 ? 'bg-red-500' : 'bg-white/40'
                            }`}
                            style={{ width: `${Math.min(diskUsedPct, 100)}%` }}
                        />
                    </div>

                    {diskLow && (
                        <div className="flex items-center gap-1 text-[11px] font-mono text-red-300 bg-red-950/40 border border-red-500/20 px-2 py-1 rounded">
                            <AlertTriangle size={12} className="shrink-0" />
                            <span>Low disk space (&lt;5 GB remaining)</span>
                        </div>
                    )}
                </div>
            )}

            {/* ONLINE PLAYERS */}
            <div className="bg-black/40 border border-white/10 p-3.5 rounded-xl flex flex-col min-h-[140px] backdrop-blur-sm">
                <div className="text-xs text-white/50 mb-2 uppercase tracking-wider flex justify-between font-mono">
                    <span>Online Players</span>
                    <span className="text-mc-green font-mono font-bold">{vitals?.player_count || 0}</span>
                </div>

                <div className="flex-1 overflow-y-auto custom-scrollbar space-y-1.5 max-h-40 pr-1">
                    {vitals?.player_list && vitals.player_list.length > 0 ? (
                        vitals.player_list.map(player => (
                            <div key={player.name} className="flex items-center gap-2.5 bg-white/5 p-1.5 rounded-lg border border-white/5 hover:bg-white/10 transition-colors">
                                <img
                                    src={`https://api.mineatar.io/face/${player.uuid || player.name}`}
                                    alt={player.name}
                                    className="w-6 h-6 rounded bg-black/50"
                                />
                                <span className="text-xs font-mono text-white/90 font-bold truncate">{player.name}</span>
                            </div>
                        ))
                    ) : (
                        <div className="flex h-20 items-center justify-center text-white/30 text-xs font-mono italic">
                            No players online
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}

// Lightweight SVG Sparkline Component
function MiniSparkline({ data, min, max, color, height = 24 }: { data: number[]; min: number; max: number; color: string; height?: number }) {
    if (!data || data.length < 2) return null;

    const width = 240;
    const range = max - min || 1;

    const points = data.map((val, idx) => {
        const x = (idx / (data.length - 1)) * width;
        const normalized = Math.max(0, Math.min(1, (val - min) / range));
        const y = height - (normalized * (height - 4)) - 2;
        return `${x.toFixed(1)},${y.toFixed(1)}`;
    }).join(' ');

    return (
        <svg viewBox={`0 0 ${width} ${height}`} className="w-full overflow-visible" style={{ height: `${height}px` }}>
            <polyline
                fill="none"
                stroke={color}
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                points={points}
                opacity="0.85"
            />
        </svg>
    );
}
