package main

import (
	"context"
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
	"time"

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
	if cfg.IsolationMode != "" && cfg.IsolationMode != "none" {
		isoCfg := minecraft.DefaultIsolationConfig()
		isoCfg.Mode = minecraft.IsolationMode(cfg.IsolationMode)
		if cfg.MinecraftUser != "" {
			isoCfg.MinecraftUser = cfg.MinecraftUser
		}
		mcServer.SetIsolationConfig(isoCfg)
		log.Printf("[Security] Java process isolation active: mode=%s", cfg.IsolationMode)
	}

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
	mux.HandleFunc("POST /api/auth/login", mcHandler.Login)
	mux.HandleFunc("POST /api/auth/2fa/verify-login", mcHandler.Handle2FAVerifyLogin)
	mux.HandleFunc("POST /api/auth/refresh", mcHandler.HandleRefreshToken)
	mux.HandleFunc("POST /api/auth/logout", mcHandler.HandleLogout)

	registerProtectedRoutes(mux, mcHandler)

	mux.Handle("/", spaHandler(distFS, fileserver))

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("Server starting on port: %s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("CRITICAL ERROR, %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	sig := <-c
	fmt.Printf("Receiving Signal [%v]. Shutting down...\n", sig)
	mcHandler.Close()
	if err := mcServer.Stop(); err != nil {
		log.Printf("Error stopping the server: %v", err)
	} else if !mcServer.WaitForStop(30 * time.Second) {
		log.Printf("Timed out waiting for Minecraft server to stop cleanly, forcing kill...")
		if killErr := mcServer.Kill(); killErr != nil {
			log.Printf("Error force-killing server: %v", killErr)
		}
		mcServer.WaitForStop(5 * time.Second)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
		_ = httpServer.Close()
	}
	fmt.Printf("Server stopped gracefully [%v]\n", sig)
}

type roleRoute struct {
	pattern string
	handler http.HandlerFunc
	roles   []string
}

// registerProtectedRoutes registers all authenticated API and WebSocket routes with
// role-based access control across viewer, operator, and admin roles.
func registerProtectedRoutes(mux *http.ServeMux, mcHandler *api.Handler) {
	viewerAndAbove := []string{auth.RoleViewer, auth.RoleOperator, auth.RoleAdmin}
	operatorAndAbove := []string{auth.RoleOperator, auth.RoleAdmin}
	adminOnly := []string{auth.RoleAdmin}

	routes := []roleRoute{
		// Read-only telemetry, logs, config & self-service 2FA (viewer+)
		{"GET /status", mcHandler.HandleStatus, viewerAndAbove},
		{"GET /logs", mcHandler.HandleLogs, viewerAndAbove},
		{"GET /config", mcHandler.GetConfig, viewerAndAbove},
		{"GET /ws", mcHandler.SocketHandler, viewerAndAbove},

		{"GET /api/players", mcHandler.HandleGetPlayers, viewerAndAbove},
		{"GET /api/players/banned", mcHandler.HandleGetBanned, viewerAndAbove},
		{"GET /api/players/ops", mcHandler.HandleGetOps, viewerAndAbove},
		{"GET /api/players/rejected", mcHandler.HandleGetRejected, viewerAndAbove},

		{"GET /api/updater/versions", mcHandler.HandleGetVersions, viewerAndAbove},
		{"GET /api/updater/check", mcHandler.HandleCheckUpdate, viewerAndAbove},
		{"GET /api/worlds", mcHandler.HandleGetWorlds, viewerAndAbove},

		{"GET /api/auth/2fa/status", mcHandler.Handle2FAStatus, viewerAndAbove},
		{"POST /api/auth/2fa/setup", mcHandler.Handle2FASetup, viewerAndAbove},
		{"POST /api/auth/2fa/enable", mcHandler.Handle2FAEnable, viewerAndAbove},
		{"POST /api/auth/2fa/disable", mcHandler.Handle2FADisable, viewerAndAbove},

		{"GET /api/backups", mcHandler.HandleGetBackups, viewerAndAbove},
		{"GET /api/backups/download", mcHandler.HandleDownloadBackup, viewerAndAbove},
		{"GET /api/schedules", mcHandler.HandleListSchedules, viewerAndAbove},
		{"GET /api/schedules/logs", mcHandler.HandleGetScheduleLogs, viewerAndAbove},

		{"GET /api/plugins", mcHandler.HandleGetPlugins, viewerAndAbove},
		{"GET /api/plugins/geyser/status", mcHandler.HandleGetGeyserStatus, viewerAndAbove},
		{"GET /api/plugins/market/search", mcHandler.HandleSearchMarketPlugins, viewerAndAbove},

		{"GET /api/flags", mcHandler.HandleGetFlags, viewerAndAbove},
		{"GET /api/flags/presets", mcHandler.HandleGetFlagPresets, viewerAndAbove},
		{"GET /api/audit", mcHandler.HandleGetAuditLogs, viewerAndAbove},

		{"GET /api/crash", mcHandler.HandleListCrashReports, viewerAndAbove},
		{"GET /api/crash/{id}", mcHandler.HandleGetCrashReport, viewerAndAbove},
		{"GET /api/crash/ai-config", mcHandler.HandleGetAISettings, viewerAndAbove},

		{"GET /api/profiler/reports", mcHandler.HandleListProfilerReports, viewerAndAbove},
		{"GET /api/profiler/reports/{id}", mcHandler.HandleGetProfilerReport, viewerAndAbove},

		// Server lifecycle, players, worlds, backups, schedules, crash/profiler operations (operator+)
		{"POST /start", mcHandler.Start, operatorAndAbove},
		{"POST /stop", mcHandler.Stop, operatorAndAbove},
		{"POST /whitelist_add", mcHandler.WhiteListing, operatorAndAbove},

		{"POST /api/players", mcHandler.HandleAddPlayer, operatorAndAbove},
		{"DELETE /api/players", mcHandler.HandleRemovePlayer, operatorAndAbove},
		{"POST /api/players/banned", mcHandler.HandleBanPlayer, operatorAndAbove},
		{"DELETE /api/players/banned", mcHandler.HandleUnbanPlayer, operatorAndAbove},
		{"POST /api/players/ops", mcHandler.HandleOpPlayer, operatorAndAbove},
		{"DELETE /api/players/rejected", mcHandler.HandleDeleteRejected, operatorAndAbove},

		{"POST /api/worlds/active", mcHandler.HandleSetActiveWorld, operatorAndAbove},
		{"POST /api/worlds/create", mcHandler.HandleCreateWorld, operatorAndAbove},
		{"POST /api/worlds/duplicate", mcHandler.HandleDuplicateWorld, operatorAndAbove},
		{"DELETE /api/worlds", mcHandler.HandleDeleteWorld, operatorAndAbove},

		{"POST /api/backups/create", mcHandler.HandleCreateBackup, operatorAndAbove},
		{"POST /api/backups/restore", mcHandler.HandleRestoreBackup, operatorAndAbove},
		{"DELETE /api/backups", mcHandler.HandleDeleteBackup, operatorAndAbove},

		{"POST /api/schedules", mcHandler.HandleCreateSchedule, operatorAndAbove},
		{"PUT /api/schedules", mcHandler.HandleUpdateSchedule, operatorAndAbove},
		{"POST /api/schedules/toggle", mcHandler.HandleToggleSchedule, operatorAndAbove},
		{"POST /api/schedules/run", mcHandler.HandleRunSchedule, operatorAndAbove},
		{"DELETE /api/schedules", mcHandler.HandleDeleteSchedule, operatorAndAbove},
		{"DELETE /api/schedules/logs", mcHandler.HandleClearScheduleLogs, operatorAndAbove},

		{"POST /api/crash/analyze", mcHandler.HandleAnalyzeCrash, operatorAndAbove},
		{"DELETE /api/crash", mcHandler.HandleDeleteCrashReport, operatorAndAbove},
		{"POST /api/crash/ai-explain", mcHandler.HandleAIExplainCrash, operatorAndAbove},

		{"DELETE /api/profiler/reports", mcHandler.HandleDeleteProfilerReport, operatorAndAbove},
		{"DELETE /api/profiler/reports/{id}", mcHandler.HandleDeleteProfilerReport, operatorAndAbove},
		{"POST /api/profiler/health", mcHandler.HandleProfilerHealth, operatorAndAbove},
		{"POST /api/profiler/trigger", mcHandler.HandleTriggerProfiler, operatorAndAbove},

		// Privileged system, config, console command, updater, users, plugins, flags & audit admin routes
		{"POST /command", mcHandler.SendCommand, adminOnly},
		{"POST /kill", mcHandler.Kill, adminOnly},
		{"POST /config", mcHandler.PostConfig, adminOnly},
		{"POST /update", mcHandler.HandleUpdate, adminOnly},
		{"POST /api/updater/apply", mcHandler.HandleUpdate, adminOnly},

		{"GET /api/users", mcHandler.HandleListUsers, adminOnly},
		{"POST /api/users", mcHandler.HandleCreateUser, adminOnly},
		{"PUT /api/users/password", mcHandler.HandleUpdatePassword, adminOnly},
		{"DELETE /api/users", mcHandler.HandleDeleteUser, adminOnly},
		{"POST /api/users/reset-2fa", mcHandler.HandleAdminReset2FA, adminOnly},

		{"POST /api/plugins/toggle", mcHandler.HandleTogglePlugin, adminOnly},
		{"DELETE /api/plugins", mcHandler.HandleDeletePlugin, adminOnly},
		{"POST /api/plugins/upload", mcHandler.HandleUploadPlugin, adminOnly},
		{"POST /api/plugins/geyser/update", mcHandler.HandleUpdateGeyser, adminOnly},
		{"POST /api/plugins/market/install", mcHandler.HandleInstallMarketPlugin, adminOnly},

		{"POST /api/flags", mcHandler.HandleSaveFlags, adminOnly},
		{"DELETE /api/audit", mcHandler.HandleClearAuditLogs, adminOnly},
		{"POST /api/crash/ai-config", mcHandler.HandleSaveAISettings, adminOnly},
	}

	for _, rt := range routes {
		mux.Handle(rt.pattern, auth.AuthMiddleware(auth.RequireRole(rt.roles...)(rt.handler)))
	}
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
