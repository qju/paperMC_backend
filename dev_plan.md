# Lodestone: Master Development Plan

**Mission:** Create the "Best in Class" High-Performance Single Server Manager for Minecraft (PaperMC & Forks).
**Philosophy:** Zero Dependencies. Maximum Power. Single Binary.

---

## 🏗️ Phase 1: Core Architecture Hardening
*Focus: Production-grade infrastructure, security, and persistence.*

### Milestone 1.1: The Persistence Layer (SQLite & Migration Engine)
- [x] **Task:** Create `internal/database` package.
- [x] **Task:** Implement `modernc.org/sqlite` driver (CGO-free).
- [x] **Task:** Design schema: `users` (id, username, password_hash, role) and `rejected_players`.
- [x] **Task:** Create `Store` interface for decoupling DB from business logic.
- [x] **Task:** Implement Atomic Versioned SQLite Migration Engine (`internal/database/migrations.go`) using `PRAGMA user_version` and baseline auto-adoption.

### Milestone 1.2: Security & Auth
- [x] **Task:** Replace Basic Auth with JWT (JSON Web Tokens).
- [x] **Task:** Implement `internal/auth` middleware with role claims.
- [x] **Task:** Hash passwords using `bcrypt`.
- [x] **Task:** Create `POST /login` authentication endpoint.
- [x] **Task:** Implement Web User Control endpoints (`GET /api/users`, `POST /api/users`, `PUT /api/users/password`, `DELETE /api/users`).

### Milestone 1.3: Real-Time Communication (WebSockets)
- [x] **Task:** Implement centralized WebSocket broadcast hub (`gorilla/websocket`).
- [x] **Task:** Unified bi-directional console stream (send command -> receive streamed logs).
- [x] **Task:** Implement listener subscriber pattern decoupling logger from direct socket operations.

---

## 🛠️ Phase 2: "Crafty Parity" (The Essentials)
*Focus: Adding daily-driver operational features.*

### Milestone 2.1: Modernized Resource & Vitals Monitoring
- [x] **Task:** Multi-Core CPU tracking (`gopsutil/cpu`) with individual per-core workload breakdown (Core 0 vs Core 1) and process vs host load.
- [x] **Task:** Storage headroom monitoring (`gopsutil/disk`) with low disk space alerts (<5GB).
- [x] **Task:** Minecraft engine health indicators (dynamic real-time TPS & MSPT tracking via Paper/Spigot `tps` / `mspt` poll parsing and lag warning detection with automated poll suppression, JVM threads, boot readiness gate (`Done` detection + 5s post-startup stabilization grace period preventing early command exceptions), and live uptime counter).
- [x] **Task:** Centralized WebSocket vitals broadcast (`/ws`) with real-time push replacing HTTP polling.
- [x] **Task:** Rolling in-memory metrics ring buffer (last 30 samples) with live SVG sparkline graphs.


### Milestone 2.2: The Backup Engine
- [x] **Task:** Create `internal/backup` package.
- [x] **Task:** Implement "Snapshot Logic" (`save-off` -> Flush -> Zip -> `save-on`).
- [x] **Task:** API Endpoint `POST /api/backups/create`.
- [x] **Task:** API Endpoint `GET /api/backups` (List existing archive zips with metadata).
- [x] **Task:** API Endpoint `GET /api/backups/download?file=...`.
- [x] **Task:** Restore logic (Stop server -> Unzip -> Validate -> Start server).
- [x] **Task:** React UI for backup creation, progress, download, and restoration.

### Milestone 2.3: World Management & Diagnostics
- [x] **Task:** Implement pure-Go zero-dependency GZIP binary NBT parser (`ReadLevelDat`) extracting MC version, gamemode, difficulty, hardcore, and last played time.
- [x] **Task:** Support PaperMC 26.1+ Unified Dimension directories (`world/dimensions/`) and Legacy sibling folders (`{world}_nether`, `{world}_the_end`).
- [x] **Task:** Safe world cloning (`POST /api/worlds/duplicate`) with `save-all flush` synchronization.
- [x] **Task:** Safe world deletion (`DELETE /api/worlds`) preventing active world destruction.
- [x] **Task:** Synchronized world switching (`POST /api/worlds/active`) with graceful restart and 30s timeout guard.

