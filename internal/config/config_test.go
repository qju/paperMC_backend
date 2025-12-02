package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Setup: Reset env vars after test
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("MC_WORKDIR")
		os.Unsetenv("JAR_FILE")
		os.Unsetenv("RAM")
		os.Unsetenv("ADMIN_USER")
		os.Unsetenv("ADMIN_PASS")
	}()

	// Case 1: Panic when ADMIN_PASS is missing
	os.Unsetenv("ADMIN_PASS")
	assert.Panics(t, func() { Load() }, "Load should panic when ADMIN_PASS is missing")

	// Case 2: Default values
	os.Setenv("ADMIN_PASS", "secret")
	os.Unsetenv("PORT") // Ensure other envs are unset to test defaults
	config := Load()
	assert.Equal(t, "8080", config.Port)
	assert.Equal(t, "./paperMC", config.WorkDir)
	assert.Equal(t, "server.jar", config.JarFile)
	assert.Equal(t, "2048M", config.RAM)
	assert.Equal(t, "admin", config.AdminUser)
	assert.Equal(t, "secret", config.AdminPass)

	// Case 3: Custom values
	os.Setenv("PORT", "9090")
	os.Setenv("MC_WORKDIR", "/tmp/mc")
	os.Setenv("JAR_FILE", "paper.jar")
	os.Setenv("RAM", "4096M")
	os.Setenv("ADMIN_USER", "operator")
	os.Setenv("ADMIN_PASS", "topsecret")

	config = Load()
	assert.Equal(t, "9090", config.Port)
	assert.Equal(t, "/tmp/mc", config.WorkDir)
	assert.Equal(t, "paper.jar", config.JarFile)
	assert.Equal(t, "4096M", config.RAM)
	assert.Equal(t, "operator", config.AdminUser)
	assert.Equal(t, "topsecret", config.AdminPass)
}
