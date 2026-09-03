import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { Terminal, Users as UsersIcon, Settings, LogOut, HardDrive, Menu, Globe, DownloadCloud, Shield } from 'lucide-react';
import { useState, useEffect, useCallback } from 'react';
import { useSocket } from '../hooks/useSocket';
import VitalsPanel from '../components/VitalsPanel';
import type { Vitals } from '../types';

export default function DashboardLayout() {
    const navigate = useNavigate();
    const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

    const { liveVitals } = useSocket();
    const [vitals, setVitals] = useState<Vitals | null>(null);

    const handleLogout = useCallback(() => {
        localStorage.removeItem('token');
        navigate('/login');
    }, [navigate]);

    // Initial Fetch for instantaneous stats before WebSocket streams
    useEffect(() => {
        const fetchInitialVitals = async () => {
            const token = localStorage.getItem('token');
            if (!token) {
                navigate('/login');
                return;
            }

            try {
                const res = await fetch('/status', {
                    headers: { 'Authorization': `Bearer ${token}` }
                });

                if (res.status === 401) {
                    handleLogout();
                    return;
                }

                if (res.ok) {
                    const data = await res.json();
                    setVitals(data);
                }
            } catch (err) {
                console.error("Failed to fetch initial vitals", err);
            }
        };

        fetchInitialVitals();
    }, [navigate, handleLogout]);

    // Sync live vitals whenever WebSocket broadcasts a vitals packet
    useEffect(() => {
        if (liveVitals) {
            setVitals(liveVitals);
        }
    }, [liveVitals]);

    return (
        <div className="flex h-screen overflow-hidden bg-dirt-pattern text-white">

            {/* MOBILE HEADER */}
            <div className="md:hidden fixed top-0 w-full h-16 bg-black/90 border-b border-white/10 z-50 flex items-center justify-between px-4">
                <span className="font-pixel text-2xl text-mc-diamond">Lodestone</span>
                <button onClick={() => setMobileMenuOpen(!mobileMenuOpen)} className="text-white">
                    <Menu />
                </button>
            </div>

            {/* LEFT SIDEBAR */}
            <aside className={`
        fixed md:static inset-y-0 left-0 z-40 w-64 transform transition-transform duration-300 ease-in-out
        bg-black/80 backdrop-blur-xl border-r border-white/10 flex flex-col
        ${mobileMenuOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}
      `}>
                <div className="p-6 border-b border-white/10 hidden md:block">
                    <h1 className="font-pixel text-3xl text-mc-diamond tracking-wider drop-shadow-md">
                        Lodestone
                    </h1>
                    <p className="text-xs text-white/50 font-mono mt-1">Manager v2.0</p>
                </div>

                <nav className="flex-1 p-4 space-y-2 mt-16 md:mt-0">
                    <NavItem to="/" icon={<Terminal size={20} />} label="Console" onClick={() => setMobileMenuOpen(false)} />
                    <NavItem to="/players" icon={<UsersIcon size={20} />} label="Players" onClick={() => setMobileMenuOpen(false)} />
                    <NavItem to="/worlds" icon={<Globe size={20} />} label="Worlds" onClick={() => setMobileMenuOpen(false)} />
                    <NavItem to="/updater" icon={<DownloadCloud size={20} />} label="Updates & Versions" onClick={() => setMobileMenuOpen(false)} />
                    <NavItem to="/users" icon={<Shield size={20} />} label="Web Users" onClick={() => setMobileMenuOpen(false)} />
                    <NavItem to="/config" icon={<Settings size={20} />} label="Server Config" onClick={() => setMobileMenuOpen(false)} />
                    <NavItem to="/backups" icon={<HardDrive size={20} />} label="Backups" onClick={() => setMobileMenuOpen(false)} />
                </nav>

                <div className="p-4 border-t border-white/10">
                    <button
                        onClick={handleLogout}
                        className="flex items-center gap-3 w-full px-4 py-3 text-red-400 hover:bg-red-900/20 rounded-md transition-colors font-mono text-sm uppercase tracking-wide"
                    >
                        <LogOut size={18} />
                        Logout
                    </button>
                </div>
            </aside>

            {/* MAIN CONTENT */}
            <main className="flex-1 overflow-y-auto custom-scrollbar relative pt-16 md:pt-0 flex flex-col min-h-0">
                <div className="flex-1 p-4 md:p-6 flex flex-col min-h-0">
                    <Outlet />
                </div>
            </main>

            {/* RIGHT SIDEBAR (Live Modern Vitals) */}
            <aside className="hidden xl:flex w-84 bg-black/60 backdrop-blur-md border-l border-white/10 flex-col p-5 overflow-hidden">
                <VitalsPanel vitals={vitals} />
            </aside>

            {/* Mobile Overlay */}
            {mobileMenuOpen && (
                <div
                    className="fixed inset-0 bg-black/50 z-30 md:hidden"
                    onClick={() => setMobileMenuOpen(false)}
                />
            )}
        </div>
    );
}

function NavItem({ to, icon, label, onClick }: { to: string, icon: React.ReactNode, label: string, onClick?: () => void }) {
    return (
        <NavLink
            to={to}
            onClick={onClick}
            className={({ isActive }) => `
        flex items-center gap-3 px-4 py-3 rounded-md transition-all font-mono text-sm border
        ${isActive
                    ? 'bg-mc-green/20 text-mc-green border-mc-green/30 shadow-[0_0_10px_rgba(85,170,85,0.2)]'
                    : 'border-transparent text-white/70 hover:bg-white/5 hover:text-white'}
      `}
        >
            {icon}
            <span>{label}</span>
        </NavLink>
    );
}
