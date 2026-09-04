import { useEffect, useState } from 'react';
import { 
    HardDrive, RefreshCw, Plus, Download, RotateCcw, Trash2, 
    CheckCircle, XCircle, ShieldAlert, Archive, Globe, Clock, FileCheck 
} from 'lucide-react';
import type { BackupInfo } from '../types';

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

export default function Backups() {
    const [backups, setBackups] = useState<BackupInfo[]>([]);
    const [activeWorld, setActiveWorld] = useState<string>('world');
    const [loading, setLoading] = useState(true);
    const [refreshTrigger, setRefreshTrigger] = useState(0);

    // Modal state
    const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
    const [backupType, setBackupType] = useState<'world' | 'full'>('world');
    const [customWorldName, setCustomWorldName] = useState('');
    const [creating, setCreating] = useState(false);

    // Restore state
    const [restoreTarget, setRestoreTarget] = useState<BackupInfo | null>(null);
    const [restoring, setRestoring] = useState(false);

    // Delete state
    const [deleteTarget, setDeleteTarget] = useState<BackupInfo | null>(null);
    const [deleting, setDeleting] = useState(false);

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
        const fetchBackups = async () => {
            const token = localStorage.getItem('token');
            try {
                const res = await fetch('/api/backups', {
                    headers: { 'Authorization': `Bearer ${token}` }
                });
                if (res.ok) {
                    const data = await res.json();
                    setActiveWorld(data.active_world || 'world');
                    setBackups(data.backups || []);
                    if (!customWorldName) {
                        setCustomWorldName(data.active_world || 'world');
                    }
                }
            } catch (err) {
                console.error("Failed to fetch backups", err);
                showToast("Failed to load backups list", "error");
            } finally {
                setLoading(false);
            }
        };

        fetchBackups();
    }, [refreshTrigger]);

    const handleCreateBackup = async () => {
        setCreating(true);
        const token = localStorage.getItem('token');
        try {
            const payload = {
                type: backupType,
                world_name: backupType === 'world' ? (customWorldName.trim() || activeWorld) : undefined
            };

            const res = await fetch('/api/backups/create', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify(payload)
            });

            if (res.ok) {
                const created: BackupInfo = await res.json();
                showToast(`Backup created: ${created.filename}`, 'success');
                setIsCreateModalOpen(false);
                refresh();
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to create backup', 'error');
            }
        } catch {
            showToast('Network error while creating backup', 'error');
        } finally {
            setCreating(false);
        }
    };

    const handleDownload = async (filename: string) => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch(`/api/backups/download?file=${encodeURIComponent(filename)}`, {
                headers: { 'Authorization': `Bearer ${token}` }
            });

            if (!res.ok) {
                const err = await res.json();
                showToast(err.error || 'Failed to download archive', 'error');
                return;
            }

            const blob = await res.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            window.URL.revokeObjectURL(url);
            document.body.removeChild(a);
        } catch {
            showToast('Failed to initiate download', 'error');
        }
    };

    const handleRestore = async () => {
        if (!restoreTarget) return;
        setRestoring(true);
        const token = localStorage.getItem('token');

        try {
            const res = await fetch('/api/backups/restore', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ file: restoreTarget.filename })
            });

            if (res.ok) {
                showToast(`Backup ${restoreTarget.filename} restored successfully`, 'success');
                setRestoreTarget(null);
                refresh();
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to restore backup', 'error');
            }
        } catch {
            showToast('Network error while restoring backup', 'error');
        } finally {
            setRestoring(false);
        }
    };

    const handleDelete = async () => {
        if (!deleteTarget) return;
        setDeleting(true);
        const token = localStorage.getItem('token');

        try {
            const res = await fetch(`/api/backups?file=${encodeURIComponent(deleteTarget.filename)}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${token}` }
            });

            if (res.ok) {
                showToast(`Backup ${deleteTarget.filename} deleted`, 'success');
                setDeleteTarget(null);
                refresh();
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to delete backup', 'error');
            }
        } catch {
            showToast('Network error while deleting backup', 'error');
        } finally {
            setDeleting(false);
        }
    };

    const formatDate = (isoStr: string) => {
        try {
            const d = new Date(isoStr);
            return d.toLocaleString(undefined, {
                year: 'numeric',
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
            });
        } catch {
            return isoStr;
        }
    };

    return (
        <div className="p-4 sm:p-6 space-y-6 max-w-7xl mx-auto">
            {/* Header */}
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b border-white/10 pb-5">
                <div>
                    <h1 className="font-pixel text-2xl sm:text-3xl text-mc-diamond flex items-center gap-3">
                        <Archive size={28} className="text-mc-diamond" />
                        Backup Engine & Snapshots
                    </h1>
                    <p className="text-sm text-white/60 font-mono mt-1">
                        Zero-data-loss snapshots with chunk freeze, SHA-256 verification, and atomic restoration.
                    </p>
                </div>
                <div className="flex items-center gap-3 self-stretch sm:self-auto">
                    <button
                        onClick={refresh}
                        disabled={loading}
                        className="p-2.5 bg-white/5 hover:bg-white/10 border border-white/10 rounded-md text-white/80 hover:text-white transition-all flex items-center justify-center"
                        title="Refresh Backups"
                    >
                        <RefreshCw size={18} className={loading ? 'animate-spin' : ''} />
                    </button>
                    <button
                        onClick={() => {
                            setCustomWorldName(activeWorld);
                            setIsCreateModalOpen(true);
                        }}
                        className="px-4 py-2.5 bg-mc-diamond/20 hover:bg-mc-diamond/30 border border-mc-diamond/40 text-mc-diamond rounded-md font-mono text-sm font-semibold flex items-center gap-2 transition-all shadow-lg hover:shadow-mc-diamond/10"
                    >
                        <Plus size={18} />
                        Take Snapshot
                    </button>
                </div>
            </div>

            {/* Backups Table / Card Container */}
            <div className="bg-black/40 backdrop-blur-md border border-white/10 rounded-lg overflow-hidden shadow-2xl">
                <div className="p-4 bg-white/5 border-b border-white/10 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                        <HardDrive size={18} className="text-mc-diamond" />
                        <span className="font-mono text-sm font-semibold tracking-wider uppercase text-white/90">
                            Available Archives ({backups.length})
                        </span>
                    </div>
                    <span className="text-xs font-mono text-white/50">
                        Storage Location: <code className="text-mc-diamond">/backups</code>
                    </span>
                </div>

                {loading ? (
                    <div className="p-12 text-center text-white/40 font-mono animate-pulse">
                        Scanning backup storage archives...
                    </div>
                ) : backups.length === 0 ? (
                    <div className="p-12 text-center space-y-3">
                        <Archive size={40} className="mx-auto text-white/20" />
                        <h3 className="font-mono text-lg text-white/70">No backups found</h3>
                        <p className="text-sm text-white/40 font-mono max-w-md mx-auto">
                            Take your first snapshot to safeguard world progress, dimension folders, and server configuration.
                        </p>
                    </div>
                ) : (
                    <div className="overflow-x-auto">
                        <table className="w-full text-left font-mono text-sm">
                            <thead className="bg-white/5 border-b border-white/10 text-white/50 text-xs uppercase tracking-wider">
                                <tr>
                                    <th className="py-3 px-4">Archive Details</th>
                                    <th className="py-3 px-4">Scope</th>
                                    <th className="py-3 px-4">Size</th>
                                    <th className="py-3 px-4">Created Date</th>
                                    <th className="py-3 px-4 text-right">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-white/5">
                                {backups.map(b => (
                                    <tr key={b.filename} className="hover:bg-white/[0.02] transition-colors group">
                                        <td className="py-3.5 px-4">
                                            <div className="font-semibold text-white/90 flex items-center gap-2">
                                                <Archive size={16} className="text-mc-diamond shrink-0" />
                                                <span className="break-all">{b.filename}</span>
                                            </div>
                                            {b.checksum_sha256 && (
                                                <div className="text-[11px] text-white/40 font-mono flex items-center gap-1 mt-0.5" title={`SHA256: ${b.checksum_sha256}`}>
                                                    <FileCheck size={12} className="text-green-400 shrink-0" />
                                                    <span className="truncate max-w-xs">{b.checksum_sha256.substring(0, 16)}...</span>
                                                </div>
                                            )}
                                        </td>
                                        <td className="py-3.5 px-4">
                                            {b.backup_type === 'world' ? (
                                                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-emerald-900/40 text-emerald-300 border border-emerald-500/30">
                                                    <Globe size={12} />
                                                    World ({b.world_name || activeWorld})
                                                </span>
                                            ) : (
                                                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-purple-900/40 text-purple-300 border border-purple-500/30">
                                                    <HardDrive size={12} />
                                                    Full Server
                                                </span>
                                            )}
                                        </td>
                                        <td className="py-3.5 px-4 text-white/80 font-medium">
                                            {b.formatted_size}
                                        </td>
                                        <td className="py-3.5 px-4 text-white/60 text-xs">
                                            <div className="flex items-center gap-1.5">
                                                <Clock size={14} className="text-white/40 shrink-0" />
                                                <span>{formatDate(b.created_at)}</span>
                                            </div>
                                        </td>
                                        <td className="py-3.5 px-4 text-right">
                                            <div className="flex items-center justify-end gap-2">
                                                <button
                                                    onClick={() => handleDownload(b.filename)}
                                                    className="p-1.5 bg-white/5 hover:bg-white/10 text-white/70 hover:text-white rounded border border-white/10 transition-colors"
                                                    title="Download ZIP archive"
                                                >
                                                    <Download size={15} />
                                                </button>
                                                <button
                                                    onClick={() => setRestoreTarget(b)}
                                                    className="p-1.5 bg-amber-900/20 hover:bg-amber-900/40 text-amber-400 rounded border border-amber-500/30 transition-colors"
                                                    title="Restore this backup"
                                                >
                                                    <RotateCcw size={15} />
                                                </button>
                                                <button
                                                    onClick={() => setDeleteTarget(b)}
                                                    className="p-1.5 bg-red-900/20 hover:bg-red-900/40 text-red-400 rounded border border-red-500/30 transition-colors"
                                                    title="Delete backup"
                                                >
                                                    <Trash2 size={15} />
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

            {/* CREATE BACKUP MODAL */}
            {isCreateModalOpen && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
                    <div className="bg-neutral-900 border border-white/15 rounded-lg max-w-md w-full p-6 shadow-2xl space-y-5">
                        <div className="flex items-center justify-between border-b border-white/10 pb-4">
                            <h3 className="font-pixel text-xl text-mc-diamond flex items-center gap-2">
                                <Archive size={20} />
                                Create Backup Snapshot
                            </h3>
                            <button
                                onClick={() => setIsCreateModalOpen(false)}
                                className="text-white/40 hover:text-white"
                            >
                                <XCircle size={20} />
                            </button>
                        </div>

                        <div className="space-y-4 font-mono text-sm">
                            <div>
                                <label className="block text-xs uppercase tracking-wider text-white/50 mb-2">
                                    Backup Scope
                                </label>
                                <div className="grid grid-cols-2 gap-3">
                                    <button
                                        type="button"
                                        onClick={() => setBackupType('world')}
                                        className={`p-3 rounded-lg border text-left flex flex-col gap-1 transition-all ${
                                            backupType === 'world'
                                                ? 'bg-emerald-950/40 border-emerald-500/60 text-white'
                                                : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10'
                                        }`}
                                    >
                                        <span className="font-semibold text-emerald-400 flex items-center gap-1.5">
                                            <Globe size={15} /> World
                                        </span>
                                        <span className="text-xs text-white/50">Active level & dimensions</span>
                                    </button>

                                    <button
                                        type="button"
                                        onClick={() => setBackupType('full')}
                                        className={`p-3 rounded-lg border text-left flex flex-col gap-1 transition-all ${
                                            backupType === 'full'
                                                ? 'bg-purple-950/40 border-purple-500/60 text-white'
                                                : 'bg-white/5 border-white/10 text-white/60 hover:bg-white/10'
                                        }`}
                                    >
                                        <span className="font-semibold text-purple-400 flex items-center gap-1.5">
                                            <HardDrive size={15} /> Full Server
                                        </span>
                                        <span className="text-xs text-white/50">Entire directory & jars</span>
                                    </button>
                                </div>
                            </div>

                            {backupType === 'world' && (
                                <div>
                                    <label className="block text-xs uppercase tracking-wider text-white/50 mb-1.5">
                                        Target World Name
                                    </label>
                                    <input
                                        type="text"
                                        value={customWorldName}
                                        onChange={e => setCustomWorldName(e.target.value)}
                                        placeholder={activeWorld}
                                        className="w-full bg-black/50 border border-white/15 rounded px-3 py-2 text-white font-mono text-sm focus:outline-none focus:border-mc-diamond"
                                    />
                                    <p className="text-[11px] text-white/40 mt-1">
                                        Default is currently active world (<code className="text-mc-diamond">{activeWorld}</code>).
                                    </p>
                                </div>
                            )}

                            <div className="bg-amber-950/30 border border-amber-500/30 rounded p-3 text-xs text-amber-200/80 space-y-1">
                                <p className="font-semibold flex items-center gap-1 text-amber-300">
                                    <ShieldAlert size={14} /> Zero-Downtime Guarantee
                                </p>
                                <p>
                                    If the Minecraft server is online, Lodestone flushes dirty memory chunks to disk via <code className="text-amber-100">save-all flush</code> and safely freezes autosaves during compression.
                                </p>
                            </div>
                        </div>

                        <div className="flex items-center justify-end gap-3 pt-3 border-t border-white/10 font-mono text-sm">
                            <button
                                onClick={() => setIsCreateModalOpen(false)}
                                className="px-4 py-2 bg-white/5 hover:bg-white/10 text-white/70 hover:text-white rounded border border-white/10 transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleCreateBackup}
                                disabled={creating}
                                className="px-4 py-2 bg-mc-diamond/20 hover:bg-mc-diamond/30 text-mc-diamond font-semibold rounded border border-mc-diamond/40 transition-colors flex items-center gap-2"
                            >
                                {creating ? <RefreshCw size={16} className="animate-spin" /> : <Plus size={16} />}
                                {creating ? 'Snapshotting...' : 'Create Snapshot'}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* RESTORE CONFIRMATION MODAL */}
            {restoreTarget && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
                    <div className="bg-neutral-900 border border-amber-500/30 rounded-lg max-w-md w-full p-6 shadow-2xl space-y-5">
                        <div className="flex items-center gap-3 text-amber-400 border-b border-white/10 pb-4">
                            <ShieldAlert size={24} className="shrink-0" />
                            <h3 className="font-pixel text-xl">Confirm Restoration</h3>
                        </div>

                        <div className="space-y-3 font-mono text-sm text-white/80">
                            <p>
                                Are you sure you want to restore <code className="text-amber-300 font-bold">{restoreTarget.filename}</code>?
                            </p>
                            <div className="bg-red-950/40 border border-red-500/30 rounded p-3 text-xs text-red-200 space-y-1">
                                <p className="font-bold text-red-300">Warning:</p>
                                <p>Existing world data and server files will be replaced with the contents of this archive.</p>
                                <p>If the server is currently running, it will automatically shut down, extract the backup, and restart.</p>
                            </div>
                        </div>

                        <div className="flex items-center justify-end gap-3 pt-3 border-t border-white/10 font-mono text-sm">
                            <button
                                onClick={() => setRestoreTarget(null)}
                                disabled={restoring}
                                className="px-4 py-2 bg-white/5 hover:bg-white/10 text-white/70 hover:text-white rounded border border-white/10 transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleRestore}
                                disabled={restoring}
                                className="px-4 py-2 bg-amber-600 hover:bg-amber-500 text-black font-semibold rounded transition-colors flex items-center gap-2"
                            >
                                {restoring ? <RefreshCw size={16} className="animate-spin" /> : <RotateCcw size={16} />}
                                {restoring ? 'Restoring Archive...' : 'Confirm & Restore'}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* DELETE CONFIRMATION MODAL */}
            {deleteTarget && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
                    <div className="bg-neutral-900 border border-red-500/30 rounded-lg max-w-md w-full p-6 shadow-2xl space-y-5">
                        <div className="flex items-center gap-3 text-red-400 border-b border-white/10 pb-4">
                            <Trash2 size={24} className="shrink-0" />
                            <h3 className="font-pixel text-xl">Delete Backup Archive</h3>
                        </div>

                        <p className="font-mono text-sm text-white/80">
                            Are you sure you want to permanently delete <code className="text-red-300">{deleteTarget.filename}</code> ({deleteTarget.formatted_size})?
                        </p>

                        <div className="flex items-center justify-end gap-3 pt-3 border-t border-white/10 font-mono text-sm">
                            <button
                                onClick={() => setDeleteTarget(null)}
                                disabled={deleting}
                                className="px-4 py-2 bg-white/5 hover:bg-white/10 text-white/70 hover:text-white rounded border border-white/10 transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleDelete}
                                disabled={deleting}
                                className="px-4 py-2 bg-red-600 hover:bg-red-500 text-white font-semibold rounded transition-colors flex items-center gap-2"
                            >
                                {deleting ? <RefreshCw size={16} className="animate-spin" /> : <Trash2 size={16} />}
                                {deleting ? 'Deleting...' : 'Delete Archive'}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* GLOBAL TOAST CONTAINER */}
            <div className="fixed bottom-6 right-6 z-[100] flex flex-col gap-2 pointer-events-none">
                {toasts.map(toast => (
                    <div
                        key={toast.id}
                        className={`
                            flex items-center gap-3 px-4 py-3 rounded shadow-lg border backdrop-blur-md pointer-events-auto
                            ${toast.type === 'success'
                                ? 'bg-green-900/90 border-green-500/50 text-green-100'
                                : 'bg-red-900/90 border-red-500/50 text-red-100'}
                        `}
                    >
                        {toast.type === 'success' ? <CheckCircle size={18} /> : <XCircle size={18} />}
                        <span className="font-mono text-sm">{toast.message}</span>
                    </div>
                ))}
            </div>
        </div>
    );
}
