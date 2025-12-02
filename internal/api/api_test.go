package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"paperMC_backend/internal/minecraft"
	"strings"
	"testing"
	"time"
)

// MockServerController is a manual mock implementation of ServerController
type MockServerController struct {
	GetStatusFunc     func() minecraft.Status
	WhiteListUserFunc func(username string) error
	SendCommandFunc   func(cmd string) error
	StartFunc         func() error
	StopFunc          func() error
	StreamLogsFunc    func()
	GetLogChanFunc    func() <-chan string
}

func (m *MockServerController) GetStatus() minecraft.Status {
	if m.GetStatusFunc != nil {
		return m.GetStatusFunc()
	}
	return minecraft.StatusStopped
}

func (m *MockServerController) WhiteListUser(username string) error {
	if m.WhiteListUserFunc != nil {
		return m.WhiteListUserFunc(username)
	}
	return nil
}

func (m *MockServerController) SendCommand(cmd string) error {
	if m.SendCommandFunc != nil {
		return m.SendCommandFunc(cmd)
	}
	return nil
}

func (m *MockServerController) Start() error {
	if m.StartFunc != nil {
		return m.StartFunc()
	}
	return nil
}

func (m *MockServerController) Stop() error {
	if m.StopFunc != nil {
		return m.StopFunc()
	}
	return nil
}

func (m *MockServerController) StreamLogs() {
	if m.StreamLogsFunc != nil {
		m.StreamLogsFunc()
	}
}

func (m *MockServerController) GetLogChan() <-chan string {
	if m.GetLogChanFunc != nil {
		return m.GetLogChanFunc()
	}
	return nil
}

func TestGetStatus(t *testing.T) {
	mockCtrl := &MockServerController{
		GetStatusFunc: func() minecraft.Status {
			return minecraft.StatusRunning
		},
	}
	handler := NewServerHandler(mockCtrl)

	req, _ := http.NewRequest("GET", "/status", nil)
	rr := httptest.NewRecorder()

	handler.GetStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
	var resp StatusResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Status != "Running" {
		t.Errorf("Expected response status 'Running', got %v", resp.Status)
	}
}

func TestStart(t *testing.T) {
	startCalled := false
	streamLogsCalled := false

	mockCtrl := &MockServerController{
		StartFunc: func() error {
			startCalled = true
			return nil
		},
		StreamLogsFunc: func() {
			streamLogsCalled = true
		},
	}
	handler := NewServerHandler(mockCtrl)

	req, _ := http.NewRequest("POST", "/start", nil)
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	// Wait for goroutine to call StreamLogs
	time.Sleep(10 * time.Millisecond)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
	if !startCalled {
		t.Error("Start was not called")
	}
	if !streamLogsCalled {
		t.Error("StreamLogs was not called")
	}
}

func TestStartError(t *testing.T) {
	mockCtrl := &MockServerController{
		StartFunc: func() error {
			return errors.New("already running")
		},
	}
	handler := NewServerHandler(mockCtrl)

	req, _ := http.NewRequest("POST", "/start", nil)
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest, got %v", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "already running") {
		t.Errorf("Expected error message 'already running', got %v", rr.Body.String())
	}
}

func TestStop(t *testing.T) {
	stopCalled := false
	mockCtrl := &MockServerController{
		StopFunc: func() error {
			stopCalled = true
			return nil
		},
	}
	handler := NewServerHandler(mockCtrl)

	req, _ := http.NewRequest("POST", "/stop", nil)
	rr := httptest.NewRecorder()

	handler.Stop(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
	if !stopCalled {
		t.Error("Stop was not called")
	}
}

func TestSendCommand(t *testing.T) {
	var cmdReceived string
	mockCtrl := &MockServerController{
		SendCommandFunc: func(cmd string) error {
			cmdReceived = cmd
			return nil
		},
	}
	handler := NewServerHandler(mockCtrl)

	body := []byte(`{"command": "say hello"}`)
	req, _ := http.NewRequest("POST", "/command", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.SendCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
	if cmdReceived != "say hello" {
		t.Errorf("Expected command 'say hello', got %v", cmdReceived)
	}
}

func TestWhitelisting(t *testing.T) {
	var userReceived string
	mockCtrl := &MockServerController{
		WhiteListUserFunc: func(username string) error {
			userReceived = username
			return nil
		},
	}
	handler := NewServerHandler(mockCtrl)

	body := []byte(`{"command": "Steve"}`)
	req, _ := http.NewRequest("POST", "/whitelist_add", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.Whitelisting(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
	if userReceived != "Steve" {
		t.Errorf("Expected user 'Steve', got %v", userReceived)
	}
}

func TestHandleLogs(t *testing.T) {
	logChan := make(chan string)
	mockCtrl := &MockServerController{
		GetLogChanFunc: func() <-chan string {
			return logChan
		},
	}
	handler := NewServerHandler(mockCtrl)

	// Create a context that we can cancel to stop the handler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", "/logs", nil)
	rr := httptest.NewRecorder()

	// Handle logs blocks, so we run it in a goroutine
	go func() {
		handler.HandleLogs(rr, req)
	}()

	// Send a log message
	logChan <- "Test Log"

	// Give it a moment to process
	time.Sleep(100 * time.Millisecond)

	if !strings.Contains(rr.Body.String(), "data: Test Log") {
		t.Errorf("Expected log 'data: Test Log' in response, got %v", rr.Body.String())
	}

	// Cancel context to stop the handler goroutine
	cancel()
	// Allow some time for cleanup if needed
	time.Sleep(10 * time.Millisecond)
}

func TestBasicAuth(t *testing.T) {
	handler := &Handler{}
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	protected := handler.BasicAuth(testHandler, "admin", "secret")

	// No Auth
	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status Unauthorized (no auth), got %v", rr.Code)
	}

	// Wrong Auth
	req, _ = http.NewRequest("GET", "/", nil)
	req.SetBasicAuth("admin", "wrong")
	rr = httptest.NewRecorder()
	protected.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status Unauthorized (wrong auth), got %v", rr.Code)
	}

	// Correct Auth
	req, _ = http.NewRequest("GET", "/", nil)
	req.SetBasicAuth("admin", "secret")
	rr = httptest.NewRecorder()
	protected.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
}
