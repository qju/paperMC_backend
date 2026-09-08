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

export interface Schedule {
    id: number;
    name: string;
    cron_expr: string;
    action_type: 'backup' | 'restart' | 'command' | 'broadcast' | 'start' | 'stop' | string;
    payload?: string;
    is_enabled: boolean;
    created_at: string;
    next_run_at?: string;
}

export interface ScheduleLog {
    id: number;
    schedule_id: number;
    schedule_name: string;
    action_type: string;
    status: 'success' | 'failed' | string;
    duration_ms: number;
    error_message?: string;
    executed_at: string;
}

export interface PluginInfo {
    filename: string;
    name: string;
    version: string;
    main?: string;
    description?: string;
    authors?: string[];
    website?: string;
    api_version?: string;
    dependencies?: string[];
    soft_dependencies?: string[];
    is_enabled: boolean;
    size_bytes: number;
    formatted_size: string;
    mod_time: string;
}

export interface GeyserStatus {
    installed: boolean;
    installed_file?: string;
    installed_version?: string;
    installed_build?: number;
    installed_sha256?: string;
    is_enabled: boolean;
    latest_version: string;
    latest_build: number;
    latest_sha256: string;
    update_available: boolean;
    supported_bedrock: string;
    latest_bedrock_ver: string;
    recent_changes?: string[];
    release_date: string;
}

export interface FloodgateStatus {
    installed: boolean;
    installed_file?: string;
    installed_version?: string;
    installed_build?: number;
    installed_sha256?: string;
    is_enabled: boolean;
    latest_version: string;
    latest_build: number;
    latest_sha256: string;
    update_available: boolean;
    recent_changes?: string[];
    release_date: string;
}

export interface BedrockBridgeStatus {
    geyser: GeyserStatus;
    floodgate: FloodgateStatus;
    overall_status: 'ready' | 'update_available' | 'incomplete' | 'missing' | string;
    bedrock_support_info: string;
}

export interface ModrinthHit {
    project_id: string;
    slug: string;
    title: string;
    description: string;
    categories: string[];
    icon_url: string;
    author: string;
    downloads: number;
    followers: number;
    date_modified: string;
    latest_version: string;
}

export interface ModrinthSearchResult {
    hits: ModrinthHit[];
    total_hits: number;
    limit: number;
    offset: number;
}

export interface ServerFlagsConfig {
    ram: string;
    preset: 'aikar' | 'minimal' | 'none' | 'custom' | string;
    custom_flags: string;
    updated_at?: string;
}

export interface FlagsStatusResponse {
    configured: ServerFlagsConfig;
    effective_flags: string[];
    active_args?: string[];
    restart_required: boolean;
    server_running: boolean;
}

export interface FlagPresetInfo {
    id: string;
    name: string;
    description: string;
    doc_url: string;
    sample_flags: string[];
}

export interface AuditLog {
    id: number;
    username: string;
    action: string;
    endpoint: string;
    method: string;
    details: string;
    ip_address: string;
    status_code: number;
    created_at: string;
}

export interface AuditLogsResponse {
    logs: AuditLog[];
    total: number;
    page: number;
    limit: number;
    total_pages: number;
}

export interface CrashReport {
    id: number;
    source: 'runtime' | 'crash_file' | 'manual' | string;
    category: string;
    title: string;
    culprit?: string;
    summary: string;
    recommendation: string;
    raw_log: string;
    created_at: string;
}

export interface CrashReportsResponse {
    reports: CrashReport[];
    total: number;
    limit: number;
    offset: number;
}

export interface AISettings {
    provider: 'openai' | 'gemini' | 'ollama' | 'groq' | 'deepseek' | string;
    api_key: string;
    has_api_key?: boolean;
    model: string;
    base_url: string;
    is_enabled: boolean;
    updated_at?: string;
}

export interface AIExplanationResponse {
    explanation: string;
    provider: string;
    model: string;
}

