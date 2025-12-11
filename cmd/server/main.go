package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"paperMC_backend/internal/api"
	"paperMC_backend/internal/auth"
	"paperMC_backend/internal/config"
	"paperMC_backend/internal/database"
	"paperMC_backend/internal/minecraft"
	"syscall"
)

func main() {
	cfg := config.Load()
	mcServer := minecraft.NewServer(cfg.WorkDir, cfg.JarFile, cfg.RAM)
	store, err := database.NewSQLiteStore(cfg.DbName)
	if err != nil {
		log.Fatalf("CRITICAL ERROR, %v", err)
	}
	defer store.Close()

	mcHandler := api.NewServerHandler(mcServer, store)

	mux := http.NewServeMux()

	// Public Routes
	mux.HandleFunc("POST /login", mcHandler.Login)
	mux.Handle("/", http.FileServer(http.Dir("./web/static")))

	// Protected Routes in a Map
	// Key = Path, Value = Handler Function
	protectedRoutes := map[string]http.HandlerFunc{
		"GET /status": mcHandler.GetStatus,
		"GET /logs":   mcHandler.HandleLogs,
		"GET /config": mcHandler.GetConfig,

		"POST /command":       mcHandler.SendCommand,
		"POST /whitelist_add": mcHandler.Whitelisting,
		"POST /start":         mcHandler.Start,
		"POST /stop":          mcHandler.Stop,
		"POST /config":        mcHandler.PostConfig,
		"POST /update":        mcHandler.HandleUpdate,
	}

	for path, handler := range protectedRoutes {
		mux.Handle(path, auth.AuthMiddleware(http.HandlerFunc(handler)))
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
