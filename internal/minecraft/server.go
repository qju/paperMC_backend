package minecraft

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type Status int

const (
	StatusStopped  Status = iota //0
	StatusStarting               //1
	StatusRunning                //2
)

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
	WorkDir string
	JarPath string
	RAM     string
	Args    []string
	LogChan chan string

	// Prive fields
	cmd    *exec.Cmd
	mu     sync.Mutex
	status Status
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (s *Server) Start() error {
	// Try Lock the server Mutex immediately
	s.mu.Lock()
	defer s.mu.Unlock()
	// Check if server is already running if not return an error
	if s.status == StatusRunning {
		return errors.New("Server is already running")
	}
	s.cmd = exec.Command("java", "-Xmx"+s.RAM, "-Xms"+s.RAM, "-jar", s.JarPath, "nogui")
	s.cmd.Dir = s.WorkDir

	pipe_in, err_in := s.cmd.StdinPipe()
	if err_in != nil {
		return err_in
	}
	s.stdin = pipe_in

	pipe_out, err_out := s.cmd.StdoutPipe()
	if err_out != nil {
		return err_out
	}
	s.stdout = pipe_out

	if err := s.cmd.Start(); err != nil {
		return err
	}
	s.status = StatusRunning
	return nil
}

func (s *Server) Stop() error {
	if s.GetStatus() == StatusStopped {
		return errors.New("server already stopped")
	}
	s.SendCommand("stop")
	if err := s.cmd.Wait(); err != nil {
		return err
	}

	s.mu.Lock()
	s.status = StatusStopped
	s.mu.Unlock()

	return nil
}

func (s *Server) StreamLogs() {
	scanner := bufio.NewScanner(s.stdout)

	for scanner.Scan() {
		fmt.Printf("[MC] %s\n", scanner.Text())
		select {
		case s.LogChan <- "[MC] " + scanner.Text():
			// message sent
		default:
			// No one listening, drop message to prevent listenning
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading log %v\n", err)
	}
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

func NewServer(workDir string, jarPath string, ram string) *Server {
	return &Server{
		WorkDir: workDir,
		JarPath: jarPath,
		RAM:     ram,
		LogChan: make(chan string),
		status:  StatusStopped,

		Args: []string{},
	}
}

//["java", "-Xms9216M", "-Xmx9216M", "-XX:+AlwaysPreTouch", "-XX:+DisableExplicitGC", "-XX:+ParallelRefProcEnabled", "-XX:+PerfDisableSharedMem", "-XX:+UnlockExperimentalVMOptions", "-XX:+UseG1GC", "-XX:G1HeapRegionSize=8M", "-XX:G1HeapWastePercent=5", "-XX:G1MaxNewSizePercent=40", "-XX:G1MixedGCCountTarget=4", "-XX:G1MixedGCLiveThresholdPercent=90", "-XX:G1NewSizePercent=30", "-XX:G1RSetUpdatingPauseTimePercent=5", "-XX:G1ReservePercent=20", "-XX:InitiatingHeapOccupancyPercent=15", "-XX:MaxGCPauseMillis=200", "-XX:MaxTenuringThreshold=1", "-XX:SurvivorRatio=32", "-Dusing.aikars.flags=https://mcflags.emc.gs", "-Daikars.new.flags=true"]
