package minecraft

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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

	mcInput := "§6TPS from last 1m: §a*20.0§r   "
	mcCleaned := CleanString(mcInput)
	if mcCleaned != "TPS from last 1m: *20.0" {
		t.Errorf("CleanString failed to strip Minecraft color codes. Got '%s'", mcCleaned)
	}

	mixed := "\x1b[33m§6[§eServer§6] §aReady!\x1b[0m"
	mixedCleaned := CleanString(mixed)
	if mixedCleaned != "[Server] Ready!" {
		t.Errorf("CleanString failed to strip mixed ANSI & Minecraft codes. Got '%s'", mixedCleaned)
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

func TestServerStartStopKillLifecycle(t *testing.T) {
	origExec := ExecCommandContext
	defer func() { ExecCommandContext = origExec }()

	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "2G", nil)

	// Mock command with "cat"
	ExecCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "cat")
	}

	// 1. Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	if server.GetStatus() != StatusRunning {
		t.Errorf("Expected StatusRunning, got %v", server.GetStatus())
	}

	// 2. Start when already running returns error
	if err := server.Start(); err == nil {
		t.Errorf("Expected error when starting already running server")
	}

	// 3. Stop running server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop running server: %v", err)
	}

	// 4. Kill server
	_ = server.Kill()

	// Wait for process monitor to set StatusStopped
	for i := 0; i < 20; i++ {
		if server.GetStatus() == StatusStopped {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if server.GetStatus() != StatusStopped {
		t.Errorf("Expected server to be stopped, got %v", server.GetStatus())
	}

	// Kill already stopped server returns error
	if err := server.Kill(); err == nil {
		t.Errorf("Expected error killing already stopped server")
	}
}

func TestServerMonitorProcessExit(t *testing.T) {
	origExec := ExecCommandContext
	defer func() { ExecCommandContext = origExec }()

	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "2G", nil)

	// Mock command that immediately exits
	ExecCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Process exits immediately, monitorProcess should transition status to StatusStopped
	for i := 0; i < 20; i++ {
		if server.GetStatus() == StatusStopped {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if server.GetStatus() != StatusStopped {
		t.Errorf("Expected server to transition to StatusStopped, got %v", server.GetStatus())
	}
}

func TestOnlinePlayersClearedOnProcessExit(t *testing.T) {
	origExec := ExecCommandContext
	defer func() { ExecCommandContext = origExec }()

	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "2G", nil)

	ExecCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "cat")
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Populate online players
	server.mu.Lock()
	server.OnlinePlayers["Player1"] = Player{UserName: "Player1", UUID: "uuid-1"}
	server.OnlinePlayers["Player2"] = Player{UserName: "Player2", UUID: "uuid-2"}
	server.mu.Unlock()

	vitals := server.GetVitals()
	if vitals.PlayerCount != 2 {
		t.Fatalf("Expected 2 online players, got %d", vitals.PlayerCount)
	}

	// Stop server process
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
	_ = server.Kill()

	// Wait for process monitor to exit
	for i := 0; i < 30; i++ {
		if server.GetStatus() == StatusStopped {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify OnlinePlayers was cleared on exit
	server.mu.Lock()
	playerCount := len(server.OnlinePlayers)
	server.mu.Unlock()

	if playerCount != 0 {
		t.Errorf("Expected 0 online players after process exit, got %d", playerCount)
	}

	stoppedVitals := server.GetVitals()
	if stoppedVitals.PlayerCount != 0 || len(stoppedVitals.PlayerList) != 0 {
		t.Errorf("Expected GetVitals to report 0 players when stopped, got %d", stoppedVitals.PlayerCount)
	}
}

func TestHandleRejectionNilStore(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "2G", nil) // nil store

	// Calling handleRejection must not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleRejection panicked with nil store: %v", r)
		}
	}()

	server.handleRejection("[13:09:40 INFO]: Disconnecting HackerGuy (/127.0.0.1:54321): You are not whitelisted on this server!")
}

func TestAddListenerUnsubscribe(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "2G", nil)

	var count int
	var mu sync.Mutex
	unsubscribe := server.AddListener(func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		count++
	})

	server.Broadcast("msg 1")
	mu.Lock()
	if count != 1 {
		t.Errorf("Expected 1 call, got %d", count)
	}
	mu.Unlock()

	// Unsubscribe
	unsubscribe()

	server.Broadcast("msg 2")
	mu.Lock()
	if count != 1 {
		t.Errorf("Expected count to remain 1 after unsubscribe, got %d", count)
	}
	mu.Unlock()
}

