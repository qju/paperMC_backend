package main

import (
	"database/sql"
	"errors"
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

	// --- BOOTSTRAP ADMIN USER ----
	if cfg.AdminPass != "" {
		_, err := store.GetUser(cfg.AdminUser)
		if err == nil {
			log.Printf("[Init] AdminUser '%s' already exists.", cfg.AdminUser)
		} else if errors.Is(err, sql.ErrNoRows) {
			log.Printf("[Init] AdminUser '%s' not found. Creating...", cfg.AdminUser)
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
					log.Printf("[Init] Failed to create AdminUser: %v", createErr)
				} else {
					log.Printf("[Init] AdminUser '%s' created successfully!", cfg.AdminUser)
				}
			}
		} else {
			log.Printf("[Init] Error checking for admin user: %v", err)
		}
	} else {
		log.Printf("[Init] Warning: ADMIN_PASS is empty. No admin user created")
	}

	mcHandler := api.NewServerHandler(mcServer, store)
	if err := mcHandler.StartScheduler(); err != nil {
		log.Printf("[Scheduler] Warning starting scheduler: %v", err)
	}
	mux := http.NewServeMux()

	// Prepare the forwarded Files
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
		"POST /kill":          mcHandler.Kill,
		"POST /config":        mcHandler.PostConfig,
		"POST /update":        mcHandler.HandleUpdate,

		// Updater
		"GET /api/updater/versions": mcHandler.HandleGetVersions,
		"GET /api/updater/check":    mcHandler.HandleCheckUpdate,
		"POST /api/updater/apply":   mcHandler.HandleUpdate,

		// Worlds
		"GET /api/worlds":            mcHandler.HandleGetWorlds,
		"POST /api/worlds/active":    mcHandler.HandleSetActiveWorld,
		"POST /api/worlds/create":    mcHandler.HandleCreateWorld,
		"POST /api/worlds/duplicate": mcHandler.HandleDuplicateWorld,
		"DELETE /api/worlds":         mcHandler.HandleDeleteWorld,

		// User Control (Web UI Users)
		"GET /api/users":          mcHandler.HandleListUsers,
		"POST /api/users":         mcHandler.HandleCreateUser,
		"PUT /api/users/password": mcHandler.HandleUpdatePassword,
		"DELETE /api/users":       mcHandler.HandleDeleteUser,

		// Backups
		"GET /api/backups":          mcHandler.HandleGetBackups,
		"POST /api/backups/create":  mcHandler.HandleCreateBackup,
		"GET /api/backups/download": mcHandler.HandleDownloadBackup,
		"POST /api/backups/restore": mcHandler.HandleRestoreBackup,
		"DELETE /api/backups":       mcHandler.HandleDeleteBackup,

		// Schedules & Execution Logs
		"GET /api/schedules":        mcHandler.HandleListSchedules,
		"POST /api/schedules":       mcHandler.HandleCreateSchedule,
		"PUT /api/schedules":        mcHandler.HandleUpdateSchedule,
		"POST /api/schedules/toggle": mcHandler.HandleToggleSchedule,
		"POST /api/schedules/run":   mcHandler.HandleRunSchedule,
		"DELETE /api/schedules":     mcHandler.HandleDeleteSchedule,
		"GET /api/schedules/logs":   mcHandler.HandleGetScheduleLogs,
		"DELETE /api/schedules/logs": mcHandler.HandleClearScheduleLogs,

		// Plugins & Bedrock Bridge
		"GET /api/plugins":                 mcHandler.HandleGetPlugins,
		"POST /api/plugins/toggle":         mcHandler.HandleTogglePlugin,
		"DELETE /api/plugins":              mcHandler.HandleDeletePlugin,
		"POST /api/plugins/upload":         mcHandler.HandleUploadPlugin,
		"GET /api/plugins/geyser/status":   mcHandler.HandleGetGeyserStatus,
		"POST /api/plugins/geyser/update":  mcHandler.HandleUpdateGeyser,
		"GET /api/plugins/market/search":   mcHandler.HandleSearchMarketPlugins,
		"POST /api/plugins/market/install": mcHandler.HandleInstallMarketPlugin,

		// Smart Flags & JVM Tuning
		"GET /api/flags":         mcHandler.HandleGetFlags,
		"POST /api/flags":        mcHandler.HandleSaveFlags,
		"GET /api/flags/presets": mcHandler.HandleGetFlagPresets,
	}

	// Register all the protected routes
	for path, handler := range protectedRoutes {
		mux.Handle(path, auth.AuthMiddleware(handler))
	}

	mux.Handle("/", spaHandler(distFS, fileserver))

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
	mcHandler.StopScheduler()
	if err := mcServer.Stop(); err != nil {
		log.Printf("Error stopping the server: %v", err)
	}
	fmt.Printf("Server stopped gracefully [%v]", sig)
}

// spaHandler returns an http.HandlerFunc serving static files for the SPA,
// while ensuring any unmatched /api routes return JSON 404 instead of index.html.
func spaHandler(distFS fs.FS, fileserver http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"API route not found"}`))
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		file, err := distFS.Open(path)
		if err != nil {
			r.URL.Path = "/"
		} else {
			file.Close()
		}
		fileserver.ServeHTTP(w, r)
	}
}
