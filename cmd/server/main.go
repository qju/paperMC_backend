package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"paperMC_backend/internal/api"
	"paperMC_backend/internal/minecraft"
)

func main() {
	// Keeping pointer
	mcServer := minecraft.NewServer("./paperMC", "server.jar", "2048M")
	mcHandler := api.NewServerHandler(mcServer)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", mcHandler.GetStatus)
	mux.HandleFunc("POST /command", mcHandler.SendCommand)
	mux.HandleFunc("POST /start", mcHandler.Start)

	mux.Handle("/", http.FileServer(http.Dir("./web/static")))

	go func() {
		if err := http.ListenAndServe(":8080", mux); err != nil {
			log.Fatalf("CRITICAL ERROR, %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	sig := <-c
	fmt.Printf("Reciving Signal [%v]. Shutting down...\n", sig)
	if err := mcServer.Stop(); err != nil {
		log.Printf("Error stopping the server: %v", err)
	}
	fmt.Printf("Server stopped gracefully [%v]", sig)
}
