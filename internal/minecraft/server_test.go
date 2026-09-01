package minecraft

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"paperMC_backend/internal/database"
)

func TestStatusFormattingAndMarshaling(t *testing.T) {
	cases := []struct {
		status Status
		want   string
	}{
		{StatusStopped, "Stopped"},
		{StatusStarting, "Starting"},
		{StatusRunning, "Running"},
		{StatusStopping, "Stopping"},
		{Status(99), "Unknown"},
	}

	for _, tc := range cases {
		if tc.status.String() != tc.want {
			t.Errorf("Expected status string '%s', got '%s'", tc.want, tc.status.String())
		}
		bytes, err := tc.status.MarshalText()
		if err != nil || string(bytes) != tc.want {
			t.Errorf("MarshalText error or mismatch: err=%v, got=%s, want=%s", err, string(bytes), tc.want)
		}
	}
}

func TestCleanString(t *testing.T) {
	ansiInput := "\x1b[31;1mHello \x1b[32mWorld\x1b[0m   "
	cleaned := CleanString(ansiInput)
	if cleaned != "Hello World" {
		t.Errorf("CleanString failed to strip ANSI codes. Got '%s'", cleaned)
	}
}

func TestLogListenersAndBroadcastHistory(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)

	var received []string
	var mu sync.Mutex
	server.AddListener(func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msg)
	})

	// Broadcast 120 messages (ring buffer should retain only last 100)
	for i := 1; i <= 120; i++ {
		server.Broadcast(strings.Repeat("a", 5))
	}

	history := server.GetHistory()
	if len(history) != 100 {
		t.Errorf("Expected history to be capped at 100, got %d", len(history))
	}

	mu.Lock()
	recCount := len(received)
	mu.Unlock()

	if recCount != 120 {
		t.Errorf("Expected listener to receive 120 broadcasts, got %d", recCount)
	}
}

func TestSessionChangeAndVitals(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)

	// Pre-populate UUID cache manually simulating log line parse
	server.uuidCache["Alex"] = "uuid-alex-123"

	// 1. Alex joins
	server.handleSessionChange("[12:00:00 INFO]: Alex joined the game", true)

	vitals := server.GetVitals()
	if vitals.PlayerCount != 1 || len(vitals.PlayerList) != 1 {
		t.Fatalf("Expected 1 online player, got %d", vitals.PlayerCount)
	}
	if vitals.PlayerList[0].UserName != "Alex" || vitals.PlayerList[0].UUID != "uuid-alex-123" {
		t.Errorf("Expected player Alex with cached UUID, got %+v", vitals.PlayerList[0])
	}

	// 2. Steve joins without cached UUID
	server.handleSessionChange("[12:01:00 INFO]: Steve joined the game", true)
	vitals = server.GetVitals()
	if vitals.PlayerCount != 2 {
		t.Errorf("Expected 2 online players, got %d", vitals.PlayerCount)
	}

	// 3. Alex leaves
	server.handleSessionChange("[12:02:00 INFO]: Alex left the game", false)
	vitals = server.GetVitals()
	if vitals.PlayerCount != 1 || vitals.PlayerList[0].UserName != "Steve" {
		t.Errorf("Expected 1 online player (Steve), got %+v", vitals.PlayerList)
	}

	// 4. Test malformed log lines do not crash
	server.handleSessionChange("malformed log line", true)
	server.handleSessionChange("[12:00:00 INFO]: short", true)
}

func TestHandleRejectionWithDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize test SQLite store: %v", err)
	}
	defer store.Close()

	server := NewServer(tmpDir, "server.jar", "4G", store)

	// Simulated rejection log line
	rejectionLog := "[13:09:40 INFO]: Disconnecting Bob (/127.0.0.1:54321): You are not whitelisted on this server!"
	server.handleRejection(rejectionLog)

	rejected, err := store.GetRejectedPlayers()
	if err != nil {
		t.Fatalf("Failed to fetch rejected players from store: %v", err)
	}

	if len(rejected) != 1 || rejected[0].Username != "Bob" {
		t.Errorf("Expected rejected player 'Bob' in database, got: %+v", rejected)
	}

	// Invalid line format does not crash
	server.handleRejection("invalid: log")
	server.handleRejection("[13:00:00 INFO]: NoSubParts")
}

