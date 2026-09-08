import { useEffect, useState, useCallback } from 'react';
import {
    ClipboardList, RefreshCw, Trash2, Search, Filter, Shield, User,
    Terminal, Globe, HardDrive, Clock, Key, AlertCircle, CheckCircle2,
    XCircle, ArrowLeft, ArrowRight, Laptop
} from 'lucide-react';
import type { AuditLog, AuditLogsResponse } from '../types';

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

const ACTION_CATEGORIES = [
    { label: 'All Actions', value: '' },
    { label: 'Server Control', value: 'server' },
    { label: 'Authentication', value: 'auth' },
    { label: 'Players', value: 'player' },
    { label: 'Worlds', value: 'world' },
    { label: 'Config & Flags', value: 'config' },
    { label: 'Backups', value: 'backup' },
    { label: 'Plugins', value: 'plugin' },
    { label: 'Schedules', value: 'schedule' },
    { label: 'Updates', value: 'updater' },
];

function formatTimestamp(isoStr: string): { formatted: string; relative: string } {
    try {
        const date = new Date(isoStr);
        if (isNaN(date.getTime())) {
            return { formatted: isoStr, relative: '' };
        }
        const now = new Date();
        const diffMs = now.getTime() - date.getTime();
        const diffSec = Math.floor(diffMs / 1000);
        const diffMin = Math.floor(diffSec / 60);
        const diffHr = Math.floor(diffMin / 60);
        const diffDays = Math.floor(diffHr / 24);

        let relative = '';
        if (diffSec < 10) relative = 'just now';
        else if (diffSec < 60) relative = `${diffSec}s ago`;
        else if (diffMin < 60) relative = `${diffMin}m ago`;
        else if (diffHr < 24) relative = `${diffHr}h ago`;
        else if (diffDays === 1) relative = 'yesterday';
        else relative = `${diffDays}d ago`;

        const formatted = date.toLocaleString('en-US', {
            month: 'short',
            day: 'numeric',
            year: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: false
        });

        return { formatted, relative };
    } catch {
        return { formatted: isoStr, relative: '' };
    }
}

function getActionBadge(action: string) {
    const act = action.toLowerCase();
    if (act.startsWith('server.')) {
        return (
            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-medium rounded-md bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                <Terminal size={13} />
                {action}
            </span>
        );
    }
    if (act.startsWith('auth.')) {
        return (
            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-medium rounded-md bg-blue-500/15 text-blue-300 border border-blue-500/30">
                <Key size={13} />
                {action}
            </span>
        );
    }
    if (act.startsWith('player.')) {
        return (
            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-medium rounded-md bg-amber-500/15 text-amber-300 border border-amber-500/30">
                <User size={13} />
                {action}
            </span>
        );
    }
    if (act.startsWith('world.')) {
        return (
            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-medium rounded-md bg-purple-500/15 text-purple-300 border border-purple-500/30">
                <Globe size={13} />
                {action}
            </span>
        );
    }
    if (act.startsWith('backup.')) {
        return (
            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-medium rounded-md bg-cyan-500/15 text-cyan-300 border border-cyan-500/30">
                <HardDrive size={13} />
                {action}
            </span>
        );
    }
    if (act.startsWith('schedule.')) {
        return (
            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-medium rounded-md bg-yellow-500/15 text-yellow-300 border border-yellow-500/30">
                <Clock size={13} />
                {action}
            </span>
        );
    }
    if (act.startsWith('plugin.')) {
        return (
            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-medium rounded-md bg-teal-500/15 text-teal-300 border border-teal-500/30">
                <Laptop size={13} />
                {action}
            </span>
        );
    }
    if (act.startsWith('updater.')) {
        return (
            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-medium rounded-md bg-pink-500/15 text-pink-300 border border-pink-500/30">
                <Shield size={13} />
                {action}
            </span>
        );
    }
    return (
        <span className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-medium rounded-md bg-white/10 text-white/80 border border-white/20">
            <ClipboardList size={13} />
            {action}
        </span>
    );
}

function getStatusBadge(code: number) {
    if (code >= 200 && code < 300) {
        return (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-mono rounded bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
                <CheckCircle2 size={12} />
                {code}
            </span>
        );
    }
    if (code >= 400 && code < 500) {
        return (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-mono rounded bg-amber-500/20 text-amber-400 border border-amber-500/30">
                <AlertCircle size={12} />
                {code}
            </span>
        );
    }
    return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-mono rounded bg-rose-500/20 text-rose-400 border border-rose-500/30">
            <XCircle size={12} />
            {code}
        </span>
    );
}