### Milestone 2.4: PaperMC Fill v3 API & Auto-Updater
- [x] **Task:** Integrate PaperMC Fill v3 API (`https://fill.papermc.io/v3`) for release discovery.
- [x] **Task:** Major release group selector (e.g. 26.2, 1.21, etc.) and latest stable build lookup.
- [x] **Task:** Streaming download with on-the-fly SHA-256 checksum verification.
- [x] **Task:** Endpoints `GET /api/updater/versions`, `GET /api/updater/check`, `POST /api/updater/apply`.

### Milestone 2.5: The Scheduler (Cron) & Task Execution Log Viewer
- [x] **Task:** Implement `robfig/cron/v3` scheduler engine (`internal/scheduler`).
- [x] **Task:** Create DB tables `schedules` and `schedule_logs` via atomic Version 2 migration (`internal/database/migrations.go`).
- [x] **Task:** Implement scheduling runners for automated actions (`backup`, `restart`, `command`, `broadcast`, `start`, `stop`) with overlap concurrency locking and execution audit logging.
- [x] **Task:** Implement REST endpoints (`GET/POST/PUT/DELETE /api/schedules`, `POST /api/schedules/toggle`, `POST /api/schedules/run`, `GET/DELETE /api/schedules/logs`).
- [x] **Task:** Build React UI dashboard with visual presets, schedule management, instant "Run Now", and searchable Historical Task Execution Log Viewer.
- [x] **Task:** Add comprehensive unit & integration tests maintaining $\ge 80\%$ test coverage across all internal packages.

### Milestone 2.6: Comprehensive Test Coverage & Reliability Gate (Pre-v1.0 Gate)
- [x] **Task:** Elevate `internal/api` statement test coverage from 46.8% to ≥80%:
  - Test missing query parameter validation, routing error fallbacks, and malformed request payload handling.
  - Test WebSocket connection close/reconnect lifecycles, unauthorized handshake rejections, and hub registration failure modes.
  - Test world management edge-cases (invalid active world requests, non-existent world deletion, clone failure scenarios).
- [x] **Task:** Elevate `internal/minecraft` statement test coverage from 63.5% to ≥80%:
  - Test server process lifecycle edge cases: sudden exit detection, crash-loop detection, and graceful shutdown timeouts.
  - Test log streaming and ANSI parser edge cases: malformed server lines, multi-line exception stack traces, and unicode characters.
  - Test rejected connection logic and player tracking under rapid message bursts.
- [x] **Task:** Add coverage enforcement and regression gate to verify all internal packages maintain ≥80% coverage.
- [x] **Task:** Resolve reliability edge cases discovered during testing:
  - Reset `OnlinePlayers` and active player counts upon process termination or crash in `monitorProcess`.
  - Provide concurrent subscriber isolation and automatic unsubscription for SSE logs (`/api/logs`).
  - Correct Geyser API status code attribution and unmarshaling diagnostics in `GetXUID`.
  - Guard database store against nil pointer panics during console log parsing.
  - Fix string concatenation formatting in server control error responses.

---

## 🚀 Phase 3: The "Beat Crafty" Features
*Focus: Specialized Minecraft tools that general-purpose managers lack.*

### Milestone 3.1: Deep Integration & Smart Flags (JVM / Aikar's Flags Optimizer & Spark Profiler)
- [x] **Task:** **Timings Viewer & Spark Profiler Integration:**
  - Database migration v6 (`profiler_reports` table) tracking report type (`spark_profile`, `spark_health`, `timings`), title, URL, summary, raw output, and timestamp with indexes.
  - Store interface CRUD methods (`RecordProfilerReport`, `ListProfilerReports`, `GetProfilerReport`, `DeleteProfilerReport`, `ClearProfilerReports`).
  - Pure-Go Health and Profiler parsing engine (`internal/profiler/parser.go`) extracting multi-duration TPS, MSPT tick durations, dual process/system CPU load, JVM heap allocation, GC collection activity, disk usage, and generating actionable server tuning advice.
  - Console log listener in `internal/minecraft/server.go` automatically intercepting generated Spark (`spark.lucko.me`) and Aikar Timings (`timings.aikar.co`, `timings.papermc.io`) links, recording them to DB, and broadcasting `profiler_event` over WebSockets (`/ws`).
  - REST API endpoints (`GET /api/profiler/reports`, `GET /api/profiler/reports/{id}`, `DELETE /api/profiler/reports`, `POST /api/profiler/health`, `POST /api/profiler/trigger`) with audit logging.
  - Dark glassmorphic React UI dashboard (`web/src/pages/PerformanceProfiler.tsx`) with live telemetry cards (TPS, MSPT, CPU, Memory), automated tuning heuristics panel, profiler command dispatcher, real-time captured link banner, and historical reports archive table.
  - Strict $\ge 80\%$ test coverage maintained across all internal packages (`internal/profiler` 97.8%, `internal/api` 81.9%, `internal/database` 87.2%, `internal/minecraft` 92.5%).
