// Package minecraft provides a wrapper for managing a Minecraft server process.
//
// It handles server lifecycle management (starting, stopping, status checks),
// streams standard output logs, and facilitates sending commands to the server.
//
// Additionally, it includes utilities for user whitelisting by resolving:
//   - Java Edition UUIDs via the Mojang API.
//   - Bedrock/Xbox XUIDs via the Geyser API.
package minecraft

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/flags"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/process"
)

var ExecCommandContext = exec.CommandContext

type Status int

const (
	StatusStopped  Status = iota // 0
	StatusStarting               // 1
	StatusRunning                // 2
	StatusStopping               // 3
)

const FloodgatePrefix = "."

func (s Status) String() string {
	switch s {
	case StatusStopped:
		return "Stopped"
	case StatusStarting:
		return "Starting"
	case StatusRunning:
		return "Running"
	case StatusStopping:
		return "Stopping"
	default:
		return "Unknown"
	}
}

type MetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	CPU       float64 `json:"cpu"`
	RAM       uint64  `json:"ram"`
	TPS       float64 `json:"tps"`
	MSPT      float64 `json:"mspt"`
}

type Server struct {
	// Public fields
	WorkDir       string
	JarFile       string
	RAM           string
	Args          []string
	LogChan       chan string
	LogHistory    []string
	OnlinePlayers map[string]Player

	// Private fields
	uuidCache      map[string]string
	nextListenerID int
	listeners      map[int]func(string)
	startTime      time.Time
	history        []MetricPoint

	store  database.Store
	cmd    *exec.Cmd
	mu     sync.RWMutex
	status Status
	stdin  io.WriteCloser
	stdout io.ReadCloser
	proc   *process.Process
	cancel context.CancelFunc

	// Dynamic TPS & MSPT tracking
	currentTPS          float64
	currentMSPT         float64
	lastTPSUpdate       time.Time
	internalPollPending int32
	pollInterval        time.Duration

	// Active launch arguments
	activeArgs []string
}

type Vitals struct {
	Status        Status        `json:"status"`
	UptimeSeconds int64         `json:"uptime_seconds"`
	CPU           float64       `json:"cpu"`
	SystemCPU     float64       `json:"system_cpu"`
	CPUCores      []float64     `json:"cpu_cores"`
	Threads       int32         `json:"threads"`
	RAM           uint64        `json:"ram"`
	TotalMemory   string        `json:"total_memory"`
	DiskFree      uint64        `json:"disk_free"`
	DiskTotal     uint64        `json:"disk_total"`
	DiskUsedPct   float64       `json:"disk_used_pct"`
	PlayerCount   int           `json:"player_count"`
	PlayerList    []Player      `json:"player_list"`
	ActiveWorld   string        `json:"active_world"`
	TPS           float64       `json:"tps"`
	MSPT          float64       `json:"mspt"`
	History       []MetricPoint `json:"history"`
}

var uuidLogRegex = regexp.MustCompile(`UUID of player (.+) is ([0-9a-fA-F\-]+)`)
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\[[0-9;]*m`)
var mcColorRegex = regexp.MustCompile(`§[0-9a-fk-orA-FK-OR]`)
var tpsLogRegex = regexp.MustCompile(`(?i)TPS from last 1m,\s*5m,\s*15m:\s*\*?([0-9.]+),\s*\*?([0-9.]+),\s*\*?([0-9.]+)`)
var msptLogRegex = regexp.MustCompile(`(?i)(?:⏵|\>)?\s*5s:\s*([0-9.]+)\s*/\s*([0-9.]+)\s*/\s*([0-9.]+)\s*ms`)
var cantKeepUpRegex = regexp.MustCompile(`(?i)Can't keep up!.*Running\s+([0-9]+)ms\s+or\s+([0-9]+)\s+ticks behind`)
var tickQueryRegex = regexp.MustCompile(`(?i)Current tick rate:\s*([0-9.]+)(?:/s)?.*?Average tick time:\s*([0-9.]+)ms`)

func CleanString(input string) string {
	withoutAnsi := ansiRegex.ReplaceAllString(input, "")
	withoutMC := mcColorRegex.ReplaceAllString(withoutAnsi, "")
	return strings.TrimSpace(withoutMC)
}

// MarshalText implements the encoding.TextMarshaler interface.
// This overrides the default integer serialization (0, 1, 2) with strings ("Stopped", etc).
func (s Status) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *Server) Start() error {
	// Try Lock the server Mutex immediately
	s.mu.Lock()
	// Check if server is already running if not return an error
	if s.status != StatusStopped {
		s.mu.Unlock()
		return errors.New("Server Status is not Stopped. Aborting start")
	}
	// Creating the context for exec.CommandContext
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	s.status = StatusStarting

	// Resolve startup flags from DB store if available
	ram := s.RAM
	preset := flags.PresetAikar
	customFlags := ""
	if s.store != nil {
		if dbFlags, err := s.store.GetServerFlags(); err == nil && dbFlags != nil {
			if dbFlags.RAM != "" {
				ram = dbFlags.RAM
				s.RAM = dbFlags.RAM
			}
			if dbFlags.Preset != "" {
				preset = dbFlags.Preset
			}
			customFlags = dbFlags.CustomFlags
		}
	}

	cmdArgs := flags.BuildJavaArgs(ram, preset, customFlags, s.JarFile)
	s.activeArgs = make([]string, len(cmdArgs))
	copy(s.activeArgs, cmdArgs)

	s.cmd = ExecCommandContext(ctx, "java", cmdArgs...)
	s.cmd.Dir = s.WorkDir

	pipeIn, errIn := s.cmd.StdinPipe()
	if errIn != nil {
		s.mu.Unlock()
		return errIn
	}
	s.stdin = pipeIn

	pipeOut, errOut := s.cmd.StdoutPipe()
	if errOut != nil {
		s.mu.Unlock()
		return errOut
	}
	s.stdout = pipeOut

	if err := s.cmd.Start(); err != nil {
		s.mu.Unlock()
		return err
	}

	s.status = StatusRunning
	s.startTime = time.Now()
	s.currentTPS = 20.0
	s.currentMSPT = 20.0
	atomic.StoreInt32(&s.internalPollPending, 0)
	s.mu.Unlock()

	go s.monitorProcess()
	go s.StreamLogs()
	go s.pollVitals(ctx)

	return nil
}

