package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"paperMC_backend/internal/api"
	"paperMC_backend/internal/config"
	"paperMC_backend/internal/minecraft"
	"syscall"
)

func main() {
	// Keeping pointer
	cfg := config.Load()
	mcServer := minecraft.NewServer(cfg.WorkDir, cfg.JarFile, cfg.RAM)
	mcHandler := api.NewServerHandler(mcServer)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", mcHandler.GetStatus)
	mux.HandleFunc("GET /logs", mcHandler.HandleLogs)
	mux.HandleFunc("GET /config", mcHandler.GetConfig)

	mux.HandleFunc("POST /command", mcHandler.SendCommand)
	mux.HandleFunc("POST /whitelist_add", mcHandler.Whitelisting)
	mux.HandleFunc("POST /start", mcHandler.Start)
	mux.HandleFunc("POST /stop", mcHandler.Stop)
	mux.HandleFunc("POST /config", mcHandler.PostConfig)
	mux.HandleFunc("POST /update", mcHandler.HandleUpdate)

	mux.Handle("/", http.FileServer(http.Dir("./web/static")))

	protectedMux := mcHandler.BasicAuth(mux, cfg.AdminUser, cfg.AdminPass)
	go func() {
		if err := http.ListenAndServe(":"+cfg.Port, protectedMux); err != nil {
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