- [x] **Task:** **Smart Flags:** Implement a preset manager for Aikar's flags based on detected/selected RAM:
  - Database migration v3 (`server_flags` table) tracking RAM, preset (`aikar`, `minimal`, `none`, `custom`), and custom flags.
  - Dedicated flags engine (`internal/flags`) dynamically tuning G1GC parameters (adapting young gen and reserve thresholds between `<12GB` and `≥12GB`).
  - Minecraft Server integration tracking `activeArgs` from actual running Java process and detecting pending restarts.
  - REST API endpoints (`GET /api/flags`, `POST /api/flags`, `GET /api/flags/presets`).
  - Responsive "Java & Smart Flags" tab in Config Editor with RAM quick selectors, adaptive G1GC notice, preset cards, custom arguments editor, live terminal launch command preview, and active process sync state.
  - 96.2% test coverage in `internal/flags` and $\ge 80\%$ coverage across all packages with zero race conditions.

### Milestone 3.2: Plugin Management & Bedrock Bridge (Geyser & Floodgate)
- [x] **Task:** Implement `internal/plugins` scanner and pure-Go ZIP YAML parser for `plugin.yml` and `paper-plugin.yml`.
- [x] **Task:** Implement plugin operations: enable/disable toggle (`.jar` <-> `.jar.disabled`), safe deletion, and `.jar` multipart upload.
- [x] **Task:** Build dedicated **GeyserMC & Floodgate Bedrock Bridge** client (`internal/plugins/geyser.go`) tracking installed vs upstream builds, SHA-256 verification, and Bedrock client compatibility ranges.
- [x] **Task:** Integrate **Modrinth v2 API** client (`internal/plugins/modrinth.go`) for search and one-click installation with SHA-512/SHA-1 verification.
- [x] **Task:** Implement REST endpoints (`GET /api/plugins`, `POST /api/plugins/toggle`, `DELETE /api/plugins`, `POST /api/plugins/upload`, `GET /api/plugins/geyser/status`, `POST /api/plugins/geyser/update`, `GET /api/plugins/market/search`, `POST /api/plugins/market/install`).
- [x] **Task:** Build comprehensive unit tests maintaining $\ge 80\%$ test coverage.

### Milestone 3.3: Observability
- [x] **Task:** **Audit Logs:** Log every administrative API action to SQLite DB (Who did what, when, IP address, and status):
  - Database migration v4 (`audit_logs` table) tracking ID, username, action, endpoint, method, details, client IP address, status code, and timestamp with indexes.
  - Store interface methods (`RecordAuditLog`, `ListAuditLogs` with pagination and action/user filters, `ClearAuditLogs`).
  - REST endpoints (`GET /api/audit`, `DELETE /api/audit`) and centralized audit helper with client IP extraction (`X-Forwarded-For`, `X-Real-IP`, `RemoteAddr`).
  - Action auditing hooks integrated into authentication, server lifecycle (start/stop/kill/command), player moderation, world management, configuration, flags tuning, backups, plugins, schedules, and updater.
  - Responsive dark glassmorphic React dashboard (`web/src/pages/AuditLogs.tsx`) with category filters, live search, relative time badges, status code indicators, pagination, and history purge modal.
  - Comprehensive unit test coverage with all packages maintaining $\ge 80\%$ test coverage.
