import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Lock, User, LogIn, AlertCircle } from 'lucide-react';

export default function Login() {
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);

    const navigate = useNavigate();

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        setError('');

        try {
            const res = await fetch('/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password }),
            });

            if (!res.ok) {
                throw new Error('Invalid username or password');
            }

            const data = await res.json();
            // Save Token
            localStorage.setItem('token', data.token);
            // Redirect to Dashboard
            navigate('/');

        } catch (err: any) {
            setError(err.message || 'Login failed');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center p-4 bg-dirt-pattern">
            <div className="w-full max-w-md bg-black/85 backdrop-blur-xl border border-white/10 rounded-2xl shadow-2xl p-8 space-y-6">
                <div className="text-center space-y-2">
                    <h1 className="font-pixel text-4xl text-mc-diamond tracking-wider drop-shadow-md">
                        PaperMC
                    </h1>
                    <p className="text-xs text-white/50 font-mono">
                        Minecraft Server Manager v2.0
                    </p>
                </div>

                <form onSubmit={handleLogin} className="space-y-4">
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

                <div className="text-center pt-2 border-t border-white/5">
                    <span className="text-[11px] text-white/30 font-mono">
                        Protected by JWT Authentication
                    </span>
                </div>
            </div>
        </div>
    );
}
