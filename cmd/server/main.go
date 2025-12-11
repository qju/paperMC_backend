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
	// Keeping pointer
	cfg := config.Load()
	mcServer := minecraft.NewServer(cfg.WorkDir, cfg.JarFile, cfg.RAM)
	store, err := database.NewSQLiteStore("paper.db")
	if err != nil {
		log.Fatalf("CRITICAL ERROR, %v", err)
	}
	defer store.Close()

	mcHandler := api.NewServerHandler(mcServer, store)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", mcHandler.Login)
	mux.Handle("GET /status", auth.AuthMiddleware(http.HandlerFunc(mcHandler.GetStatus)))
	mux.Handle("GET /logs", auth.AuthMiddleware(http.HandlerFunc(mcHandler.HandleLogs)))
	mux.Handle("GET /config", auth.AuthMiddleware(http.HandlerFunc(mcHandler.GetConfig)))

	mux.Handle("POST /command", auth.AuthMiddleware(http.HandlerFunc(mcHandler.SendCommand)))
	mux.Handle("POST /whitelist_add", auth.AuthMiddleware(http.HandlerFunc(mcHandler.Whitelisting)))
	mux.Handle("POST /start", auth.AuthMiddleware(http.HandlerFunc(mcHandler.Start)))
	mux.Handle("POST /stop", auth.AuthMiddleware(http.HandlerFunc(mcHandler.Stop)))
	mux.Handle("POST /config", auth.AuthMiddleware(http.HandlerFunc(mcHandler.PostConfig)))
	mux.Handle("POST /update", auth.AuthMiddleware(http.HandlerFunc(mcHandler.HandleUpdate)))

	mux.Handle("/", http.FileServer(http.Dir("./web/static")))

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