- [x] **Task:** **Crash Analyst & Heuristic Log Diagnostic Engine with Optional External AI Consultation**:
  - Database migration v5 (`crash_reports` and `ai_settings` tables) tracking source, category, title, culprit, summary, recommendation, raw log, and AI settings with indexes.
  - Pure-Go Heuristic Diagnostic Engine (`internal/crash/analyst.go`) classifying 8 failure categories (`OutOfMemory`, `PortConflict`, `WatchdogTimeout`, `PluginFailure`, `CorruptedChunk`, `JavaVersionMismatch`, `EulaUnaccepted`, `DiskFull`, and fallback `Unknown`) with regex-based culprit extraction and actionable recommendations.
  - Disk scanner for `crash-reports/crash-*.txt` with automatic ingestion and newest-first sorting.
  - Optional AI diagnostic client (`internal/ai/client.go`) with automated log sanitization (scrubbing filesystem paths, passwords/tokens, player UUIDs, and IPv4 addresses) supporting OpenAI-compatible endpoints (OpenAI, Ollama, Groq, DeepSeek) and Google Gemini API.
  - Server process exit monitoring and crash event hook automatically capturing unexpected terminations, saving diagnostic reports, and broadcasting real-time `crash_alert` over WebSockets (`/ws`).
  - REST API endpoints (`GET /api/crash`, `GET /api/crash/{id}`, `POST /api/crash/analyze`, `DELETE /api/crash`, `GET /api/crash/ai-config`, `POST /api/crash/ai-config`, `POST /api/crash/ai-explain`).
  - Dark glassmorphic React UI dashboard (`web/src/pages/CrashAnalyst.tsx`) with category filters, live search, raw stack trace viewer, quick manual log analysis modal, AI settings modal, AI diagnostic consultation drawer, and real-time crash alert banner on the Console page (`web/src/pages/Console.tsx`).
  - Strict $\ge 80\%$ statement test coverage maintained across all internal backend packages (`internal/ai` 84.6%, `internal/api` 81.5%, `internal/crash` 95.3%, `internal/database` 87.0%, `internal/minecraft` 92.7%).

---

## 🎨 Phase 4: Frontend Modernization (React + Vite)
*Focus: High-performance, responsive dark glassmorphic UI.*

### Milestone 4.1: The Build Pipeline & SPA Shell
- [x] **Task:** Initialize React + Vite project in `web/`.
- [x] **Task:** Configure `go:embed` to bundle `web/dist/` into single Go binary.
- [x] **Task:** Vite proxy configuration for local development.
- [x] **Task:** High-contrast login page with branding and vector SVG favicon.

### Milestone 4.2: Feature Dashboards
- [x] **Task:** **Console:** Live terminal with ANSI color rendering, command input, auto-scroll, and real-time crash alert warning banner.
- [x] **Task:** **Player Manager:** Whitelist, Ban, Operator status, rejected connection history, real-time search, filter tabs, scroll container, and selectable pagination (10/25/50/100 per page).
- [x] **Task:** **World Manager:** Active world spotlight card, dimension badges, disk size calculator, level.dat NBT metadata, world creation, cloning, and deletion.
- [x] **Task:** **Updates & Versions:** Major release group selector, latest build detector, changelog viewer, and one-click upgrade button.
- [x] **Task:** **Web Users:** Operator account table, user creation modal, password reset modal, and safety guards.
- [x] **Task:** **Backups Page:** Backup listing, snapshot button, download links, and restore modal.
- [x] **Task:** **Config Editor:** Visual editor for `server.properties` with categorized settings (General, Gameplay, Security, Performance, RCON), search filter, and raw editor toggle with comment preservation.
- [x] **Task:** **Schedules & Logs Page:** Active schedules manager with visual presets, status toggle, Run Now trigger, and searchable execution audit log table.
- [x] **Task:** **Plugins & Bedrock Bridge Dashboard:** Installed plugins inspector with enable/disable toggle, file upload, Modrinth marketplace browser, and dedicated Geyser/Floodgate Bedrock compatibility hub with 1-click update.
- [x] **Task:** **Audit Logs Dashboard:** Chronological administrative activity audit log viewer with category badges, user filters, search, IP tracking, and history purge modal.
- [x] **Task:** **Crash Analyst Dashboard:** Heuristic diagnostics incident viewer with category filter badges, root-cause summaries, step-by-step resolution advice, raw stack trace drawer, manual log dump analyzer, AI configuration modal, and interactive AI consultation assistant.
- [x] **Task:** **Performance Profiler & Spark Dashboard:** Dark glassmorphic dashboard (`web/src/pages/PerformanceProfiler.tsx`) featuring real-time telemetry metrics (TPS, MSPT, CPU, Memory Heap), GC collection monitoring, automated JVM tuning heuristics, quick action dispatcher, custom command runner, live captured profiler URL banner, and historical reports archive with modal inspection.

