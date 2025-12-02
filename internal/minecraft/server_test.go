package minecraft

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewServer(t *testing.T) {
	s := NewServer("/tmp", "server.jar", "1024M")
	assert.NotNil(t, s)
	assert.Equal(t, "/tmp", s.WorkDir)
	assert.Equal(t, "server.jar", s.JarPath)
	assert.Equal(t, "1024M", s.RAM)
	assert.Equal(t, StatusStopped, s.GetStatus())
	assert.NotNil(t, s.LogChan)
}

func TestServerStatusString(t *testing.T) {
	assert.Equal(t, "Stopped", StatusStopped.String())
	assert.Equal(t, "Starting", StatusStarting.String())
	assert.Equal(t, "Running", StatusRunning.String())
	assert.Equal(t, "Unknown", Status(99).String())
}

// TestWhiteListUser logic is hard to test in isolation without full mocking of SendCommand
// or integration tests. However, we have tested GetUUID and GetXUID separately.