func (s *Server) Kill() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.status == StatusStopped {
		return errors.New("Server is already stopped")
	}

	if s.cancel == nil {
		return errors.New("fatal: cancel function is nil but server is not stopped")
	}

	s.cancel()
	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	if s.status != StatusRunning {
		s.mu.Unlock()
		return errors.New("server is not running")
	}
	s.status = StatusStopping
	s.mu.Unlock()

	if err := s.SendCommand("stop"); err != nil {
		s.Kill()
		return fmt.Errorf("failed to send stop command, forcing kill: %w", err)
	}
	return nil
}

func (s *Server) monitorProcess() {
	p, err := process.NewProcess(int32(s.cmd.Process.Pid))
	if err != nil {
		s.cmd.Process.Kill()
	} else {
		s.mu.Lock()
		s.proc = p
		s.mu.Unlock()
	}

	s.cmd.Wait()

	s.mu.Lock()
	s.status = StatusStopped
	s.proc = nil
	s.startTime = time.Time{}
	s.OnlinePlayers = make(map[string]Player)
	s.currentTPS = 0
	s.currentMSPT = 0
	atomic.StoreInt32(&s.internalPollPending, 0)
	s.activeArgs = nil
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (s *Server) GetVitals() Vitals {
	s.mu.Lock()
	defer s.mu.Unlock()

	onlineList := make([]Player, 0, len(s.OnlinePlayers))
	for _, p := range s.OnlinePlayers {
		onlineList = append(onlineList, p)
	}

	vitals := Vitals{
		Status:      s.status,
		TotalMemory: s.RAM,
		PlayerCount: len(onlineList),
		PlayerList:  onlineList,
		TPS:         0,
		MSPT:        0,
		History:     make([]MetricPoint, len(s.history)),
	}
	copy(vitals.History, s.history)

	// Collect host CPU metrics (Per-Core & Overall)
	if cores, err := cpu.Percent(0, true); err == nil {
		vitals.CPUCores = cores
	}
	if sysCPU, err := cpu.Percent(0, false); err == nil && len(sysCPU) > 0 {
		vitals.SystemCPU = sysCPU[0]
	}

	// Collect disk metrics
	if usage, err := disk.Usage(s.WorkDir); err == nil {
		vitals.DiskFree = usage.Free
		vitals.DiskTotal = usage.Total
		vitals.DiskUsedPct = usage.UsedPercent
	}

	// If Server is not running, return host and disk status (0 process CPU/RAM)
	if s.status != StatusRunning {
		return vitals
	}

	// Process Uptime
	if !s.startTime.IsZero() {
		vitals.UptimeSeconds = int64(time.Since(s.startTime).Seconds())
	}

	// Process CPU % and Memory
	if s.proc != nil {
		if cpuPercent, err := s.proc.Percent(0); err == nil {
			vitals.CPU = cpuPercent
		}
		if mem, err := s.proc.MemoryInfo(); err == nil {
			vitals.RAM = mem.RSS
		}
		if numThreads, err := s.proc.NumThreads(); err == nil {
			vitals.Threads = numThreads
		}
	}

	// Dynamic Minecraft TPS & MSPT when running
	tpsVal := s.currentTPS
	if tpsVal <= 0 {
		tpsVal = 20.0
	}
	msptVal := s.currentMSPT
	if msptVal <= 0 {
		msptVal = 20.0
	}
	vitals.TPS = math.Round(tpsVal*100) / 100
	vitals.MSPT = math.Round(msptVal*100) / 100

	// Append data point to history ring buffer (capped at 30 entries)
	point := MetricPoint{
		Timestamp: time.Now().Unix(),
		CPU:       vitals.CPU,
		RAM:       vitals.RAM,
		TPS:       vitals.TPS,
		MSPT:      vitals.MSPT,
	}
	s.history = append(s.history, point)
	if len(s.history) > 30 {
		s.history = s.history[1:]
	}

	vitals.History = make([]MetricPoint, len(s.history))
	copy(vitals.History, s.history)

	return vitals
}

func (s *Server) StreamLogs() {
	scanner := bufio.NewScanner(s.stdout)

	for scanner.Scan() {
		text := scanner.Text()
		cleanText := CleanString(text)

		// Parse vitals & suppress console broadcast if this was an automated poll
		isPollResponse := s.parseVitalsFromLog(cleanText)
		if !isPollResponse {
			s.Broadcast("[MC] " + text)
		}

		// Capture UUID
		if strings.Contains(cleanText, "UUID of player") {
			matches := uuidLogRegex.FindStringSubmatch(cleanText)
			if len(matches) == 3 {
				name := matches[1]
				uuid := matches[2]
				s.mu.Lock()
				s.uuidCache[name] = uuid
				s.mu.Unlock()
			}
		}

		// Check for players joining
		if strings.Contains(text, " joined the game") {
			go s.handleSessionChange(text, true)
		}

		// Check for players leaving
		if strings.Contains(text, " left the game") {
			go s.handleSessionChange(text, false)
		}

		// Check for players not on WhiteList trying to connect
		if strings.Contains(text, "): You are not whitelisted on this server!") {
			go s.handleRejection(text)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading log %v\n", err)
	}
}

func (s *Server) consumeInternalPoll() bool {
	for {
		pending := atomic.LoadInt32(&s.internalPollPending)
		if pending <= 0 {
			return false
		}
		if atomic.CompareAndSwapInt32(&s.internalPollPending, pending, pending-1) {
			return true
		}
	}
}

func (s *Server) queryTPS() {
	s.mu.Lock()
	running := (s.status == StatusRunning && s.stdin != nil)
	s.mu.Unlock()

	if !running {
		return
	}

	atomic.AddInt32(&s.internalPollPending, 1)
	if err := s.SendCommand("tps"); err != nil {
		s.consumeInternalPoll()
	}
}

func (s *Server) pollVitals(ctx context.Context) {
	interval := s.pollInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	initialDelay := 2 * time.Second
	if interval < initialDelay {
		initialDelay = interval
	}

	select {
	case <-ctx.Done():
		return
	case <-time.After(initialDelay):
		s.queryTPS()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.queryTPS()
		}
	}
}

func (s *Server) parseVitalsFromLog(cleanText string) bool {
	// 1. Paper / Spigot TPS output: "TPS from last 1m, 5m, 15m: 20.0, 20.0, 20.0"
	if matches := tpsLogRegex.FindStringSubmatch(cleanText); len(matches) >= 2 {
		if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
			s.mu.Lock()
			s.currentTPS = val
			s.lastTPSUpdate = time.Now()
			if s.currentTPS < 20.0 && s.currentTPS > 0 {
				s.currentMSPT = 1000.0 / s.currentTPS
			} else if s.currentTPS >= 20.0 && (s.currentMSPT == 0 || s.currentMSPT > 50.0) {
				s.currentMSPT = 20.0
			}
			s.mu.Unlock()
		}
		if s.consumeInternalPoll() {
			return true // Suppress automated background poll from console broadcast
		}
		return false
	}

	// 2. Paper MSPT output: "⏵ 5s: 14.5/11.2/35.8ms"
	if matches := msptLogRegex.FindStringSubmatch(cleanText); len(matches) >= 2 {
		if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
			s.mu.Lock()
			s.currentMSPT = val
			s.lastTPSUpdate = time.Now()
			if val > 50.0 {
				s.currentTPS = 1000.0 / val
			} else if s.currentTPS == 0 {
				s.currentTPS = 20.0
			}
			s.mu.Unlock()
		}
		return false
	}

	// 3. Minecraft "Can't keep up!" lag warning
	if matches := cantKeepUpRegex.FindStringSubmatch(cleanText); len(matches) >= 3 {
		behindMs, err1 := strconv.ParseFloat(matches[1], 64)
		ticksBehind, err2 := strconv.ParseFloat(matches[2], 64)
		if err1 == nil && err2 == nil && ticksBehind > 0 {
			mspt := 50.0 + (behindMs / ticksBehind)
			tps := 1000.0 / mspt
			if tps > 20.0 {
				tps = 20.0
			}
			s.mu.Lock()
			s.currentMSPT = mspt
			s.currentTPS = tps
			s.lastTPSUpdate = time.Now()
			s.mu.Unlock()
		}
		return false
	}

	// 4. Vanilla /tick query output: "Current tick rate: 20.0/s. Average tick time: 14.5ms"
	if matches := tickQueryRegex.FindStringSubmatch(cleanText); len(matches) >= 3 {
		tpsVal, err1 := strconv.ParseFloat(matches[1], 64)
		msptVal, err2 := strconv.ParseFloat(matches[2], 64)
		if err1 == nil && err2 == nil {
			s.mu.Lock()
			s.currentTPS = tpsVal
			s.currentMSPT = msptVal
			s.lastTPSUpdate = time.Now()
			s.mu.Unlock()
		}
		return false
	}

	return false
}