---

## 🛡️ Phase 5: Security Hardening, Multi-Factor Authentication & Process Isolation
*Focus: Enterprise-grade authentication, vulnerability elimination, and unprivileged Linux sandboxing.*

### Milestone 5.1: Dependency & Source-Level Vulnerability Remediation
- [x] **Task:** Audit and eliminate all vulnerable frontend npm packages (`npm audit fix` resolved all 17 vulnerabilities, achieving 0 vulnerabilities).
- [x] **Task:** Upgrade vulnerable Go dependencies (`golang.org/x/crypto` v0.57.0, `golang.org/x/sys` v0.48.0) and verify with `govulncheck` (0 application vulnerabilities).
- [x] **Task:** Remediate static hardcoded dev JWT secret fallback with cryptographically secure 256-bit random key generation (`crypto/rand`) and thread-safe key accessors.
- [x] **Task:** Harden WebSocket endpoint against Cross-Site WebSocket Hijacking (CSWSH) via strict host origin verification in `CheckOrigin`.
- [x] **Task:** Defend against Zip Slip symlink traversal attacks in backup extraction (`os.ModeSymlink` rejection).

### Milestone 5.2: Enhanced Authentication, Session Lifecycle & RFC 6238 TOTP 2FA
- [x] **Task:** Database migration v7 (`user_mfa` and `user_sessions` tables) with user FK cascading and expiry indexing.
- [x] **Task:** Implement sliding-window IP brute-force rate limiter (5 failed attempts, 15m lockout with `Retry-After` header).
- [x] **Task:** Mitigate user enumeration side-channels and timing differences on `/login` with constant-time dummy bcrypt execution and unified 401 responses.
- [x] **Task:** Implement RFC 6238 TOTP multi-factor engine with $\pm 30$s clock drift tolerance, base32 secret encoding, and `otpauth://` URI generator.
- [x] **Task:** Implement 8 emergency single-use backup recovery codes with hashed verification and database consumption.
- [x] **Task:** Implement dual-token session architecture: short-lived access JWT (15m) + rotated refresh tokens (7d) in HttpOnly strict cookies.
- [x] **Task:** Expose REST endpoints: `POST /api/auth/login`, `POST /api/auth/2fa/verify-login`, `POST /api/auth/refresh`, `POST /api/auth/logout`, `GET /api/auth/2fa/status`, `POST /api/auth/2fa/setup`, `POST /api/auth/2fa/enable`, `POST /api/auth/2fa/disable`.
- [x] **Task:** Implement Frontend 2FA challenge on Login page (`Login.tsx`) and 3-step setup wizard on Users page (`Users.tsx`) with real-time QR code rendering for authenticator apps, recovery code download/copy, and verification.

### Milestone 5.3: Java Process Isolation Engine & System Hardening
- [x] **Task:** Implement process isolation engine (`internal/minecraft/sandbox.go`) supporting `none`, `user`, and `bwrap` modes.
- [x] **Task:** Implement Bubblewrap (`bwrap`) unprivileged sandbox profile: read-only root (`--ro-bind / /`), isolated private `/tmp`, dropped capabilities (`--cap-drop ALL`), unshared PID/UTS/IPC namespaces, and automatic masking of sensitive credentials and database files (`paper.db`, `.env`).
- [x] **Task:** Implement POSIX DAC user credential separation via `SysProcAttr.Credential` (`ISOLATION_MODE=user`).
- [x] **Task:** Harden systemd service configuration (`ProtectSystem=strict`, `ProtectHome=read-only`, `NoNewPrivileges=yes`, `PrivateTmp=yes`, `RestrictSUIDSGID=yes`).
- [x] **Task:** Maintain strict $\ge 80\%$ statement test coverage across all internal packages with deterministic pass.
