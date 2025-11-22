# PaperMC Backend

This project is a backend server for managing a PaperMC Minecraft server. It provides a web interface and an API to control the server.

## Project Structure

```
├── .gitignore
├── go.mod
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── api/
│   │   └── api.go
│   └── minecraft/
│       └── server.go
├── paperMC/
└── web/
    └── static/
        └── index.html
```

## Getting Started

To get a local copy up and running follow these simple steps.

### Prerequisites

*   Go programming language
*   A working PaperMC server installation

### Installation

1.  Clone the repo
    ```sh
    git clone https://example.com/your_project.git
    ```
2.  Install Go packages
    ```sh
    go mod download
    ```

## Usage

To run the server, execute the following command:

```sh
go run cmd/server/main.go
```

The web interface will be available at `http://localhost:8080`.
