import { useEffect, useState, useCallback } from 'react';
import {
    AlertTriangle, RefreshCw, Trash2, Search, Filter, Shield, Terminal,
    Sparkles, Bug, ChevronDown, ChevronRight, CheckCircle2,
    XCircle, Clock, Copy, Check, AlertCircle, Cpu, Zap, HardDrive, HelpCircle
} from 'lucide-react';
import type { CrashReport, CrashReportsResponse, AISettings, AIExplanationResponse } from '../types';

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

const CATEGORY_FILTERS = [
    { label: 'All Categories', value: '' },
    { label: 'Out of Memory', value: 'OutOfMemory' },
    { label: 'Port Conflict', value: 'PortConflict' },
    { label: 'Watchdog Timeout', value: 'WatchdogTimeout' },
    { label: 'Plugin Failure', value: 'PluginFailure' },
    { label: 'Corrupted Chunk', value: 'CorruptedChunk' },
    { label: 'Java Version', value: 'JavaVersionMismatch' },
    { label: 'EULA Unaccepted', value: 'EulaUnaccepted' },
    { label: 'Disk Full', value: 'DiskFull' },
];

function getCategoryBadge(category: string) {
    switch (category) {
        case 'OutOfMemory':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-red-500/20 text-red-300 border border-red-500/40">
                    <Cpu size={13} />
                    Out of Memory
                </span>
            );
        case 'PortConflict':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-amber-500/20 text-amber-300 border border-amber-500/40">
                    <Zap size={13} />
                    Port Conflict
                </span>
            );
        case 'WatchdogTimeout':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-yellow-500/20 text-yellow-300 border border-yellow-500/40">
                    <Clock size={13} />
                    Watchdog Timeout
                </span>
            );
        case 'PluginFailure':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-purple-500/20 text-purple-300 border border-purple-500/40">
                    <Bug size={13} />
                    Plugin Failure
                </span>
            );
        case 'CorruptedChunk':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-rose-500/20 text-rose-300 border border-rose-500/40">
                    <HardDrive size={13} />
                    Corrupted Chunk
                </span>
            );
        case 'JavaVersionMismatch':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-blue-500/20 text-blue-300 border border-blue-500/40">
                    <Terminal size={13} />
                    Java Mismatch
                </span>
            );
        case 'EulaUnaccepted':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-emerald-500/20 text-emerald-300 border border-emerald-500/40">
                    <Shield size={13} />
                    EULA Required
                </span>
            );
        case 'DiskFull':
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-orange-500/20 text-orange-300 border border-orange-500/40">
                    <HardDrive size={13} />
                    Disk Full
                </span>
            );
        default:
            return (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-semibold rounded-md bg-slate-500/20 text-slate-300 border border-slate-500/40">
                    <HelpCircle size={13} />
                    {category || 'Unknown'}
                </span>
            );
    }
}

