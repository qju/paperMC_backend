package minecraft

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// Status represents the current state of the Minecraft server.
type Status int

const (
	StatusStopped  Status = iota // 0
	StatusStarting               // 1
	StatusRunning                // 2
)

// FloodgatePrefix is the prefix used for Bedrock players connected via Floodgate.
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

// Server controls the Minecraft server process.
type Server struct {
	// Public fields
	WorkDir string
	JarPath string
	RAM     string
	Args    []string
	LogChan chan string

	// Private fields
	cmd    *exec.Cmd
	mu     sync.Mutex
	status Status
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

// NewServer creates a new Server instance.
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

// Start launches the Minecraft server process.
// It returns an error if the server is already running.
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
	s.status = StatusRunning
	return nil
}

// Stop sends the "stop" command to the server and waits for it to exit.
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

// StreamLogs reads the server's stdout and broadcasts it to LogChan.
// It runs until the stdout pipe is closed.
func (s *Server) StreamLogs() {
	scanner := bufio.NewScanner(s.stdout)

	for scanner.Scan() {
		s.Broadcast("[MC] " + scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading log %v\n", err)
	}
}

// Broadcast sends a message to stdout and the LogChan if there is a listener.
func (s *Server) Broadcast(msg string) {
	// Sent msg to os output
	fmt.Println(msg)

	// Sent msg to frontend
	select {
	case s.LogChan <- msg:
		// Send successfully
	default:
		// No browser connected, drop the msg
	}
}

// SendCommand writes a command to the server's stdin.
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

// GetStatus returns the current status of the server.
func (s *Server) GetStatus() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// WhiteListUser adds a user to the whitelist.
// It attempts to resolve the user as a Java Edition player first, then as a Bedrock player via Geyser.
func (s *Server) WhiteListUser(username string) error {
	uuid, err := GetUUID(username)

	if err == nil {
		s.Broadcast(fmt.Sprintf("[System] Whitelisting Java UUID: %s for %s\n", uuid, username))
		return s.SendCommand("whitelist add " + username)
	}

	xuid, err := GetXUID(username)
	if err == nil {
		s.Broadcast(fmt.Sprintf("[System] Whitelisting Xbox XUID: %s for %s\n", xuid, username))

		finalName := username
		if !strings.HasPrefix(username, FloodgatePrefix) {
			finalName = FloodgatePrefix + username
		}
		return s.SendCommand("fwhitelist add " + finalName)
	}

	s.Broadcast(fmt.Sprintf("[ERROR] User %s not found on Mojang or Xbox live: %s", username, err))
	return fmt.Errorf("user not found on Mojang or Xbox Live")
}
