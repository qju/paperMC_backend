import { useState } from 'react';
import { useNavigate } from 'react-router-dom';

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
                throw new Error('Invalid Credentials');
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
        <div style={{ display: 'flex', justifyContent: 'center', width: '100%' }}>
            <div style={{
                border: '4px solid #000',
                padding: '30px',
                backgroundColor: '#c6c6c6',
                width: '350px',
                boxShadow: '8px 8px 0 rgba(0,0,0,0.5)'
            }}>
                <h1 style={{
                    marginTop: 0,
                    textAlign: 'center',
                    color: '#333',
                    fontSize: '2.5rem',
                    textShadow: '2px 2px 0 #fff'
                }}>
                    PaperMC Manager
                </h1>

                <form onSubmit={handleLogin} style={{ display: 'flex', flexDirection: 'column', gap: '15px' }}>
                    <div style={{ display: 'flex', flexDirection: 'column' }}>
                        <label style={{ color: '#333' }}>Username</label>
                        <input
                            type="text"
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            disabled={loading}
                        />
                    </div>

                    <div style={{ display: 'flex', flexDirection: 'column' }}>
                        <label style={{ color: '#333' }}>Password</label>
                        <input
                            type="password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            disabled={loading}
                        />
                    </div>

                    {error && (
                        <div style={{ color: '#aa0000', textAlign: 'center', fontWeight: 'bold' }}>
                            {error}
                        </div>
                    )}

                    <button
                        type="submit"
                        style={{ backgroundColor: loading ? '#555' : '#55aa55', marginTop: '10px' }}
                        disabled={loading}
                    >
                        {loading ? 'Authenticating...' : 'Enter Console >'}
                    </button>
                </form>
            </div>
        </div>
    );
}
