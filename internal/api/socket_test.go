package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"paperMC_backend/internal/auth"
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

func TestSocketHandler_OriginSecurity(t *testing.T) {
	tempDir := t.TempDir()
	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", nil)
	handler := NewServerHandler(mcServer, nil)

	server := httptest.NewServer(http.HandlerFunc(handler.SocketHandler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 1. Cross-origin from unauthorized external domain must be rejected
	evilHeader := http.Header{}
	evilHeader.Set("Origin", "http://evil-attacker.com")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, evilHeader)
	if err == nil {
		t.Fatalf("Expected connection from evil.com to be rejected, but it succeeded")
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for cross-origin attack, got %d", resp.StatusCode)
	}

	// 2. Authorized local origin (e.g. Vite frontend dev server) must succeed
	localHeader := http.Header{}
	localHeader.Set("Origin", "http://localhost:5173")
	wsLocal, _, err := websocket.DefaultDialer.Dial(wsURL, localHeader)
	if err != nil {
		t.Fatalf("Expected connection from localhost dev origin to succeed, got error: %v", err)
	}
	wsLocal.Close()

	// 3. Same origin must succeed
	sameHeader := http.Header{}
	sameHeader.Set("Origin", server.URL)
	wsSame, _, err := websocket.DefaultDialer.Dial(wsURL, sameHeader)
	if err != nil {
		t.Fatalf("Expected same origin connection to succeed, got: %v", err)
	}
	wsSame.Close()
}

func TestSocketHandler_CommandRBAC(t *testing.T) {
	origExec := minecraft.ExecCommandContext
	defer func() { minecraft.ExecCommandContext = origExec }()
	minecraft.ExecCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "cat")
	}

	tempDir := t.TempDir()
	mcServer := minecraft.NewServer(tempDir, "server.jar", "2G", nil)
	cmdEchoed := make(chan string, 10)
	removeListener := mcServer.AddListener(func(line string) {
		cmdEchoed <- line
	})
	defer removeListener()

	if err := mcServer.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer func() {
		_ = mcServer.Kill()
		mcServer.WaitForStop(2 * time.Second)
	}()

	handler := NewServerHandler(mcServer, nil)

	connectWithRole := func(role string) (*websocket.Conn, func()) {
		mux := http.NewServeMux()
		mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			claims := &auth.Claims{Username: role + "_user", Role: role}
			ctx := context.WithValue(r.Context(), auth.UserKey, claims)
			handler.SocketHandler(w, r.WithContext(ctx))
		})
		srv := httptest.NewServer(mux)
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			srv.Close()
			t.Fatalf("Dial as %q failed: %v", role, err)
		}
		return ws, func() {
			ws.Close()
			srv.Close()
		}
	}

	for _, deniedRole := range []string{auth.RoleViewer, auth.RoleOperator, ""} {
		ws, cleanup := connectWithRole(deniedRole)
		if err := ws.WriteJSON(WSMessage{Type: "command", Data: "say denied_" + deniedRole}); err != nil {
			cleanup()
			t.Fatalf("WriteJSON failed for %q: %v", deniedRole, err)
		}

		_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
		foundForbidden := false
		for {
			var msg WSMessage
			if err := ws.ReadJSON(&msg); err != nil {
				break
			}
			if msg.Type == "error" {
				if text, ok := msg.Data.(string); ok && strings.Contains(text, "Forbidden") {
					foundForbidden = true
					break
				}
			}
		}
		cleanup()
		if !foundForbidden {
			t.Errorf("Expected Forbidden error over WS for role %q", deniedRole)
		}
	}

	// Admin role is allowed to execute commands over WebSocket.
	wsAdmin, cleanupAdmin := connectWithRole(auth.RoleAdmin)
	defer cleanupAdmin()
	if err := wsAdmin.WriteJSON(WSMessage{Type: "command", Data: "say admin_allowed"}); err != nil {
		t.Fatalf("WriteJSON failed for admin: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case line := <-cmdEchoed:
			if strings.Contains(line, "say denied_") {
				t.Fatalf("Non-admin WS command executed on stdin: %q", line)
			}
			if strings.Contains(line, "say admin_allowed") {
				return
			}
		case <-deadline:
			t.Fatal("Timed out waiting for admin WS command execution")
		}
	}
}