func (s *Server) handleRejection(logLine string) {
	// Line format example:
	//[13:09:40 INFO]: Disconnecting Bob (/ip:port): You are not whitelisted on this server!
	//
	// 1. Extract Username
	// Split by ": "
	parts := strings.Split(logLine, ": ")
	if len(parts) < 3 {
		return
	}
	// Take the part after ".../INFO]" -> "Bob lost conection"
	// Split by " "
	subParts := strings.Split(parts[1], " ")
	if len(subParts) < 3 {
		return
	}
	username := CleanString(subParts[1])

	// 2. Persist to DB
	if username != "" && s.store != nil {
		s.Broadcast("[WARN] Detected blocked player. Saving to DB user: " + username)
		if err := s.store.UpsertRejectedPlayer(username); err != nil {
			s.Broadcast("[Error] Failed to save rejected player: " + err.Error())
		}
	}
}

func (s *Server) handleSessionChange(logLine string, joining bool) {
	parts := strings.Split(logLine, "]: ")
	if len(parts) < 2 {
		return
	}
	message := parts[1]

	words := strings.Split(message, " ")
	if len(words) < 4 {
		return
	}

	username := CleanString(words[0])
	if username == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if joining {
		uuid, exist := s.uuidCache[username]
		if !exist {
			uuid = ""
		}
		s.OnlinePlayers[username] = Player{
			UserName: username,
			UUID:     uuid,
		}
		delete(s.uuidCache, username)
	} else {
		delete(s.OnlinePlayers, username)
	}
}