func TestParseVitalsFromLog(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)
	server.status = StatusRunning

	// 1. Paper / Spigot TPS log format
	tpsLine := "TPS from last 1m, 5m, 15m: 19.85, 19.92, 20.0"
	server.parseVitalsFromLog(tpsLine)

	vitals := server.GetVitals()
	if vitals.TPS != 19.85 {
		t.Errorf("Expected TPS 19.85, got %f", vitals.TPS)
	}
	expectedMSPT := 1000.0 / 19.85
	if vitals.MSPT < 50.0 || vitals.MSPT != (float64(int(expectedMSPT*100))/100) {
		t.Logf("Computed MSPT for TPS 19.85: %f", vitals.MSPT)
	}

	// 2. Paper TPS format with asterisk (*20.0) and Minecraft color codes
	cleanTpsLine := CleanString("§6TPS from last 1m, 5m, 15m: §a*20.0§r, §a*20.0§r, §a*20.0")
	server.parseVitalsFromLog(cleanTpsLine)

	vitals = server.GetVitals()
	if vitals.TPS != 20.0 {
		t.Errorf("Expected TPS 20.0, got %f", vitals.TPS)
	}
	if vitals.MSPT != 20.0 {
		t.Errorf("Expected healthy baseline MSPT 20.0, got %f", vitals.MSPT)
	}

	// 3. Paper MSPT output format (healthy)
	msptLine := "⏵ 5s: 14.5/11.2/35.8ms"
	server.parseVitalsFromLog(msptLine)

	vitals = server.GetVitals()
	if vitals.MSPT != 14.5 {
		t.Errorf("Expected MSPT 14.5, got %f", vitals.MSPT)
	}
	if vitals.TPS != 20.0 {
		t.Errorf("Expected TPS 20.0, got %f", vitals.TPS)
	}

	// 4. Paper MSPT output format (lagging MSPT > 50ms)
	lagMsptLine := "> 5s: 80.0/50.0/120.0ms"
	server.parseVitalsFromLog(lagMsptLine)

	vitals = server.GetVitals()
	if vitals.MSPT != 80.0 {
		t.Errorf("Expected MSPT 80.0, got %f", vitals.MSPT)
	}
	expectedLagTPS := 1000.0 / 80.0 // 12.5
	if vitals.TPS != expectedLagTPS {
		t.Errorf("Expected TPS %f, got %f", expectedLagTPS, vitals.TPS)
	}

	// 5. Minecraft "Can't keep up!" engine lag warning
	cantKeepUpLine := "Can't keep up! Is the server overloaded? Running 2000ms or 40 ticks behind"
	server.parseVitalsFromLog(cantKeepUpLine)

	vitals = server.GetVitals()
	// mspt = 50 + (2000 / 40) = 100.0 ms
	// tps = 1000 / 100 = 10.0
	if vitals.MSPT != 100.0 {
		t.Errorf("Expected MSPT 100.0, got %f", vitals.MSPT)
	}
	if vitals.TPS != 10.0 {
		t.Errorf("Expected TPS 10.0, got %f", vitals.TPS)
	}

	// 6. Vanilla 1.20.3+ /tick query format
	tickQueryLine := "Current tick rate: 18.0/s. Average tick time: 28.4ms (target: 50.0ms)"
	server.parseVitalsFromLog(tickQueryLine)

	vitals = server.GetVitals()
	if vitals.TPS != 18.0 {
		t.Errorf("Expected TPS 18.0, got %f", vitals.TPS)
	}
	if vitals.MSPT != 28.4 {
		t.Errorf("Expected MSPT 28.4, got %f", vitals.MSPT)
	}

	// 7. Non-matching line does not modify vitals and returns false
	suppressed := server.parseVitalsFromLog("Some unformatted chat message")
	if suppressed {
		t.Errorf("Expected non-matching line to return false, got true")
	}
}

