import React, { useEffect, useState, useMemo, useCallback } from 'react';
import {
    Clock, Plus, Play, Pause, Trash2, Edit2, RefreshCw, CheckCircle2,
    XCircle, AlertTriangle, Search, Filter, History, Calendar,
    HardDrive, RotateCcw, Terminal, MessageSquare, Power
} from 'lucide-react';
import type { Schedule, ScheduleLog } from '../types';

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

const PRESETS = [
    { label: 'Daily at 04:00 AM (Default Backup)', cron: '0 4 * * *', action: 'backup', payload: 'world' },
    { label: 'Every 6 Hours', cron: '0 */6 * * *', action: 'backup', payload: 'world' },
    { label: 'Daily at 05:00 AM (Nightly Restart)', cron: '0 5 * * *', action: 'restart', payload: '15' },
    { label: 'Hourly Broadcast Notice', cron: '0 * * * *', action: 'broadcast', payload: 'Remember to vote for the server and visit our Discord!' },
    { label: 'Every 30 Minutes World Save', cron: '*/30 * * * *', action: 'command', payload: 'save-all' },
    { label: 'Weekly on Sunday at 03:00 AM (Full Backup)', cron: '0 3 * * 0', action: 'backup', payload: 'full' },
];

function humanReadableCron(cron: string): string {
    const trimmed = cron.trim();
    if (trimmed === '0 4 * * *') return 'Every day at 04:00 AM';
    if (trimmed === '0 5 * * *') return 'Every day at 05:00 AM';
    if (trimmed === '0 0 * * *' || trimmed === '@daily') return 'Every day at midnight';
    if (trimmed === '0 */6 * * *') return 'Every 6 hours';
    if (trimmed === '0 * * * *' || trimmed === '@hourly') return 'Every hour on the hour';
    if (trimmed === '*/30 * * * *') return 'Every 30 minutes';
    if (trimmed === '*/15 * * * *') return 'Every 15 minutes';
    if (trimmed === '0 3 * * 0') return 'Every Sunday at 03:00 AM';
    return 'Custom schedule';
}

function getActionIcon(action: string) {
    switch (action) {
        case 'backup':
            return <HardDrive size={16} className="text-blue-400" />;
        case 'restart':
            return <RotateCcw size={16} className="text-amber-400" />;
        case 'command':
            return <Terminal size={16} className="text-purple-400" />;
        case 'broadcast':
            return <MessageSquare size={16} className="text-emerald-400" />;
        case 'start':
            return <Play size={16} className="text-green-400" />;
        case 'stop':
            return <Power size={16} className="text-rose-400" />;
        default:
            return <Clock size={16} className="text-white/60" />;
    }
}

function getActionBadge(action: string) {
    switch (action) {
        case 'backup':
            return <span className="px-2 py-0.5 text-xs font-mono rounded bg-blue-500/20 text-blue-300 border border-blue-500/30 uppercase">Backup</span>;
        case 'restart':
            return <span className="px-2 py-0.5 text-xs font-mono rounded bg-amber-500/20 text-amber-300 border border-amber-500/30 uppercase">Restart</span>;
        case 'command':
            return <span className="px-2 py-0.5 text-xs font-mono rounded bg-purple-500/20 text-purple-300 border border-purple-500/30 uppercase">Command</span>;
        case 'broadcast':
            return <span className="px-2 py-0.5 text-xs font-mono rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 uppercase">Broadcast</span>;
        case 'start':
            return <span className="px-2 py-0.5 text-xs font-mono rounded bg-green-500/20 text-green-300 border border-green-500/30 uppercase">Start</span>;
        case 'stop':
            return <span className="px-2 py-0.5 text-xs font-mono rounded bg-rose-500/20 text-rose-300 border border-rose-500/30 uppercase">Stop</span>;
        default:
            return <span className="px-2 py-0.5 text-xs font-mono rounded bg-white/10 text-white/70 border border-white/20 uppercase">{action}</span>;
    }
}

