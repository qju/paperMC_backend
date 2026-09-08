# Lodestone - High-Performance Minecraft Server Manager

[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://golang.org/) 
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Lodestone** is a high-performance, single-binary management dashboard and backend controller for PaperMC and modern Minecraft servers. Built in pure Go with zero runtime CGO dependencies, it provides real-time WebSocket console streaming, player access control, rich world diagnostics with GZIP NBT parsing, automated PaperMC Fill v3 updates, Web user management, and an embedded React SPA interface.

## Features

- **Multi-Factor Authentication (2FA) & Session Hardening:** RFC 6238 TOTP two-factor authentication compatible with Google Authenticator, Microsoft Authenticator, 1Password, Authy, and Apple Passwords with real-time QR code mobile pairing and guided setup wizard, 8 single-use emergency recovery backup codes with file export, sliding-window brute-force rate limiter (5 attempts, 15m lockout), anti-enumeration timing equalization via dummy bcrypt checks, and dual-token session architecture (short-lived access tokens + auto-rotated refresh tokens in HttpOnly strict cookies).
- **Java Process Isolation Engine & Sandboxing:** Unprivileged Linux process isolation via Bubblewrap (`bwrap`) with read-only root (`--ro-bind / /`), isolated private `/tmp`, dropped capabilities (`--cap-drop ALL`), unshared PID/IPC/UTS namespaces, and automatic masking of sensitive credentials and database files (`paper.db`, `.env`). Also supports POSIX DAC user separation (`ISOLATION_MODE=user`).
- **Defensive Engineering & Vulnerability Hardening:** Zero known dependencies vulnerabilities (`govulncheck` clean, `npm audit` 0 vulnerabilities), WebSocket Cross-Site Hijacking (CSWSH) origin validation, and Zip Slip symlink traversal rejection in backup restoration.
- **Multi-Core & Real-Time Vitals Monitoring:** Track per-core CPU load (Core 0 vs Core 1), JVM process vs host system utilization, JVM threads, memory RSS, disk storage headroom (<5GB alerts), live uptime counter, TPS/MSPT engine tick rate, and rolling time-series sparklines via real-time WebSocket push (`/ws`).
- **Real-Time Console:** Bidirectional WebSocket streaming with centralized broadcast hub and ANSI terminal emulation.
- **Player Management:** Whitelist, Ban, Operator controls, rejected connection intelligence, live search, and pagination.
- **Rich World Diagnostics:** Pure-Go zero-dependency GZIP binary NBT parser (`ReadLevelDat`), Modern (26.1+ `world/dimensions/`) and Legacy dimension discovery, safe duplication, and deletion.
- **PaperMC Fill v3 Updater:** Version family selector (26.2, 1.21, etc.), latest stable build detection, and streaming download with on-the-fly SHA-256 validation.
- **Web User Administration:** Multi-user authentication control panel with bcrypt hashing, password rotation, and role management.
- **Atomic SQLite Migration Engine:** Versioned schema migrations using native `PRAGMA user_version` with automatic production database adoption.
- **Server Configuration Editor:** Categorized visual controls (General, Gameplay, Security, Performance, RCON) and raw editor with comment preservation for `server.properties`.
- **Backup Engine & Snapshots:** Zero-data-loss snapshots coordinated with Minecraft autosave freezing (`save-off` -> `save-all flush` -> archive -> `save-on`), pure-Go ZIP compression with ZipSlip defense, on-the-fly SHA-256 verification, one-click restoration, and archive downloads.
- **Automated Task Scheduler & Execution Audit Logs:** Background cron engine (`robfig/cron/v3`) for automated world backups, scheduled server restarts with countdown notifications, maintenance commands, and chat broadcasts, backed by a persistent execution audit log tracking run statuses, durations in milliseconds, and error details.
- **Plugin Manager & Geyser Bedrock Bridge:** Comprehensive Paper/Spigot plugin control with pure-Go ZIP manifest extraction, enable/disable toggle without file deletion, custom `.jar` upload, and Modrinth v2 marketplace search. Includes a dedicated GeyserMC & Floodgate hub with Bedrock client version compatibility tracking and one-click upstream updates.
- **Smart Flags & JVM Optimizer:** Dynamic Aikar's G1GC flag tuning based on configured heap RAM (automatically adjusting Young Generation boundaries and reserve thresholds for `<12GB` vs `≥12GB`), presets (`aikar`, `minimal`, `none`, `custom`), active Java arguments tracking, and restart synchronization detection.
- **Administrative Action Audit Logs:** Immutable chronological audit trail recording operator identity, client IP addresses (resolving `X-Forwarded-For` and `X-Real-IP`), HTTP methods, endpoints, response status codes, and operational context for all server lifecycle events, configuration adjustments, player moderation, backups, plugins, and scheduled tasks.
- **Performance Profiler & Spark / Timings Engine:** Automated console log interception and database archiving of Spark (`spark.lucko.me`) and Paper Timings (`timings.aikar.co`, `timings.papermc.io`) reports, real-time WebSocket link broadcasts (`profiler_event`), interactive profiler command execution (`/spark sampler`, `/spark health`, `/timings paste`), multi-metric health telemetry parsing (TPS, MSPT, CPU, Memory Heap, GC Activity, Disk), and automated JVM/server tuning recommendations.
- **Embedded SPA UI:** Modern dark glassmorphic React + TypeScript dashboard embedded via `go:embed`.



## Getting Started

### Prerequisites

- Go 1.22 or later
- Java 21 or later (to execute PaperMC)
- Node.js (only required if building/modifying the frontend)
- Bubblewrap `bwrap` (optional, recommended on Linux for process isolation)

### Installation

1. Clone the repository:
   ```sh
   git clone <repository-url>
   cd paperMC_backend
   ```

2. Place your PaperMC server JAR file in the working directory (default: `./paperMS` or configured via `MC_WORKDIR`).

### Configuration

The application is configured using environment variables:

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PORT` | Web server listening port. | `8080` |
| `MC_WORKDIR` | Working directory for the Minecraft server. | `./paperMS` |
| `JAR_FILE` | Server JAR filename. | `server.jar` |
| `RAM` | RAM allocation for Minecraft JVM. | `8G` |
| `DBNAME` | SQLite database filepath. | `paper.db` |
| `ADMIN_USER` | Initial admin username (bootstrapped on startup). | `admin` |
| `ADMIN_PASS` | Initial admin password. | **Required** |
| `JWT_SECRET` | Secret key for signing JWT tokens. | Ephemeral 256-bit crypto key |
| `ISOLATION_MODE` | Process isolation strategy: `bwrap`, `user`, or `none`. | `none` |
| `MINECRAFT_USER` | Target username for POSIX DAC user separation (`ISOLATION_MODE=user`). | `minecraft` |
| `MINECRAFT_UID` | Explicit target UID for POSIX process isolation. | `0` (auto-lookup) |
| `MINECRAFT_GID` | Explicit target GID for POSIX process isolation. | `0` (auto-lookup) |

### Running the Server Locally

1. Set the required `ADMIN_PASS` environment variable:
   ```sh
   export ADMIN_PASS="your-secret-password"
   ```
2. Start the server (or run `./dev.sh` to run backend + React dev UI concurrently):
   ```sh
   go run cmd/server/main.go
   ```
3. Open `http://localhost:8080` in your browser.

---

## 🏗️ Building & Cross-Compilation

Because Lodestone uses modern, pure-Go SQLite bindings without CGO (`modernc.org/sqlite`), static binaries can be effortlessly cross-compiled for any target architecture directly from any OS.

### Quick Build Commands (via Makefile)

```sh
# Build native binary + frontend UI bundle
make build

# Cross-compile for Linux ARM64 (Raspberry Pi 4/5, Oracle Cloud Ampere, AWS Graviton)
make build-arm64

# Cross-compile for Linux AMD64 (Standard x86_64 VPS / Dedicated servers)
make build-amd64

# Cross-compile all architectures simultaneously
make build-all
```

All compiled binaries are generated into the `bin/` directory with stripped debug symbols (`-ldflags="-s -w"`) resulting in standalone static executables (~12MB).

---

## 🚀 Automated Remote Deployment

Lodestone includes an automated deployment script ([`scripts/deploy.sh`](scripts/deploy.sh)) to compile, transfer, install, and restart the service on remote servers with a single command.

### 1. One-Command Deploy

```sh
./scripts/deploy.sh --host ubuntu@192.168.1.100 --dir /opt/lodestone --arch arm64 --service lodestone
```

### 2. Interactive Mode & Config Saving

Run `./scripts/deploy.sh` without arguments to enter interactive mode. You will be prompted for the remote SSH target and installation folder, with the option to save your settings to `.deploy.env` for rapid future deployments:

```sh
# Future deployments only require:
make deploy
# or
./scripts/deploy.sh
```

### 3. Production Systemd Service

A production systemd unit template is provided in [`scripts/lodestone.service`](scripts/lodestone.service).

To install on your remote Linux host:
```sh
# Copy service file to systemd directory
sudo cp scripts/lodestone.service /etc/systemd/system/lodestone.service

# Reload daemon and enable service
sudo systemctl daemon-reload
sudo systemctl enable --now lodestone

# View live service logs
journalctl -u lodestone -f
```


## API Endpoints

### Public Endpoints
- `POST /login`: Authenticate and obtain a JWT bearer token (`{"username": "...", "password": "..."}`).

### Protected Server & Console Endpoints (Requires `Authorization: Bearer <token>`)
- `GET /status`: Server vitals (process status, CPU%, RAM RSS, player count, active world).
- `GET /ws`: WebSocket stream for live console broadcast and command submission.
- `POST /command`: Execute a console command (`{"command": "..."}`).
- `POST /start`: Start the Minecraft server.
- `POST /stop`: Gracefully stop the Minecraft server (`stop`).
- `POST /kill`: Force terminate the server process.
- `GET /config`: Read `server.properties` as JSON.
- `POST /config`: Update `server.properties` preserving comments and layout.

### Player Management Endpoints
- `GET /api/players`: List whitelisted players.
- `POST /api/players`: Add player to whitelist (`{"username": "..."}`).
- `DELETE /api/players?username=...`: Remove player from whitelist.
- `GET /api/players/banned`: List banned players.
- `POST /api/players/banned`: Ban player (`{"username": "...", "reason": "..."}`).
- `DELETE /api/players/banned?username=...`: Unban player.
- `GET /api/players/ops`: List operator players.
- `POST /api/players/ops?action=add|remove`: Add or remove operator status (`{"username": "..."}`).
- `GET /api/players/rejected`: List blocked connection attempts from SQLite.
- `DELETE /api/players/rejected?username=...`: Dismiss rejected player log.

### World Management Endpoints
- `GET /api/worlds`: List all worlds with NBT metadata, dimensions, and disk size.
- `POST /api/worlds/active`: Switch active world (`{"world_name": "..."}`).
- `POST /api/worlds/create`: Create a new world (`{"world_name": "...", "seed": "..."}`).
- `POST /api/worlds/duplicate`: Safely clone a world with `save-all flush` (`{"source_name": "...", "target_name": "..."}`).
- `DELETE /api/worlds?world_name=...`: Delete an inactive world.

### Updater Endpoints (PaperMC Fill v3)
- `GET /api/updater/versions`: Fetch available PaperMC version groups and builds.
- `GET /api/updater/check`: Check for latest build in a version family.
- `POST /api/updater/apply`: Download and verify SHA-256 checksum of selected build.

### Web User Management Endpoints
- `GET /api/users`: List operator accounts (ID, username, role).
- `POST /api/users`: Create a new user (`{"username": "...", "password": "...", "role": "..."}`).
- `PUT /api/users/password`: Reset user password (`{"username": "...", "password": "..."}`).
- `DELETE /api/users?username=...`: Delete a user (preventing deletion of last remaining user).

### Backup & Snapshot Endpoints
- `GET /api/backups`: List existing backup archives with size, creation timestamp, world name, and checksum.
- `POST /api/backups/create`: Create a coordinated snapshot (`{"type": "world"|"full", "world_name": "..."}`).
- `GET /api/backups/download?file=...`: Stream and download a backup archive zip.
- `POST /api/backups/restore`: Restore server or world from archive (`{"file": "..."}`).
- `DELETE /api/backups?file=...`: Delete an archive from storage.

### Automation & Scheduler Endpoints
- `GET /api/schedules`: List all automated schedules with next run timestamps.
- `POST /api/schedules`: Create a scheduled job (`{"name": "...", "cron_expr": "...", "action_type": "...", "payload": "..."}`).
- `PUT /api/schedules`: Update schedule parameters, cron expression, or action payload.
- `POST /api/schedules/toggle?id=...`: Toggle schedule state (enable/pause).
- `POST /api/schedules/run?id=...`: Trigger manual immediate background execution ("Run Now").
- `DELETE /api/schedules?id=...`: Delete a schedule and cascade delete its execution logs.
- `GET /api/schedules/logs?schedule_id=...&limit=...`: Retrieve historical execution audit logs with run durations and error traces.
- `DELETE /api/schedules/logs?schedule_id=...`: Purge historical execution audit logs.

### Plugin Management & Bedrock Bridge Endpoints
- `GET /api/plugins`: List installed plugins with extracted metadata, file sizes, and active/disabled states.
- `POST /api/plugins/toggle`: Toggle plugin between active (`.jar`) and disabled (`.jar.disabled`).
- `DELETE /api/plugins?filename=...`: Delete plugin file safely.
- `POST /api/plugins/upload`: Multipart file upload for custom `.jar` plugins.
- `GET /api/plugins/geyser/status`: Bedrock Bridge status report: installed vs latest upstream Geyser & Floodgate builds, SHA-256 hashes, and supported Bedrock version compatibility.
- `POST /api/plugins/geyser/update`: One-click update/install for Geyser, Floodgate, or both (`{"target": "geyser"|"floodgate"|"both"}`).
- `GET /api/plugins/market/search?query=...`: Search Paper/Spigot plugins on Modrinth v2.
- `POST /api/plugins/market/install`: One-click install from Modrinth (`{"project_id": "...", "version_id": "..."}`).

### Smart Flags & JVM Optimizer Endpoints
- `GET /api/flags`: Retrieve configured flags, calculated effective JVM flags, running process arguments, and restart required status.
- `POST /api/flags`: Save JVM settings (`{"ram": "8G", "preset": "aikar", "custom_flags": "..."}`).
- `GET /api/flags/presets?ram=...`: Retrieve available optimization presets (`aikar`, `minimal`, `none`, `custom`) and sample flags for the given RAM allocation.

### Administrative Action Audit Log Endpoints
- `GET /api/audit?page=...&limit=...&action=...&username=...`: Paginated retrieval of administrative action records with category filtering.
- `DELETE /api/audit`: Purge historical audit log entries.

### Crash Analyst & AI Diagnostic Endpoints
- `GET /api/crash?limit=...&offset=...`: Retrieve recorded crash incidents with automatic disk crash report ingestion.
- `GET /api/crash/{id}`: Fetch detailed diagnostic breakdown for a specific crash report.
- `POST /api/crash/analyze`: Heuristically classify raw stack traces or recent server logs (`{"log": "...", "save": true|false}`).
- `DELETE /api/crash?id=...`: Delete an individual report or purge all recorded crash reports (`?id=all`).
- `GET /api/crash/ai-config`: Retrieve AI provider settings with masked API key.
- `POST /api/crash/ai-config`: Save AI diagnostic configuration (supporting OpenAI, Gemini, Ollama, Groq, DeepSeek).
- `POST /api/crash/ai-explain`: Request deep external LLM diagnostic consultation with automated sensitive log sanitization.

### Performance Profiler & Timings / Spark Engine Endpoints
- `GET /api/profiler/reports?limit=...&offset=...&type=...`: Retrieve recorded Spark and Timings profiler reports with pagination and type filtering.
- `GET /api/profiler/reports/{id}`: Fetch single report by ID.
- `DELETE /api/profiler/reports?id=...`: Delete individual report or purge all recorded profiler reports (`?id=all`).
- `POST /api/profiler/health`: Retrieve or parse performance health snapshot (TPS, MSPT, CPU, Heap, GC, Disk) with automated tuning advice (`{"log": "...", "save": true|false}`).
- `POST /api/profiler/trigger`: Dispatch Spark or Timings commands directly to running server (`{"action": "health"|"sampler_start"|"sampler_stop"|"timings_paste"|"timings_reset"|"custom", "command": "..."}`).

### Authentication & Multi-Factor (2FA) Endpoints
- `POST /api/auth/login` (or `POST /login`): Authenticate with anti-enumeration protection and brute-force sliding-window rate limiting. Returns token and session refresh token or `mfa_required` challenge.
- `POST /api/auth/2fa/verify-login`: Complete login challenge using TOTP code or emergency backup recovery code.
- `POST /api/auth/refresh`: Rotate refresh token and issue a fresh short-lived access token (reads from HttpOnly cookie or payload).
- `POST /api/auth/logout`: Revoke active session and invalidate refresh cookies.
- `GET /api/auth/2fa/status`: Check whether 2FA is active for the current authenticated user.
- `POST /api/auth/2fa/setup`: Generate a new 160-bit TOTP secret, `otpauth://` URI, and 8 single-use backup recovery codes.
- `POST /api/auth/2fa/enable`: Confirm and activate 2FA with an initial 6-digit TOTP code.
- `POST /api/auth/2fa/disable`: Deactivate 2FA using a valid code or account password.
- `POST /api/users/reset-2fa`: Administratively reset 2FA for a user (erases secret, backup codes, and revokes active sessions).

## Project Status

- [x] Core Process Manager & Lifecycle Engine
- [x] Centralized WebSocket Console Hub & ANSI Rendering
- [x] Smart Player Access Control (Whitelist, Bans, Ops, Rejections)
- [x] Rich World Diagnostics & Pure-Go GZIP NBT Parser
- [x] PaperMC Fill v3 API & Auto-Updater with SHA-256 Validation
- [x] Web User Administration Control Panel
- [x] Atomic SQLite Migration Engine (`PRAGMA user_version`)
- [x] Visual Server Configuration Editor (`server.properties`)
- [x] Milestone 2.2: Backup Engine & Snapshots
- [x] Milestone 2.5: Cron Task Scheduler & Execution Log Viewer
- [x] Milestone 2.6: Testing Gap Closure & Hardening (≥80% Coverage Gate)
- [x] Milestone 3.1: Performance Profiler & Timings / Spark Engine
- [x] Milestone 3.1 (Smart Flags): Smart Flags & Aikar's JVM Optimizer
- [x] Milestone 3.2: Modrinth Plugin Manager & Geyser Bedrock Bridge
- [x] Milestone 3.3 (Audit): Administrative Action Audit Logs
- [x] Milestone 3.3 (Crash Analyst): Crash Analyst & Heuristic Log Diagnostic Engine with Optional AI Consultation
- [x] Milestone 5.0 (Security & Hardening): Vulnerability Remediation, MFA/TOTP 2FA, Anti-Enumeration & Rate-Limiting, Session Revocation, and Java Bubblewrap Process Isolation Sandbox

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

