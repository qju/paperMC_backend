import { useEffect, useState } from 'react';
import {
    Shield, Plus, Key, Trash2, RefreshCw, CheckCircle, XCircle,
    ShieldCheck, ShieldAlert, KeyRound, Copy, Check, QrCode,
    Download, ArrowRight, ArrowLeft, AlertTriangle, Smartphone, RotateCcw
} from 'lucide-react';
import QRCode from 'qrcode';

interface User {
    id: number;
    username: string;
    role: string;
    mfa_enabled?: boolean;
}

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

export default function Users() {
    const [users, setUsers] = useState<User[]>([]);
    const [loading, setLoading] = useState(true);
    const [toasts, setToasts] = useState<Toast[]>([]);

    const currentUsername = (() => {
        try {
            const token = localStorage.getItem('token');
            if (!token) return '';
            const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')));
            return payload.username || '';
        } catch {
            return '';
        }
    })();

    // Create User Modal
    const [showCreateModal, setShowCreateModal] = useState(false);
    const [newUsername, setNewUsername] = useState('');
    const [newPassword, setNewPassword] = useState('');
    const [newRole, setNewRole] = useState('admin');
    const [creating, setCreating] = useState(false);

    // Reset Password Modal
    const [resetTargetUser, setResetTargetUser] = useState<string | null>(null);
    const [resetPassword, setResetPassword] = useState('');
    const [resetting, setResetting] = useState(false);

    // Admin Reset 2FA Modal
    const [reset2FATargetUser, setReset2FATargetUser] = useState<string | null>(null);
    const [resetting2FA, setResetting2FA] = useState(false);

    // Two-Factor Authentication (2FA) State
    const [mfaEnabled, setMfaEnabled] = useState(false);
    const [mfaLoading, setMfaLoading] = useState(false);
    const [showSetupModal, setShowSetupModal] = useState(false);
    const [setupStep, setSetupStep] = useState<1 | 2 | 3>(1);
    const [mfaSecret, setMfaSecret] = useState('');
    const [mfaAuthURL, setMfaAuthURL] = useState('');
    const [qrCodeDataURL, setQrCodeDataURL] = useState('');
    const [showManualSecret, setShowManualSecret] = useState(false);
    const [mfaBackupCodes, setMfaBackupCodes] = useState<string[]>([]);
    const [mfaVerifyCode, setMfaVerifyCode] = useState('');
    const [activatingMFA, setActivatingMFA] = useState(false);
    const [copiedSecret, setCopiedSecret] = useState(false);
    const [copiedBackupCodes, setCopiedBackupCodes] = useState(false);
    const [showDisableModal, setShowDisableModal] = useState(false);
    const [disablePasswordOrCode, setDisablePasswordOrCode] = useState('');
    const [disablingMFA, setDisablingMFA] = useState(false);

    const showToast = (message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, message, type }]);
        setTimeout(() => {
            setToasts(prev => prev.filter(t => t.id !== id));
        }, 3500);
    };

    const safeParseJSON = async (res: Response) => {
        const text = await res.text();
        try {
            return JSON.parse(text);
        } catch {
            return null;
        }
    };

    const fetchUsers = async () => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/users', {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            const data = await safeParseJSON(res);
            if (data === null) {
                showToast("Backend returned invalid response. Please restart the backend server.", 'error');
                return;
            }
            if (res.ok) {
                setUsers(Array.isArray(data) ? data : []);
            } else {
                showToast(data.error || "Failed to fetch users", 'error');
            }
        } catch (err) {
            console.error("Failed to load users", err);
            showToast("Network error while loading users", 'error');
        } finally {
            setLoading(false);
        }
    };

    const fetch2FAStatus = async () => {
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/auth/2fa/status', {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            if (res.ok) {
                const data = await res.json();
                setMfaEnabled(Boolean(data.enabled));
            }
        } catch (err) {
            console.error("Failed to load 2FA status", err);
        }
    };

    useEffect(() => {
        fetchUsers();
        fetch2FAStatus();
    }, []);

    const handleStart2FASetup = async () => {
        setMfaLoading(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/auth/2fa/setup', {
                method: 'POST',
                headers: { 'Authorization': `Bearer ${token}` }
            });
            const data = await safeParseJSON(res);
            if (res.ok && data) {
                setMfaSecret(data.secret);
                setMfaAuthURL(data.otpauth_url);
                setMfaBackupCodes(data.backup_codes || []);
                setMfaVerifyCode('');
                setCopiedSecret(false);
                setCopiedBackupCodes(false);
                setShowManualSecret(false);
                setSetupStep(1);

                if (data.otpauth_url) {
                    try {
                        const qrUrl = await QRCode.toDataURL(data.otpauth_url, {
                            width: 220,
                            margin: 2,
                            color: {
                                dark: '#000000',
                                light: '#ffffff'
                            }
                        });
                        setQrCodeDataURL(qrUrl);
                    } catch (qrErr) {
                        console.error("Failed generating QR code", qrErr);
                    }
                }

                setShowSetupModal(true);
            } else {
                showToast(data?.error || "Failed to initialize 2FA setup", 'error');
            }
        } catch (err) {
            showToast("Network error starting 2FA setup", 'error');
        } finally {
            setMfaLoading(false);
        }
    };

    const handleDownloadBackupCodes = () => {
        const text = [
            '====================================================',
            ' LODESTONE MINECRAFT MANAGER - 2FA RECOVERY CODES',
            ` Generated: ${new Date().toLocaleString()}`,
            '====================================================',
            '',
            'IMPORTANT: Each backup recovery code can be used ONCE.',
            'Store this file securely (e.g. in your password manager).',
            '',
            ...mfaBackupCodes.map((c, i) => ` Code #${i + 1}: ${c}`),
            '',
            '===================================================='
        ].join('\n');

        const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `lodestone-2fa-backup-codes-${new Date().toISOString().slice(0, 10)}.txt`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
        showToast('Backup recovery codes downloaded (.txt)', 'success');
    };

    const handleConfirm2FA = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!mfaVerifyCode.trim()) return;
        setActivatingMFA(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/auth/2fa/enable', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ code: mfaVerifyCode.trim() })
            });
            const data = await safeParseJSON(res);
            if (res.ok) {
                showToast("Two-factor authentication successfully enabled!", 'success');
                setMfaEnabled(true);
                setShowSetupModal(false);
                setMfaVerifyCode('');
                fetchUsers();
            } else {
                showToast(data?.error || "Invalid verification code", 'error');
            }
        } catch (err) {
            showToast("Network error activating 2FA", 'error');
        } finally {
            setActivatingMFA(false);
        }
    };

    const handleDisable2FA = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!disablePasswordOrCode.trim()) return;
        setDisablingMFA(true);
        const token = localStorage.getItem('token');
        try {
            const clean = disablePasswordOrCode.trim();
            const isCodeOnly = /^[0-9A-Za-z-]{6,9}$/.test(clean) && clean.length <= 9;
            const body = isCodeOnly
                ? { code: clean }
                : { password: clean };

            const res = await fetch('/api/auth/2fa/disable', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(body)
            });
            const data = await safeParseJSON(res);
            if (res.ok) {
                showToast("Two-factor authentication disabled", 'success');
                setMfaEnabled(false);
                setShowDisableModal(false);
                setDisablePasswordOrCode('');
                fetchUsers();
            } else {
                showToast(data?.error || "Failed to disable 2FA", 'error');
            }
        } catch (err) {
            showToast("Network error disabling 2FA", 'error');
        } finally {
            setDisablingMFA(false);
        }
    };

    const handleCreateUser = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!newUsername.trim() || !newPassword.trim()) return;

        setCreating(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/users', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    username: newUsername.trim(),
                    password: newPassword.trim(),
                    role: newRole
                })
            });

            const data = await safeParseJSON(res);
            if (data === null) {
                showToast("Backend returned invalid response. Please restart backend server.", 'error');
                return;
            }

            if (res.ok) {
                showToast(`User "${newUsername.trim()}" created successfully`, 'success');
                setShowCreateModal(false);
                setNewUsername('');
                setNewPassword('');
                setNewRole('admin');
                fetchUsers();
            } else {
                showToast(data.error || "Failed to create user", 'error');
            }
        } catch (err) {
            console.error("Create user error", err);
            showToast("Network error creating user", 'error');
        } finally {
            setCreating(false);
        }
    };

    const handleResetPassword = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!resetTargetUser || !resetPassword.trim()) return;

        setResetting(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/users/password', {
                method: 'PUT',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    username: resetTargetUser,
                    password: resetPassword.trim()
                })
            });

            const data = await safeParseJSON(res);
            if (data === null) {
                showToast("Backend returned invalid response. Please restart backend server.", 'error');
                return;
            }

            if (res.ok) {
                showToast(`Password for "${resetTargetUser}" updated`, 'success');
                setResetTargetUser(null);
                setResetPassword('');
            } else {
                showToast(data.error || "Failed to update password", 'error');
            }
        } catch (err) {
            console.error("Reset password error", err);
            showToast("Network error updating password", 'error');
        } finally {
            setResetting(false);
        }
    };

    const handleDeleteUser = async (username: string) => {
        if (!confirm(`Are you sure you want to delete user "${username}"?`)) {
            return;
        }

        const token = localStorage.getItem('token');
        try {
            const res = await fetch(`/api/users?username=${encodeURIComponent(username)}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${token}` }
            });

            const data = await safeParseJSON(res);
            if (data === null) {
                showToast("Backend returned invalid response. Please restart backend server.", 'error');
                return;
            }

            if (res.ok) {
                showToast(`User "${username}" deleted`, 'success');
                fetchUsers();
            } else {
                showToast(data.error || "Failed to delete user", 'error');
            }
        } catch (err) {
            console.error("Delete user error", err);
            showToast("Network error deleting user", 'error');
        }
    };

    const handleAdminReset2FA = async () => {
        if (!reset2FATargetUser) return;
        setResetting2FA(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/users/reset-2fa', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ username: reset2FATargetUser })
            });

            const data = await safeParseJSON(res);
            if (res.ok) {
                showToast(`2FA for "${reset2FATargetUser}" has been reset. Active sessions revoked.`, 'success');
                setReset2FATargetUser(null);
                fetchUsers();
                if (reset2FATargetUser === currentUsername) {
                    setMfaEnabled(false);
                }
            } else {
                showToast(data?.error || "Failed to reset 2FA", 'error');
            }
        } catch (err) {
            console.error("Admin reset 2FA error", err);
            showToast("Network error resetting 2FA", 'error');
        } finally {
            setResetting2FA(false);
        }
    };

    return (
        <div className="space-y-6 relative min-h-[500px]">
            {/* TOASTS */}
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

            {/* CREATE USER MODAL */}
            {showCreateModal && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
                    <div className="bg-black/90 border border-white/20 rounded-xl p-6 w-full max-w-md space-y-4 shadow-2xl">
                        <div className="flex justify-between items-center">
                            <h3 className="text-xl font-pixel text-mc-diamond flex items-center gap-2">
                                <Plus size={20} /> Create Web Manager User
                            </h3>
                            <button onClick={() => setShowCreateModal(false)} className="text-white/50 hover:text-white">
                                <XCircle size={20} />
                            </button>
                        </div>
                        <form onSubmit={handleCreateUser} className="space-y-4">
                            <div>
                                <label className="block text-xs uppercase font-mono text-white/50 mb-1">Username</label>
                                <input
                                    type="text"
                                    value={newUsername}
                                    onChange={(e) => setNewUsername(e.target.value)}
                                    placeholder="e.g. operator1"
                                    required
                                    className="w-full bg-black/60 border border-white/20 rounded p-3 text-white font-mono focus:border-mc-diamond focus:outline-none"
                                />
                            </div>
                            <div>
                                <label className="block text-xs uppercase font-mono text-white/50 mb-1">Password</label>
                                <input
                                    type="password"
                                    value={newPassword}
                                    onChange={(e) => setNewPassword(e.target.value)}
                                    placeholder="Minimum 4 characters"
                                    required
                                    minLength={4}
                                    className="w-full bg-black/60 border border-white/20 rounded p-3 text-white font-mono focus:border-mc-diamond focus:outline-none"
                                />
                            </div>
                            <div>
                                <label className="block text-xs uppercase font-mono text-white/50 mb-1">Role</label>
                                <select
                                    value={newRole}
                                    onChange={(e) => setNewRole(e.target.value)}
                                    className="w-full bg-black/60 border border-white/20 rounded p-3 text-white font-mono focus:border-mc-diamond focus:outline-none"
                                >
                                    <option value="admin">Administrator (Full Control)</option>
                                    <option value="operator">Operator</option>
                                    <option value="viewer">Viewer (Read Only)</option>
                                </select>
                            </div>
                            <div className="flex justify-end gap-3 pt-2">
                                <button
                                    type="button"
                                    onClick={() => setShowCreateModal(false)}
                                    className="px-4 py-2 rounded font-mono text-sm bg-white/10 hover:bg-white/20 text-white"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={creating}
                                    className="px-4 py-2 rounded font-mono font-bold text-sm bg-green-600 hover:bg-green-500 text-white flex items-center gap-2"
                                >
                                    {creating ? <RefreshCw size={16} className="animate-spin" /> : <Plus size={16} />}
                                    {creating ? 'Creating...' : 'Create User'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* RESET PASSWORD MODAL */}
            {resetTargetUser && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
                    <div className="bg-black/90 border border-white/20 rounded-xl p-6 w-full max-w-md space-y-4 shadow-2xl">
                        <div className="flex justify-between items-center">
                            <h3 className="text-xl font-pixel text-mc-gold flex items-center gap-2">
                                <Key size={20} /> Reset Password
                            </h3>
                            <button onClick={() => setResetTargetUser(null)} className="text-white/50 hover:text-white">
                                <XCircle size={20} />
                            </button>
                        </div>
                        <p className="text-xs font-mono text-white/60">
                            Reset password for <strong className="text-white">{resetTargetUser}</strong>.
                        </p>
                        <form onSubmit={handleResetPassword} className="space-y-4">
                            <div>
                                <label className="block text-xs uppercase font-mono text-white/50 mb-1">New Password</label>
                                <input
                                    type="password"
                                    value={resetPassword}
                                    onChange={(e) => setResetPassword(e.target.value)}
                                    placeholder="Enter new password"
                                    required
                                    minLength={4}
                                    className="w-full bg-black/60 border border-white/20 rounded p-3 text-white font-mono focus:border-mc-gold focus:outline-none"
                                />
                            </div>
                            <div className="flex justify-end gap-3 pt-2">
                                <button
                                    type="button"
                                    onClick={() => setResetTargetUser(null)}
                                    className="px-4 py-2 rounded font-mono text-sm bg-white/10 hover:bg-white/20 text-white"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={resetting}
                                    className="px-4 py-2 rounded font-mono font-bold text-sm bg-green-600 hover:bg-green-500 text-white flex items-center gap-2"
                                >
                                    {resetting ? <RefreshCw size={16} className="animate-spin" /> : <Key size={16} />}
                                    {resetting ? 'Updating...' : 'Update Password'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* 2FA SETUP MODAL - 3-STEP GUIDED FLOW */}
            {showSetupModal && (
                <div className="fixed inset-0 bg-black/85 backdrop-blur-md z-50 flex items-center justify-center p-4 overflow-y-auto">
                    <div className="bg-zinc-950/95 border border-white/20 rounded-2xl p-6 w-full max-w-lg space-y-5 shadow-2xl my-8">
                        {/* Modal Header */}
                        <div className="flex justify-between items-center border-b border-white/10 pb-3">
                            <h3 className="text-lg font-pixel text-mc-diamond flex items-center gap-2">
                                <ShieldCheck size={20} /> Setup Two-Factor Authentication
                            </h3>
                            <button onClick={() => setShowSetupModal(false)} className="text-white/50 hover:text-white transition-colors">
                                <XCircle size={20} />
                            </button>
                        </div>

                        {/* Step Indicator / Stepper */}
                        <div className="grid grid-cols-3 gap-2 p-1.5 bg-white/5 border border-white/10 rounded-xl text-xs font-mono text-center">
                            <div
                                className={`py-1.5 px-2 rounded-lg flex items-center justify-center gap-1.5 transition-colors ${
                                    setupStep === 1
                                        ? 'bg-mc-diamond/20 text-mc-diamond font-bold border border-mc-diamond/40'
                                        : setupStep > 1
                                        ? 'text-emerald-400 font-semibold'
                                        : 'text-white/40'
                                }`}
                            >
                                {setupStep > 1 ? <Check size={14} className="text-emerald-400" /> : <span className="w-4 h-4 rounded-full bg-white/10 text-[10px] flex items-center justify-center">1</span>}
                                <span>1. Scan QR</span>
                            </div>
                            <div
                                className={`py-1.5 px-2 rounded-lg flex items-center justify-center gap-1.5 transition-colors ${
                                    setupStep === 2
                                        ? 'bg-mc-diamond/20 text-mc-diamond font-bold border border-mc-diamond/40'
                                        : setupStep > 2
                                        ? 'text-emerald-400 font-semibold'
                                        : 'text-white/40'
                                }`}
                            >
                                {setupStep > 2 ? <Check size={14} className="text-emerald-400" /> : <span className="w-4 h-4 rounded-full bg-white/10 text-[10px] flex items-center justify-center">2</span>}
                                <span>2. Backup</span>
                            </div>
                            <div
                                className={`py-1.5 px-2 rounded-lg flex items-center justify-center gap-1.5 transition-colors ${
                                    setupStep === 3
                                        ? 'bg-mc-diamond/20 text-mc-diamond font-bold border border-mc-diamond/40'
                                        : 'text-white/40'
                                }`}
                            >
                                <span className="w-4 h-4 rounded-full bg-white/10 text-[10px] flex items-center justify-center">3</span>
                                <span>3. Verify</span>
                            </div>
                        </div>

                        {/* STEP 1: SCAN QR CODE */}
                        {setupStep === 1 && (
                            <div className="space-y-4 animate-in fade-in zoom-in-95 duration-200">
                                <div className="text-center space-y-1">
                                    <h4 className="font-mono font-bold text-sm text-white flex items-center justify-center gap-1.5">
                                        <QrCode size={16} className="text-mc-diamond" /> Scan with Your Authenticator App
                                    </h4>
                                    <p className="text-xs font-mono text-white/50">
                                        Compatible with Google Authenticator, Microsoft Authenticator, 1Password, Authy, or Apple Passwords.
                                    </p>
                                </div>

                                {/* QR Code Display */}
                                <div className="flex flex-col items-center justify-center p-4 bg-white rounded-xl shadow-2xl mx-auto w-fit border-2 border-white/10">
                                    {qrCodeDataURL ? (
                                        <img
                                            src={qrCodeDataURL}
                                            alt="2FA QR Code"
                                            className="w-48 h-48 block select-none"
                                        />
                                    ) : (
                                        <div className="w-48 h-48 flex flex-col items-center justify-center text-zinc-500 font-mono text-xs gap-2">
                                            <RefreshCw size={24} className="animate-spin text-emerald-600" />
                                            <span>Generating QR Code...</span>
                                        </div>
                                    )}
                                </div>

                                {/* How-to instructions */}
                                <div className="p-3 bg-white/5 border border-white/10 rounded-xl text-xs font-mono text-white/70 space-y-1.5">
                                    <div className="flex items-center gap-2 text-white font-bold">
                                        <Smartphone size={15} className="text-mc-diamond" />
                                        <span>How to connect:</span>
                                    </div>
                                    <ol className="list-decimal list-inside space-y-1 text-white/60 text-[11px] pl-1">
                                        <li>Open your authenticator app on your mobile phone.</li>
                                        <li>Tap <strong className="text-white">+</strong> or <strong className="text-white">Add Account</strong> and select <strong className="text-white">Scan QR Code</strong>.</li>
                                        <li>Point your camera at the QR code above.</li>
                                    </ol>
                                </div>

                                {/* Collapsible Manual Key */}
                                <div className="border border-white/10 rounded-xl overflow-hidden">
                                    <button
                                        type="button"
                                        onClick={() => setShowManualSecret(!showManualSecret)}
                                        className="w-full px-3 py-2 text-left text-xs font-mono text-white/60 hover:text-white bg-black/40 flex items-center justify-between transition-colors"
                                    >
                                        <span>Can't scan the QR code? Enter key manually</span>
                                        <span className="text-[11px] text-mc-diamond">{showManualSecret ? 'Hide Key' : 'Show Key'}</span>
                                    </button>
                                    {showManualSecret && (
                                        <div className="p-3 bg-black/60 border-t border-white/10 space-y-2">
                                            <div className="flex items-center gap-2">
                                                <code className="flex-1 font-mono text-xs text-mc-gold select-all break-all bg-black/80 px-2.5 py-1.5 rounded-lg border border-white/10">
                                                    {mfaSecret}
                                                </code>
                                                <button
                                                    type="button"
                                                    onClick={() => {
                                                        navigator.clipboard.writeText(mfaSecret);
                                                        setCopiedSecret(true);
                                                        setTimeout(() => setCopiedSecret(false), 2000);
                                                    }}
                                                    className="px-2.5 py-1.5 rounded-lg text-xs font-mono bg-white/10 hover:bg-white/20 text-white flex items-center gap-1.5 transition-colors shrink-0"
                                                >
                                                    {copiedSecret ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
                                                    {copiedSecret ? 'Copied' : 'Copy'}
                                                </button>
                                            </div>
                                            {mfaAuthURL && (
                                                <div className="text-right">
                                                    <a
                                                        href={mfaAuthURL}
                                                        className="text-[11px] font-mono text-mc-diamond/80 hover:text-mc-diamond hover:underline"
                                                    >
                                                        Open in Authenticator App &rarr;
                                                    </a>
                                                </div>
                                            )}
                                        </div>
                                    )}
                                </div>

                                {/* Step 1 Footer Buttons */}
                                <div className="flex justify-between items-center pt-3 border-t border-white/10">
                                    <button
                                        type="button"
                                        onClick={() => setShowSetupModal(false)}
                                        className="px-4 py-2 rounded-lg font-mono text-xs text-white/50 hover:text-white transition-colors"
                                    >
                                        Cancel
                                    </button>
                                    <button
                                        type="button"
                                        onClick={() => setSetupStep(2)}
                                        className="px-5 py-2.5 rounded-lg font-mono font-bold text-xs bg-emerald-600 hover:bg-emerald-500 text-white flex items-center gap-1.5 transition-all shadow-lg active:scale-[0.99]"
                                    >
                                        <span>Next: Save Backup Codes</span>
                                        <ArrowRight size={14} />
                                    </button>
                                </div>
                            </div>
                        )}

                        {/* STEP 2: EMERGENCY RECOVERY BACKUP CODES */}
                        {setupStep === 2 && (
                            <div className="space-y-4 animate-in fade-in zoom-in-95 duration-200">
                                <div className="p-3.5 bg-yellow-500/10 border border-yellow-500/30 rounded-xl text-xs font-mono text-yellow-200/90 space-y-1.5">
                                    <div className="flex items-center gap-2 font-bold text-yellow-300">
                                        <AlertTriangle size={16} />
                                        <span>Save Your Emergency Recovery Codes</span>
                                    </div>
                                    <p className="text-[11px] text-yellow-100/70 leading-relaxed">
                                        If you lose your phone or cannot access your authenticator app, these 8 single-use codes are the <strong className="text-white">only way</strong> to log into your administrator account.
                                    </p>
                                </div>

                                {/* Grid of Codes */}
                                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 p-3.5 bg-black/60 border border-white/10 rounded-xl">
                                    {mfaBackupCodes.map((code, idx) => (
                                        <div
                                            key={idx}
                                            className="font-mono text-xs text-white/90 bg-white/5 border border-white/10 px-2.5 py-2 rounded-lg text-center select-all font-semibold tracking-wider shadow-sm"
                                        >
                                            {code}
                                        </div>
                                    ))}
                                </div>

                                {/* Backup Actions */}
                                <div className="flex flex-wrap gap-2 pt-1">
                                    <button
                                        type="button"
                                        onClick={() => {
                                            navigator.clipboard.writeText(mfaBackupCodes.join('\n'));
                                            setCopiedBackupCodes(true);
                                            setTimeout(() => setCopiedBackupCodes(false), 2000);
                                        }}
                                        className="flex-1 py-2 px-3 rounded-lg font-mono text-xs bg-white/10 hover:bg-white/20 text-white flex items-center justify-center gap-1.5 transition-colors border border-white/10"
                                    >
                                        {copiedBackupCodes ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
                                        {copiedBackupCodes ? 'Codes Copied!' : 'Copy All Codes'}
                                    </button>
                                    <button
                                        type="button"
                                        onClick={handleDownloadBackupCodes}
                                        className="flex-1 py-2 px-3 rounded-lg font-mono text-xs bg-mc-diamond/10 hover:bg-mc-diamond/20 border border-mc-diamond/30 text-mc-diamond flex items-center justify-center gap-1.5 transition-colors"
                                    >
                                        <Download size={14} /> Download Backup (.txt)
                                    </button>
                                </div>

                                <p className="text-[11px] font-mono text-white/40 text-center">
                                    Each recovery code is single-use. Store them in a password manager or secure vault.
                                </p>

                                {/* Step 2 Footer Buttons */}
                                <div className="flex justify-between items-center pt-3 border-t border-white/10">
                                    <button
                                        type="button"
                                        onClick={() => setSetupStep(1)}
                                        className="px-4 py-2 rounded-lg font-mono text-xs text-white/60 hover:text-white flex items-center gap-1.5 transition-colors"
                                    >
                                        <ArrowLeft size={14} /> Back
                                    </button>
                                    <button
                                        type="button"
                                        onClick={() => setSetupStep(3)}
                                        className="px-5 py-2.5 rounded-lg font-mono font-bold text-xs bg-emerald-600 hover:bg-emerald-500 text-white flex items-center gap-1.5 transition-all shadow-lg active:scale-[0.99]"
                                    >
                                        <span>Next: Verify Code</span>
                                        <ArrowRight size={14} />
                                    </button>
                                </div>
                            </div>
                        )}

                        {/* STEP 3: CONFIRM & ACTIVATE 2FA */}
                        {setupStep === 3 && (
                            <form onSubmit={handleConfirm2FA} className="space-y-4 animate-in fade-in zoom-in-95 duration-200">
                                <div className="p-3 bg-white/5 border border-white/10 rounded-xl text-xs font-mono text-white/70 space-y-1">
                                    <div className="flex items-center gap-2 text-white font-bold">
                                        <ShieldCheck size={16} className="text-emerald-400" />
                                        <span>Verify Connection & Activate</span>
                                    </div>
                                    <p className="text-[11px] text-white/60 leading-relaxed">
                                        Enter the 6-digit code currently shown in your authenticator app for <strong>Lodestone</strong> to confirm setup.
                                    </p>
                                </div>

                                <div>
                                    <label className="block text-xs uppercase font-mono text-white/60 mb-2 flex items-center gap-1.5">
                                        <KeyRound size={14} className="text-mc-diamond" /> 6-Digit Authenticator Code
                                    </label>
                                    <input
                                        type="text"
                                        autoFocus
                                        value={mfaVerifyCode}
                                        onChange={(e) => setMfaVerifyCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                                        placeholder="000000"
                                        maxLength={6}
                                        required
                                        className="w-full bg-black/60 border border-white/20 rounded-xl p-3 text-center text-white font-mono text-2xl tracking-[0.4em] focus:border-mc-diamond focus:bg-black/90 focus:outline-none transition-colors"
                                    />
                                </div>

                                <div className="flex justify-between items-center pt-3 border-t border-white/10">
                                    <button
                                        type="button"
                                        onClick={() => setSetupStep(2)}
                                        className="px-4 py-2 rounded-lg font-mono text-xs text-white/60 hover:text-white flex items-center gap-1.5 transition-colors"
                                    >
                                        <ArrowLeft size={14} /> Back
                                    </button>
                                    <button
                                        type="submit"
                                        disabled={activatingMFA || mfaVerifyCode.trim().length !== 6}
                                        className="px-5 py-2.5 rounded-lg font-mono font-bold text-sm bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white flex items-center gap-2 transition-all shadow-lg active:scale-[0.99]"
                                    >
                                        {activatingMFA ? <RefreshCw size={16} className="animate-spin" /> : <ShieldCheck size={16} />}
                                        {activatingMFA ? 'Verifying Code...' : 'Activate 2FA Protection'}
                                    </button>
                                </div>
                            </form>
                        )}
                    </div>
                </div>
            )}

            {/* 2FA DISABLE MODAL */}
            {showDisableModal && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
                    <div className="bg-black/90 border border-white/20 rounded-xl p-6 w-full max-w-md space-y-4 shadow-2xl">
                        <div className="flex justify-between items-center">
                            <h3 className="text-lg font-pixel text-red-400 flex items-center gap-2">
                                <ShieldAlert size={20} /> Disable Two-Factor Authentication
                            </h3>
                            <button onClick={() => setShowDisableModal(false)} className="text-white/50 hover:text-white">
                                <XCircle size={20} />
                            </button>
                        </div>
                        <p className="text-xs font-mono text-white/60">
                            Enter your current account password or an active 6-digit verification code to confirm deactivation:
                        </p>
                        <form onSubmit={handleDisable2FA} className="space-y-4">
                            <div>
                                <label className="block text-xs uppercase font-mono text-white/50 mb-1">
                                    Password or 6-Digit Code
                                </label>
                                <input
                                    type="password"
                                    value={disablePasswordOrCode}
                                    onChange={(e) => setDisablePasswordOrCode(e.target.value)}
                                    placeholder="Enter password or 000000"
                                    required
                                    className="w-full bg-black/60 border border-white/20 rounded p-3 text-white font-mono focus:border-red-500 focus:outline-none"
                                />
                            </div>
                            <div className="flex justify-end gap-3 pt-2">
                                <button
                                    type="button"
                                    onClick={() => setShowDisableModal(false)}
                                    className="px-4 py-2 rounded font-mono text-sm bg-white/10 hover:bg-white/20 text-white"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={disablingMFA || !disablePasswordOrCode.trim()}
                                    className="px-4 py-2 rounded font-mono font-bold text-sm bg-red-600 hover:bg-red-500 text-white flex items-center gap-2 transition-colors shadow-lg"
                                >
                                    {disablingMFA ? <RefreshCw size={16} className="animate-spin" /> : <ShieldAlert size={16} />}
                                    {disablingMFA ? 'Disabling...' : 'Confirm & Disable 2FA'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* ADMIN RESET 2FA MODAL */}
            {reset2FATargetUser && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
                    <div className="bg-zinc-950 border border-yellow-500/30 rounded-2xl p-6 w-full max-w-md space-y-4 shadow-2xl">
                        <div className="flex justify-between items-center border-b border-white/10 pb-3">
                            <h3 className="text-lg font-pixel text-yellow-400 flex items-center gap-2">
                                <AlertTriangle size={20} /> Reset User 2FA
                            </h3>
                            <button onClick={() => setReset2FATargetUser(null)} className="text-white/50 hover:text-white transition-colors">
                                <XCircle size={20} />
                            </button>
                        </div>
                        <div className="p-3.5 bg-yellow-500/10 border border-yellow-500/30 rounded-xl text-xs font-mono text-yellow-200/90 space-y-2">
                            <p>
                                Are you sure you want to administratively reset Two-Factor Authentication for <strong className="text-white">"{reset2FATargetUser}"</strong>?
                            </p>
                            <ul className="list-disc list-inside space-y-1 text-yellow-100/70 text-[11px] pl-1">
                                <li>Current authenticator secret and backup recovery codes will be erased immediately.</li>
                                <li>All active sessions for this user will be revoked, requiring re-login.</li>
                                <li>The user can log in with their password and pair a new authenticator device.</li>
                            </ul>
                        </div>
                        <div className="flex justify-end gap-3 pt-2">
                            <button
                                type="button"
                                onClick={() => setReset2FATargetUser(null)}
                                className="px-4 py-2 rounded-lg font-mono text-sm bg-white/10 hover:bg-white/20 text-white transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={handleAdminReset2FA}
                                disabled={resetting2FA}
                                className="px-4 py-2 rounded-lg font-mono font-bold text-sm bg-red-600 hover:bg-red-500 text-white flex items-center gap-2 transition-colors shadow-lg"
                            >
                                {resetting2FA ? <RefreshCw size={16} className="animate-spin" /> : <RotateCcw size={16} />}
                                {resetting2FA ? 'Resetting...' : 'Confirm & Reset 2FA'}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Header */}
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-3xl font-pixel text-mc-diamond">Web Manager Users</h1>
                    <p className="text-white/50 font-mono text-sm">Control authentication credentials and manage operator access</p>
                </div>
                <div className="flex items-center gap-3">
                    <button
                        onClick={() => {
                            fetchUsers();
                            fetch2FAStatus();
                        }}
                        className="p-2 bg-white/5 hover:bg-white/10 rounded-full transition-colors"
                        title="Refresh Users"
                    >
                        <RefreshCw size={20} className={loading || mfaLoading ? "animate-spin" : ""} />
                    </button>
                    <button
                        onClick={() => setShowCreateModal(true)}
                        className="bg-green-600 hover:bg-green-500 text-white font-mono font-bold py-2 px-4 rounded-lg flex items-center gap-2 transition-colors shadow-lg text-sm"
                    >
                        <Plus size={16} /> Add User
                    </button>
                </div>
            </div>

            {/* TWO-FACTOR AUTHENTICATION SECURITY CARD (PERSONAL ACCOUNT) */}
            <div className="bg-black/60 border border-white/10 rounded-xl p-6 backdrop-blur-md mb-6">
                <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                    <div className="flex items-start gap-3.5">
                        <div className={`p-2.5 rounded-xl border ${mfaEnabled ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'}`}>
                            {mfaEnabled ? <ShieldCheck size={26} /> : <ShieldAlert size={26} />}
                        </div>
                        <div>
                            <div className="flex items-center gap-2.5">
                                <h3 className="font-pixel text-lg text-white">
                                    Your Account 2FA Security {currentUsername && <span className="text-mc-diamond text-sm font-mono font-normal">(@{currentUsername})</span>}
                                </h3>
                                <span className={`text-[10px] font-mono uppercase px-2 py-0.5 rounded border font-bold ${
                                    mfaEnabled
                                        ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
                                        : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'
                                }`}>
                                    {mfaEnabled ? 'Active' : 'Disabled'}
                                </span>
                            </div>
                            <p className="text-xs font-mono text-white/50 mt-1 max-w-2xl leading-relaxed">
                                Manage time-based two-factor authentication (TOTP) for your individual session. In Lodestone, each administrator and operator maintains their own independent 2FA secret and backup recovery codes.
                            </p>
                        </div>
                    </div>

                    <div className="shrink-0">
                        {mfaEnabled ? (
                            <button
                                onClick={() => {
                                    setDisablePasswordOrCode('');
                                    setShowDisableModal(true);
                                }}
                                className="px-4 py-2 rounded-lg font-mono font-bold text-xs bg-red-950/40 hover:bg-red-900/60 border border-red-500/30 text-red-300 transition-colors"
                            >
                                Disable 2FA
                            </button>
                        ) : (
                            <button
                                onClick={handleStart2FASetup}
                                disabled={mfaLoading}
                                className="px-4 py-2 rounded-lg font-mono font-bold text-xs bg-emerald-600 hover:bg-emerald-500 text-white flex items-center gap-2 shadow-lg transition-colors"
                            >
                                {mfaLoading ? <RefreshCw size={14} className="animate-spin" /> : <ShieldCheck size={14} />}
                                Setup 2FA
                            </button>
                        )}
                    </div>
                </div>
            </div>

            {/* Users List */}
            <div className="bg-black/60 border border-white/10 rounded-xl overflow-hidden backdrop-blur-md">
                <div className="p-4 border-b border-white/10 flex justify-between items-center">
                    <h3 className="font-pixel text-lg text-white flex items-center gap-2">
                        <Shield size={20} className="text-mc-diamond" /> Authorized Web Users ({users.length})
                    </h3>
                </div>

                <div className="divide-y divide-white/5">
                    {users.length > 0 ? (
                        users.map(u => (
                            <div key={u.id} className="p-4 hover:bg-white/5 transition-colors flex justify-between items-center">
                                <div className="flex items-center gap-3">
                                    <div className="w-9 h-9 rounded-lg bg-black/60 border border-white/10 flex items-center justify-center text-mc-diamond font-bold font-mono">
                                        {u.username[0]?.toUpperCase()}
                                    </div>
                                    <div>
                                        <div className="font-mono text-base font-bold text-white flex items-center gap-2">
                                            {u.username}
                                            <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded bg-white/10 text-white/80 border border-white/10">
                                                {u.role}
                                            </span>
                                            {u.username === currentUsername && (
                                                <span className="text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-mc-diamond/20 text-mc-diamond border border-mc-diamond/30">
                                                    You
                                                </span>
                                            )}
                                        </div>
                                        <div className="text-xs font-mono text-white/40">User ID: #{u.id}</div>
                                    </div>
                                </div>

                                <div className="flex items-center gap-4">
                                    {/* 2FA Status Badge */}
                                    <div className="hidden sm:flex items-center">
                                        {u.mfa_enabled ? (
                                            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-mono font-semibold bg-emerald-500/10 border border-emerald-500/30 text-emerald-400">
                                                <ShieldCheck size={14} /> 2FA Active
                                            </span>
                                        ) : (
                                            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-mono text-white/40 bg-white/5 border border-white/10">
                                                <ShieldAlert size={14} /> 2FA Inactive
                                            </span>
                                        )}
                                    </div>

                                    <div className="flex items-center gap-2">
                                        {/* Reset 2FA Button (Admin Break-Glass) */}
                                        {u.mfa_enabled && (
                                            <button
                                                onClick={() => setReset2FATargetUser(u.username)}
                                                title={`Reset 2FA for ${u.username}`}
                                                className="p-2 bg-yellow-500/10 hover:bg-yellow-500/20 text-yellow-300 border border-yellow-500/30 rounded-lg transition-colors flex items-center gap-1.5 text-xs font-mono"
                                            >
                                                <RotateCcw size={15} />
                                                <span className="hidden md:inline">Reset 2FA</span>
                                            </button>
                                        )}

                                        <button
                                            onClick={() => {
                                                setResetTargetUser(u.username);
                                                setResetPassword('');
                                            }}
                                            title="Change Password"
                                            className="p-2 bg-white/5 hover:bg-white/10 text-mc-gold rounded-lg transition-colors"
                                        >
                                            <Key size={16} />
                                        </button>
                                        <button
                                            onClick={() => handleDeleteUser(u.username)}
                                            disabled={users.length <= 1}
                                            title={users.length <= 1 ? "Cannot delete the only remaining user" : "Delete User"}
                                            className={`p-2 rounded-lg transition-colors border ${
                                                users.length <= 1
                                                    ? 'opacity-30 cursor-not-allowed bg-black/20 text-white/30 border-transparent'
                                                    : 'bg-red-950/40 hover:bg-red-900/60 text-red-400 border-red-500/20'
                                            }`}
                                        >
                                            <Trash2 size={16} />
                                        </button>
                                    </div>
                                </div>
                            </div>
                        ))
                    ) : (
                        <div className="p-8 text-center text-white/30 font-mono text-sm">
                            {loading ? "Loading users..." : "No users found."}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
