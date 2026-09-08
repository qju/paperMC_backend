import { useEffect, useState } from 'react';
import { Shield, Plus, Key, Trash2, RefreshCw, CheckCircle, XCircle, ShieldCheck, ShieldAlert, KeyRound, Copy, Check } from 'lucide-react';

interface User {
    id: number;
    username: string;
    role: string;
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

    // Two-Factor Authentication (2FA) State
    const [mfaEnabled, setMfaEnabled] = useState(false);
    const [mfaLoading, setMfaLoading] = useState(false);
    const [showSetupModal, setShowSetupModal] = useState(false);
    const [mfaSecret, setMfaSecret] = useState('');
    const [mfaAuthURL, setMfaAuthURL] = useState('');
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

            {/* 2FA SETUP MODAL */}
            {showSetupModal && (
                <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4 overflow-y-auto">
                    <div className="bg-black/90 border border-white/20 rounded-xl p-6 w-full max-w-lg space-y-5 shadow-2xl my-8">
                        <div className="flex justify-between items-center border-b border-white/10 pb-3">
                            <h3 className="text-lg font-pixel text-mc-diamond flex items-center gap-2">
                                <ShieldCheck size={20} /> Setup Two-Factor Authentication
                            </h3>
                            <button onClick={() => setShowSetupModal(false)} className="text-white/50 hover:text-white">
                                <XCircle size={20} />
                            </button>
                        </div>

                        {/* Step 1: Secret Key */}
                        <div className="space-y-2">
                            <label className="block text-xs uppercase font-mono text-white/70 font-bold">
                                Step 1: Add Key to Authenticator App
                            </label>
                            <p className="text-xs font-mono text-white/50">
                                Open Google Authenticator, 1Password, or Authy, choose <em>Add Account &gt; Enter key manually</em>:
                            </p>
                            <div className="flex items-center gap-2 p-2.5 bg-black/60 border border-white/10 rounded-lg">
                                <code className="flex-1 font-mono text-sm tracking-wider text-mc-gold select-all break-all">
                                    {mfaSecret}
                                </code>
                                <button
                                    type="button"
                                    onClick={() => {
                                        navigator.clipboard.writeText(mfaSecret);
                                        setCopiedSecret(true);
                                        setTimeout(() => setCopiedSecret(false), 2000);
                                    }}
                                    className="px-2.5 py-1.5 rounded text-xs font-mono bg-white/10 hover:bg-white/20 text-white flex items-center gap-1.5 transition-colors"
                                    title="Copy Secret"
                                >
                                    {copiedSecret ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
                                    {copiedSecret ? "Copied" : "Copy"}
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

                        {/* Step 2: Backup Recovery Codes */}
                        <div className="space-y-2">
                            <div className="flex justify-between items-center">
                                <label className="block text-xs uppercase font-mono text-white/70 font-bold">
                                    Step 2: Save Emergency Recovery Codes
                                </label>
                                <button
                                    type="button"
                                    onClick={() => {
                                        navigator.clipboard.writeText(mfaBackupCodes.join('\n'));
                                        setCopiedBackupCodes(true);
                                        setTimeout(() => setCopiedBackupCodes(false), 2000);
                                    }}
                                    className="text-[11px] font-mono text-mc-diamond hover:underline flex items-center gap-1"
                                >
                                    {copiedBackupCodes ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} />}
                                    {copiedBackupCodes ? "Codes Copied" : "Copy All Codes"}
                                </button>
                            </div>
                            <p className="text-xs font-mono text-white/50">
                                Store these single-use codes safely. Each code can be used once if you lose access to your device.
                            </p>
                            <div className="grid grid-cols-2 sm:grid-cols-4 gap-1.5 p-3 bg-black/60 border border-white/10 rounded-lg">
                                {mfaBackupCodes.map((code, idx) => (
                                    <div key={idx} className="font-mono text-xs text-white/80 bg-white/5 px-2 py-1 rounded text-center select-all">
                                        {code}
                                    </div>
                                ))}
                            </div>
                        </div>

                        {/* Step 3: Confirmation Code */}
                        <form onSubmit={handleConfirm2FA} className="space-y-4 pt-2 border-t border-white/10">
                            <div>
                                <label className="block text-xs uppercase font-mono text-white/70 font-bold mb-1.5 flex items-center gap-1.5">
                                    <KeyRound size={14} className="text-mc-diamond" /> Step 3: Enter 6-Digit Code to Activate
                                </label>
                                <input
                                    type="text"
                                    value={mfaVerifyCode}
                                    onChange={(e) => setMfaVerifyCode(e.target.value)}
                                    placeholder="000000"
                                    maxLength={6}
                                    required
                                    className="w-full bg-black/60 border border-white/20 rounded-lg p-3 text-center text-white font-mono text-lg tracking-widest focus:border-mc-diamond focus:outline-none"
                                />
                            </div>

                            <div className="flex justify-end gap-3 pt-1">
                                <button
                                    type="button"
                                    onClick={() => setShowSetupModal(false)}
                                    className="px-4 py-2 rounded font-mono text-sm bg-white/10 hover:bg-white/20 text-white"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={activatingMFA || mfaVerifyCode.trim().length !== 6}
                                    className="px-5 py-2 rounded font-mono font-bold text-sm bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white flex items-center gap-2 transition-colors shadow-lg"
                                >
                                    {activatingMFA ? <RefreshCw size={16} className="animate-spin" /> : <ShieldCheck size={16} />}
                                    {activatingMFA ? 'Verifying...' : 'Activate 2FA'}
                                </button>
                            </div>
                        </form>
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

            {/* TWO-FACTOR AUTHENTICATION SECURITY CARD */}
            <div className="bg-black/60 border border-white/10 rounded-xl p-6 backdrop-blur-md mb-6">
                <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                    <div className="flex items-start gap-3.5">
                        <div className={`p-2.5 rounded-xl border ${mfaEnabled ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400' : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'}`}>
                            {mfaEnabled ? <ShieldCheck size={26} /> : <ShieldAlert size={26} />}
                        </div>
                        <div>
                            <div className="flex items-center gap-2.5">
                                <h3 className="font-pixel text-lg text-white">Two-Factor Authentication (2FA)</h3>
                                <span className={`text-[10px] font-mono uppercase px-2 py-0.5 rounded border font-bold ${
                                    mfaEnabled
                                        ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
                                        : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'
                                }`}>
                                    {mfaEnabled ? 'Active' : 'Disabled'}
                                </span>
                            </div>
                            <p className="text-xs font-mono text-white/50 mt-1 max-w-2xl leading-relaxed">
                                Protect your administrator account with RFC 6238 time-based one-time passwords (TOTP) and 8 emergency backup recovery codes. Works with Google Authenticator, 1Password, and Authy.
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
                                        </div>
                                        <div className="text-xs font-mono text-white/40">User ID: #{u.id}</div>
                                    </div>
                                </div>

                                <div className="flex items-center gap-2">
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
