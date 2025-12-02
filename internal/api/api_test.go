package api

import (
	"bytes"
	"encoding/json"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"paperMC_backend/internal/minecraft"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockServerController is a mock implementation of ServerController
type MockServerController struct {
	mock.Mock
}

func (m *MockServerController) GetStatus() minecraft.Status {
	args := m.Called()
	return args.Get(0).(minecraft.Status)
}

func (m *MockServerController) WhiteListUser(username string) error {
	args := m.Called(username)
	return args.Error(0)
}

func (m *MockServerController) SendCommand(cmd string) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func (m *MockServerController) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockServerController) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockServerController) StreamLogs() {
	m.Called()
}

func (m *MockServerController) GetLogChan() <-chan string {
	args := m.Called()
	return args.Get(0).(<-chan string)
}

func TestGetStatus(t *testing.T) {
	mockCtrl := new(MockServerController)
	handler := NewServerHandler(mockCtrl)

	mockCtrl.On("GetStatus").Return(minecraft.StatusRunning)

	req, _ := http.NewRequest("GET", "/status", nil)
	rr := httptest.NewRecorder()

	handler.GetStatus(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp StatusResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "Running", resp.Status)
}

func TestStart(t *testing.T) {
	mockCtrl := new(MockServerController)
	handler := NewServerHandler(mockCtrl)

	mockCtrl.On("Start").Return(nil)
	mockCtrl.On("StreamLogs").Return()

	req, _ := http.NewRequest("POST", "/start", nil)
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	// Wait for goroutine to call StreamLogs
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockCtrl.AssertExpectations(t)
}

func TestStartError(t *testing.T) {
	mockCtrl := new(MockServerController)
	handler := NewServerHandler(mockCtrl)

	mockCtrl.On("Start").Return(errors.New("already running"))

	req, _ := http.NewRequest("POST", "/start", nil)
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "already running")
	mockCtrl.AssertExpectations(t)
}

func TestStop(t *testing.T) {
	mockCtrl := new(MockServerController)
	handler := NewServerHandler(mockCtrl)

	mockCtrl.On("Stop").Return(nil)

	req, _ := http.NewRequest("POST", "/stop", nil)
	rr := httptest.NewRecorder()

	handler.Stop(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockCtrl.AssertExpectations(t)
}

func TestSendCommand(t *testing.T) {
	mockCtrl := new(MockServerController)
	handler := NewServerHandler(mockCtrl)

	mockCtrl.On("SendCommand", "say hello").Return(nil)

	body := []byte(`{"command": "say hello"}`)
	req, _ := http.NewRequest("POST", "/command", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.SendCommand(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockCtrl.AssertExpectations(t)
}

func TestWhitelisting(t *testing.T) {
	mockCtrl := new(MockServerController)
	handler := NewServerHandler(mockCtrl)

	mockCtrl.On("WhiteListUser", "Steve").Return(nil)

	body := []byte(`{"command": "Steve"}`)
	req, _ := http.NewRequest("POST", "/whitelist_add", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.Whitelisting(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockCtrl.AssertExpectations(t)
}

func TestHandleLogs(t *testing.T) {
	mockCtrl := new(MockServerController)
	handler := NewServerHandler(mockCtrl)

	logChan := make(chan string)
	mockCtrl.On("GetLogChan").Return((<-chan string)(logChan))

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

	// Can't easily test streaming response with httptest.NewRecorder fully,
	// but we can check if it wrote something.
	assert.Contains(t, rr.Body.String(), "data: Test Log")

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
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	// Wrong Auth
	req, _ = http.NewRequest("GET", "/", nil)
	req.SetBasicAuth("admin", "wrong")
	rr = httptest.NewRecorder()
	protected.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	// Correct Auth
	req, _ = http.NewRequest("GET", "/", nil)
	req.SetBasicAuth("admin", "secret")
	rr = httptest.NewRecorder()
	protected.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}
