import { useEffect, useState } from 'react';
import { RefreshCw, DownloadCloud, CheckCircle, XCircle, ArrowUpCircle, ShieldCheck, HardDrive, Tag } from 'lucide-react';

interface VersionGroup {
    group: string;
    versions: string[];
}

interface ProjectVersionsResponse {
    project: string;
    version_groups: VersionGroup[];
}

interface CheckUpdateResponse {
    project: string;
    version: string;
    latest_build: number;
    latest_hash: string;
    current_hash: string;
    update_available: boolean;
    channel: string;
    file_name: string;
    size: number;
}

interface Toast {
    id: number;
    message: string;
    type: 'success' | 'error';
}

export default function Updater() {
    const [versionGroups, setVersionGroups] = useState<VersionGroup[]>([]);
    const [selectedGroup, setSelectedGroup] = useState<string>('');
    const [selectedVersion, setSelectedVersion] = useState<string>('');
    const [buildInfo, setBuildInfo] = useState<CheckUpdateResponse | null>(null);

    const [loadingVersions, setLoadingVersions] = useState(true);
    const [checkingBuild, setCheckingBuild] = useState(false);
    const [updating, setUpdating] = useState(false);
    const [toasts, setToasts] = useState<Toast[]>([]);

    const showToast = (message: string, type: 'success' | 'error') => {
        const id = Date.now();
        setToasts(prev => [...prev, { id, message, type }]);
        setTimeout(() => {
            setToasts(prev => prev.filter(t => t.id !== id));
        }, 4000);
    };

    // 1. Fetch available major version groups
    useEffect(() => {
        const fetchVersions = async () => {
            const token = localStorage.getItem('token');
            try {
                const res = await fetch('/api/updater/versions', {
                    headers: { 'Authorization': `Bearer ${token}` }
                });
                if (res.ok) {
                    const data: ProjectVersionsResponse = await res.json();
                    setVersionGroups(data.version_groups || []);
                    if (data.version_groups && data.version_groups.length > 0) {
                        const firstGroup = data.version_groups[0];
                        setSelectedGroup(firstGroup.group);
                        setSelectedVersion(firstGroup.versions[0] || firstGroup.group);
                    }
                }
            } catch (err) {
                console.error("Failed to fetch versions", err);
            } finally {
                setLoadingVersions(false);
            }
        };
        fetchVersions();
    }, []);

    // 2. When selected group changes, default to its newest specific version
    const handleGroupChange = (groupName: string) => {
        setSelectedGroup(groupName);
        const group = versionGroups.find(g => g.group === groupName);
        if (group && group.versions.length > 0) {
            setSelectedVersion(group.versions[0]);
        } else {
            setSelectedVersion(groupName);
        }
    };

    // 3. Fetch build info and compare hashes whenever selected version changes
    useEffect(() => {
        if (!selectedVersion) return;

        const checkVersionBuild = async () => {
            setCheckingBuild(true);
            const token = localStorage.getItem('token');
            try {
                const res = await fetch(`/api/updater/check?version=${selectedVersion}`, {
                    headers: { 'Authorization': `Bearer ${token}` }
                });
                if (res.ok) {
                    const data: CheckUpdateResponse = await res.json();
                    setBuildInfo(data);
                } else {
                    setBuildInfo(null);
                }
            } catch (err) {
                console.error("Failed to check build", err);
                setBuildInfo(null);
            } finally {
                setCheckingBuild(false);
            }
        };

        checkVersionBuild();
    }, [selectedVersion]);

    // 4. Trigger download and atomic update
    const handleApplyUpdate = async () => {
        if (!selectedVersion) return;

        if (!confirm(`Are you sure you want to install PaperMC version ${selectedVersion}? If running, the server will restart automatically.`)) {
            return;
        }

        setUpdating(true);
        const token = localStorage.getItem('token');
        try {
            const res = await fetch('/api/updater/apply', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ version: selectedVersion })
            });

            const data = await res.json();
            if (res.ok) {
                showToast(`Successfully installed PaperMC ${selectedVersion} (Build ${data.build_id})`, 'success');
                // Refresh build check info
                const checkRes = await fetch(`/api/updater/check?version=${selectedVersion}`, {
                    headers: { 'Authorization': `Bearer ${token}` }
                });
                if (checkRes.ok) {
                    setBuildInfo(await checkRes.json());
                }
            } else {
                showToast(data.error || "Update installation failed", 'error');
            }
        } catch (err) {
            console.error("Update request failed", err);
            showToast("Network error: Could not reach server", 'error');
        } finally {
            setUpdating(false);
        }
    };

    const currentGroup = versionGroups.find(g => g.group === selectedGroup);
    const formatBytes = (bytes: number) => (bytes / (1024 * 1024)).toFixed(1) + " MB";

    return (
        <div className="space-y-6 relative min-h-[500px]">
            {/* TOAST NOTIFICATIONS */}
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

            {/* Header */}
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-3xl font-pixel text-mc-diamond">PaperMC Version & Updates</h1>
                    <p className="text-white/50 font-mono text-sm">Select major releases and switch versions via PaperMC Fill v3 API</p>
                </div>
            </div>

            {/* Version Selectors */}
            <div className="bg-black/60 border border-white/10 rounded-lg p-6 backdrop-blur-md">
                <h2 className="text-xl font-pixel text-white mb-4 flex items-center gap-2">
                    <Tag size={22} className="text-mc-diamond" /> Select Target Version
                </h2>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    {/* Major Version Family */}
                    <div>
                        <label className="block text-xs uppercase tracking-wider text-white/50 font-mono mb-2">
                            Major Release Group
                        </label>
                        <select
                            value={selectedGroup}
                            onChange={(e) => handleGroupChange(e.target.value)}
                            disabled={loadingVersions || updating}
                            className="w-full bg-black/50 border border-white/20 rounded p-3 text-white font-mono focus:border-mc-diamond focus:outline-none transition-colors"
                        >
                            {versionGroups.map(g => (
                                <option key={g.group} value={g.group}>
                                    Paper {g.group} {g.group === versionGroups[0]?.group ? '(Latest Release)' : ''}
                                </option>
                            ))}
                        </select>
                    </div>

                    {/* Specific Sub-version */}
                    <div>
                        <label className="block text-xs uppercase tracking-wider text-white/50 font-mono mb-2">
                            Specific Version
                        </label>
                        <select
                            value={selectedVersion}
                            onChange={(e) => setSelectedVersion(e.target.value)}
                            disabled={loadingVersions || updating}
                            className="w-full bg-black/50 border border-white/20 rounded p-3 text-white font-mono focus:border-mc-diamond focus:outline-none transition-colors"
                        >
                            {currentGroup?.versions.map(v => (
                                <option key={v} value={v}>
                                    {v}
                                </option>
                            ))}
                        </select>
                    </div>
                </div>
            </div>

            {/* Build Information & Action Card */}
            <div className="bg-black/60 border border-white/10 rounded-lg p-6 backdrop-blur-md">
                <div className="flex justify-between items-center mb-6">
                    <h2 className="text-xl font-pixel text-white flex items-center gap-2">
                        <HardDrive size={22} className="text-mc-gold" /> Latest Build Details
                    </h2>
                    {checkingBuild && (
                        <div className="flex items-center gap-2 text-xs font-mono text-white/50">
                            <RefreshCw size={14} className="animate-spin" /> Querying Fill v3...
                        </div>
                    )}
                </div>

                {buildInfo ? (
                    <div className="space-y-6">
                        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                            <div className="bg-black/40 border border-white/5 p-4 rounded">
                                <div className="text-xs text-white/40 font-mono uppercase mb-1">Build Number</div>
                                <div className="text-xl font-mono text-mc-diamond">Build #{buildInfo.latest_build}</div>
                            </div>
                            <div className="bg-black/40 border border-white/5 p-4 rounded">
                                <div className="text-xs text-white/40 font-mono uppercase mb-1">Release Channel</div>
                                <div className="text-xl font-mono text-mc-green">{buildInfo.channel}</div>
                            </div>
                            <div className="bg-black/40 border border-white/5 p-4 rounded">
                                <div className="text-xs text-white/40 font-mono uppercase mb-1">Download Size</div>
                                <div className="text-xl font-mono text-white/90">{formatBytes(buildInfo.size)}</div>
                            </div>
                            <div className="bg-black/40 border border-white/5 p-4 rounded">
                                <div className="text-xs text-white/40 font-mono uppercase mb-1">Target Artifact</div>
                                <div className="text-sm font-mono text-white/80 truncate" title={buildInfo.file_name}>
                                    {buildInfo.file_name}
                                </div>
                            </div>
                        </div>

                        {/* SHA256 Verification Details */}
                        <div className="bg-black/40 border border-white/5 p-4 rounded space-y-2 font-mono text-xs">
                            <div className="flex items-center gap-2 text-white/50">
                                <ShieldCheck size={14} className="text-green-400" /> SHA-256 Checksum:
                            </div>
                            <div className="text-white/80 break-all bg-black/60 p-2 rounded border border-white/5">
                                {buildInfo.latest_hash}
                            </div>
                        </div>

                        {/* Status & Action */}
                        <div className="flex flex-col sm:flex-row items-center justify-between gap-4 pt-4 border-t border-white/10">
                            <div className="flex items-center gap-3">
                                {buildInfo.update_available ? (
                                    <div className="flex items-center gap-2 text-mc-gold font-mono text-sm">
                                        <ArrowUpCircle size={18} /> Update or Version Change Available
                                    </div>
                                ) : (
                                    <div className="flex items-center gap-2 text-mc-green font-mono text-sm">
                                        <CheckCircle size={18} /> Current Server JAR matches this build
                                    </div>
                                )}
                            </div>

                            <button
                                onClick={handleApplyUpdate}
                                disabled={updating}
                                className={`
                                    flex items-center gap-2 px-6 py-3 rounded font-mono font-bold text-sm transition-all
                                    ${updating
                                        ? 'bg-white/10 text-white/40 cursor-not-allowed'
                                        : 'bg-green-600 hover:bg-green-500 text-white shadow-lg'}
                                `}
                            >
                                {updating ? (
                                    <>
                                        <RefreshCw size={16} className="animate-spin" /> Installing {selectedVersion}...
                                    </>
                                ) : (
                                    <>
                                        <DownloadCloud size={18} />
                                        {buildInfo.update_available ? `Install Paper ${selectedVersion}` : `Reinstall Build #${buildInfo.latest_build}`}
                                    </>
                                )}
                            </button>
                        </div>
                    </div>
                ) : (
                    <div className="p-8 text-center text-white/30 font-mono">
                        {checkingBuild ? 'Loading build information...' : 'No build metadata available for this version.'}
                    </div>
                )}
            </div>
        </div>
    );
}
