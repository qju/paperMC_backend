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

type Status int

const (
	StatusStopped  Status = iota //0
	StatusStarting               //1
	StatusRunning                //2
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
	WorkDir string
	JarFile string
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
	go s.StreamLogs()
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
		s.Broadcast("[MC] " + scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading log %v\n", err)
	}
}

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
		WorkDir: workDir,
		JarFile: jarFile,
		RAM:     ram,
		LogChan: make(chan string),
		status:  StatusStopped,

		Args: []string{},
	}
}

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
	// Failure: Neither API found a user

}