export default function CrashAnalyst() {
    const [reports, setReports] = useState<CrashReport[]>([]);
    const [total, setTotal] = useState(0);
    const [loading, setLoading] = useState(true);
    const [refreshing, setRefreshing] = useState(false);
    const [categoryFilter, setCategoryFilter] = useState('');
    const [searchQuery, setSearchQuery] = useState('');

    // Expanded stack traces
    const [expandedIds, setExpandedIds] = useState<Record<number, boolean>>({});
    const [copiedId, setCopiedId] = useState<number | null>(null);

    // Modals
    const [isAnalyzeModalOpen, setIsAnalyzeModalOpen] = useState(false);
    const [manualLogInput, setManualLogInput] = useState('');
    const [analyzingManual, setAnalyzingManual] = useState(false);

    const [isAISettingsOpen, setIsAISettingsOpen] = useState(false);
    const [aiSettings, setAISettings] = useState<AISettings>({
        provider: 'openai',
        api_key: '',
        model: 'gpt-4o-mini',
        base_url: '',
        is_enabled: false,
    });
    const [savingAI, setSavingAI] = useState(false);

    // AI Explain Drawer / Modal
    const [activeAIReport, setActiveAIReport] = useState<CrashReport | null>(null);
    const [aiCustomPrompt, setAICustomPrompt] = useState('');
    const [aiExplanation, setAIExplanation] = useState<string | null>(null);
    const [explainingAI, setExplainingAI] = useState(false);
    const [aiMeta, setAIMeta] = useState<{ provider: string; model: string } | null>(null);

    // Purge Confirmation
    const [isPurgeModalOpen, setIsPurgeModalOpen] = useState(false);
    const [purging, setPurging] = useState(false);

    // Toasts
    const [toasts, setToasts] = useState<Toast[]>([]);

    const showToast = (message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts((prev) => [...prev, { id, message, type }]);
        setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
        }, 4000);
    };

    const fetchReports = useCallback(async (isRefresh = false) => {
        if (isRefresh) setRefreshing(true);
        else setLoading(true);

        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/crash?limit=100&offset=0', {
                headers: { Authorization: `Bearer ${token}` },
            });
            if (res.ok) {
                const data: CrashReportsResponse = await res.json();
                setReports(data.reports || []);
                setTotal(data.total || 0);
            } else {
                showToast('Failed to load crash reports', 'error');
            }
        } catch (err) {
            console.error('Error loading crash reports:', err);
            showToast('Network error loading crash reports', 'error');
        } finally {
            setLoading(false);
            setRefreshing(false);
        }
    }, []);

    const fetchAISettings = useCallback(async () => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/crash/ai-config', {
                headers: { Authorization: `Bearer ${token}` },
            });
            if (res.ok) {
                const data: AISettings = await res.json();
                setAISettings(data);
            }
        } catch (err) {
            console.error('Error fetching AI settings:', err);
        }
    }, []);

    useEffect(() => {
        fetchReports();
        fetchAISettings();
    }, [fetchReports, fetchAISettings]);

    const toggleExpand = (id: number) => {
        setExpandedIds((prev) => ({ ...prev, [id]: !prev[id] }));
    };

    const copyLog = (id: number, text: string) => {
        navigator.clipboard.writeText(text);
        setCopiedId(id);
        setTimeout(() => setCopiedId(null), 2000);
        showToast('Crash dump copied to clipboard', 'success');
    };

    const handleDelete = async (id: number) => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch(`/api/crash?id=${id}`, {
                method: 'DELETE',
                headers: { Authorization: `Bearer ${token}` },
            });
            if (res.ok) {
                showToast('Crash report deleted', 'success');
                fetchReports();
            } else {
                showToast('Failed to delete report', 'error');
            }
        } catch {
            showToast('Error deleting crash report', 'error');
        }
    };

    const handlePurgeAll = async () => {
        setPurging(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/crash?id=all', {
                method: 'DELETE',
                headers: { Authorization: `Bearer ${token}` },
            });
            if (res.ok) {
                showToast('All crash reports cleared', 'success');
                setIsPurgeModalOpen(false);
                fetchReports();
            } else {
                showToast('Failed to clear crash reports', 'error');
            }
        } catch {
            showToast('Network error while clearing reports', 'error');
        } finally {
            setPurging(false);
        }
    };

    const handleSaveAISettings = async (e: React.FormEvent) => {
        e.preventDefault();
        setSavingAI(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/crash/ai-config', {
                method: 'POST',
                headers: {
                    Authorization: `Bearer ${token}`,
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(aiSettings),
            });
            if (res.ok) {
                const updated: AISettings = await res.json();
                setAISettings(updated);
                showToast('AI diagnostic settings saved successfully', 'success');
                setIsAISettingsOpen(false);
            } else {
                showToast('Failed to save AI settings', 'error');
            }
        } catch {
            showToast('Network error saving AI settings', 'error');
        } finally {
            setSavingAI(false);
        }
    };

    const handleAnalyzeManual = async (e: React.FormEvent) => {
        e.preventDefault();
        setAnalyzingManual(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/crash/analyze', {
                method: 'POST',
                headers: {
                    Authorization: `Bearer ${token}`,
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    log: manualLogInput,
                    save: true,
                }),
            });
            if (res.ok) {
                showToast('Log analyzed and recorded successfully', 'success');
                setIsAnalyzeModalOpen(false);
                setManualLogInput('');
                fetchReports();
            } else {
                showToast('Failed to analyze log', 'error');
            }
        } catch {
            showToast('Network error analyzing log', 'error');
        } finally {
            setAnalyzingManual(false);
        }
    };

    const openAIExplain = (report: CrashReport) => {
        setActiveAIReport(report);
        setAIExplanation(null);
        setAICustomPrompt('');
        setAIMeta(null);
    };

    const requestAIExplanation = async () => {
        if (!activeAIReport) return;
        setExplainingAI(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/crash/ai-explain', {
                method: 'POST',
                headers: {
                    Authorization: `Bearer ${token}`,
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    report_id: activeAIReport.id,
                    custom_prompt: aiCustomPrompt,
                }),
            });
            if (res.ok) {
                const data: AIExplanationResponse = await res.json();
                setAIExplanation(data.explanation);
                setAIMeta({ provider: data.provider, model: data.model });
            } else {
                const errData = await res.json().catch(() => null);
                showToast(errData?.error || 'AI diagnostic consultation failed', 'error');
            }
        } catch {
            showToast('Network error during AI consultation', 'error');
        } finally {
            setExplainingAI(false);
        }
    };

    // Filtered reports
    const filteredReports = reports.filter((r) => {
        if (categoryFilter && r.category !== categoryFilter) return false;
        if (searchQuery) {
            const q = searchQuery.toLowerCase();
            const matchTitle = r.title.toLowerCase().includes(q);
            const matchCulprit = (r.culprit || '').toLowerCase().includes(q);
            const matchSummary = r.summary.toLowerCase().includes(q);
            if (!matchTitle && !matchCulprit && !matchSummary) return false;
        }
        return true;
    });

    return (
        <div className="flex flex-col h-full space-y-6 overflow-y-auto pb-10">
            {/* TOASTS */}
            <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 pointer-events-none">
                {toasts.map((toast) => (
                    <div
                        key={toast.id}
                        className={`flex items-center gap-2 px-4 py-3 rounded-lg shadow-lg border backdrop-blur-md pointer-events-auto transition-all duration-300 font-mono text-sm ${
                            toast.type === 'success'
                                ? 'bg-emerald-950/90 border-emerald-500/50 text-emerald-200'
                                : 'bg-rose-950/90 border-rose-500/50 text-rose-200'
                        }`}
                    >
                        {toast.type === 'success' ? (
                            <CheckCircle2 size={16} className="text-emerald-400 shrink-0" />
                        ) : (
                            <XCircle size={16} className="text-rose-400 shrink-0" />
                        )}
                        <span>{toast.message}</span>
                    </div>
                ))}
            </div>

            {/* HEADER */}
            <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4 bg-black/40 border border-white/10 rounded-xl p-6 backdrop-blur-sm">
                <div className="flex items-center gap-4">
                    <div className="p-3 bg-red-500/15 border border-red-500/30 rounded-xl text-red-400">
                        <AlertTriangle size={28} />
                    </div>
                    <div>
                        <h1 className="text-2xl font-bold font-pixel tracking-wide text-white">
                            Crash Analyst & Diagnostics
                        </h1>
                        <p className="text-sm text-white/60 font-mono mt-0.5">
                            Automated heuristic classification, culprit pinpointing, and optional AI troubleshooting ({total} {total === 1 ? 'incident' : 'incidents'})
                        </p>
                    </div>
                </div>

                <div className="flex flex-wrap items-center gap-2.5">
                    <button
                        onClick={() => setIsAnalyzeModalOpen(true)}
                        className="flex items-center gap-2 px-3.5 py-2 rounded-lg bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 border border-emerald-500/40 font-mono text-xs font-semibold transition-colors"
                    >
                        <Terminal size={15} />
                        Analyze Log Dump
                    </button>

                    <button
                        onClick={() => setIsAISettingsOpen(true)}
                        className="flex items-center gap-2 px-3.5 py-2 rounded-lg bg-purple-600/20 hover:bg-purple-600/30 text-purple-300 border border-purple-500/40 font-mono text-xs font-semibold transition-colors"
                    >
                        <Sparkles size={15} />
                        AI Settings {aiSettings.is_enabled && <span className="w-2 h-2 rounded-full bg-purple-400 animate-pulse" />}
                    </button>

                    <button
                        onClick={() => setIsPurgeModalOpen(true)}
                        disabled={reports.length === 0}
                        className="flex items-center gap-2 px-3 py-2 rounded-lg bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 border border-rose-500/40 font-mono text-xs transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                        <Trash2 size={15} />
                        Clear All
                    </button>

                    <button
                        onClick={() => fetchReports(true)}
                        disabled={refreshing}
                        className="flex items-center gap-2 px-3 py-2 rounded-lg bg-white/5 hover:bg-white/10 text-white/80 border border-white/10 font-mono text-xs transition-colors disabled:opacity-50"
                    >
                        <RefreshCw size={15} className={refreshing ? 'animate-spin' : ''} />
                        Refresh
                    </button>
                </div>
            </div>

            {/* FILTERS & SEARCH */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3 bg-black/30 border border-white/10 rounded-xl p-4">
                <div className="relative md:col-span-2">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-white/40" size={16} />
                    <input
                        type="text"
                        placeholder="Search crash title, culprit, or root-cause summary..."
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="w-full pl-9 pr-4 py-2 bg-black/40 border border-white/10 rounded-lg text-sm text-white placeholder-white/40 focus:outline-none focus:border-red-500/50 font-mono"
                    />
                </div>

                <div className="relative">
                    <Filter className="absolute left-3 top-1/2 -translate-y-1/2 text-white/40" size={16} />
                    <select
                        value={categoryFilter}
                        onChange={(e) => setCategoryFilter(e.target.value)}
                        className="w-full pl-9 pr-4 py-2 bg-black/40 border border-white/10 rounded-lg text-sm text-white/90 focus:outline-none focus:border-red-500/50 font-mono appearance-none"
                    >
                        {CATEGORY_FILTERS.map((cat) => (
                            <option key={cat.value} value={cat.value} className="bg-neutral-900 text-white">
                                {cat.label}
                            </option>
                        ))}
                    </select>
                </div>
            </div>

            {/* REPORTS LIST */}
            {loading ? (
                <div className="flex flex-col items-center justify-center p-16 bg-black/20 border border-white/10 rounded-xl text-white/60 space-y-3 font-mono">
                    <RefreshCw className="animate-spin text-red-400" size={32} />
                    <p>Analyzing crash telemetry...</p>
                </div>
            ) : filteredReports.length === 0 ? (
                <div className="flex flex-col items-center justify-center p-16 bg-black/20 border border-white/10 rounded-xl text-white/50 space-y-3 text-center">
                    <CheckCircle2 size={40} className="text-emerald-400/80" />
                    <div>
                        <p className="text-lg font-semibold text-white/80">No Crash Incidents Recorded</p>
                        <p className="text-sm font-mono text-white/40 mt-1">
                            {categoryFilter || searchQuery
                                ? 'No crash reports match your current filters.'
                                : 'Your PaperMC server is stable with zero detected fatal crashes.'}
                        </p>
                    </div>
                </div>
            ) : (
                <div className="space-y-4">
                    {filteredReports.map((report) => (
                        <div
                            key={report.id}
                            className="bg-black/50 border border-white/10 hover:border-white/20 rounded-xl p-5 backdrop-blur-md transition-all space-y-4"
                        >
                            {/* Top Card Bar */}
                            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-white/5 pb-3">
                                <div className="flex flex-wrap items-center gap-2.5">
                                    {getCategoryBadge(report.category)}
                                    <span className="text-xs font-mono text-white/40 bg-white/5 px-2 py-0.5 rounded">
                                        Source: {report.source}
                                    </span>
                                    <span className="text-xs font-mono text-white/40 flex items-center gap-1">
                                        <Clock size={12} />
                                        {new Date(report.created_at).toLocaleString()}
                                    </span>
                                </div>

                                <div className="flex items-center gap-2">
                                    {aiSettings.is_enabled && (
                                        <button
                                            onClick={() => openAIExplain(report)}
                                            className="flex items-center gap-1.5 px-3 py-1 bg-purple-600/20 hover:bg-purple-600/30 text-purple-300 border border-purple-500/40 rounded-lg text-xs font-mono font-medium transition-colors"
                                        >
                                            <Sparkles size={13} />
                                            Ask AI
                                        </button>
                                    )}
                                    <button
                                        onClick={() => copyLog(report.id, report.raw_log)}
                                        className="p-1.5 hover:bg-white/10 text-white/60 hover:text-white rounded-lg transition-colors"
                                        title="Copy Full Log"
                                    >
                                        {copiedId === report.id ? <Check size={15} className="text-emerald-400" /> : <Copy size={15} />}
                                    </button>
                                    <button
                                        onClick={() => handleDelete(report.id)}
                                        className="p-1.5 hover:bg-rose-900/30 text-rose-400 hover:text-rose-300 rounded-lg transition-colors"
                                        title="Delete Report"
                                    >
                                        <Trash2 size={15} />
                                    </button>
                                </div>
                            </div>

                            {/* Title & Culprit */}
                            <div>
                                <h3 className="text-lg font-bold text-white flex items-center gap-2">
                                    {report.title}
                                </h3>
                                {report.culprit && (
                                    <p className="text-xs font-mono text-amber-400 mt-1">
                                        Pinpointed Culprit: <span className="underline decoration-amber-400/50">{report.culprit}</span>
                                    </p>
                                )}
                            </div>

                            {/* Summary & Resolution Box */}
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div className="bg-neutral-900/70 border border-white/5 rounded-lg p-3.5 space-y-1">
                                    <h4 className="text-xs font-mono font-semibold text-white/60 uppercase tracking-wider">
                                        Root Cause Analysis
                                    </h4>
                                    <p className="text-xs text-white/90 leading-relaxed">{report.summary}</p>
                                </div>

                                <div className="bg-emerald-950/20 border border-emerald-500/30 rounded-lg p-3.5 space-y-1.5">
                                    <h4 className="text-xs font-mono font-semibold text-emerald-400 uppercase tracking-wider flex items-center gap-1.5">
                                        <CheckCircle2 size={13} /> Recommended Resolution
                                    </h4>
                                    <p className="text-xs text-emerald-200/90 whitespace-pre-line leading-relaxed font-mono">
                                        {report.recommendation}
                                    </p>
                                </div>
                            </div>

                            {/* Collapsible Stack Trace */}
                            <div className="pt-1">
                                <button
                                    onClick={() => toggleExpand(report.id)}
                                    className="flex items-center gap-1.5 text-xs font-mono text-white/50 hover:text-white/80 transition-colors"
                                >
                                    {expandedIds[report.id] ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                                    {expandedIds[report.id] ? 'Hide Raw Stack Trace' : 'View Raw Stack Trace'}
                                </button>

                                {expandedIds[report.id] && (
                                    <div className="mt-2.5 bg-black/80 border border-white/10 rounded-lg p-3 max-h-72 overflow-y-auto font-mono text-xs text-white/70 whitespace-pre-wrap leading-relaxed select-all">
                                        {report.raw_log}
                                    </div>
                                )}
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* MODAL: ANALYZE MANUAL LOG */}
            {isAnalyzeModalOpen && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
                    <div className="bg-neutral-900 border border-white/15 rounded-xl w-full max-w-2xl p-6 shadow-2xl space-y-4">
                        <div className="flex items-center justify-between border-b border-white/10 pb-3">
                            <h3 className="text-lg font-bold text-white flex items-center gap-2">
                                <Terminal size={20} className="text-emerald-400" />
                                Analyze Raw Crash / Error Log
                            </h3>
                            <button
                                onClick={() => setIsAnalyzeModalOpen(false)}
                                className="text-white/50 hover:text-white"
                            >
                                <XCircle size={20} />
                            </button>
                        </div>

                        <form onSubmit={handleAnalyzeManual} className="space-y-4">
                            <div>
                                <label className="block text-xs font-mono text-white/70 mb-1">
                                    Paste Crash Dump, Error Trace, or Leave Blank to Use Recent Server History
                                </label>
                                <textarea
                                    rows={8}
                                    value={manualLogInput}
                                    onChange={(e) => setManualLogInput(e.target.value)}
                                    placeholder="Paste Java stack trace or crash log here (e.g. java.lang.OutOfMemoryError, BindException, etc.)..."
                                    className="w-full bg-black/60 border border-white/10 rounded-lg p-3 text-xs font-mono text-white placeholder-white/30 focus:outline-none focus:border-emerald-500/50"
                                />
                            </div>

                            <div className="flex justify-end gap-2.5 pt-2 border-t border-white/10">
                                <button
                                    type="button"
                                    onClick={() => setIsAnalyzeModalOpen(false)}
                                    className="px-4 py-2 rounded-lg bg-white/5 hover:bg-white/10 text-white/80 text-xs font-mono"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={analyzingManual}
                                    className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-mono font-semibold flex items-center gap-2 disabled:opacity-50"
                                >
                                    {analyzingManual ? <RefreshCw className="animate-spin" size={14} /> : <Zap size={14} />}
                                    Run Diagnosis
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* MODAL: AI SETTINGS */}
            {isAISettingsOpen && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
                    <div className="bg-neutral-900 border border-white/15 rounded-xl w-full max-w-lg p-6 shadow-2xl space-y-4">
                        <div className="flex items-center justify-between border-b border-white/10 pb-3">
                            <h3 className="text-lg font-bold text-white flex items-center gap-2">
                                <Sparkles size={20} className="text-purple-400" />
                                AI Diagnostics Configuration
                            </h3>
                            <button
                                onClick={() => setIsAISettingsOpen(false)}
                                className="text-white/50 hover:text-white"
                            >
                                <XCircle size={20} />
                            </button>
                        </div>

                        <form onSubmit={handleSaveAISettings} className="space-y-4">
                            <div className="flex items-center justify-between bg-black/40 p-3 rounded-lg border border-white/10">
                                <div>
                                    <p className="text-sm font-semibold text-white">Enable AI Troubleshooting</p>
                                    <p className="text-xs text-white/50">Allow sending sanitized crash logs to external LLMs</p>
                                </div>
                                <input
                                    type="checkbox"
                                    checked={aiSettings.is_enabled}
                                    onChange={(e) => setAISettings({ ...aiSettings, is_enabled: e.target.checked })}
                                    className="w-5 h-5 accent-purple-600 rounded cursor-pointer"
                                />
                            </div>

                            <div>
                                <label className="block text-xs font-mono text-white/70 mb-1">Provider</label>
                                <select
                                    value={aiSettings.provider}
                                    onChange={(e) => {
                                        const p = e.target.value;
                                        const defaultModel = p === 'gemini' ? 'gemini-1.5-flash' : 'gpt-4o-mini';
                                        setAISettings({ ...aiSettings, provider: p, model: defaultModel });
                                    }}
                                    className="w-full bg-black/40 border border-white/10 rounded-lg p-2.5 text-sm text-white font-mono focus:outline-none focus:border-purple-500/50"
                                >
                                    <option value="openai">OpenAI (or Compatible: Groq, DeepSeek)</option>
                                    <option value="gemini">Google Gemini</option>
                                    <option value="ollama">Ollama (Local)</option>
                                </select>
                            </div>

                            <div>
                                <label className="block text-xs font-mono text-white/70 mb-1">
                                    API Key {aiSettings.has_api_key && <span className="text-emerald-400">(Configured)</span>}
                                </label>
                                <input
                                    type="password"
                                    placeholder={aiSettings.has_api_key ? '••••••••••••••••' : 'sk-... or AIzaSy...'}
                                    value={aiSettings.api_key}
                                    onChange={(e) => setAISettings({ ...aiSettings, api_key: e.target.value })}
                                    className="w-full bg-black/40 border border-white/10 rounded-lg p-2.5 text-sm text-white font-mono focus:outline-none focus:border-purple-500/50"
                                />
                                <p className="text-[11px] text-white/40 mt-1">
                                    Sensitive server paths, passwords, and IPs are automatically redacted before transmission.
                                </p>
                            </div>

                            <div>
                                <label className="block text-xs font-mono text-white/70 mb-1">Model Name</label>
                                <input
                                    type="text"
                                    value={aiSettings.model}
                                    onChange={(e) => setAISettings({ ...aiSettings, model: e.target.value })}
                                    placeholder="gpt-4o-mini, gemini-1.5-flash, llama3.1, etc."
                                    className="w-full bg-black/40 border border-white/10 rounded-lg p-2.5 text-sm text-white font-mono focus:outline-none focus:border-purple-500/50"
                                />
                            </div>

                            <div>
                                <label className="block text-xs font-mono text-white/70 mb-1">
                                    Base URL <span className="text-white/40">(Optional - for Ollama or proxies)</span>
                                </label>
                                <input
                                    type="text"
                                    value={aiSettings.base_url}
                                    onChange={(e) => setAISettings({ ...aiSettings, base_url: e.target.value })}
                                    placeholder="http://localhost:11434/v1 or https://api.groq.com/openai/v1"
                                    className="w-full bg-black/40 border border-white/10 rounded-lg p-2.5 text-sm text-white font-mono focus:outline-none focus:border-purple-500/50"
                                />
                            </div>

                            <div className="flex justify-end gap-2.5 pt-2 border-t border-white/10">
                                <button
                                    type="button"
                                    onClick={() => setIsAISettingsOpen(false)}
                                    className="px-4 py-2 rounded-lg bg-white/5 hover:bg-white/10 text-white/80 text-xs font-mono"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={savingAI}
                                    className="px-4 py-2 rounded-lg bg-purple-600 hover:bg-purple-500 text-white text-xs font-mono font-semibold flex items-center gap-2 disabled:opacity-50"
                                >
                                    {savingAI ? <RefreshCw className="animate-spin" size={14} /> : <Check size={14} />}
                                    Save Settings
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* DRAWER / MODAL: AI CONSULTATION */}
            {activeAIReport && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
                    <div className="bg-neutral-900 border border-purple-500/30 rounded-xl w-full max-w-3xl p-6 shadow-2xl space-y-4 max-h-[90vh] flex flex-col">
                        <div className="flex items-center justify-between border-b border-white/10 pb-3">
                            <div className="flex items-center gap-2.5">
                                <div className="p-2 bg-purple-500/20 border border-purple-500/40 rounded-lg text-purple-300">
                                    <Sparkles size={18} />
                                </div>
                                <div>
                                    <h3 className="text-base font-bold text-white">AI Diagnostic Assistant</h3>
                                    <p className="text-xs text-white/50 font-mono">
                                        Analyzing: {activeAIReport.title}
                                    </p>
                                </div>
                            </div>
                            <button
                                onClick={() => setActiveAIReport(null)}
                                className="text-white/50 hover:text-white"
                            >
                                <XCircle size={20} />
                            </button>
                        </div>

                        <div className="flex-1 overflow-y-auto space-y-4 pr-1">
                            {/* Prompt customization */}
                            <div className="space-y-1.5">
                                <label className="block text-xs font-mono text-white/70">
                                    Additional Question / Context (Optional)
                                </label>
                                <div className="flex gap-2">
                                    <input
                                        type="text"
                                        value={aiCustomPrompt}
                                        onChange={(e) => setAICustomPrompt(e.target.value)}
                                        placeholder="e.g., Can I fix this without wiping the Nether world?"
                                        className="flex-1 bg-black/50 border border-white/10 rounded-lg px-3 py-2 text-xs font-mono text-white placeholder-white/30 focus:outline-none focus:border-purple-500/50"
                                    />
                                    <button
                                        onClick={requestAIExplanation}
                                        disabled={explainingAI}
                                        className="px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-xs font-mono font-semibold flex items-center gap-2 disabled:opacity-50 transition-colors"
                                    >
                                        {explainingAI ? <RefreshCw className="animate-spin" size={14} /> : <Sparkles size={14} />}
                                        Consult AI
                                    </button>
                                </div>
                            </div>

                            {/* Response Box */}
                            {explainingAI ? (
                                <div className="p-12 text-center space-y-3 bg-black/30 rounded-lg border border-purple-500/20">
                                    <RefreshCw className="animate-spin text-purple-400 mx-auto" size={28} />
                                    <p className="text-sm font-mono text-purple-300">
                                        Sanitizing stack trace and consulting {aiSettings.provider} ({aiSettings.model})...
                                    </p>
                                </div>
                            ) : aiExplanation ? (
                                <div className="bg-black/60 border border-purple-500/30 rounded-lg p-5 space-y-3">
                                    <div className="flex items-center justify-between border-b border-white/5 pb-2 text-xs font-mono text-purple-300/80">
                                        <span>AI Diagnostic Analysis</span>
                                        {aiMeta && (
                                            <span className="bg-purple-950/60 px-2 py-0.5 rounded border border-purple-500/30">
                                                {aiMeta.provider} • {aiMeta.model}
                                            </span>
                                        )}
                                    </div>
                                    <div className="text-xs font-sans text-white/90 whitespace-pre-wrap leading-relaxed">
                                        {aiExplanation}
                                    </div>
                                </div>
                            ) : (
                                <div className="p-8 text-center text-xs font-mono text-white/40 border border-dashed border-white/10 rounded-lg">
                                    Click "Consult AI" above to send the sanitized crash signature to your configured LLM for deep analysis.
                                </div>
                            )}
                        </div>

                        <div className="flex justify-end pt-3 border-t border-white/10">
                            <button
                                onClick={() => setActiveAIReport(null)}
                                className="px-4 py-2 rounded-lg bg-white/10 hover:bg-white/15 text-white text-xs font-mono"
                            >
                                Close
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* MODAL: PURGE CONFIRMATION */}
            {isPurgeModalOpen && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
                    <div className="bg-neutral-900 border border-rose-500/30 rounded-xl w-full max-w-md p-6 shadow-2xl space-y-4">
                        <div className="flex items-center gap-3 text-rose-400">
                            <div className="p-2.5 bg-rose-500/10 border border-rose-500/30 rounded-xl">
                                <AlertCircle size={24} />
                            </div>
                            <div>
                                <h3 className="text-lg font-bold text-white">Clear All Crash Reports</h3>
                                <p className="text-xs text-white/60">This action will delete all recorded crash history.</p>
                            </div>
                        </div>

                        <p className="text-xs font-mono text-white/70 bg-black/40 p-3 rounded-lg border border-white/5">
                            Are you sure you want to purge all recorded crash diagnostics? This action cannot be undone.
                        </p>

                        <div className="flex justify-end gap-2.5 pt-2">
                            <button
                                onClick={() => setIsPurgeModalOpen(false)}
                                disabled={purging}
                                className="px-4 py-2 rounded-lg bg-white/5 hover:bg-white/10 text-white/80 text-xs font-mono"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handlePurgeAll}
                                disabled={purging}
                                className="px-4 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-mono font-semibold flex items-center gap-2 disabled:opacity-50"
                            >
                                {purging ? <RefreshCw className="animate-spin" size={14} /> : <Trash2 size={14} />}
                                Confirm Clear All
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