export default function Schedules() {
    const [activeTab, setActiveTab] = useState<'schedules' | 'logs'>('schedules');
    const [schedules, setSchedules] = useState<Schedule[]>([]);
    const [logs, setLogs] = useState<ScheduleLog[]>([]);
    const [loading, setLoading] = useState(true);
    const [logsLoading, setLogsLoading] = useState(false);
    const [refreshing, setRefreshing] = useState(false);
    const [toasts, setToasts] = useState<Toast[]>([]);

    // Modal states
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [editingSchedule, setEditingSchedule] = useState<Schedule | null>(null);
    const [formName, setFormName] = useState('');
    const [formCron, setFormCron] = useState('0 4 * * *');
    const [formAction, setFormAction] = useState('backup');
    const [formPayload, setFormPayload] = useState('world');
    const [formEnabled, setFormEnabled] = useState(true);
    const [saving, setSaving] = useState(false);

    // Confirmation Modals
    const [deleteScheduleTarget, setDeleteScheduleTarget] = useState<Schedule | null>(null);
    const [deleting, setDeleting] = useState(false);
    const [runningScheduleId, setRunningScheduleId] = useState<number | null>(null);
    const [clearLogsModalOpen, setClearLogsModalOpen] = useState(false);
    const [clearingLogs, setClearingLogs] = useState(false);

    // Filters for logs
    const [logScheduleFilter, setLogScheduleFilter] = useState<string>('all');
    const [logStatusFilter, setLogStatusFilter] = useState<'all' | 'success' | 'failed'>('all');
    const [logSearchQuery, setLogSearchQuery] = useState('');

    const showToast = useCallback((message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, message, type }]);
        setTimeout(() => {
            setToasts(prev => prev.filter(t => t.id !== id));
        }, 3500);
    }, []);

    const fetchSchedules = useCallback(async () => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/schedules', {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                const data = await res.json();
                setSchedules(data.schedules || []);
            } else {
                showToast('Failed to fetch schedules', 'error');
            }
        } catch (err) {
            console.error('Error fetching schedules:', err);
            showToast('Network error fetching schedules', 'error');
        }
    }, [showToast]);

    const fetchLogs = useCallback(async () => {
        setLogsLoading(true);
        const token = localStorage.getItem('token');
        try {
            let url = '/api/schedules/logs?limit=250';
            if (logScheduleFilter !== 'all') {
                url += `&schedule_id=${logScheduleFilter}`;
            }
            const res = await fetch(url, {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                const data = await res.json();
                setLogs(data.logs || []);
            } else {
                showToast('Failed to load task execution logs', 'error');
            }
        } catch (err) {
            console.error('Error fetching logs:', err);
            showToast('Network error loading logs', 'error');
        } finally {
            setLogsLoading(false);
        }
    }, [logScheduleFilter, showToast]);

    const refreshData = async () => {
        setRefreshing(true);
        await Promise.all([fetchSchedules(), fetchLogs()]);
        setRefreshing(false);
    };

    useEffect(() => {
        const init = async () => {
            setLoading(true);
            await Promise.all([fetchSchedules(), fetchLogs()]);
            setLoading(false);
        };
        init();
    }, [fetchSchedules, fetchLogs]);

    useEffect(() => {
        if (activeTab === 'logs') {
            fetchLogs();
        }
    }, [fetchLogs, activeTab]);

    const handleOpenCreateModal = () => {
        setEditingSchedule(null);
        setFormName('');
        setFormCron('0 4 * * *');
        setFormAction('backup');
        setFormPayload('world');
        setFormEnabled(true);
        setIsModalOpen(true);
    };

    const handleOpenEditModal = (s: Schedule) => {
        setEditingSchedule(s);
        setFormName(s.name);
        setFormCron(s.cron_expr);
        setFormAction(s.action_type);
        setFormPayload(s.payload || '');
        setFormEnabled(s.is_enabled);
        setIsModalOpen(true);
    };

    const handleApplyPreset = (preset: typeof PRESETS[number]) => {
        setFormCron(preset.cron);
        setFormAction(preset.action);
        setFormPayload(preset.payload);
        if (!formName) {
            setFormName(preset.label);
        }
    };

    const handleSaveSchedule = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!formName.trim() || !formCron.trim()) {
            showToast('Name and Cron expression are required', 'error');
            return;
        }

        setSaving(true);
        const token = localStorage.getItem('token');
        try {
            const isEdit = !!editingSchedule;
            const url = '/api/schedules';
            const method = isEdit ? 'PUT' : 'POST';
            const payload = {
                id: editingSchedule?.id,
                name: formName.trim(),
                cron_expr: formCron.trim(),
                action_type: formAction,
                payload: formPayload.trim(),
                is_enabled: formEnabled
            };

            const res = await fetch(url, {
                method,
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify(payload)
            });

            if (res.ok) {
                showToast(`Schedule ${isEdit ? 'updated' : 'created'} successfully`, 'success');
                setIsModalOpen(false);
                await fetchSchedules();
            } else {
                const data = await res.json();
                showToast(data.error || 'Failed to save schedule', 'error');
            }
        } catch (err) {
            console.error('Save schedule error:', err);
            showToast('Network error saving schedule', 'error');
        } finally {
            setSaving(false);
        }
    };

    const handleToggleSchedule = async (id: number) => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch(`/api/schedules/toggle?id=${id}`, {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                const updated: Schedule = await res.json();
                setSchedules(prev => prev.map(s => s.id === id ? updated : s));
                showToast(`Schedule ${updated.is_enabled ? 'enabled' : 'paused'}`, 'success');
            } else {
                const data = await res.json();
                showToast(data.error || 'Failed to toggle schedule', 'error');
            }
        } catch (err) {
            console.error('Toggle error:', err);
            showToast('Network error toggling schedule', 'error');
        }
    };

    const handleRunNow = async (id: number, name: string) => {
        setRunningScheduleId(id);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch(`/api/schedules/run?id=${id}`, {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                showToast(`Task '${name}' executed successfully!`, 'success');
                // Refresh logs immediately to show the new run record
                fetchLogs();
            } else {
                const data = await res.json();
                showToast(data.error || `Task '${name}' failed to execute`, 'error');
                fetchLogs();
            }
        } catch (err) {
            console.error('Run now error:', err);
            showToast('Network error executing task', 'error');
        } finally {
            setRunningScheduleId(null);
        }
    };

    const handleDeleteSchedule = async () => {
        if (!deleteScheduleTarget) return;
        setDeleting(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch(`/api/schedules?id=${deleteScheduleTarget.id}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                showToast(`Schedule '${deleteScheduleTarget.name}' deleted`, 'success');
                setDeleteScheduleTarget(null);
                await Promise.all([fetchSchedules(), fetchLogs()]);
            } else {
                const data = await res.json();
                showToast(data.error || 'Failed to delete schedule', 'error');
            }
        } catch (err) {
            console.error('Delete error:', err);
            showToast('Network error deleting schedule', 'error');
        } finally {
            setDeleting(false);
        }
    };

    const handleClearLogs = async () => {
        setClearingLogs(true);
        const token = localStorage.getItem('token');
        try {
            let url = '/api/schedules/logs';
            if (logScheduleFilter !== 'all') {
                url += `?schedule_id=${logScheduleFilter}`;
            }
            const res = await fetch(url, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                showToast('Execution logs cleared successfully', 'success');
                setClearLogsModalOpen(false);
                await fetchLogs();
            } else {
                const data = await res.json();
                showToast(data.error || 'Failed to clear logs', 'error');
            }
        } catch (err) {
            console.error('Clear logs error:', err);
            showToast('Network error clearing logs', 'error');
        } finally {
            setClearingLogs(false);
        }
    };

    // Filtered logs calculation
    const filteredLogs = useMemo(() => {
        return logs.filter(log => {
            if (logStatusFilter !== 'all' && log.status !== logStatusFilter) return false;
            if (logSearchQuery.trim()) {
                const query = logSearchQuery.toLowerCase();
                const matchName = log.schedule_name.toLowerCase().includes(query);
                const matchErr = (log.error_message || '').toLowerCase().includes(query);
                const matchAction = log.action_type.toLowerCase().includes(query);
                if (!matchName && !matchErr && !matchAction) return false;
            }
            return true;
        });
    }, [logs, logStatusFilter, logSearchQuery]);

    // Statistics
    const totalRuns = logs.length;
    const successRuns = logs.filter(l => l.status === 'success').length;
    const failedRuns = logs.filter(l => l.status === 'failed').length;

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
                        <Clock className="text-mc-diamond" size={28} />
                        Automated Schedules & Task Logs
                    </h1>
                    <p className="text-sm text-white/50 font-mono mt-1">
                        Cron task automation, recurring backups, maintenance commands, and execution audit history.
                    </p>
                </div>

                <div className="flex items-center gap-3">
                    <button
                        onClick={refreshData}
                        disabled={refreshing}
                        className="flex items-center gap-2 px-3 py-2 rounded bg-white/5 hover:bg-white/10 border border-white/10 text-white/80 transition-colors text-sm font-mono"
                        title="Refresh Schedules and Logs"
                    >
                        <RefreshCw size={16} className={refreshing ? 'animate-spin' : ''} />
                        Refresh
                    </button>
                    <button
                        onClick={handleOpenCreateModal}
                        className="flex items-center gap-2 px-4 py-2 rounded bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/50 text-mc-diamond font-mono text-sm transition-colors shadow-lg shadow-mc-diamond/10"
                    >
                        <Plus size={16} />
                        New Schedule
                    </button>
                </div>
            </div>

            {/* VIEW TAB SWITCHER */}
            <div className="flex items-center gap-2 border-b border-white/10 pb-2 font-mono text-sm">
                <button
                    onClick={() => setActiveTab('schedules')}
                    className={`flex items-center gap-2 px-4 py-2 rounded transition-colors ${
                        activeTab === 'schedules'
                            ? 'bg-white/10 text-white border border-white/20'
                            : 'text-white/60 hover:text-white hover:bg-white/5'
                    }`}
                >
                    <Calendar size={16} className={activeTab === 'schedules' ? 'text-mc-diamond' : ''} />
                    Active Schedules
                    <span className="ml-1 px-2 py-0.5 text-xs rounded-full bg-white/10 text-white/80">
                        {schedules.length}
                    </span>
                </button>

                <button
                    onClick={() => setActiveTab('logs')}
                    className={`flex items-center gap-2 px-4 py-2 rounded transition-colors ${
                        activeTab === 'logs'
                            ? 'bg-white/10 text-white border border-white/20'
                            : 'text-white/60 hover:text-white hover:bg-white/5'
                    }`}
                >
                    <History size={16} className={activeTab === 'logs' ? 'text-mc-diamond' : ''} />
                    Historical Run Logs
                    <span className={`ml-1 px-2 py-0.5 text-xs rounded-full ${
                        failedRuns > 0 ? 'bg-red-500/20 text-red-300 border border-red-500/30' : 'bg-white/10 text-white/80'
                    }`}>
                        {totalRuns}
                    </span>
                </button>
            </div>

            {/* TAB CONTENT: ACTIVE SCHEDULES */}
            {activeTab === 'schedules' && (
                <div className="space-y-4">
                    {loading ? (
                        <div className="text-center py-16 text-white/40 font-mono flex items-center justify-center gap-3">
                            <RefreshCw className="animate-spin" size={20} />
                            Loading schedules...
                        </div>
                    ) : schedules.length === 0 ? (
                        <div className="bg-black/40 border border-white/10 rounded-lg p-12 text-center">
                            <Clock className="mx-auto text-white/20 mb-4" size={48} />
                            <h3 className="text-lg font-mono text-white/80 mb-2">No Automated Schedules Configured</h3>
                            <p className="text-sm text-white/50 max-w-md mx-auto mb-6">
                                Set up automated world backups, scheduled server restarts, announcement broadcasts, or custom console commands.
                            </p>
                            <button
                                onClick={handleOpenCreateModal}
                                className="inline-flex items-center gap-2 px-4 py-2 rounded bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/50 text-mc-diamond font-mono text-sm transition-colors"
                            >
                                <Plus size={16} />
                                Create First Schedule
                            </button>
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            {schedules.map(schedule => {
                                const isRunning = runningScheduleId === schedule.id;
                                const isEnabled = schedule.is_enabled;
                                return (
                                    <div
                                        key={schedule.id}
                                        className={`bg-black/50 border rounded-lg p-5 transition-all flex flex-col justify-between ${
                                            isEnabled
                                                ? 'border-white/15 hover:border-white/30 shadow-lg'
                                                : 'border-white/5 opacity-70 bg-black/30'
                                        }`}
                                    >
                                        <div>
                                            {/* TOP ROW: Name & Action Badge */}
                                            <div className="flex items-start justify-between gap-3 mb-3">
                                                <div className="flex items-center gap-2.5">
                                                    <div className="p-2 rounded bg-white/5 border border-white/10">
                                                        {getActionIcon(schedule.action_type)}
                                                    </div>
                                                    <div>
                                                        <h3 className="font-mono font-medium text-white text-base">
                                                            {schedule.name}
                                                        </h3>
                                                        <div className="flex items-center gap-2 mt-1">
                                                            {getActionBadge(schedule.action_type)}
                                                            {schedule.payload && (
                                                                <span className="text-xs font-mono text-white/50 truncate max-w-[200px]" title={schedule.payload}>
                                                                    payload: <code className="text-white/80">{schedule.payload}</code>
                                                                </span>
                                                            )}
                                                        </div>
                                                    </div>
                                                </div>

                                                {/* Status Toggle Switch */}
                                                <button
                                                    onClick={() => handleToggleSchedule(schedule.id)}
                                                    className={`px-3 py-1 rounded-full text-xs font-mono font-semibold flex items-center gap-1.5 transition-colors border ${
                                                        isEnabled
                                                            ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40 hover:bg-emerald-500/30'
                                                            : 'bg-white/5 text-white/40 border-white/10 hover:bg-white/10 hover:text-white/60'
                                                    }`}
                                                    title={isEnabled ? "Click to Pause" : "Click to Enable"}
                                                >
                                                    {isEnabled ? <CheckCircle2 size={12} /> : <Pause size={12} />}
                                                    {isEnabled ? 'ACTIVE' : 'PAUSED'}
                                                </button>
                                            </div>

                                            {/* CRON & NEXT RUN INFO */}
                                            <div className="bg-white/5 border border-white/5 rounded p-3 my-3 font-mono text-xs space-y-1.5">
                                                <div className="flex items-center justify-between text-white/70">
                                                    <span className="flex items-center gap-1.5">
                                                        <Clock size={13} className="text-mc-diamond" />
                                                        Cron Expression:
                                                    </span>
                                                    <code className="bg-black/60 px-2 py-0.5 rounded text-mc-diamond font-bold">
                                                        {schedule.cron_expr}
                                                    </code>
                                                </div>
                                                <div className="text-white/40 text-[11px]">
                                                    {humanReadableCron(schedule.cron_expr)}
                                                </div>

                                                <div className="pt-2 border-t border-white/5 flex items-center justify-between">
                                                    <span className="text-white/60">Next Scheduled Run:</span>
                                                    <span className={isEnabled && schedule.next_run_at ? "text-emerald-400 font-medium" : "text-white/30 italic"}>
                                                        {isEnabled && schedule.next_run_at
                                                            ? new Date(schedule.next_run_at).toLocaleString()
                                                            : 'Schedule paused'}
                                                    </span>
                                                </div>
                                            </div>
                                        </div>

                                        {/* ACTION BUTTONS */}
                                        <div className="flex items-center justify-between gap-2 pt-3 border-t border-white/10 mt-2">
                                            <button
                                                onClick={() => handleRunNow(schedule.id, schedule.name)}
                                                disabled={isRunning}
                                                className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-mc-diamond/10 hover:bg-mc-diamond/20 border border-mc-diamond/30 text-mc-diamond text-xs font-mono transition-colors disabled:opacity-50"
                                                title="Execute this job immediately in the background"
                                            >
                                                {isRunning ? (
                                                    <RefreshCw size={13} className="animate-spin" />
                                                ) : (
                                                    <Play size={13} />
                                                )}
                                                {isRunning ? 'Running...' : 'Run Now'}
                                            </button>

                                            <div className="flex items-center gap-2">
                                                <button
                                                    onClick={() => handleOpenEditModal(schedule)}
                                                    className="p-1.5 rounded bg-white/5 hover:bg-white/10 text-white/70 hover:text-white border border-white/10 transition-colors"
                                                    title="Edit Schedule"
                                                >
                                                    <Edit2 size={14} />
                                                </button>
                                                <button
                                                    onClick={() => setDeleteScheduleTarget(schedule)}
                                                    className="p-1.5 rounded bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20 transition-colors"
                                                    title="Delete Schedule"
                                                >
                                                    <Trash2 size={14} />
                                                </button>
                                            </div>
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                </div>
            )}

            {/* TAB CONTENT: TASK EXECUTION LOGS */}
            {activeTab === 'logs' && (
                <div className="space-y-4">
                    {/* STATS OVERVIEW CARDS */}
                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                        <div className="bg-black/40 border border-white/10 rounded-lg p-4 font-mono">
                            <span className="text-xs text-white/50 uppercase">Total Executions</span>
                            <div className="text-2xl font-bold text-white mt-1">{totalRuns}</div>
                        </div>
                        <div className="bg-black/40 border border-white/10 rounded-lg p-4 font-mono">
                            <span className="text-xs text-emerald-400/80 uppercase">Successful Runs</span>
                            <div className="text-2xl font-bold text-emerald-400 mt-1">{successRuns}</div>
                        </div>
                        <div className="bg-black/40 border border-white/10 rounded-lg p-4 font-mono">
                            <span className="text-xs text-rose-400/80 uppercase">Failures / Errors</span>
                            <div className="text-2xl font-bold text-rose-400 mt-1">{failedRuns}</div>
                        </div>
                    </div>

                    {/* LOGS FILTER BAR */}
                    <div className="bg-black/40 border border-white/10 rounded-lg p-4 flex flex-col md:flex-row md:items-center justify-between gap-4 font-mono text-sm">
                        <div className="flex flex-wrap items-center gap-3">
                            {/* Schedule Filter */}
                            <div className="flex items-center gap-2">
                                <Filter size={14} className="text-white/50" />
                                <select
                                    value={logScheduleFilter}
                                    onChange={(e) => setLogScheduleFilter(e.target.value)}
                                    className="bg-black/60 border border-white/15 rounded px-2.5 py-1.5 text-xs text-white focus:outline-none focus:border-mc-diamond"
                                >
                                    <option value="all">All Schedules</option>
                                    {schedules.map(s => (
                                        <option key={s.id} value={s.id}>{s.name}</option>
                                    ))}
                                </select>
                            </div>

                            {/* Status Filter */}
                            <div className="flex items-center gap-1 bg-black/60 border border-white/15 rounded p-0.5 text-xs">
                                <button
                                    onClick={() => setLogStatusFilter('all')}
                                    className={`px-2.5 py-1 rounded transition-colors ${
                                        logStatusFilter === 'all' ? 'bg-white/20 text-white font-bold' : 'text-white/60 hover:text-white'
                                    }`}
                                >
                                    All ({logs.length})
                                </button>
                                <button
                                    onClick={() => setLogStatusFilter('success')}
                                    className={`px-2.5 py-1 rounded transition-colors ${
                                        logStatusFilter === 'success' ? 'bg-emerald-500/20 text-emerald-300 font-bold' : 'text-white/60 hover:text-white'
                                    }`}
                                >
                                    Success ({successRuns})
                                </button>
                                <button
                                    onClick={() => setLogStatusFilter('failed')}
                                    className={`px-2.5 py-1 rounded transition-colors ${
                                        logStatusFilter === 'failed' ? 'bg-rose-500/20 text-rose-300 font-bold' : 'text-white/60 hover:text-white'
                                    }`}
                                >
                                    Failed ({failedRuns})
                                </button>
                            </div>
                        </div>

                        <div className="flex items-center gap-3">
                            {/* Search Input */}
                            <div className="relative">
                                <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-white/40" />
                                <input
                                    type="text"
                                    placeholder="Search logs..."
                                    value={logSearchQuery}
                                    onChange={(e) => setLogSearchQuery(e.target.value)}
                                    className="bg-black/60 border border-white/15 rounded pl-8 pr-3 py-1.5 text-xs text-white placeholder-white/40 focus:outline-none focus:border-mc-diamond w-44 md:w-56"
                                />
                            </div>

                            {/* Clear Logs Button */}
                            {logs.length > 0 && (
                                <button
                                    onClick={() => setClearLogsModalOpen(true)}
                                    className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-red-500/10 hover:bg-red-500/20 border border-red-500/30 text-red-300 text-xs font-mono transition-colors"
                                    title="Purge historical task logs"
                                >
                                    <Trash2 size={13} />
                                    Clear Logs
                                </button>
                            )}
                        </div>
                    </div>

                    {/* LOGS AUDIT TABLE */}
                    <div className="bg-black/40 border border-white/10 rounded-lg overflow-hidden font-mono">
                        {logsLoading ? (
                            <div className="text-center py-16 text-white/40 flex items-center justify-center gap-3">
                                <RefreshCw className="animate-spin" size={18} />
                                Loading execution logs...
                            </div>
                        ) : filteredLogs.length === 0 ? (
                            <div className="text-center py-16 text-white/40">
                                <History size={36} className="mx-auto text-white/20 mb-3" />
                                <p className="text-sm">No task execution logs match current filters.</p>
                            </div>
                        ) : (
                            <div className="overflow-x-auto custom-scrollbar">
                                <table className="w-full text-left border-collapse text-xs">
                                    <thead>
                                        <tr className="border-b border-white/10 bg-white/5 text-white/60 uppercase text-[11px]">
                                            <th className="py-3 px-4">Executed At</th>
                                            <th className="py-3 px-4">Task Name</th>
                                            <th className="py-3 px-4">Action</th>
                                            <th className="py-3 px-4">Status</th>
                                            <th className="py-3 px-4">Duration</th>
                                            <th className="py-3 px-4">Details / Errors</th>
                                        </tr>
                                    </thead>
                                    <tbody className="divide-y divide-white/5">
                                        {filteredLogs.map(log => {
                                            const isSuccess = log.status === 'success';
                                            return (
                                                <tr key={log.id} className="hover:bg-white/5 transition-colors">
                                                    <td className="py-3 px-4 text-white/70 whitespace-nowrap">
                                                        {new Date(log.executed_at).toLocaleString()}
                                                    </td>
                                                    <td className="py-3 px-4 font-semibold text-white">
                                                        {log.schedule_name}
                                                    </td>
                                                    <td className="py-3 px-4">
                                                        {getActionBadge(log.action_type)}
                                                    </td>
                                                    <td className="py-3 px-4 whitespace-nowrap">
                                                        <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[11px] font-bold border ${
                                                            isSuccess
                                                                ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                                                                : 'bg-rose-500/15 text-rose-400 border-rose-500/30'
                                                        }`}>
                                                            {isSuccess ? <CheckCircle2 size={12} /> : <AlertTriangle size={12} />}
                                                            {log.status.toUpperCase()}
                                                        </span>
                                                    </td>
                                                    <td className="py-3 px-4 text-white/60 whitespace-nowrap">
                                                        {log.duration_ms < 1000 ? `${log.duration_ms}ms` : `${(log.duration_ms / 1000).toFixed(2)}s`}
                                                    </td>
                                                    <td className="py-3 px-4">
                                                        {log.error_message ? (
                                                            <div className="text-rose-300 bg-rose-950/30 border border-rose-500/20 rounded px-2 py-1 text-[11px] max-w-md break-words">
                                                                {log.error_message}
                                                            </div>
                                                        ) : (
                                                            <span className="text-white/30 italic">Completed cleanly</span>
                                                        )}
                                                    </td>
                                                </tr>
                                            );
                                        })}
                                    </tbody>
                                </table>
                            </div>
                        )}
                    </div>
                </div>
            )}

            {/* CREATE / EDIT SCHEDULE MODAL */}
            {isModalOpen && (
                <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4">
                    <div className="bg-zinc-950 border border-white/20 rounded-lg max-w-lg w-full p-6 shadow-2xl space-y-4 font-mono text-sm max-h-[90vh] overflow-y-auto custom-scrollbar">
                        <div className="flex items-center justify-between border-b border-white/10 pb-3">
                            <h3 className="text-lg font-bold text-white flex items-center gap-2">
                                <Clock className="text-mc-diamond" size={20} />
                                {editingSchedule ? 'Edit Schedule' : 'Create Automated Schedule'}
                            </h3>
                            <button
                                onClick={() => setIsModalOpen(false)}
                                className="text-white/40 hover:text-white"
                            >
                                <XCircle size={20} />
                            </button>
                        </div>

                        <form onSubmit={handleSaveSchedule} className="space-y-4">
                            {/* Preset Selector */}
                            {!editingSchedule && (
                                <div>
                                    <label className="block text-xs text-white/60 uppercase mb-1.5">
                                        Quick Schedule Presets
                                    </label>
                                    <div className="grid grid-cols-1 gap-1.5 max-h-36 overflow-y-auto custom-scrollbar border border-white/10 rounded p-1 bg-black/40">
                                        {PRESETS.map((preset, idx) => (
                                            <button
                                                key={idx}
                                                type="button"
                                                onClick={() => handleApplyPreset(preset)}
                                                className="text-left px-2.5 py-1.5 rounded hover:bg-white/10 text-xs text-white/80 transition-colors flex items-center justify-between"
                                            >
                                                <span>{preset.label}</span>
                                                <code className="text-mc-diamond text-[11px] bg-black/40 px-1.5 py-0.5 rounded">
                                                    {preset.cron}
                                                </code>
                                            </button>
                                        ))}
                                    </div>
                                </div>
                            )}

                            {/* Task Name */}
                            <div>
                                <label className="block text-xs text-white/60 uppercase mb-1">
                                    Schedule Name *
                                </label>
                                <input
                                    type="text"
                                    required
                                    placeholder="e.g. Daily World Backup"
                                    value={formName}
                                    onChange={(e) => setFormName(e.target.value)}
                                    className="w-full bg-black/60 border border-white/15 rounded px-3 py-2 text-white focus:outline-none focus:border-mc-diamond"
                                />
                            </div>

                            {/* Cron Expression */}
                            <div>
                                <label className="block text-xs text-white/60 uppercase mb-1 flex items-center justify-between">
                                    <span>Cron Expression (5-Field Syntax) *</span>
                                    <span className="text-mc-diamond lowercase font-normal">{humanReadableCron(formCron)}</span>
                                </label>
                                <input
                                    type="text"
                                    required
                                    placeholder="0 4 * * *"
                                    value={formCron}
                                    onChange={(e) => setFormCron(e.target.value)}
                                    className="w-full bg-black/60 border border-white/15 rounded px-3 py-2 text-white font-mono focus:outline-none focus:border-mc-diamond"
                                />
                                <p className="text-[11px] text-white/40 mt-1">
                                    Format: <code>minute hour day-of-month month day-of-week</code> (e.g. <code>0 4 * * *</code> for 4:00 AM daily).
                                </p>
                            </div>

                            {/* Action Type */}
                            <div>
                                <label className="block text-xs text-white/60 uppercase mb-1">
                                    Action Type *
                                </label>
                                <select
                                    value={formAction}
                                    onChange={(e) => {
                                        const val = e.target.value;
                                        setFormAction(val);
                                        if (val === 'backup' && !formPayload) setFormPayload('world');
                                        if (val === 'restart' && !formPayload) setFormPayload('10');
                                        if (val === 'command' && !formPayload) setFormPayload('save-all');
                                    }}
                                    className="w-full bg-black/60 border border-white/15 rounded px-3 py-2 text-white focus:outline-none focus:border-mc-diamond"
                                >
                                    <option value="backup">World / Server Backup (backup)</option>
                                    <option value="restart">Graceful Server Restart (restart)</option>
                                    <option value="broadcast">Announcement Broadcast (broadcast)</option>
                                    <option value="command">Console Command Execution (command)</option>
                                    <option value="start">Start Server (start)</option>
                                    <option value="stop">Stop Server (stop)</option>
                                </select>
                            </div>

                            {/* Action Payload */}
                            {formAction !== 'start' && formAction !== 'stop' && (
                                <div>
                                    <label className="block text-xs text-white/60 uppercase mb-1">
                                        {formAction === 'backup' && 'World Name or Backup Target'}
                                        {formAction === 'restart' && 'Countdown Seconds'}
                                        {formAction === 'broadcast' && 'Broadcast Announcement Text'}
                                        {formAction === 'command' && 'Minecraft Console Command'}
                                    </label>
                                    <input
                                        type="text"
                                        placeholder={
                                            formAction === 'backup' ? 'world (or full)' :
                                            formAction === 'restart' ? '10' :
                                            formAction === 'broadcast' ? 'Daily restart in 5 minutes!' :
                                            'save-all'
                                        }
                                        value={formPayload}
                                        onChange={(e) => setFormPayload(e.target.value)}
                                        className="w-full bg-black/60 border border-white/15 rounded px-3 py-2 text-white focus:outline-none focus:border-mc-diamond"
                                    />
                                    <p className="text-[11px] text-white/40 mt-1">
                                        {formAction === 'backup' && 'Enter specific world folder name or "full" for full server backup.'}
                                        {formAction === 'restart' && 'Number of seconds to broadcast countdown before restarting.'}
                                        {formAction === 'broadcast' && 'Will be announced in Minecraft chat.'}
                                        {formAction === 'command' && 'Command sent directly to server console without leading slash.'}
                                    </p>
                                </div>
                            )}

                            {/* Enabled Checkbox */}
                            <div className="flex items-center gap-2 pt-2">
                                <input
                                    type="checkbox"
                                    id="formEnabled"
                                    checked={formEnabled}
                                    onChange={(e) => setFormEnabled(e.target.checked)}
                                    className="rounded bg-black/60 border-white/20 text-mc-diamond focus:ring-0"
                                />
                                <label htmlFor="formEnabled" className="text-xs text-white/80 cursor-pointer">
                                    Enable this schedule immediately upon saving
                                </label>
                            </div>

                            {/* MODAL BUTTONS */}
                            <div className="flex justify-end gap-3 pt-4 border-t border-white/10">
                                <button
                                    type="button"
                                    onClick={() => setIsModalOpen(false)}
                                    className="px-4 py-2 rounded bg-white/5 hover:bg-white/10 text-white/70 text-xs font-mono transition-colors"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={saving}
                                    className="flex items-center gap-2 px-4 py-2 rounded bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/50 text-mc-diamond font-mono text-xs transition-colors"
                                >
                                    {saving && <RefreshCw size={14} className="animate-spin" />}
                                    {editingSchedule ? 'Save Changes' : 'Create Schedule'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* DELETE CONFIRMATION MODAL */}
            {deleteScheduleTarget && (
                <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4">
                    <div className="bg-zinc-950 border border-red-500/30 rounded-lg max-w-md w-full p-6 shadow-2xl space-y-4 font-mono text-sm">
                        <div className="flex items-center gap-3 text-red-400">
                            <AlertTriangle size={24} />
                            <h3 className="text-lg font-bold text-white">Delete Schedule</h3>
                        </div>
                        <p className="text-white/70 text-xs leading-relaxed">
                            Are you sure you want to delete the schedule{' '}
                            <span className="text-white font-bold">"{deleteScheduleTarget.name}"</span>?
                            This will also remove all associated historical run logs. This action cannot be undone.
                        </p>
                        <div className="flex justify-end gap-3 pt-2">
                            <button
                                onClick={() => setDeleteScheduleTarget(null)}
                                className="px-4 py-2 rounded bg-white/5 hover:bg-white/10 text-white/70 text-xs transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleDeleteSchedule}
                                disabled={deleting}
                                className="flex items-center gap-2 px-4 py-2 rounded bg-red-500/20 hover:bg-red-500/30 border border-red-500/50 text-red-300 text-xs transition-colors"
                            >
                                {deleting && <RefreshCw size={14} className="animate-spin" />}
                                Delete Schedule
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* CLEAR LOGS CONFIRMATION MODAL */}
            {clearLogsModalOpen && (
                <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4">
                    <div className="bg-zinc-950 border border-red-500/30 rounded-lg max-w-md w-full p-6 shadow-2xl space-y-4 font-mono text-sm">
                        <div className="flex items-center gap-3 text-red-400">
                            <Trash2 size={24} />
                            <h3 className="text-lg font-bold text-white">Clear Task Execution Logs</h3>
                        </div>
                        <p className="text-white/70 text-xs leading-relaxed">
                            {logScheduleFilter === 'all'
                                ? 'Are you sure you want to permanently delete all execution audit logs across all schedules?'
                                : `Are you sure you want to clear historical execution logs for the selected schedule?`}
                        </p>
                        <div className="flex justify-end gap-3 pt-2">
                            <button
                                onClick={() => setClearLogsModalOpen(false)}
                                className="px-4 py-2 rounded bg-white/5 hover:bg-white/10 text-white/70 text-xs transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleClearLogs}
                                disabled={clearingLogs}
                                className="flex items-center gap-2 px-4 py-2 rounded bg-red-500/20 hover:bg-red-500/30 border border-red-500/50 text-red-300 text-xs transition-colors"
                            >
                                {clearingLogs && <RefreshCw size={14} className="animate-spin" />}
                                Clear Logs
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
