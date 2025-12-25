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

