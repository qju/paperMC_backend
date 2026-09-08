import { useState, useRef, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Lock, User, LogIn, AlertCircle, ShieldCheck, ArrowLeft, KeyRound } from 'lucide-react';

export default function Login() {
    const [step, setStep] = useState<'credentials' | '2fa'>('credentials');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [mfaToken, setMfaToken] = useState('');
    const [twoFactorCode, setTwoFactorCode] = useState('');
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);

    const codeInputRef = useRef<HTMLInputElement>(null);
    const navigate = useNavigate();

    useEffect(() => {
        if (step === '2fa' && codeInputRef.current) {
            codeInputRef.current.focus();
        }
    }, [step]);

    const handleCredentialsLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        setError('');

        try {
            const res = await fetch('/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username: username.trim(), password }),
            });

            const data = await res.json().catch(() => ({}));

            if (!res.ok) {
                if (res.status === 429) {
                    throw new Error('Too many failed login attempts. Please wait before trying again.');
                }
                throw new Error(data.error || 'Invalid username or password');
            }

            // Check if 2FA challenge is required
            if (data.mfa_required) {
                setMfaToken(data.mfa_token);
                setStep('2fa');
                setTwoFactorCode('');
                setError('');
                return;
            }

            if (data.token) {
                // Standard login success without 2FA
                localStorage.setItem('token', data.token);
                navigate('/');
                return;
            }

            throw new Error('Unexpected response from authentication server');

        } catch (err: any) {
            setError(err.message || 'Login failed');
        } finally {
            setLoading(false);
        }
    };

    const handleVerify2FA = async (e: React.FormEvent) => {
        e.preventDefault();
        const code = twoFactorCode.trim();
        if (!code) {
            setError('Please enter your 6-digit code or backup recovery code');
            return;
        }

        setLoading(true);
        setError('');

        try {
            const res = await fetch('/api/auth/2fa/verify-login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    mfa_token: mfaToken,
                    code: code,
                }),
            });

            const data = await res.json().catch(() => ({}));

            if (!res.ok) {
                throw new Error(data.error || 'Invalid verification code. Please check your authenticator or backup code.');
            }

            if (data.token) {
                localStorage.setItem('token', data.token);
                navigate('/');
                return;
            }

            throw new Error('Unexpected response verifying two-factor authentication');

        } catch (err: any) {
            setError(err.message || 'Two-factor verification failed');
        } finally {
            setLoading(false);
        }
    };

    const handleBackToCredentials = () => {
        setStep('credentials');
        setMfaToken('');
        setTwoFactorCode('');
        setError('');
    };

    return (
        <div className="min-h-screen flex items-center justify-center p-4 bg-dirt-pattern">
            <div className="w-full max-w-md bg-black/85 backdrop-blur-xl border border-white/10 rounded-2xl shadow-2xl p-8 space-y-6">
                <div className="text-center space-y-2">
                    <h1 className="font-pixel text-4xl text-mc-diamond tracking-wider drop-shadow-md">
                        Lodestone
                    </h1>
                    <p className="text-xs text-white/50 font-mono">
                        Minecraft Server Manager v2.0
                    </p>
                </div>

                {step === 'credentials' ? (
                    <form onSubmit={handleCredentialsLogin} className="space-y-4">
                        {/* Username Input */}
                        <div className="space-y-1.5">
                            <label className="block text-xs uppercase tracking-wider text-white/60 font-mono flex items-center gap-1.5">
                                <User size={14} className="text-mc-diamond" /> Username
                            </label>
                            <input
                                type="text"
                                value={username}
                                onChange={(e) => setUsername(e.target.value)}
                                disabled={loading}
                                required
                                placeholder="Enter username (e.g. admin)"
                                className="w-full bg-black/60 border border-white/20 rounded-lg p-3 text-white font-mono text-sm placeholder:text-white/30 focus:border-mc-diamond focus:bg-black/90 focus:outline-none transition-colors"
                            />
                        </div>

                        {/* Password Input */}
                        <div className="space-y-1.5">
                            <label className="block text-xs uppercase tracking-wider text-white/60 font-mono flex items-center gap-1.5">
                                <Lock size={14} className="text-mc-gold" /> Password
                            </label>
                            <input
                                type="password"
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                disabled={loading}
                                required
                                placeholder="••••••••••••"
                                className="w-full bg-black/60 border border-white/20 rounded-lg p-3 text-white font-mono text-sm placeholder:text-white/30 focus:border-mc-gold focus:bg-black/90 focus:outline-none transition-colors"
                            />
                        </div>

                        {/* Error Banner */}
                        {error && (
                            <div className="flex items-center gap-2 p-3 bg-red-900/40 border border-red-500/50 rounded-lg text-red-200 text-xs font-mono">
                                <AlertCircle size={16} className="text-red-400 shrink-0" />
                                <span>{error}</span>
                            </div>
                        )}

                        {/* Submit Button */}
                        <button
                            type="submit"
                            disabled={loading}
                            className={`
                                w-full flex items-center justify-center gap-2 py-3.5 px-4 rounded-lg font-mono font-bold text-sm tracking-wide transition-all shadow-lg mt-2
                                ${loading
                                    ? 'bg-white/10 text-white/30 cursor-not-allowed'
                                    : 'bg-green-600 hover:bg-green-500 text-white active:scale-[0.99]'}
                            `}
                        >
                            <LogIn size={18} />
                            {loading ? 'Authenticating...' : 'Enter Console >'}
                        </button>
                    </form>
                ) : (
                    <form onSubmit={handleVerify2FA} className="space-y-5 animate-in fade-in zoom-in-95 duration-200">
                        <div className="p-4 bg-emerald-950/40 border border-emerald-500/30 rounded-xl text-center space-y-2">
                            <div className="inline-flex p-2.5 bg-emerald-500/10 rounded-full border border-emerald-500/20 text-emerald-400 mb-1">
                                <ShieldCheck size={26} />
                            </div>
                            <h2 className="font-mono font-bold text-sm text-white">
                                Two-Factor Authentication
                            </h2>
                            <p className="text-xs text-white/60 font-mono leading-relaxed">
                                Enter the 6-digit code from your authenticator app, or an 8-character backup recovery code.
                            </p>
                        </div>

                        {/* 2FA Code Input */}
                        <div className="space-y-1.5">
                            <label className="block text-xs uppercase tracking-wider text-white/60 font-mono flex items-center gap-1.5">
                                <KeyRound size={14} className="text-mc-diamond" /> Verification Code
                            </label>
                            <input
                                ref={codeInputRef}
                                type="text"
                                value={twoFactorCode}
                                onChange={(e) => setTwoFactorCode(e.target.value)}
                                disabled={loading}
                                required
                                maxLength={19}
                                placeholder="000000 or XXXX-XXXX"
                                className="w-full bg-black/60 border border-white/20 rounded-lg p-3 text-center text-white font-mono text-lg tracking-widest placeholder:tracking-normal placeholder:text-white/20 focus:border-mc-diamond focus:bg-black/90 focus:outline-none transition-colors uppercase"
                            />
                        </div>

                        {/* Error Banner */}
                        {error && (
                            <div className="flex items-center gap-2 p-3 bg-red-900/40 border border-red-500/50 rounded-lg text-red-200 text-xs font-mono">
                                <AlertCircle size={16} className="text-red-400 shrink-0" />
                                <span>{error}</span>
                            </div>
                        )}

                        {/* Verify Button */}
                        <button
                            type="submit"
                            disabled={loading}
                            className={`
                                w-full flex items-center justify-center gap-2 py-3.5 px-4 rounded-lg font-mono font-bold text-sm tracking-wide transition-all shadow-lg
                                ${loading
                                    ? 'bg-white/10 text-white/30 cursor-not-allowed'
                                    : 'bg-emerald-600 hover:bg-emerald-500 text-white active:scale-[0.99]'}
                            `}
                        >
                            <ShieldCheck size={18} />
                            {loading ? 'Verifying Code...' : 'Verify & Sign In >'}
                        </button>

                        {/* Back to credentials button */}
                        <button
                            type="button"
                            onClick={handleBackToCredentials}
                            disabled={loading}
                            className="w-full flex items-center justify-center gap-1.5 py-2 text-xs font-mono text-white/50 hover:text-white transition-colors"
                        >
                            <ArrowLeft size={14} /> Back to username & password
                        </button>
                    </form>
                )}

                <div className="text-center pt-2 border-t border-white/5">
                    <span className="text-[11px] text-white/30 font-mono">
                        Protected by RFC 6238 Multi-Factor Authentication
                    </span>
                </div>
            </div>
        </div>
    );
}
