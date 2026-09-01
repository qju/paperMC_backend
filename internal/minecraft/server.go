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
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"paperMC_backend/internal/database"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/process"
)

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
	uuidCache map[string]string
	listeners []func(string)
	startTime time.Time
	history   []MetricPoint

	store  database.Store
	cmd    *exec.Cmd
	mu     sync.RWMutex
	status Status
	stdin  io.WriteCloser
	stdout io.ReadCloser
	proc   *process.Process
	cancel context.CancelFunc
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
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func CleanString(input string) string {
	return strings.TrimSpace(ansiRegex.ReplaceAllString(input, ""))
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
	s.cmd = exec.CommandContext(ctx, "java", "-Xmx"+s.RAM, "-Xms"+s.RAM, "-jar", s.JarFile, "nogui")
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
	s.mu.Unlock()

	go s.monitorProcess()
	go s.StreamLogs()

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
	var err error
	s.proc, err = process.NewProcess(int32(s.cmd.Process.Pid))
	if err != nil {
		s.cmd.Process.Kill()
	}

	s.cmd.Wait()

	s.mu.Lock()
	s.status = StatusStopped
	s.proc = nil
	s.startTime = time.Time{}
	s.mu.Unlock()

	s.cancel()
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

	// Baseline healthy Minecraft TPS & MSPT when running
	vitals.TPS = 20.0
	vitals.MSPT = 18.5

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
		s.Broadcast("[MC] " + text)

		cleanText := CleanString(text)

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
	if username != "" {
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

func (s *Server) AddListener(listener func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, listener)
}

func (s *Server) Broadcast(msg string) {
	// Add message to the LogHistory and copy listeners under lock
	s.mu.Lock()
	s.LogHistory = append(s.LogHistory, msg)
	if len(s.LogHistory) > 100 {
		s.LogHistory = s.LogHistory[1:]
	}
	currentListeners := make([]func(string), len(s.listeners))
	copy(currentListeners, s.listeners)
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

func NewServer(workDir string, jarFile string, ram string, store database.Store) *Server {
	return &Server{
		WorkDir:       workDir,
		JarFile:       jarFile,
		RAM:           ram,
		LogChan:       make(chan string),
		LogHistory:    make([]string, 0),
		history:       make([]MetricPoint, 0, 30),
		listeners:     make([]func(string), 0),
		status:        StatusStopped,
		store:         store,
		OnlinePlayers: make(map[string]Player),
		uuidCache:     make(map[string]string),

		Args: []string{},
	}
}
