package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"paperMC_backend/internal/api"
	"paperMC_backend/internal/auth"
	"paperMC_backend/internal/config"
	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
)

func main() {
	cfg := config.Load()
	mcServer := minecraft.NewServer(cfg.WorkDir, cfg.JarFile, cfg.RAM)
	store, err := database.NewSQLiteStore(cfg.DBName)
	if err != nil {
		log.Fatalf("CRITICAL ERROR, %v", err)
	}
	defer store.Close()

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

	// Public Routes
	mux.HandleFunc("POST /login", mcHandler.Login)
	mux.Handle("/", http.FileServer(http.Dir("./web/static")))
	mux.Handle("login/", http.FileServer(http.Dir("./web/static/login")))

	// Protected Routes in a Map
	// Key = Path, Value = Handler Function
	protectedRoutes := map[string]http.HandlerFunc{
		"GET /status": mcHandler.GetStatus,
		"GET /logs":   mcHandler.HandleLogs,
		"GET /config": mcHandler.GetConfig,
		// The webSocket Endpoint
		"GET /ws": mcHandler.SocketHandler,

		"POST /command":       mcHandler.SendCommand,
		"POST /whitelist_add": mcHandler.Whitelisting,
		"POST /start":         mcHandler.Start,
		"POST /stop":          mcHandler.Stop,
		"POST /config":        mcHandler.PostConfig,
		"POST /update":        mcHandler.HandleUpdate,
	}

	// Register all the protected routes
	for path, handler := range protectedRoutes {
		mux.Handle(path, auth.AuthMiddleware(handler))
	}

	go func() {
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
