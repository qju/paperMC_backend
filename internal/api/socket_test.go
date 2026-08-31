package api

import (
	"testing"
	"time"
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
