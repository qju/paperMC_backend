import { useEffect, useState, useCallback, useId } from 'react';
import {
    Activity, Play, Square, ExternalLink, RefreshCw, Trash2, Search,
    Cpu, HardDrive, Clock, CheckCircle2, AlertTriangle, Copy, Check,
    Terminal, Zap, ChevronRight, X, Sparkles, Flame, HelpCircle
} from 'lucide-react';
import { useSocket } from '../hooks/useSocket';
import type { ProfilerReport, ProfilerReportsResponse, HealthSnapshot } from '../types';

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

const REPORT_TYPE_FILTERS = [
    { label: 'All Reports', value: '' },
    { label: 'Spark Profiles', value: 'spark_profile' },
    { label: 'Spark Health', value: 'spark_health' },
    { label: 'Aikar Timings', value: 'timings' },
];

function getTypeBadge(type: string) {
    switch (type) {
        case 'spark_profile':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-amber-500/20 text-amber-300 border border-amber-500/40">
                    <Flame size={13} />
                    Spark Profile
                </span>
            );
        case 'spark_health':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-emerald-500/20 text-emerald-300 border border-emerald-500/40">
                    <Activity size={13} />
                    Spark Health
                </span>
            );
        case 'timings':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-blue-500/20 text-blue-300 border border-blue-500/40">
                    <Clock size={13} />
                    Aikar Timings
                </span>
            );
        default:
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-slate-500/20 text-slate-300 border border-slate-500/40">
                    <HelpCircle size={13} />
                    {type || 'Report'}
                </span>
            );
    }
}

