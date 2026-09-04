package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"paperMC_backend/internal/minecraft"
)

func TestHubRegisterAndBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client1 := &Client{
		hub:  hub,
		send: make(chan WSMessage, 10),
	}
	client2 := &Client{
		hub:  hub,
		send: make(chan WSMessage, 10),
	}

	hub.register <- client1
	hub.register <- client2

	time.Sleep(20 * time.Millisecond)

	testMsg := WSMessage{Type: "log", Data: "test log output"}
	hub.Broadcast(testMsg)

	select {
	case msg := <-client1.send:
		if msg.Data != testMsg.Data {
			t.Errorf("Client1 expected %q, got %q", testMsg.Data, msg.Data)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for message on Client1")
	}

	select {
	case msg := <-client2.send:
		if msg.Data != testMsg.Data {
			t.Errorf("Client2 expected %q, got %q", testMsg.Data, msg.Data)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for message on Client2")
	}

	// Test unregister
	hub.unregister <- client1
	time.Sleep(20 * time.Millisecond)

	// After unregister, client1.send should be closed
	_, ok := <-client1.send
	if ok {
		t.Error("Expected client1 send channel to be closed after unregister")
	}
}

func TestSocketHandlerIntegration(t *testing.T) {
	tempDir := t.TempDir()
	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", nil)
	mcServer.Broadcast("Initial log 1")
	mcServer.Broadcast("Initial log 2")

	handler := NewServerHandler(mcServer, nil)

	server := httptest.NewServer(http.HandlerFunc(handler.SocketHandler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to dial websocket: %v", err)
	}
	defer ws.Close()

	// Read initial replay messages
	var initialMsg WSMessage
	if err := ws.ReadJSON(&initialMsg); err != nil {
		t.Fatalf("Failed to read replay message: %v", err)
	}
	if initialMsg.Type != "log" {
		t.Errorf("Expected message type 'log', got %s", initialMsg.Type)
	}

	// Send command
	cmdMsg := WSMessage{Type: "command", Data: "say hello"}
	if err := ws.WriteJSON(cmdMsg); err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}

	// Read until we receive error message
	_ = ws.SetReadDeadline(time.Now().Add(1 * time.Second))
	foundError := false
	for i := 0; i < 5; i++ {
		var reply WSMessage
		if err := ws.ReadJSON(&reply); err != nil {
			break
		}
		if reply.Type == "error" {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Expected error reply for command sent to stopped server")
	}

	// Test broadcast to connected client
	hubMsg := WSMessage{Type: "log", Data: "Broadcast message test"}
	handler.hub.Broadcast(hubMsg)

	_ = ws.SetReadDeadline(time.Now().Add(1 * time.Second))
	var receivedBroadcast WSMessage
	if err := ws.ReadJSON(&receivedBroadcast); err != nil {
		t.Fatalf("Failed to read broadcast message: %v", err)
	}
	if receivedBroadcast.Data != hubMsg.Data {
		t.Errorf("Expected broadcast data %q, got %q", hubMsg.Data, receivedBroadcast.Data)
	}
}