export default function AuditLogs() {
    const [logs, setLogs] = useState<AuditLog[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [limit, setLimit] = useState(25);
    const [totalPages, setTotalPages] = useState(1);
    const [loading, setLoading] = useState(true);
    const [refreshing, setRefreshing] = useState(false);

    // Filters
    const [actionFilter, setActionFilter] = useState('');
    const [userFilter, setUserFilter] = useState('');
    const [searchQuery, setSearchQuery] = useState('');

    // Purge confirmation modal
    const [isPurgeModalOpen, setIsPurgeModalOpen] = useState(false);
    const [purging, setPurging] = useState(false);

    // Toasts
    const [toasts, setToasts] = useState<Toast[]>([]);

    const showToast = (message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, message, type }]);
        setTimeout(() => {
            setToasts(prev => prev.filter(t => t.id !== id));
        }, 3500);
    };

    const fetchLogs = useCallback(async (isRefresh = false) => {
        if (isRefresh) setRefreshing(true);
        else setLoading(true);

        const token = localStorage.getItem('token');
        try {
            const params = new URLSearchParams();
            params.set('page', page.toString());
            params.set('limit', limit.toString());
            if (actionFilter) params.set('action', actionFilter);
            if (userFilter) params.set('username', userFilter);

            const res = await fetch(`/api/audit?${params.toString()}`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });

            if (res.ok) {
                const data: AuditLogsResponse = await res.json();
                setLogs(data.logs || []);
                setTotal(data.total || 0);
                setTotalPages(data.total_pages || 1);
            } else {
                showToast('Failed to load audit logs', 'error');
            }
        } catch (err) {
            console.error('Error fetching audit logs:', err);
            showToast('Network error while fetching logs', 'error');
        } finally {
            setLoading(false);
            setRefreshing(false);
        }
    }, [page, limit, actionFilter, userFilter]);

    useEffect(() => {
        fetchLogs();
    }, [fetchLogs]);

    const handlePurge = async () => {
        setPurging(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/audit', {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${token}` }
            });

            if (res.ok) {
                showToast('Audit log history purged successfully', 'success');
                setIsPurgeModalOpen(false);
                setPage(1);
                fetchLogs(true);
            } else {
                showToast('Failed to purge audit logs', 'error');
            }
        } catch (err) {
            console.error('Error purging audit logs:', err);
            showToast('Network error while purging logs', 'error');
        } finally {
            setPurging(false);
        }
    };

    // Client-side quick search filter on current page
    const filteredLogs = logs.filter(log => {
        if (!searchQuery) return true;
        const q = searchQuery.toLowerCase();
        return (
            log.username.toLowerCase().includes(q) ||
            log.action.toLowerCase().includes(q) ||
            log.details.toLowerCase().includes(q) ||
            log.endpoint.toLowerCase().includes(q) ||
            log.ip_address.toLowerCase().includes(q)
        );
    });

    return (
        <div className="flex-1 flex flex-col h-full overflow-hidden bg-black/40 p-4 md:p-6 text-white">
            {/* TOASTS */}
            <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
                {toasts.map(t => (
                    <div
                        key={t.id}
                        className={`px-4 py-3 rounded-lg border flex items-center gap-2 shadow-2xl backdrop-blur-md font-mono text-sm ${
                            t.type === 'success'
                                ? 'bg-emerald-950/90 border-emerald-500/50 text-emerald-200'
                                : 'bg-rose-950/90 border-rose-500/50 text-rose-200'
                        }`}
                    >
                        {t.type === 'success' ? <CheckCircle2 size={16} /> : <XCircle size={16} />}
                        {t.message}
                    </div>
                ))}
            </div>

            {/* HEADER */}
            <div className="flex flex-col md:flex-row md:items-center justify-between pb-6 border-b border-white/10 gap-4">
                <div>
                    <h1 className="font-pixel text-3xl text-mc-diamond tracking-wider flex items-center gap-3">
                        <ClipboardList className="text-mc-diamond" size={28} />
                        Audit Logs
                    </h1>
                    <p className="text-xs md:text-sm text-white/50 font-mono mt-1">
                        Cryptographic, immutable history of administrative actions, config mutations, and security events.
                    </p>
                </div>

                <div className="flex items-center gap-3">
                    <button
                        onClick={() => fetchLogs(true)}
                        disabled={loading || refreshing}
                        className="flex items-center gap-2 bg-white/10 hover:bg-white/15 active:scale-95 text-white/80 hover:text-white px-3.5 py-2 rounded-lg border border-white/10 text-xs font-mono uppercase tracking-wider transition-all disabled:opacity-50"
                        title="Refresh Logs"
                    >
                        <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />
                        Refresh
                    </button>

                    <button
                        onClick={() => setIsPurgeModalOpen(true)}
                        disabled={loading || total === 0}
                        className="flex items-center gap-2 bg-rose-500/20 hover:bg-rose-500/30 active:scale-95 text-rose-300 border border-rose-500/40 px-3.5 py-2 rounded-lg text-xs font-mono uppercase tracking-wider transition-all disabled:opacity-40"
                        title="Purge Logs History"
                    >
                        <Trash2 size={14} />
                        Purge History
                    </button>
                </div>
            </div>

            {/* CONTROLS & FILTER BAR */}
            <div className="mt-4 p-4 bg-black/50 backdrop-blur-md border border-white/10 rounded-xl flex flex-wrap items-center justify-between gap-4">
                <div className="flex flex-wrap items-center gap-3 flex-1 min-w-[280px]">
                    {/* Search Input */}
                    <div className="relative flex-1 min-w-[200px]">
                        <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-white/40" />
                        <input
                            type="text"
                            placeholder="Filter by keyword, IP, user, details..."
                            value={searchQuery}
                            onChange={e => setSearchQuery(e.target.value)}
                            className="w-full bg-white/5 border border-white/15 rounded-lg pl-9 pr-3 py-2 text-xs font-mono text-white placeholder-white/40 focus:outline-none focus:border-mc-diamond/60 transition-colors"
                        />
                    </div>

                    {/* Action Category Filter */}
                    <div className="flex items-center gap-2">
                        <Filter size={15} className="text-white/40" />
                        <select
                            value={actionFilter}
                            onChange={e => {
                                setActionFilter(e.target.value);
                                setPage(1);
                            }}
                            aria-label="Filter by action category"
                            className="bg-stone-900 border border-white/15 rounded-lg px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-mc-diamond/60 transition-colors"
                        >
                            {ACTION_CATEGORIES.map(c => (
                                <option key={c.value} value={c.value}>
                                    {c.label}
                                </option>
                            ))}
                        </select>
                    </div>

                    {/* Username Filter */}
                    <div className="flex items-center gap-2">
                        <User size={15} className="text-white/40" />
                        <input
                            type="text"
                            placeholder="Filter user..."
                            value={userFilter}
                            onChange={e => {
                                setUserFilter(e.target.value);
                                setPage(1);
                            }}
                            className="w-28 md:w-36 bg-white/5 border border-white/15 rounded-lg px-3 py-2 text-xs font-mono text-white placeholder-white/40 focus:outline-none focus:border-mc-diamond/60 transition-colors"
                        />
                    </div>
                </div>

                {/* Limit selector */}
                <div className="flex items-center gap-2 text-xs font-mono text-white/60">
                    <span>Rows:</span>
                    <select
                        value={limit}
                        onChange={e => {
                            setLimit(Number(e.target.value));
                            setPage(1);
                        }}
                        aria-label="Rows per page"
                        className="bg-stone-900 border border-white/15 rounded-lg px-2.5 py-1.5 text-xs font-mono text-white focus:outline-none focus:border-mc-diamond/60"
                    >
                        <option value={15}>15</option>
                        <option value={25}>25</option>
                        <option value={50}>50</option>
                        <option value={100}>100</option>
                    </select>
                </div>
            </div>

            {/* LOGS TABLE CONTAINER */}
            <div className="flex-1 mt-4 overflow-hidden flex flex-col bg-black/50 backdrop-blur-md border border-white/10 rounded-xl">
                <div className="flex-1 overflow-auto">
                    <table className="w-full text-left border-collapse">
                        <thead className="sticky top-0 bg-stone-950/90 backdrop-blur-md border-b border-white/10 text-xs font-mono uppercase tracking-wider text-white/60 z-10">
                            <tr>
                                <th className="p-3.5">Time</th>
                                <th className="p-3.5">Operator</th>
                                <th className="p-3.5">Action</th>
                                <th className="p-3.5">Method & Route</th>
                                <th className="p-3.5">Status</th>
                                <th className="p-3.5">Client IP</th>
                                <th className="p-3.5">Context & Details</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/5 font-mono text-xs">
                            {loading ? (
                                <tr>
                                    <td colSpan={7} className="p-12 text-center text-white/40 font-mono">
                                        <RefreshCw size={24} className="animate-spin mx-auto mb-2 text-mc-diamond/60" />
                                        Loading audit log trail...
                                    </td>
                                </tr>
                            ) : filteredLogs.length === 0 ? (
                                <tr>
                                    <td colSpan={7} className="p-12 text-center text-white/40 font-mono">
                                        <ClipboardList size={28} className="mx-auto mb-2 text-white/20" />
                                        No audit log records match your current filters.
                                    </td>
                                </tr>
                            ) : (
                                filteredLogs.map(log => {
                                    const { formatted, relative } = formatTimestamp(log.created_at);
                                    return (
                                        <tr key={log.id} className="hover:bg-white/[0.03] transition-colors">
                                            {/* Time */}
                                            <td className="p-3.5 whitespace-nowrap text-white/80">
                                                <div>{formatted}</div>
                                                {relative && (
                                                    <div className="text-[10px] text-white/40">{relative}</div>
                                                )}
                                            </td>

                                            {/* Operator / User */}
                                            <td className="p-3.5 whitespace-nowrap">
                                                <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded bg-white/10 text-white font-medium text-xs">
                                                    <User size={12} className="text-mc-diamond" />
                                                    {log.username}
                                                </span>
                                            </td>

                                            {/* Action */}
                                            <td className="p-3.5 whitespace-nowrap">
                                                {getActionBadge(log.action)}
                                            </td>

                                            {/* Method & Route */}
                                            <td className="p-3.5 whitespace-nowrap">
                                                <span className="text-white/40 font-semibold mr-1.5">{log.method}</span>
                                                <span className="text-white/70">{log.endpoint}</span>
                                            </td>

                                            {/* Status Code */}
                                            <td className="p-3.5 whitespace-nowrap">
                                                {getStatusBadge(log.status_code)}
                                            </td>

                                            {/* Client IP */}
                                            <td className="p-3.5 whitespace-nowrap text-white/60">
                                                <span className="inline-flex items-center gap-1">
                                                    <Globe size={11} className="text-white/30" />
                                                    {log.ip_address || '127.0.0.1'}
                                                </span>
                                            </td>

                                            {/* Details / Payload */}
                                            <td className="p-3.5 max-w-md truncate text-white/80" title={log.details}>
                                                {log.details || <span className="text-white/20 italic">—</span>}
                                            </td>
                                        </tr>
                                    );
                                })
                            )}
                        </tbody>
                    </table>
                </div>

                {/* PAGINATION FOOTER */}
                <div className="p-3.5 border-t border-white/10 bg-stone-950/60 flex flex-col sm:flex-row items-center justify-between gap-3 font-mono text-xs text-white/60">
                    <div>
                        Showing <span className="text-white font-semibold">{total > 0 ? (page - 1) * limit + 1 : 0}</span> to{' '}
                        <span className="text-white font-semibold">{Math.min(page * limit, total)}</span> of{' '}
                        <span className="text-white font-semibold">{total}</span> events
                    </div>

                    <div className="flex items-center gap-2">
                        <button
                            onClick={() => setPage(prev => Math.max(prev - 1, 1))}
                            disabled={page <= 1 || loading}
                            className="flex items-center gap-1 px-3 py-1.5 rounded-md bg-white/10 hover:bg-white/15 active:scale-95 disabled:opacity-30 transition-all"
                        >
                            <ArrowLeft size={13} />
                            Prev
                        </button>

                        <span className="px-2 text-white/80">
                            Page <span className="text-white font-bold">{page}</span> of {totalPages || 1}
                        </span>

                        <button
                            onClick={() => setPage(prev => Math.min(prev + 1, totalPages))}
                            disabled={page >= totalPages || loading}
                            className="flex items-center gap-1 px-3 py-1.5 rounded-md bg-white/10 hover:bg-white/15 active:scale-95 disabled:opacity-30 transition-all"
                        >
                            Next
                            <ArrowRight size={13} />
                        </button>
                    </div>
                </div>
            </div>

            {/* PURGE CONFIRMATION MODAL */}
            {isPurgeModalOpen && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
                    <div className="bg-stone-900 border border-rose-500/40 rounded-xl p-6 max-w-md w-full shadow-2xl space-y-4">
                        <div className="flex items-center gap-3 text-rose-400">
                            <Trash2 size={24} />
                            <h3 className="font-pixel text-xl tracking-wider text-rose-300">
                                Purge Audit Logs
                            </h3>
                        </div>

                        <p className="text-sm text-white/70 font-mono leading-relaxed">
                            Are you sure you want to purge all <strong className="text-white">{total}</strong> audit records? This action cannot be undone and permanently erases the compliance event trail.
                        </p>

                        <div className="flex justify-end gap-3 pt-2">
                            <button
                                onClick={() => setIsPurgeModalOpen(false)}
                                disabled={purging}
                                className="px-4 py-2 rounded-lg bg-white/10 hover:bg-white/15 text-white/80 font-mono text-xs uppercase tracking-wider transition-colors disabled:opacity-50"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handlePurge}
                                disabled={purging}
                                className="px-4 py-2 rounded-lg bg-rose-600 hover:bg-rose-500 text-white font-mono text-xs uppercase tracking-wider font-semibold shadow-lg shadow-rose-900/40 transition-colors flex items-center gap-2 disabled:opacity-50"
                            >
                                {purging ? <RefreshCw size={14} className="animate-spin" /> : <Trash2 size={14} />}
                                Confirm Purge
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
