# World Management Implementation Plan

## Overview
This document outlines the plan to implement World Management (creation and switching) for the PaperMC backend and frontend.

## 1. Backend Changes

### 1.1 New Endpoints in `internal/api/api.go`
We will introduce a new set of API routes to handle world management:
- **`GET /api/worlds`**: Scans the Minecraft working directory (e.g., `paperMS/`) for existing worlds. A directory is considered a valid world if it contains a `level.dat` file. It will also return which world is currently active by reading `server.properties`.
- **`POST /api/worlds/active`**: Switches the active world.
    - Accepts a JSON payload: `{"world_name": "new_world"}`.
    - Updates the `level-name` property in `server.properties`.
    - Automatically stops and restarts the server if it is currently running to apply the new world.

### 1.2 Configuration Updates (`internal/config/properties.go`)
- Ensure we can reliably read and update the `level-name` property.
- Add a helper function to retrieve just the active world name.

### 1.3 File System Checks
- Add logic to scan directories and detect `level.dat` inside the server working directory to list available worlds.

## 2. Frontend Changes

### 2.1 API Integration
- Add functions to interact with the new backend endpoints (`GET /api/worlds`, `POST /api/worlds/active`).

### 2.2 New UI Component: Worlds Page
- Create a new page: `web/src/pages/Worlds.tsx`.
- Add a route in `web/src/App.tsx` (`/worlds`).
- Add a navigation link in the `DashboardLayout.tsx` sidebar.

### 2.3 User Interface Details
- **List Worlds**: Display a list of available worlds fetched from the backend.
- **Active World Indicator**: Visually highlight the currently active world.
- **Switch World**: Provide a button next to each inactive world to make it active. This will show a confirmation dialog warning the user that the server will restart.
- **Create World**: Provide a text input and a "Create & Switch" button. This will send a request to set the active world to a new name. The Minecraft server will automatically generate the required files on the next startup.

## 3. Step-by-Step Execution
1. Implement directory scanning logic in the backend to find worlds.
2. Create the backend HTTP handlers for listing and switching worlds.
3. Hook up the new routes in the main server setup (`cmd/server/main.go` and `internal/api/api.go`).
4. Develop the frontend `Worlds.tsx` page with the required state management.
5. Update frontend routing and navigation.
6. Test creating a new world, verifying that `server.properties` updates and the server restarts.
7. Test switching back to the old world.