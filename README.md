# PaperMC Backend

[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://golang.org/) 
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A backend server and management dashboard for PaperMC Minecraft servers. It provides a real-time WebSocket console, process lifecycle management, player access controls, multi-world management, and an embedded React SPA interface.

## Features

- Start, stop, and kill the Minecraft server process.
- Bidirectional real-time console with WebSocket broadcast hub.
- Player management: Whitelist, Ban, Operator status, and unauthorized connection intelligence.
- Java UUID (Mojang API) and Bedrock XUID (GeyserMC API) resolution.
- Multi-world creation and switching with automated restart.
- JWT-based authentication and SQLite persistence.
- Embedded modern React + TypeScript dashboard.
- Single binary with zero external runtime dependencies (CGO-free).

## Getting Started

### Prerequisites

- Go 1.22 or later
- Java 21 or later
- Node.js (for frontend development/builds)
- A PaperMC server JAR file

### Installation

1. Clone the repository:
   ```sh
   git clone <repository-url>
   ```
2. Navigate to the project directory:
   ```sh
   cd paperMC_backend
   ```
3. Place your PaperMC server JAR file in the working directory (default: `./paperMS` or configured via `MC_WORKDIR`).

### Configuration

The application is configured using environment variables:

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PORT` | The port for the web server. | `8080` |
| `MC_WORKDIR` | The working directory for the Minecraft server. | `./paperMS` |
| `JAR_FILE` | The name of the server JAR file. | `server.jar` |
| `RAM` | The amount of RAM to allocate to the server. | `8G` |
| `DBNAME` | SQLite database file path. | `paper.db` |
| `ADMIN_USER` | Initial administrative username. | `admin` |
| `ADMIN_PASS` | Initial administrative password. | **Required** |
| `JWT_SECRET` | Secret key for signing JWT tokens. | Dev fallback |

### Running the Server

1. Set the required `ADMIN_PASS` environment variable:
   ```sh
   export ADMIN_PASS="your-secret-password"
   ```
2. Run the application:
   ```sh
   go run cmd/server/main.go
   ```
The manager will be accessible at `http://localhost:8080`.

## API Endpoints

### Public Endpoints
- `POST /login`: Authenticate and obtain a JWT bearer token.
  - **Body:** `{"username": "admin", "password": "your-password"}`

### Protected Endpoints (Requires `Authorization: Bearer <token>` or `?token=<token>`)
- `GET /status`: Retrieve real-time server vitals (status, CPU, RAM RSS, online player list, active world).
- `GET /ws`: WebSocket endpoint for real-time console streaming and interactive command execution.
- `POST /command`: Send a console command (`{"command": "..."}`).
- `POST /start`: Start the Minecraft server.
- `POST /stop`: Gracefully stop the Minecraft server.
- `POST /kill`: Force terminate the Minecraft server process.
- `GET /config`: Load `server.properties` as JSON.
- `POST /config`: Update `server.properties` while preserving comments and layout.
- `POST /update`: Check and perform atomic PaperMC updates.

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
- `DELETE /api/players/rejected?username=...`: Dismiss rejected player record.

### World Management Endpoints
- `GET /api/worlds`: List active world and available inactive worlds.
- `POST /api/worlds/active`: Switch active world or create a new world (`{"world_name": "...", "seed": "..."}`).

## Web Interface

The web interface is built with React, Vite, and Tailwind CSS, and is embedded directly into the Go binary at compile time via `go:embed`.

## Project Status

- [x] Core Process Manager
- [x] WebSocket Console Hub & Concurrency Hardening
- [x] Smart Whitelister & Player Management
- [x] Multi-World Switching & Creation
- [x] JWT Authentication & SQLite Persistence
- [ ] Config Editor UI
- [ ] Auto-Updater V3 Hardening
- [ ] Backup Engine
- [ ] Smart Flag Manager

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.