package minecraft

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	s := NewServer("/tmp", "server.jar", "1024M")
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.WorkDir != "/tmp" {
		t.Errorf("Expected WorkDir /tmp, got %s", s.WorkDir)
	}
	if s.JarPath != "server.jar" {
		t.Errorf("Expected JarPath server.jar, got %s", s.JarPath)
	}
	if s.RAM != "1024M" {
		t.Errorf("Expected RAM 1024M, got %s", s.RAM)
	}
	if s.GetStatus() != StatusStopped {
		t.Errorf("Expected status Stopped, got %v", s.GetStatus())
	}
	if s.LogChan == nil {
		t.Error("Expected LogChan to be initialized")
	}
}

func TestServerStatusString(t *testing.T) {
	if StatusStopped.String() != "Stopped" {
		t.Errorf("Expected 'Stopped', got %s", StatusStopped.String())
	}
	if StatusStarting.String() != "Starting" {
		t.Errorf("Expected 'Starting', got %s", StatusStarting.String())
	}
	if StatusRunning.String() != "Running" {
		t.Errorf("Expected 'Running', got %s", StatusRunning.String())
	}
	if Status(99).String() != "Unknown" {
		t.Errorf("Expected 'Unknown', got %s", Status(99).String())
	}
}
