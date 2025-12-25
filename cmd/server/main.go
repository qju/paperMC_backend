package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"paperMC_backend/internal/api"
	"paperMC_backend/internal/auth"
	"paperMC_backend/internal/config"
	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
	"paperMC_backend/web"
)

func main() {
	cfg := config.Load()
	store, err := database.NewSQLiteStore(cfg.DBName)
	if err != nil {
		log.Fatalf("CRITICAL ERROR, %v", err)
	}
	defer store.Close()
	mcServer := minecraft.NewServer(cfg.WorkDir, cfg.JarFile, cfg.RAM, store)

	// --- BOOTSTRA ADMIN USER ----
	// IF ADMIN_PASS is ser, ensure the user exists
	if cfg.AdminPass != "" {
		_, err := store.GetUser(cfg.AdminUser)
		if err == nil {
			// User likely doesn't exist in database, let's creat it
			log.Printf("[Init] User '%s' not found. Creating...", cfg.AdminUser)
		} else if err == sql.ErrNoRows {
			hashedPass, hashErr := auth.HashPassword(cfg.AdminPass)
			if hashErr != nil {
				log.Printf("[Init] Failed to hash password: %v", hashErr)
			} else {
				adminUser := &database.User{
					Username: cfg.AdminUser,
					Password: hashedPass,
					Role:     "admin",
				}
				if createErr := store.CreateUser(adminUser); createErr != nil {
					log.Printf("[Init] Failed to create AdminUser: %c", createErr)
				} else {
					log.Printf("[Init] AdminUser '%s' ceated successfully!", cfg.AdminUser)
				}
			}
		} else {
			log.Printf("[Init] Error checking for admin user '%v'", err)
		}

	} else {
		log.Printf("[Init] Warning: ADMIN_PASS is mepty. No admin user created")
	}

	mcHandler := api.NewServerHandler(mcServer, store)
	mux := http.NewServeMux()

	// Prepare the forwared Files
	distFS, err := fs.Sub(web.DistFs, "dist")
	if err != nil {
		log.Fatalf("Failed to load embedded frontend: %v", err)
	}

	// Create a standard file server
	fileserver := http.FileServer(http.FS(distFS))

	// --- DEFINES ROUTES ---

	// Public Routes
	mux.HandleFunc("POST /login", mcHandler.Login)

	// Protected Routes in a Map
	// Key = Path, Value = Handler Function
	protectedRoutes := map[string]http.HandlerFunc{
		"GET /status": mcHandler.HandleStatus,
		"GET /logs":   mcHandler.HandleLogs,
		"GET /config": mcHandler.GetConfig,
		"GET /ws":     mcHandler.SocketHandler,

		// Player Manager - WhiteList
		"GET /api/players":    mcHandler.HandleGetPlayers,
		"POST /api/players":   mcHandler.HandleAddPlayer,
		"DELETE /api/players": mcHandler.HandleRemovePlayer,

		// Player Manager - Banned
		"GET /api/players/banned":    mcHandler.HandleGetBanned,
		"POST /api/players/banned":   mcHandler.HandleBanPlayer,
		"DELETE /api/players/banned": mcHandler.HandleUnbanPlayer,

		// Player Manager - Ops
		"GET /api/players/ops":  mcHandler.HandleGetOps,
		"POST /api/players/ops": mcHandler.HandleOpPlayer, // ?action=add|remove

		// Player Manager - Rejected (DB)
		"GET /api/players/rejected":    mcHandler.HandleGetRejected,
		"DELETE /api/players/rejected": mcHandler.HandleDeleteRejected,

		"POST /command":       mcHandler.SendCommand,
		"POST /whitelist_add": mcHandler.WhiteListing,
		"POST /start":         mcHandler.Start,
		"POST /stop":          mcHandler.Stop,
		"POST /config":        mcHandler.PostConfig,
		"POST /update":        mcHandler.HandleUpdate,
	}

	// Register all the protected routes
	for path, handler := range protectedRoutes {
		mux.Handle(path, auth.AuthMiddleware(handler))
	}

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// try to server the requested file (e.g., /assers/style.css)
		path := strings.TrimPrefix(r.URL.Path, "/")

		file, err := distFS.Open(path)
		if err != nil {
			r.URL.Path = "/"
		} else {
			file.Close()
		}
		fileserver.ServeHTTP(w, r)

	}))

	go func() {
		log.Printf("Server starting on port: %s", cfg.Port)
		if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
			log.Fatalf("CRITICAL ERROR, %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	sig := <-c
	fmt.Printf("Receiving Signal [%v]. Shutting down...\n", sig)
	if err := mcServer.Stop(); err != nil {
		log.Printf("Error stopping the server: %v", err)
	}
	fmt.Printf("Server stopped gracefully [%v]", sig)
}
