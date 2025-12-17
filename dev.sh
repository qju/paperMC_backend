#!/bin/bash

# Function to kill the Go process when the script exits
cleanup() {
    echo -e "\n🛑 Shutting down environment..."
    kill $BACKEND_PID 2>/dev/null
}

# Register the cleanup function to run on exit (Ctrl+C)
trap cleanup EXIT

echo "🚀 Starting PaperMC Manager Dev Environment..."

# 1. Start Backend in the background
echo "🐘 Booting Go Backend..."
go run cmd/server/main.go &
BACKEND_PID=$! # Save the Process ID

# Wait a moment for Go to start (optional, just for cleaner logs)
sleep 2

# 2. Start Frontend in the foreground
echo "⚛️  Booting React Frontend..."
cd web
npm run dev

# The script hangs here while the frontend runs.
# When you press Ctrl+C, the 'trap' triggers and kills the backend.
