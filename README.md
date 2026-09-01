# Lodestone - High-Performance Minecraft Server Manager

[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://golang.org/) 
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Lodestone** is a high-performance, single-binary management dashboard and backend controller for PaperMC and modern Minecraft servers. Built in pure Go with zero runtime CGO dependencies, it provides real-time WebSocket console streaming, player access control, rich world diagnostics with GZIP NBT parsing, automated PaperMC Fill v3 updates, Web user management, and an embedded React SPA interface.

## Features

- **Lifecycle & Resource Monitoring:** Start, stop, and kill the Minecraft server process with live CPU and RSS RAM metrics (`gopsutil`).
- **Real-Time Console:** Bidirectional WebSocket streaming with centralized broadcast hub and ANSI terminal emulation.
- **Player Management:** Whitelist, Ban, Operator controls, rejected connection intelligence, live search, and pagination.
- **Rich World Diagnostics:** Pure-Go zero-dependency GZIP binary NBT parser (`ReadLevelDat`), Modern (26.1+ `world/dimensions/`) and Legacy dimension discovery, safe duplication, and deletion.
- **PaperMC Fill v3 Updater:** Version family selector (26.2, 1.21, etc.), latest stable build detection, and streaming download with on-the-fly SHA-256 validation.
- **Web User Administration:** Multi-user authentication control panel with bcrypt hashing, password rotation, and role management.
- **Atomic SQLite Migration Engine:** Versioned schema migrations using native `PRAGMA user_version` with automatic production database adoption.
- **Server Configuration Editor:** Categorized visual controls (General, Gameplay, Security, Performance, RCON) and raw editor with comment preservation for `server.properties`.
- **Embedded SPA UI:** Modern dark glassmorphic React + TypeScript dashboard embedded via `go:embed`.


## Getting Started

### Prerequisites

- Go 1.22 or later
- Java 21 or later (to execute PaperMC)
- Node.js (only required if building/modifying the frontend)

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
| `JWT_SECRET` | Secret key for signing JWT tokens. | Dev fallback |

### Running the Server

1. Set the required `ADMIN_PASS` environment variable:
   ```sh
   export ADMIN_PASS="your-secret-password"
   ```
2. Start the server:
   ```sh
   go run cmd/server/main.go
   ```
3. Open `http://localhost:8080` in your browser.

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

## Project Status

- [x] Core Process Manager & Lifecycle Engine
- [x] Centralized WebSocket Console Hub & ANSI Rendering
- [x] Smart Player Access Control (Whitelist, Bans, Ops, Rejections)
- [x] Rich World Diagnostics & Pure-Go GZIP NBT Parser
- [x] PaperMC Fill v3 API & Auto-Updater with SHA-256 Validation
- [x] Web User Administration Control Panel
- [x] Atomic SQLite Migration Engine (`PRAGMA user_version`)
- [x] Visual Server Configuration Editor (`server.properties`)
- [ ] Milestone 2.2: Backup Engine & Snapshots
- [ ] Milestone 2.5: Cron Task Scheduler
- [ ] Milestone 3.2: Modrinth/Hangar Plugin Manager


## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
