import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { Terminal, Users, Settings, LogOut, HardDrive, Menu, Activity, Cpu, Zap } from 'lucide-react';
import { useState, useEffect, useCallback } from 'react';

// 1. Define the Shape of the API Response
interface Vitals {
    status: string;
    cpu: number;
    ram: number;        // in Bytes (RSS)
    total_memory: string; // e.g., "4G", "1024M"
    players: number;
}

export default function DashboardLayout() {
    const navigate = useNavigate();
    const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

    // 2. State for Vitals
    const [vitals, setVitals] = useState<Vitals | null>(null);
    const [isPolling] = useState(true);

    const handleLogout = useCallback(() => {
        localStorage.removeItem('token');
        navigate('/login');
    }, [navigate]);

    // 3. The Polling Hook
    useEffect(() => {
        if (!isPolling) return;

        const fetchVitals = async () => {
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
                console.error("Failed to fetch vitals", err);
            }
        };

        // Initial fetch
        fetchVitals();

        // Poll every 1 second
        const interval = setInterval(fetchVitals, 10000);
        return () => clearInterval(interval);
    }, [isPolling, navigate, handleLogout]);

    // 4. Helper to parse RAM string (e.g. "4G" -> 4) for the progress bar
    const parseMaxRam = (ramStr: string): number => {
        if (!ramStr) return 1;
        const value = parseInt(ramStr.slice(0, -1));
        const unit = ramStr.slice(-1).toUpperCase();
        if (unit === 'G') return value * 1024 * 1024 * 1024;
        if (unit === 'M') return value * 1024 * 1024;
        return value;
    };

    // Helper to format Bytes to GB
    const formatBytes = (bytes: number) => (bytes / 1024 / 1024 / 1024).toFixed(1);

    // Calculate Percentages
    const maxRamBytes = vitals ? parseMaxRam(vitals.total_memory) : 1;
    const ramPercent = vitals ? Math.min((vitals.ram / maxRamBytes) * 100, 100) : 0;
    const cpuPercent = vitals ? Math.min(vitals.cpu, 100) : 0;
    const isOnline = vitals?.status === "Running";

    return (
        <div className="flex h-screen overflow-hidden bg-dirt-pattern text-white">

            {/* MOBILE HEADER */}
            <div className="md:hidden fixed top-0 w-full h-16 bg-black/90 border-b border-white/10 z-50 flex items-center justify-between px-4">
                <span className="font-pixel text-2xl text-mc-diamond">PaperMC</span>
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
                        PaperMC
                    </h1>
                    <p className="text-xs text-white/50 font-mono mt-1">Manager v2.0</p>
                </div>

                <nav className="flex-1 p-4 space-y-2 mt-16 md:mt-0">
                    <NavItem to="/" icon={<Terminal size={20} />} label="Console" onClick={() => setMobileMenuOpen(false)} />
                    <NavItem to="/players" icon={<Users size={20} />} label="Players" onClick={() => setMobileMenuOpen(false)} />
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
            <main className="flex-1 overflow-auto relative pt-16 md:pt-0 flex flex-col">
                <div className="flex-1 p-4 md:p-6 overflow-hidden">
                    <Outlet />
                </div>
            </main>

            {/* RIGHT SIDEBAR (Live Vitals) */}
            <aside className="hidden xl:flex w-80 bg-black/60 backdrop-blur-md border-l border-white/10 flex-col p-6 gap-6">

                <div className="flex items-center gap-2 mb-2 text-mc-gold font-pixel text-xl">
                    <Activity size={20} />
                    <span>Server Vitals</span>
                </div>

                {/* STATUS */}
                <div className="bg-black/40 border border-white/10 p-4 rounded-lg">
                    <div className="text-xs text-white/50 mb-1 uppercase tracking-wider">Status</div>
                    <div className="flex items-center gap-2">
                        <div className={`w-3 h-3 rounded-full shadow-[0_0_8px] transition-colors ${isOnline ? 'bg-mc-green shadow-[#55aa55]' : 'bg-red-500 shadow-red-500'}`}></div>
                        <span className={`font-mono text-lg ${isOnline ? 'text-mc-green' : 'text-red-400'}`}>
                            {vitals?.status || "Offline"}
                        </span>
                    </div>
                </div>

                {/* RAM */}
                <div className="bg-black/40 border border-white/10 p-4 rounded-lg">
                    <div className="flex justify-between items-center mb-2">
                        <div className="flex items-center gap-2 text-xs text-white/50 uppercase tracking-wider">
                            <Zap size={14} /> RAM (RSS)
                        </div>
                        <span className="text-xs font-mono text-mc-diamond">
                            {vitals ? formatBytes(vitals.ram) : 0} GB / {vitals?.total_memory || "?"}
                        </span>
                    </div>
                    <div className="w-full h-2 bg-white/10 rounded-full overflow-hidden">
                        <div
                            className="h-full bg-mc-diamond shadow-[0_0_10px_#55ffff] transition-all duration-500"
                            style={{ width: `${ramPercent}%` }}
                        ></div>
                    </div>
                </div>

                {/* CPU */}
                <div className="bg-black/40 border border-white/10 p-4 rounded-lg">
                    <div className="flex justify-between items-center mb-2">
                        <div className="flex items-center gap-2 text-xs text-white/50 uppercase tracking-wider">
                            <Cpu size={14} /> CPU Load
                        </div>
                        <span className="text-xs font-mono text-mc-gold">
                            {vitals?.cpu.toFixed(1) || 0}%
                        </span>
                    </div>
                    <div className="w-full h-2 bg-white/10 rounded-full overflow-hidden">
                        <div
                            className="h-full bg-mc-gold shadow-[0_0_10px_#ffaa00] transition-all duration-500"
                            style={{ width: `${cpuPercent}%` }}
                        ></div>
                    </div>
                </div>

                {/* PLAYERS (Placeholder for Milestone 3.1) */}
                <div className="flex-1 bg-black/40 border border-white/10 p-4 rounded-lg flex flex-col">
                    <div className="text-xs text-white/50 mb-3 uppercase tracking-wider flex justify-between">
                        <span>Online Players</span>
                        <span>{vitals?.players || 0}</span>
                    </div>
                    <div className="text-white/30 text-xs italic text-center mt-4">
                        Player list coming in Milestone 3.1
                    </div>
                </div>

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
