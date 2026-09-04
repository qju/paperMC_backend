export interface Player {
    uuid: string;
    name: string;
    created?: string;
    source?: string;
    expires?: string;
    reason?: string;
    level?: number;
}

export interface RejectedPlayer {
    username: string;
    count: number;
    last_seen: string;
}

// The Unified Player Object for the table
export interface UnifiedPlayer {
    name: string;
    uuid?: string;
    isOnline: boolean;
    isWhitelisted: boolean;
    isBanned: boolean;
    isOp: boolean;
    isRejected: boolean;
    reason?: string; // Ban reason
    rejectionCount?: number;
}

export interface MetricPoint {
    timestamp: number;
    cpu: number;
    ram: number;
    tps: number;
    mspt: number;
}

export interface Vitals {
    status: string;
    uptime_seconds: number;
    cpu: number;
    system_cpu: number;
    cpu_cores?: number[];
    threads?: number;
    ram: number;
    total_memory: string;
    disk_free?: number;
    disk_total?: number;
    disk_used_pct?: number;
    player_count: number;
    player_list: Array<{ name: string; uuid: string }>;
    active_world: string;
    tps?: number;
    mspt?: number;
    history?: MetricPoint[];
}

export interface BackupInfo {
    filename: string;
    size_bytes: number;
    formatted_size: string;
    created_at: string;
    backup_type: 'world' | 'full' | string;
    world_name?: string;
    checksum_sha256?: string;
}