func TestServerLifecycleErrorsWhenStopped(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)

	if server.GetStatus() != StatusStopped {
		t.Errorf("Expected status Stopped, got %v", server.GetStatus())
	}

	if err := server.Stop(); err == nil {
		t.Errorf("Expected error stopping non-running server, got nil")
	}

	if err := server.Kill(); err == nil {
		t.Errorf("Expected error killing non-running server, got nil")
	}

	if err := server.SendCommand("say hi"); err == nil {
		t.Errorf("Expected error sending command to non-running server, got nil")
	}
}

type mockBufferCloser struct {
	strings.Builder
	closed bool
}

func (m *mockBufferCloser) Close() error {
	m.closed = true
	return nil
}

func TestRunningServerCommands(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)

	mockIn := &mockBufferCloser{}
	server.stdin = mockIn
	server.status = StatusRunning

	if err := server.SendCommand("say Hello"); err != nil {
		t.Fatalf("SendCommand failed: %v", err)
	}

	if !strings.Contains(mockIn.String(), "say Hello\n") {
		t.Errorf("Expected mock stdin to contain command, got '%s'", mockIn.String())
	}

	// Test Stop() sends "stop" command
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
	if !strings.Contains(mockIn.String(), "stop\n") {
		t.Errorf("Expected mock stdin to contain stop command, got '%s'", mockIn.String())
	}

	// Test Kill() with cancel func
	cancelled := false
	server.status = StatusRunning
	server.cancel = func() {
		cancelled = true
	}
	if err := server.Kill(); err != nil {
		t.Fatalf("Kill() failed: %v", err)
	}
	if !cancelled {
		t.Errorf("Expected cancel func to be invoked by Kill()")
	}
}

type stringReadCloser struct {
	*strings.Reader
}

func (s *stringReadCloser) Close() error {
	return nil
}

func TestStreamLogs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize test SQLite store: %v", err)
	}
	defer store.Close()

	server := NewServer(tmpDir, "server.jar", "4G", store)

	logData := `[12:00:00 INFO]: UUID of player Notch is 069a79f4-44e9-4726-a5be-fca90e38aaf5
[12:00:01 INFO]: Notch joined the game
[12:00:02 INFO]: Disconnecting Hacker (/1.2.3.4:5678): You are not whitelisted on this server!
[12:00:03 INFO]: Notch left the game
`
	server.stdout = &stringReadCloser{Reader: strings.NewReader(logData)}

	// StreamLogs runs until EOF
	server.StreamLogs()

	// Wait up to 1 second for async goroutines in StreamLogs to persist
	var rejected []database.RejectedPlayer
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		rejected, _ = store.GetRejectedPlayers()
		if len(rejected) > 0 {
			break
		}
	}

	// Verify rejected player saved
	if len(rejected) != 1 || rejected[0].Username != "Hacker" {
		t.Errorf("Expected Hacker in rejected players, got: %+v", rejected)
	}
}

func TestModernizedVitalsAndMultiCore(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "8G", nil)

	// 1. Stopped state vitals
	vitals := server.GetVitals()
	if vitals.Status != StatusStopped {
		t.Errorf("Expected Status Stopped, got %v", vitals.Status)
	}
	if vitals.DiskTotal == 0 {
		t.Errorf("Expected non-zero disk total")
	}

	// 2. Simulated running state with history appending
	server.status = StatusRunning
	server.startTime = time.Now().Add(-120 * time.Second) // 2 minutes ago

	for i := 0; i < 40; i++ {
		v := server.GetVitals()
		if v.UptimeSeconds < 120 {
			t.Errorf("Expected uptime >= 120s, got %d", v.UptimeSeconds)
		}
	}

	finalVitals := server.GetVitals()
	// History buffer must be capped at 30 entries
	if len(finalVitals.History) != 30 {
		t.Errorf("Expected history capped at 30 points, got %d", len(finalVitals.History))
	}
	if finalVitals.TPS != 20.0 {
		t.Errorf("Expected healthy TPS 20.0, got %f", finalVitals.TPS)
	}
}


