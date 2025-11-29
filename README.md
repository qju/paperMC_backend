# PaperMC Backend

[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://golang.org/) 
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A backend server for managing a PaperMC Minecraft server. It provides a simple HTTP API to start, stop, and interact with the server, as well as a web interface for basic server management.

## Features

- Start and stop the Minecraft server.
- Send commands to the server console.
- View server logs in real-time.
- Basic web interface for server control.
- Protected API endpoints with basic authentication.
- Docker support for easy deployment.

## Getting Started

### Prerequisites

- Go 1.22 or later
- Java 21 or later
- A PaperMC server JAR file

### Installation

1.  Clone the repository:
    ```sh
    git clone <repository-url>
    ```
2.  Navigate to the project directory:
    ```sh
    cd paperMC_backend
    ```
3.  Place your PaperMC server JAR file in the `paperMC` directory. You may need to create this directory. By default, the application will look for `server.jar`.

### Configuration

The application is configured using environment variables:

| Variable       | Description                                     | Default          |
| -------------- | ----------------------------------------------- | ---------------- |
| `PORT`         | The port for the web server.                    | `8080`           |
| `MC_WORKDIR`   | The working directory for the Minecraft server. | `./paperMC`      |
| `JAR_FILE`     | The name of the server JAR file.                | `server.jar`     |
| `RAM`          | The amount of RAM to allocate to the server.    | `2048M`          |
| `ADMIN_USER`   | The username for basic authentication.          | `admin`          |
| `ADMIN_PASS`   | The password for basic authentication.          | **Required**     |

### Running the server

1.  Set the required `ADMIN_PASS` environment variable:
    ```sh
    export ADMIN_PASS="your-secret-password"
    ```
2.  Run the application:
    ```sh
    go run cmd/server/main.go
    ```
The server will be accessible at `http://localhost:8080`.

## API Endpoints

All endpoints are protected by basic authentication.

- `GET /status`: Get the current status of the server.
- `GET /logs`: Stream server logs using Server-Sent Events.
- `POST /command`: Send a command to the server.
    - **Body:** `{"command": "your-command"}`
- `POST /start`: Start the Minecraft server.
- `POST /stop`: Stop the Minecraft server.

## Docker Deployment

The project includes a `dockerfile` for containerized deployment.

1.  Build the Docker image:
    ```sh
    docker build -t papermc-backend .
    ```
2.  Run the Docker container:
    ```sh
    docker run -d \
      -p 8080:8080 \
      -p 25565:25565 \
      -v ./paperMC:/app/paperMC \
      -e ADMIN_PASS="your-secret-password" \
      --name papermc-backend \
      papermc-backend
    ```
This will start the backend server and mount the `paperMC` directory from your host to the container, persisting your server data.

## Web Interface

A simple web interface is available at the root URL (`/`) to control the server.

## Project status

[x] Core Process Manager

[x] Log Streaming (SSE)

[x] Smart Whitelister + model popup

[ ] Modal Aesthetics Upgrade

[ ] Config Editor

[ ] Auto-Updater

[ ] Backup System

[ ] Secure Auth

[ ] Smart Flag Manager

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.