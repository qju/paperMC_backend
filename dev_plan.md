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
- [x] **Task:** Minecraft engine health indicators (dynamic real-time TPS & MSPT tracking via Paper/Spigot `tps` / `mspt` poll parsing and lag warning detection with automated poll suppression, JVM threads, and live uptime counter).
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

### Milestone 3.1: Deep Integration
- [ ] **Task:** **Timings Viewer:** Parse `timings report` output.
- [ ] **Task:** **Smart Flags:** Implement a preset manager for Aikaras flags based on detected system RAM.

### Milestone 3.2: Plugin Management & Bedrock Bridge (Geyser & Floodgate)
- [x] **Task:** Implement `internal/plugins` scanner and pure-Go ZIP YAML parser for `plugin.yml` and `paper-plugin.yml`.
- [x] **Task:** Implement plugin operations: enable/disable toggle (`.jar` <-> `.jar.disabled`), safe deletion, and `.jar` multipart upload.
- [x] **Task:** Build dedicated **GeyserMC & Floodgate Bedrock Bridge** client (`internal/plugins/geyser.go`) tracking installed vs upstream builds, SHA-256 verification, and Bedrock client compatibility ranges.
- [x] **Task:** Integrate **Modrinth v2 API** client (`internal/plugins/modrinth.go`) for search and one-click installation with SHA-512/SHA-1 verification.
- [x] **Task:** Implement REST endpoints (`GET /api/plugins`, `POST /api/plugins/toggle`, `DELETE /api/plugins`, `POST /api/plugins/upload`, `GET /api/plugins/geyser/status`, `POST /api/plugins/geyser/update`, `GET /api/plugins/market/search`, `POST /api/plugins/market/install`).
- [x] **Task:** Build comprehensive unit tests maintaining $\ge 80\%$ test coverage.

### Milestone 3.3: Observability
- [ ] **Task:** **Audit Logs:** Log every API action to DB (Who did what, when).
- [ ] **Task:** **Crash Analyst:** Basic heuristic analysis of stack traces.

---

## 🎨 Phase 4: Frontend Modernization (React + Vite)
*Focus: High-performance, responsive dark glassmorphic UI.*

### Milestone 4.1: The Build Pipeline & SPA Shell
- [x] **Task:** Initialize React + Vite project in `web/`.
- [x] **Task:** Configure `go:embed` to bundle `web/dist/` into single Go binary.
- [x] **Task:** Vite proxy configuration for local development.
- [x] **Task:** High-contrast login page with branding and vector SVG favicon.

### Milestone 4.2: Feature Dashboards
- [x] **Task:** **Console:** Live terminal with ANSI color rendering, command input, and auto-scroll.
- [x] **Task:** **Player Manager:** Whitelist, Ban, Operator status, rejected connection history, real-time search, filter tabs, scroll container, and selectable pagination (10/25/50/100 per page).
- [x] **Task:** **World Manager:** Active world spotlight card, dimension badges, disk size calculator, level.dat NBT metadata, world creation, cloning, and deletion.
- [x] **Task:** **Updates & Versions:** Major release group selector, latest build detector, changelog viewer, and one-click upgrade button.
- [x] **Task:** **Web Users:** Operator account table, user creation modal, password reset modal, and safety guards.
- [x] **Task:** **Backups Page:** Backup listing, snapshot button, download links, and restore modal.
- [x] **Task:** **Config Editor:** Visual editor for `server.properties` with categorized settings (General, Gameplay, Security, Performance, RCON), search filter, and raw editor toggle with comment preservation.
- [x] **Task:** **Schedules & Logs Page:** Active schedules manager with visual presets, status toggle, Run Now trigger, and searchable execution audit log table.
- [x] **Task:** **Plugins & Bedrock Bridge Dashboard:** Installed plugins inspector with enable/disable toggle, file upload, Modrinth marketplace browser, and dedicated Geyser/Floodgate Bedrock compatibility hub with 1-click update.
