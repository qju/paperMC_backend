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
	"errors"
	"fmt"
	"github.com/shirou/gopsutil/v3/process"
	"io"
	"os/exec"
	"sync"
)

type Status int

const (
	StatusStopped  Status = iota // 0
	StatusStarting               // 1
	StatusRunning                // 2
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
	default:
		return "Unknown"
	}
}

type Server struct {
	// Public fields
	WorkDir    string
	JarFile    string
	RAM        string
	Args       []string
	LogChan    chan string
	LogHistory []string

	// Private fields
	cmd    *exec.Cmd
	mu     sync.Mutex
	status Status
	stdin  io.WriteCloser
	stdout io.ReadCloser
	proc   *process.Process
}

type Vitals struct {
	Status      Status  `json:"status"`
	CPU         float64 `json:"cpu"`
	RAM         uint64  `json:"ram"`
	TotalMemory string  `json:"total_memory"`
	Players     int     `json:"players"`
}

// MarshalText implements the encoding.TextMarshaler interface.
// This overrides the default integer serialization (0, 1, 2) with strings ("Stopped", etc).
func (s Status) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *Server) Start() error {
	// Try Lock the server Mutex immediately
	s.mu.Lock()
	defer s.mu.Unlock()
	// Check if server is already running if not return an error
	if s.status == StatusRunning {
		return errors.New("Server is already running")
	}
	s.cmd = exec.Command("java", "-Xmx"+s.RAM, "-Xms"+s.RAM, "-jar", s.JarFile, "nogui")
	s.cmd.Dir = s.WorkDir

	pipeIn, errIn := s.cmd.StdinPipe()
	if errIn != nil {
		return errIn
	}
	s.stdin = pipeIn

	pipeOut, errOut := s.cmd.StdoutPipe()
	if errOut != nil {
		return errOut
	}
	s.stdout = pipeOut

	if err := s.cmd.Start(); err != nil {
		return err
	}
	// 2. Create process inspector
	var err error
	s.proc, err = process.NewProcess(int32(s.cmd.Process.Pid))
	if err != nil {
		s.cmd.Process.Kill()
		return err
	}

	s.status = StatusRunning
	go s.StreamLogs()
	return nil
}

func (s *Server) Stop() error {
	if s.GetStatus() == StatusStopped {
		return errors.New("server already stopped")
	}
	s.SendCommand("stop")
	if err := s.cmd.Wait(); err != nil {
		s.mu.Lock()
		s.proc = nil
		s.status = StatusStopped
		s.mu.Unlock()
		return err
	}

	s.mu.Lock()
	s.proc = nil
	s.status = StatusStopped
	s.mu.Unlock()

	return nil
}

func (s *Server) GetVitals() Vitals {
	// ToDo: if satus failes in front end add Text Marshal
	s.mu.Lock()
	defer s.mu.Unlock()

	vitals := Vitals{
		Status:      s.status,
		TotalMemory: s.RAM,
		Players:     0, // Placeholder
	}

	// 1. If Server is not running, returm basic status (0 CPU/RAM)
	if s.cmd == nil || s.cmd.Process == nil || s.status != StatusRunning {
		return vitals
	}

	// 3. GET CPU %
	cpu, err := s.proc.Percent(0)
	if err != nil {
		return vitals
	}
	vitals.CPU = cpu

	// 4. Get RAM usage
	mem, err := s.proc.MemoryInfo()
	if err != nil {
		return vitals
	}
	vitals.RAM = mem.RSS

	return vitals
}

func (s *Server) StreamLogs() {
	scanner := bufio.NewScanner(s.stdout)

	for scanner.Scan() {
		s.Broadcast("[MC] " + scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading log %v\n", err)
	}
}

func (s *Server) Broadcast(msg string) {
	// Sent msg to frontend
	select {
	case s.LogChan <- msg: // Send successfully
	default: // No browser connected, drop the msg
	}

	// Add message to the LogHistory and
	s.mu.Lock()
	defer s.mu.Unlock()

	s.LogHistory = append(s.LogHistory, msg)

	// Ring Buffer: keep max 100 lines
	if len(s.LogHistory) > 100 {
		s.LogHistory = s.LogHistory[1:]
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

	if s.status != StatusRunning {
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

func NewServer(workDir string, jarFile string, ram string) *Server {
	return &Server{
		WorkDir:    workDir,
		JarFile:    jarFile,
		RAM:        ram,
		LogChan:    make(chan string),
		status:     StatusStopped,
		LogHistory: make([]string, 0),

		Args: []string{},
	}
}
