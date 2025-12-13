package api

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all orgins for development
	},
}

// WSMessage defines the JSON format for all websocket traffic
type WSMessage struct {
	Type string `json:"type"` // "log" or "command"
	Data string `json:"data"`
}

func (h *Handler) SocketHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Upgrade HTTP to websocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	// 2. REPLAY HISTORY (Immediate Context)
	history := h.mc.GetHistory()
	for _, line := range history {
		if err := conn.WriteJSON(WSMessage{Type: "log", Data: line}); err != nil {
			log.Println("Write error", err)
		}
	}

	// 3. Start Writer Pump (Server -> Browser)
	// IMPORTANT: we use a quite channel to stop this goroutine if the socket closes
	quit := make(chan struct{})

	go func() {
		defer close(quit) // Ensure we clean up
		for {
			select {
			case msg := <-h.mc.LogChan:
				err := conn.WriteJSON(WSMessage{Type: "log", Data: msg})
				if err != nil {
					log.Println("WS Write Error", err)
					return
				}
			case <-r.Context().Done():
				return
			}
		}
	}()

	// 4. START READR PUMP (Browser -> Server)
	// This blocks the main handler function until the connection dies
	for {
		var msg WSMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("WS Read Error/Closed", err)
			break // Breake the loop, which returns from the function, closing the conn
		}

		if msg.Type == "command" {
			// execute the command
			if err := h.mc.SendCommand(msg.Data); err != nil {
				conn.WriteJSON(WSMessage{Type: "error", Data: err.Error()})
			}
		}
	}
}
