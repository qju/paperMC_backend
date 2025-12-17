import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { Terminal, Users, Settings, LogOut, HardDrive, Menu, Activity, Cpu, Zap } from 'lucide-react';
import { useState } from 'react';

export default function DashboardLayout() {
    const navigate = useNavigate();
    const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

    const handleLogout = () => {
        localStorage.removeItem('token');
        navigate('/login');
    };

    return (
        <div className="flex h-screen overflow-hidden bg-dirt-pattern text-white">

            {/* MOBILE HEADER */}
            <div className="md:hidden fixed top-0 w-full h-16 bg-black/90 border-b border-white/10 z-50 flex items-center justify-between px-4">
                <span className="font-pixel text-2xl text-mc-diamond">PaperMC</span>
                <button onClick={() => setMobileMenuOpen(!mobileMenuOpen)} className="text-white">
                    <Menu />
                </button>
            </div>

            {/* LEFT SIDEBAR (Navigation) */}
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

            {/* MAIN CONTENT AREA */}
            <main className="flex-1 overflow-auto relative pt-16 md:pt-0 flex flex-col">
                <div className="flex-1 p-4 md:p-6 overflow-hidden">
                    <Outlet />
                </div>
            </main>

            {/* RIGHT SIDEBAR (Status Panel) */}
            {/* Hidden on Tablet/Mobile (xl:flex means only visible on extra large screens) */}
            <aside className="hidden xl:flex w-80 bg-black/60 backdrop-blur-md border-l border-white/10 flex-col p-6 gap-6">

                <div className="flex items-center gap-2 mb-2 text-mc-gold font-pixel text-xl">
                    <Activity size={20} />
                    <span>Server Vitals</span>
                </div>

                {/* Vital Card: Status */}
                <div className="bg-black/40 border border-white/10 p-4 rounded-lg">
                    <div className="text-xs text-white/50 mb-1 uppercase tracking-wider">Status</div>
                    <div className="flex items-center gap-2">
                        <div className="w-3 h-3 rounded-full bg-mc-green shadow-[0_0_8px_#55aa55]"></div>
                        <span className="font-mono text-lg text-mc-green">Online</span>
                    </div>
                </div>

                {/* Vital Card: RAM */}
                <div className="bg-black/40 border border-white/10 p-4 rounded-lg">
                    <div className="flex justify-between items-center mb-2">
                        <div className="flex items-center gap-2 text-xs text-white/50 uppercase tracking-wider">
                            <Zap size={14} /> RAM Usage
                        </div>
                        <span className="text-xs font-mono text-mc-diamond">2.4 GB / 8 GB</span>
                    </div>
                    {/* Progress Bar */}
                    <div className="w-full h-2 bg-white/10 rounded-full overflow-hidden">
                        <div className="h-full bg-mc-diamond w-[30%] shadow-[0_0_10px_#55ffff]"></div>
                    </div>
                </div>

                {/* Vital Card: CPU */}
                <div className="bg-black/40 border border-white/10 p-4 rounded-lg">
                    <div className="flex justify-between items-center mb-2">
                        <div className="flex items-center gap-2 text-xs text-white/50 uppercase tracking-wider">
                            <Cpu size={14} /> CPU Load
                        </div>
                        <span className="text-xs font-mono text-mc-gold">12%</span>
                    </div>
                    {/* Progress Bar */}
                    <div className="w-full h-2 bg-white/10 rounded-full overflow-hidden">
                        <div className="h-full bg-mc-gold w-[12%] shadow-[0_0_10px_#ffaa00]"></div>
                    </div>
                </div>

                {/* Mini Player List */}
                <div className="flex-1 bg-black/40 border border-white/10 p-4 rounded-lg flex flex-col">
                    <div className="text-xs text-white/50 mb-3 uppercase tracking-wider flex justify-between">
                        <span>Online Players</span>
                        <span>3 / 20</span>
                    </div>

                    <ul className="space-y-2 overflow-y-auto pr-2 custom-scrollbar">
                        {['Notch', 'Jeb_', 'Dinnerbone'].map(user => (
                            <li key={user} className="flex items-center gap-3 p-2 hover:bg-white/5 rounded transition-colors cursor-pointer">
                                {/* Steve Head Avatar Placeholder */}
                                <img
                                    src={`https://api.mineatar.io/face/${user}`}
                                    alt={user}
                                    className="w-6 h-6 rounded-sm pixelated"
                                />
                                <span className="font-mono text-sm text-white/80">{user}</span>
                            </li>
                        ))}
                    </ul>
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

// Helper for Links (unchanged)
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