export default function PerformanceProfiler() {
    const searchInputId = useId();
    const typeFilterSelectId = useId();
    const [reports, setReports] = useState<ProfilerReport[]>([]);
    const [total, setTotal] = useState(0);
    const [loading, setLoading] = useState(true);
    const [refreshing, setRefreshing] = useState(false);
    const [typeFilter, setTypeFilter] = useState('');
    const [searchTerm, setSearchTerm] = useState('');

    // Health snapshot state
    const [health, setHealth] = useState<HealthSnapshot | null>(null);
    const [healthLoading, setHealthLoading] = useState(false);

    // Trigger action states
    const [triggerLoading, setTriggerLoading] = useState<string | null>(null);
    const [customCmd, setCustomCmd] = useState('');

    // Modal view states
    const [selectedReport, setSelectedReport] = useState<ProfilerReport | null>(null);
    const [deleteModalOpen, setDeleteModalOpen] = useState(false);
    const [reportToDelete, setReportToDelete] = useState<number | 'all' | null>(null);

    // Clipboard copy indicators
    const [copiedReportId, setCopiedReportId] = useState<number | null>(null);
    const [copiedRaw, setCopiedRaw] = useState(false);

    // Toast alerts
    const [toasts, setToasts] = useState<Toast[]>([]);

    const addToast = useCallback((message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts((prev) => [...prev, { id, message, type }]);
        setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
        }, 4000);
    }, []);

    // WebSocket hook for live profiler events
    const { profilerEvent, clearProfilerEvent, liveVitals } = useSocket();

    // Fetch reports
    const fetchReports = useCallback(async (silent = false) => {
        if (!silent) setRefreshing(true);
        const token = localStorage.getItem('token');
        try {
            const url = new URL('/api/profiler/reports', window.location.origin);
            url.searchParams.set('limit', '50');
            if (typeFilter) url.searchParams.set('type', typeFilter);

            const res = await fetch(url.toString(), {
                headers: { Authorization: `Bearer ${token}` },
            });
            if (res.ok) {
                const data: ProfilerReportsResponse = await res.json();
                setReports(data.reports || []);
                setTotal(data.total || 0);
            }
        } catch {
            if (!silent) addToast('Failed to fetch profiler reports', 'error');
        } finally {
            setLoading(false);
            if (!silent) setRefreshing(false);
        }
    }, [typeFilter, addToast]);

    // Fetch live health snapshot
    const fetchHealthSnapshot = useCallback(async (save = false) => {
        setHealthLoading(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/profiler/health', {
                method: 'POST',
                headers: {
                    Authorization: `Bearer ${token}`,
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ save }),
            });
            if (res.ok) {
                const data = await res.json();
                if (data.health) {
                    setHealth(data.health);
                    if (save) {
                        fetchReports(true);
                        addToast('Saved health snapshot to reports archive', 'success');
                    }
                }
            }
        } catch {
            addToast('Failed to fetch performance health telemetry', 'error');
        } finally {
            setHealthLoading(false);
        }
    }, [fetchReports, addToast]);

    // Handle profiler trigger
    const handleTrigger = async (action: string, command?: string) => {
        setTriggerLoading(action);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/profiler/trigger', {
                method: 'POST',
                headers: {
                    Authorization: `Bearer ${token}`,
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ action, command }),
            });
            const data = await res.json();
            if (res.ok) {
                addToast(`Command sent: /${data.command}`, 'success');
                if (action === 'health') {
                    // Give server ~1.5s to generate output before querying snapshot
                    setTimeout(() => fetchHealthSnapshot(false), 1500);
                }
            } else {
                addToast(data.error || 'Failed to dispatch profiler command', 'error');
            }
        } catch {
            addToast('Network error triggering profiler command', 'error');
        } finally {
            setTriggerLoading(null);
            if (action === 'custom') setCustomCmd('');
        }
    };

    // Delete single report or clear all
    const handleDelete = async () => {
        if (reportToDelete === null) return;
        const token = localStorage.getItem('token');
        try {
            const url = reportToDelete === 'all'
                ? '/api/profiler/reports?id=all'
                : `/api/profiler/reports/${reportToDelete}`;

            const res = await fetch(url, {
                method: 'DELETE',
                headers: { Authorization: `Bearer ${token}` },
            });
            if (res.ok) {
                addToast(reportToDelete === 'all' ? 'Cleared all profiler reports' : 'Report deleted', 'success');
                fetchReports(true);
                if (selectedReport && (reportToDelete === 'all' || selectedReport.id === reportToDelete)) {
                    setSelectedReport(null);
                }
            } else {
                addToast('Failed to delete report', 'error');
            }
        } catch {
            addToast('Network error during deletion', 'error');
        } finally {
            setDeleteModalOpen(false);
            setReportToDelete(null);
        }
    };

    // Initial load
    useEffect(() => {
        fetchReports();
        fetchHealthSnapshot(false);
    }, [fetchReports, fetchHealthSnapshot]);

    // Listen for WebSocket profiler link broadcasts
    useEffect(() => {
        if (profilerEvent) {
            fetchReports(true);
            addToast(`New profiler report captured: ${profilerEvent.title}`, 'success');
        }
    }, [profilerEvent, fetchReports, addToast]);

    // Copy to clipboard helper
    const copyToClipboard = (text: string, isRaw = false, reportId?: number) => {
        navigator.clipboard.writeText(text);
        if (isRaw) {
            setCopiedRaw(true);
            setTimeout(() => setCopiedRaw(false), 2000);
        } else if (reportId !== undefined) {
            setCopiedReportId(reportId);
            setTimeout(() => setCopiedReportId(null), 2000);
        }
    };

    const filteredReports = reports.filter((r) => {
        if (!searchTerm) return true;
        const s = searchTerm.toLowerCase();
        return (
            r.title.toLowerCase().includes(s) ||
            r.url.toLowerCase().includes(s) ||
            r.summary.toLowerCase().includes(s) ||
            r.report_type.toLowerCase().includes(s)
        );
    });

    // Helper for TPS color
    const getTPSColor = (tpsStr?: string) => {
        if (!tpsStr) return 'text-slate-400';
        const num = parseFloat(tpsStr);
        if (isNaN(num)) return 'text-slate-400';
        if (num >= 19.5) return 'text-emerald-400';
        if (num >= 18.0) return 'text-amber-400';
        return 'text-red-400';
    };

    return (
        <div className="space-y-6 max-w-7xl mx-auto pb-12">
            {/* TOASTS */}
            <div className="fixed bottom-6 right-6 z-50 space-y-2 pointer-events-none">
                {toasts.map((t) => (
                    <div
                        key={t.id}
                        className={`pointer-events-auto px-4 py-3 rounded-lg shadow-xl text-sm font-mono flex items-center gap-2 border backdrop-blur-md transition-all duration-300 ${
                            t.type === 'success'
                                ? 'bg-emerald-950/90 text-emerald-200 border-emerald-500/30'
                                : 'bg-red-950/90 text-red-200 border-red-500/30'
                        }`}
                    >
                        {t.type === 'success' ? <CheckCircle2 size={16} /> : <AlertTriangle size={16} />}
                        {t.message}
                    </div>
                ))}
            </div>

            {/* LIVE PROFILER EVENT BANNER */}
            {profilerEvent && (
                <div className="p-4 rounded-xl bg-gradient-to-r from-amber-950/70 via-black/80 to-amber-950/70 border border-amber-500/50 backdrop-blur-xl flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 animate-in fade-in slide-in-from-top-4 duration-300 shadow-2xl">
                    <div className="flex items-center gap-3">
                        <div className="p-2.5 rounded-lg bg-amber-500/20 text-amber-300 border border-amber-500/30">
                            <Flame size={24} className="animate-pulse text-amber-400" />
                        </div>
                        <div>
                            <div className="flex items-center gap-2">
                                <span className="text-xs font-mono font-bold uppercase tracking-wider text-amber-400">Captured Link Detected</span>
                                <span className="text-xs px-2 py-0.5 rounded bg-amber-500/20 text-amber-300">{profilerEvent.report_type}</span>
                            </div>
                            <h4 className="text-sm font-semibold text-white mt-0.5">{profilerEvent.title}</h4>
                            <p className="text-xs font-mono text-white/60 truncate max-w-md">{profilerEvent.url}</p>
                        </div>
                    </div>
                    <div className="flex items-center gap-2 w-full sm:w-auto justify-end">
                        <a
                            href={profilerEvent.url}
                            target="_blank"
                            rel="noreferrer"
                            className="flex items-center gap-1.5 px-4 py-2 bg-amber-600 hover:bg-amber-500 text-white font-mono text-xs font-semibold rounded-lg transition-colors shadow-lg"
                        >
                            Open Report <ExternalLink size={14} />
                        </a>
                        <button
                            onClick={clearProfilerEvent}
                            className="p-2 text-white/50 hover:text-white rounded-lg hover:bg-white/5 transition-colors"
                            title="Dismiss"
                        >
                            <X size={16} />
                        </button>
                    </div>
                </div>
            )}

            {/* HEADER */}
            <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4 border-b border-white/10 pb-5">
                <div>
                    <h1 className="text-2xl font-bold flex items-center gap-3 text-white font-pixel">
                        <Activity className="text-emerald-400" size={28} />
                        Performance Profiler & Spark
                    </h1>
                    <p className="text-sm text-white/60 mt-1 font-mono">
                        Real-time tick diagnostics, Spark CPU sampler control, Aikar timings reports, and automated JVM tuning advice.
                    </p>
                </div>
                <div className="flex flex-wrap items-center gap-2.5">
                    <button
                        onClick={() => handleTrigger('health')}
                        disabled={triggerLoading === 'health'}
                        className="flex items-center gap-2 px-3.5 py-2 rounded-lg bg-emerald-600/80 hover:bg-emerald-600 text-white font-mono text-xs font-semibold border border-emerald-500/40 transition-all disabled:opacity-50 shadow-md"
                    >
                        <Activity size={15} className={triggerLoading === 'health' ? 'animate-spin' : ''} />
                        Run Spark Health
                    </button>
                    <button
                        onClick={() => handleTrigger('sampler_start')}
                        disabled={triggerLoading === 'sampler_start'}
                        className="flex items-center gap-2 px-3.5 py-2 rounded-lg bg-amber-600/80 hover:bg-amber-600 text-white font-mono text-xs font-semibold border border-amber-500/40 transition-all disabled:opacity-50 shadow-md"
                    >
                        <Play size={15} />
                        Start Sampler
                    </button>
                    <button
                        onClick={() => handleTrigger('sampler_stop')}
                        disabled={triggerLoading === 'sampler_stop'}
                        className="flex items-center gap-2 px-3.5 py-2 rounded-lg bg-red-600/80 hover:bg-red-600 text-white font-mono text-xs font-semibold border border-red-500/40 transition-all disabled:opacity-50 shadow-md"
                    >
                        <Square size={15} />
                        Stop & Upload
                    </button>
                    <button
                        onClick={() => handleTrigger('timings_paste')}
                        disabled={triggerLoading === 'timings_paste'}
                        className="flex items-center gap-2 px-3.5 py-2 rounded-lg bg-blue-600/80 hover:bg-blue-600 text-white font-mono text-xs font-semibold border border-blue-500/40 transition-all disabled:opacity-50 shadow-md"
                    >
                        <Clock size={15} />
                        Paste Timings
                    </button>
                    <button
                        onClick={() => {
                            fetchReports();
                            fetchHealthSnapshot(false);
                        }}
                        disabled={refreshing}
                        className="p-2 rounded-lg bg-white/5 hover:bg-white/10 text-white/80 border border-white/10 transition-colors disabled:opacity-50"
                        title="Refresh All"
                    >
                        <RefreshCw size={18} className={refreshing ? 'animate-spin' : ''} />
                    </button>
                </div>
            </div>

            {/* HEALTH TELEMETRY GRID */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                {/* TPS CARD */}
                <div className="p-5 rounded-xl bg-black/40 border border-white/10 backdrop-blur-md relative overflow-hidden flex flex-col justify-between">
                    <div className="flex items-center justify-between text-white/60 mb-2">
                        <span className="text-xs font-mono uppercase tracking-wider font-semibold">Tick Rate (TPS)</span>
                        <Activity size={18} className="text-emerald-400" />
                    </div>
                    <div>
                        <div className={`text-2xl font-bold font-mono ${getTPSColor(health?.tps || String(liveVitals?.tps || '20.0'))}`}>
                            {health?.tps || (liveVitals?.tps ? liveVitals.tps.toFixed(2) : '20.00')}
                        </div>
                        <p className="text-xs text-white/40 font-mono mt-1">Target: 20.0 ticks / sec</p>
                    </div>
                    <div className="mt-3 w-full bg-white/5 h-1.5 rounded-full overflow-hidden">
                        <div
                            className="bg-emerald-400 h-full rounded-full transition-all duration-500"
                            style={{ width: `${Math.min(100, ((parseFloat(health?.tps || String(liveVitals?.tps || '20.0')) || 20.0) / 20.0) * 100)}%` }}
                        />
                    </div>
                </div>

                {/* MSPT CARD */}
                <div className="p-5 rounded-xl bg-black/40 border border-white/10 backdrop-blur-md relative overflow-hidden flex flex-col justify-between">
                    <div className="flex items-center justify-between text-white/60 mb-2">
                        <span className="text-xs font-mono uppercase tracking-wider font-semibold">Tick Duration (MSPT)</span>
                        <Clock size={18} className="text-amber-400" />
                    </div>
                    <div>
                        <div className="text-2xl font-bold font-mono text-amber-300">
                            {health?.mspt || (liveVitals?.mspt ? `${liveVitals.mspt.toFixed(2)} ms` : '15.00 ms')}
                        </div>
                        <p className="text-xs text-white/40 font-mono mt-1">Budget: ≤ 50.0 ms / tick</p>
                    </div>
                    <div className="mt-3 w-full bg-white/5 h-1.5 rounded-full overflow-hidden">
                        <div
                            className="bg-amber-400 h-full rounded-full transition-all duration-500"
                            style={{ width: `${Math.min(100, ((parseFloat(health?.mspt || String(liveVitals?.mspt || '15.0')) || 15.0) / 50.0) * 100)}%` }}
                        />
                    </div>
                </div>

                {/* CPU CARD */}
                <div className="p-5 rounded-xl bg-black/40 border border-white/10 backdrop-blur-md relative overflow-hidden flex flex-col justify-between">
                    <div className="flex items-center justify-between text-white/60 mb-2">
                        <span className="text-xs font-mono uppercase tracking-wider font-semibold">CPU Utilization</span>
                        <Cpu size={18} className="text-blue-400" />
                    </div>
                    <div>
                        <div className="text-2xl font-bold font-mono text-blue-300">
                            {health?.cpu_process || (liveVitals?.cpu ? `${liveVitals.cpu.toFixed(1)}%` : '0.0%')}
                        </div>
                        <p className="text-xs text-white/40 font-mono mt-1">
                            System: {health?.cpu_system || (liveVitals?.system_cpu ? `${liveVitals.system_cpu.toFixed(1)}%` : '0.0%')}
                        </p>
                    </div>
                    <div className="mt-3 w-full bg-white/5 h-1.5 rounded-full overflow-hidden">
                        <div
                            className="bg-blue-400 h-full rounded-full transition-all duration-500"
                            style={{ width: `${Math.min(100, parseFloat(health?.cpu_process || String(liveVitals?.cpu || '0.0')) || 0)}%` }}
                        />
                    </div>
                </div>

                {/* MEMORY & GC CARD */}
                <div className="p-5 rounded-xl bg-black/40 border border-white/10 backdrop-blur-md relative overflow-hidden flex flex-col justify-between">
                    <div className="flex items-center justify-between text-white/60 mb-2">
                        <span className="text-xs font-mono uppercase tracking-wider font-semibold">Memory & Heap</span>
                        <HardDrive size={18} className="text-purple-400" />
                    </div>
                    <div>
                        <div className="text-2xl font-bold font-mono text-purple-300">
                            {health?.memory_percent || 'N/A'}
                        </div>
                        <p className="text-xs text-white/40 font-mono mt-1 truncate">
                            {health?.memory_used ? `${health.memory_used} / ${health.memory_max}` : 'JVM Heap allocation'}
                        </p>
                    </div>
                    <div className="mt-3 w-full bg-white/5 h-1.5 rounded-full overflow-hidden">
                        <div
                            className="bg-purple-400 h-full rounded-full transition-all duration-500"
                            style={{ width: `${Math.min(100, parseFloat(health?.memory_percent || '0') || 0)}%` }}
                        />
                    </div>
                </div>
            </div>

            {/* ACTIONABLE TUNING HEURISTICS & GC ADVICE */}
            <div className="p-5 rounded-xl bg-black/40 border border-white/10 backdrop-blur-md space-y-3">
                <div className="flex items-center justify-between">
                    <h3 className="text-sm font-semibold font-mono uppercase tracking-wider text-white flex items-center gap-2">
                        <Sparkles size={16} className="text-amber-400" />
                        Automated Performance Advice & Tuning Heuristics
                    </h3>
                    <button
                        onClick={() => fetchHealthSnapshot(true)}
                        disabled={healthLoading}
                        className="text-xs font-mono px-3 py-1 rounded bg-white/5 hover:bg-white/10 text-white/80 border border-white/10 transition-colors disabled:opacity-50"
                    >
                        Save Snapshot to Reports
                    </button>
                </div>

                {health?.gc_summary && (
                    <div className="p-3 rounded-lg bg-purple-950/30 border border-purple-500/20 text-xs font-mono text-purple-200">
                        <span className="font-semibold text-purple-300">GC Activity: </span>
                        {health.gc_summary}
                    </div>
                )}

                <div className="space-y-2">
                    {health?.advice && health.advice.length > 0 ? (
                        health.advice.map((tip, idx) => (
                            <div
                                key={idx}
                                className="flex items-start gap-2.5 text-xs font-mono p-3 rounded-lg bg-white/5 border border-white/5 text-white/80"
                            >
                                <ChevronRight size={14} className="text-emerald-400 shrink-0 mt-0.5" />
                                <span>{tip}</span>
                            </div>
                        ))
                    ) : (
                        <p className="text-xs font-mono text-white/40 italic">
                            Run /spark health or trigger a snapshot above to evaluate server tuning.
                        </p>
                    )}
                </div>
            </div>

            {/* QUICK COMMAND TRIGGER BAR */}
            <div className="p-5 rounded-xl bg-black/40 border border-white/10 backdrop-blur-md space-y-4">
                <h3 className="text-sm font-semibold font-mono uppercase tracking-wider text-white flex items-center gap-2">
                    <Terminal size={16} className="text-mc-diamond" />
                    Profiler Command Dispatcher
                </h3>

                <div className="flex flex-wrap items-center gap-2">
                    <button
                        onClick={() => handleTrigger('custom', 'spark tps')}
                        className="px-3 py-1.5 rounded-lg bg-white/5 hover:bg-white/10 text-white/80 font-mono text-xs border border-white/10 transition-colors"
                    >
                        /spark tps
                    </button>
                    <button
                        onClick={() => handleTrigger('custom', 'spark activity')}
                        className="px-3 py-1.5 rounded-lg bg-white/5 hover:bg-white/10 text-white/80 font-mono text-xs border border-white/10 transition-colors"
                    >
                        /spark activity
                    </button>
                    <button
                        onClick={() => handleTrigger('custom', 'spark gc')}
                        className="px-3 py-1.5 rounded-lg bg-white/5 hover:bg-white/10 text-white/80 font-mono text-xs border border-white/10 transition-colors"
                    >
                        /spark gc
                    </button>
                    <button
                        onClick={() => handleTrigger('custom', 'spark tickmonitor')}
                        className="px-3 py-1.5 rounded-lg bg-white/5 hover:bg-white/10 text-white/80 font-mono text-xs border border-white/10 transition-colors"
                    >
                        /spark tickmonitor
                    </button>
                    <button
                        onClick={() => handleTrigger('timings_reset')}
                        className="px-3 py-1.5 rounded-lg bg-white/5 hover:bg-white/10 text-white/80 font-mono text-xs border border-white/10 transition-colors"
                    >
                        /timings reset
                    </button>
                </div>

                <form
                    onSubmit={(e) => {
                        e.preventDefault();
                        if (customCmd.trim()) handleTrigger('custom', customCmd.trim());
                    }}
                    className="flex items-center gap-2"
                >
                    <div className="relative flex-1">
                        <span className="absolute left-3.5 top-1/2 -translate-y-1/2 font-mono text-xs text-white/40">/</span>
                        <input
                            type="text"
                            placeholder="Enter custom profiler command (e.g. spark sampler --timeout 60)..."
                            value={customCmd}
                            onChange={(e) => setCustomCmd(e.target.value)}
                            className="w-full pl-7 pr-4 py-2.5 rounded-lg bg-black/60 border border-white/10 text-white font-mono text-xs focus:outline-none focus:border-mc-diamond transition-colors"
                        />
                    </div>
                    <button
                        type="submit"
                        disabled={!customCmd.trim() || triggerLoading === 'custom'}
                        className="px-4 py-2.5 rounded-lg bg-mc-diamond hover:bg-mc-diamond/80 text-black font-mono text-xs font-bold transition-colors disabled:opacity-50 flex items-center gap-1.5 shrink-0"
                    >
                        <Zap size={14} />
                        Run
                    </button>
                </form>
            </div>

            {/* HISTORICAL PROFILER REPORTS ARCHIVE */}
            <div className="p-5 rounded-xl bg-black/40 border border-white/10 backdrop-blur-md space-y-4">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                    <div>
                        <h3 className="text-base font-semibold text-white flex items-center gap-2">
                            Captured Reports Archive
                            <span className="text-xs px-2 py-0.5 rounded-full bg-white/10 text-white/70 font-mono">
                                {total}
                            </span>
                        </h3>
                        <p className="text-xs text-white/50 font-mono">
                            Historical Spark viewer URLs and Paper Aikar timings links.
                        </p>
                    </div>
                    {reports.length > 0 && (
                        <button
                            onClick={() => {
                                setReportToDelete('all');
                                setDeleteModalOpen(true);
                            }}
                            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-red-950/40 hover:bg-red-900/60 text-red-300 font-mono text-xs border border-red-500/30 transition-colors self-start sm:self-auto"
                        >
                            <Trash2 size={13} />
                            Clear Archive
                        </button>
                    )}
                </div>

                {/* SEARCH & FILTERS */}
                <div className="flex flex-col sm:flex-row items-center gap-3">
                    <div className="relative flex-1 w-full">
                        <label htmlFor={searchInputId} className="sr-only">Search reports by title, URL, or summary</label>
                        <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-white/40" size={15} />
                        <input
                            id={searchInputId}
                            type="text"
                            placeholder="Search reports by title, URL, summary..."
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            className="w-full pl-9 pr-4 py-2 rounded-lg bg-black/60 border border-white/10 text-white text-xs font-mono focus:outline-none focus:border-white/30"
                        />
                    </div>
                    <div className="w-full sm:w-auto">
                        <label htmlFor={typeFilterSelectId} className="sr-only">Filter reports by type</label>
                        <select
                            id={typeFilterSelectId}
                            value={typeFilter}
                            onChange={(e) => setTypeFilter(e.target.value)}
                            className="w-full sm:w-48 px-3 py-2 rounded-lg bg-black/60 border border-white/10 text-white text-xs font-mono focus:outline-none focus:border-white/30"
                        >
                            {REPORT_TYPE_FILTERS.map((f) => (
                                <option key={f.value} value={f.value} className="bg-neutral-900 text-white">
                                    {f.label}
                                </option>
                            ))}
                        </select>
                    </div>
                </div>

                {/* REPORTS TABLE */}
                {loading ? (
                    <div className="text-center py-12 text-white/40 font-mono text-xs">
                        Loading profiler reports archive...
                    </div>
                ) : filteredReports.length === 0 ? (
                    <div className="text-center py-12 border border-dashed border-white/10 rounded-xl space-y-2">
                        <Activity size={32} className="mx-auto text-white/20" />
                        <p className="text-sm font-semibold text-white/60">No profiler reports recorded</p>
                        <p className="text-xs text-white/40 font-mono">
                            Run <code className="text-amber-300">/spark sampler</code> or <code className="text-blue-300">/timings paste</code> to capture links.
                        </p>
                    </div>
                ) : (
                    <div className="overflow-x-auto rounded-lg border border-white/10">
                        <table className="w-full text-left border-collapse font-mono text-xs">
                            <thead>
                                <tr className="border-b border-white/10 bg-white/5 text-white/60">
                                    <th className="p-3 font-semibold">Type</th>
                                    <th className="p-3 font-semibold">Title</th>
                                    <th className="p-3 font-semibold">Target URL / Summary</th>
                                    <th className="p-3 font-semibold">Captured At</th>
                                    <th className="p-3 font-semibold text-right">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-white/5">
                                {filteredReports.map((report) => (
                                    <tr
                                        key={report.id}
                                        className="hover:bg-white/5 transition-colors group cursor-pointer"
                                        onClick={() => setSelectedReport(report)}
                                    >
                                        <td className="p-3 whitespace-nowrap">
                                            {getTypeBadge(report.report_type)}
                                        </td>
                                        <td className="p-3 font-semibold text-white">
                                            {report.title}
                                        </td>
                                        <td className="p-3 max-w-xs md:max-w-md truncate">
                                            {report.url ? (
                                                <a
                                                    href={report.url}
                                                    target="_blank"
                                                    rel="noreferrer"
                                                    onClick={(e) => e.stopPropagation()}
                                                    className="inline-flex items-center gap-1 text-mc-diamond hover:underline font-mono"
                                                >
                                                    {report.url}
                                                    <ExternalLink size={12} />
                                                </a>
                                            ) : (
                                                <span className="text-white/60 truncate">{report.summary}</span>
                                            )}
                                        </td>
                                        <td className="p-3 text-white/50 whitespace-nowrap">
                                            {new Date(report.created_at).toLocaleString()}
                                        </td>
                                        <td className="p-3 text-right whitespace-nowrap" onClick={(e) => e.stopPropagation()}>
                                            <div className="inline-flex items-center gap-1.5">
                                                {report.url && (
                                                    <button
                                                        onClick={() => copyToClipboard(report.url, false, report.id)}
                                                        className="p-1.5 rounded hover:bg-white/10 text-white/60 hover:text-white transition-colors"
                                                        title="Copy URL"
                                                    >
                                                        {copiedReportId === report.id ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
                                                    </button>
                                                )}
                                                <button
                                                    onClick={() => {
                                                        setReportToDelete(report.id);
                                                        setDeleteModalOpen(true);
                                                    }}
                                                    className="p-1.5 rounded hover:bg-red-500/20 text-white/40 hover:text-red-400 transition-colors"
                                                    title="Delete"
                                                >
                                                    <Trash2 size={14} />
                                                </button>
                                            </div>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}
            </div>

            {/* DETAILS MODAL */}
            {selectedReport && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-200">
                    <div className="w-full max-w-3xl rounded-2xl bg-neutral-950 border border-white/20 p-6 space-y-4 shadow-2xl overflow-hidden max-h-[90vh] flex flex-col">
                        <div className="flex items-center justify-between border-b border-white/10 pb-4">
                            <div className="flex items-center gap-3">
                                {getTypeBadge(selectedReport.report_type)}
                                <h3 className="text-lg font-bold text-white">{selectedReport.title}</h3>
                            </div>
                            <button
                                onClick={() => setSelectedReport(null)}
                                className="p-1 text-white/40 hover:text-white rounded-lg hover:bg-white/10"
                            >
                                <X size={20} />
                            </button>
                        </div>

                        <div className="flex-1 overflow-y-auto space-y-4 pr-1">
                            {selectedReport.url && (
                                <div className="p-3.5 rounded-xl bg-mc-diamond/10 border border-mc-diamond/30 flex items-center justify-between gap-4">
                                    <div className="truncate">
                                        <p className="text-xs font-mono uppercase text-mc-diamond font-semibold">Web Viewer Link</p>
                                        <p className="text-sm font-mono text-white truncate">{selectedReport.url}</p>
                                    </div>
                                    <a
                                        href={selectedReport.url}
                                        target="_blank"
                                        rel="noreferrer"
                                        className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-mc-diamond hover:bg-mc-diamond/80 text-black font-mono text-xs font-bold transition-colors shrink-0"
                                    >
                                        Open Report <ExternalLink size={14} />
                                    </a>
                                </div>
                            )}

                            {selectedReport.summary && (
                                <div className="p-4 rounded-xl bg-white/5 border border-white/10 space-y-1">
                                    <h4 className="text-xs font-mono uppercase tracking-wider text-white/60 font-semibold">Telemetry Summary</h4>
                                    <p className="text-sm text-white/90 font-mono">{selectedReport.summary}</p>
                                </div>
                            )}

                            <div className="p-4 rounded-xl bg-black/60 border border-white/10 space-y-2">
                                <div className="flex items-center justify-between">
                                    <h4 className="text-xs font-mono uppercase tracking-wider text-white/60 font-semibold">Raw Server Output</h4>
                                    <button
                                        onClick={() => copyToClipboard(selectedReport.raw_output, true)}
                                        className="flex items-center gap-1 text-xs font-mono text-mc-diamond hover:underline"
                                    >
                                        {copiedRaw ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
                                        {copiedRaw ? 'Copied' : 'Copy Output'}
                                    </button>
                                </div>
                                <pre className="text-xs font-mono p-3 rounded-lg bg-neutral-900 border border-white/5 text-white/80 overflow-x-auto whitespace-pre-wrap max-h-64">
                                    {selectedReport.raw_output || 'No raw output recorded.'}
                                </pre>
                            </div>
                        </div>

                        <div className="flex items-center justify-between pt-2 border-t border-white/10 text-xs text-white/40 font-mono">
                            <span>Captured on {new Date(selectedReport.created_at).toLocaleString()}</span>
                            <button
                                onClick={() => setSelectedReport(null)}
                                className="px-4 py-2 rounded-lg bg-white/10 hover:bg-white/20 text-white font-mono transition-colors"
                            >
                                Close
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* DELETE CONFIRMATION MODAL */}
            {deleteModalOpen && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-200">
                    <div className="w-full max-w-md rounded-2xl bg-neutral-950 border border-red-500/30 p-6 space-y-4 shadow-2xl">
                        <div className="flex items-center gap-3 text-red-400">
                            <AlertTriangle size={24} />
                            <h3 className="text-lg font-bold text-white">Confirm Deletion</h3>
                        </div>
                        <p className="text-sm text-white/70 font-mono">
                            {reportToDelete === 'all'
                                ? 'Are you sure you want to delete all profiler reports? This action cannot be undone.'
                                : 'Are you sure you want to delete this profiler report?'}
                        </p>
                        <div className="flex justify-end gap-3 pt-2">
                            <button
                                onClick={() => setDeleteModalOpen(false)}
                                className="px-4 py-2 rounded-lg bg-white/10 hover:bg-white/20 text-white text-xs font-mono transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleDelete}
                                className="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-500 text-white text-xs font-mono font-semibold transition-colors"
                            >
                                Delete
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