func (s *Server) AddListener(listener func(string)) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextListenerID++
	id := s.nextListenerID
	if s.listeners == nil {
		s.listeners = make(map[int]func(string))
	}
	s.listeners[id] = listener
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.listeners, id)
	}
}

func (s *Server) Broadcast(msg string) {
	// Add message to the LogHistory and copy listeners under lock
	s.mu.Lock()
	s.LogHistory = append(s.LogHistory, msg)
	if len(s.LogHistory) > 100 {
		s.LogHistory = s.LogHistory[1:]
	}
	currentListeners := make([]func(string), 0, len(s.listeners))
	for _, listener := range s.listeners {
		currentListeners = append(currentListeners, listener)
	}
	s.mu.Unlock()

	// Notify registered listeners (e.g. WebSocket Hub)
	for _, listener := range currentListeners {
		listener(msg)
	}

	// Sent msg to legacy channel if consumer attached
	select {
	case s.LogChan <- msg:
	default:
	}

	// Sent msg to os output
	fmt.Println(msg)
}

func (s *Server) GetHistory() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a copy to be safe
	history := make([]string, len(s.LogHistory))
	copy(history, s.LogHistory)

	return history
}

func (s *Server) SendCommand(cmd string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != StatusRunning && s.status != StatusStopping {
		return errors.New("server is already stopped")
	}

	if s.stdin == nil {
		return errors.New("input pipe is no attached")
	}

	_, err := fmt.Fprintln(s.stdin, cmd)
	return err
}

func (s *Server) GetStatus() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Server) GetActiveArgs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.activeArgs) == 0 {
		return nil
	}
	cp := make([]string, len(s.activeArgs))
	copy(cp, s.activeArgs)
	return cp
}

func NewServer(workDir string, jarFile string, ram string, store database.Store) *Server {
	return &Server{
		WorkDir:       workDir,
		JarFile:       jarFile,
		RAM:           ram,
		LogChan:       make(chan string),
		LogHistory:    make([]string, 0),
		history:       make([]MetricPoint, 0, 30),
		listeners:     make(map[int]func(string)),
		status:        StatusStopped,
		store:         store,
		OnlinePlayers: make(map[string]Player),
		uuidCache:     make(map[string]string),

		Args: []string{},
	}
}