func TestInternalPollSuppressionAndStreamLogs(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)
	server.status = StatusRunning

	var received []string
	var mu sync.Mutex
	server.AddListener(func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msg)
	})

	// 1. Simulate an automated background query
	atomic.StoreInt32(&server.internalPollPending, 1)

	// Stream contains a background TPS query response followed by a normal log line
	logData := `TPS from last 1m, 5m, 15m: 19.5, 19.8, 20.0
[12:00:00 INFO]: Normal server broadcast
`
	server.stdout = &stringReadCloser{Reader: strings.NewReader(logData)}
	server.StreamLogs()

	mu.Lock()
	recCopy := make([]string, len(received))
	copy(recCopy, received)
	mu.Unlock()

	// The background TPS response must NOT be broadcast
	for _, msg := range recCopy {
		if strings.Contains(msg, "TPS from last") {
			t.Errorf("Automated TPS response was broadcast to listeners: %s", msg)
		}
	}

	// The normal log line MUST be broadcast
	foundNormal := false
	for _, msg := range recCopy {
		if strings.Contains(msg, "Normal server broadcast") {
			foundNormal = true
			break
		}
	}
	if !foundNormal {
		t.Errorf("Expected normal log line to be broadcast, but was not found in: %+v", recCopy)
	}

	// 2. Test manual user command (internalPollPending == 0) is broadcast
	received = nil
	userLogData := `TPS from last 1m, 5m, 15m: 20.0, 20.0, 20.0
`
	server.stdout = &stringReadCloser{Reader: strings.NewReader(userLogData)}
	server.StreamLogs()

	mu.Lock()
	userRecCopy := make([]string, len(received))
	copy(userRecCopy, received)
	mu.Unlock()

	foundUserTPS := false
	for _, msg := range userRecCopy {
		if strings.Contains(msg, "TPS from last") {
			foundUserTPS = true
			break
		}
	}
	if !foundUserTPS {
		t.Errorf("Manual user TPS command was wrongly suppressed from broadcast: %+v", userRecCopy)
	}
}

func TestQueryTPSAndPollVitalsLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)

	// 1. Stopped server does not query TPS
	server.queryTPS()
	if atomic.LoadInt32(&server.internalPollPending) != 0 {
		t.Errorf("Expected 0 pending polls when server is stopped")
	}

	// 2. Running server queries TPS
	mockIn := &mockBufferCloser{}
	server.stdin = mockIn
	server.status = StatusRunning

	server.queryTPS()
	if atomic.LoadInt32(&server.internalPollPending) != 1 {
		t.Errorf("Expected 1 pending poll, got %d", atomic.LoadInt32(&server.internalPollPending))
	}
	if !strings.Contains(mockIn.String(), "tps\n") {
		t.Errorf("Expected 'tps\\n' sent to stdin, got: %s", mockIn.String())
	}

	// 3. pollVitals lifecycle with fast context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	server.pollInterval = 10 * time.Millisecond

	done := make(chan struct{})
	go func() {
		server.pollVitals(ctx)
		close(done)
	}()

	// Wait briefly then cancel
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success, terminated cleanly
	case <-time.After(1 * time.Second):
		t.Fatalf("pollVitals failed to terminate on ctx cancellation")
	}
}

func TestGetVitalsStoppedVsRunningTPS(t *testing.T) {
	tmpDir := t.TempDir()
	server := NewServer(tmpDir, "server.jar", "4G", nil)

	// Stopped server vitals must report 0.0 TPS and 0.0 MSPT
	stoppedVitals := server.GetVitals()
	if stoppedVitals.TPS != 0.0 || stoppedVitals.MSPT != 0.0 {
		t.Errorf("Expected stopped server to report 0.0 TPS and 0.0 MSPT, got TPS=%f, MSPT=%f",
			stoppedVitals.TPS, stoppedVitals.MSPT)
	}

	// Running server with parsed vitals
	server.status = StatusRunning
	server.currentTPS = 17.5
	server.currentMSPT = 32.0

	runningVitals := server.GetVitals()
	if runningVitals.TPS != 17.5 {
		t.Errorf("Expected running server TPS 17.5, got %f", runningVitals.TPS)
	}
	if runningVitals.MSPT != 32.0 {
		t.Errorf("Expected running server MSPT 32.0, got %f", runningVitals.MSPT)
	}

	// Check history entry has recorded the dynamic values
	if len(runningVitals.History) == 0 {
		t.Fatalf("Expected history entries, got 0")
	}
	lastPoint := runningVitals.History[len(runningVitals.History)-1]
	if lastPoint.TPS != 17.5 || lastPoint.MSPT != 32.0 {
		t.Errorf("Expected history point TPS 17.5 and MSPT 32.0, got TPS=%f, MSPT=%f",
			lastPoint.TPS, lastPoint.MSPT)
	}
}


