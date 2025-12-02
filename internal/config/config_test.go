package config

import (
	"os"
	"testing"
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
	t.Run("Panic on missing ADMIN_PASS", func(t *testing.T) {
		os.Unsetenv("ADMIN_PASS")
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Load did not panic as expected when ADMIN_PASS is missing")
			}
		}()
		Load()
	})

	// Case 2: Default values
	t.Run("Default values", func(t *testing.T) {
		os.Setenv("ADMIN_PASS", "secret")
		os.Unsetenv("PORT") // Ensure other envs are unset to test defaults
		os.Unsetenv("MC_WORKDIR")
		os.Unsetenv("JAR_FILE")
		os.Unsetenv("RAM")
		os.Unsetenv("ADMIN_USER")

		config := Load()
		assertString(t, "8080", config.Port, "Port")
		assertString(t, "./paperMC", config.WorkDir, "WorkDir")
		assertString(t, "server.jar", config.JarFile, "JarFile")
		assertString(t, "2048M", config.RAM, "RAM")
		assertString(t, "admin", config.AdminUser, "AdminUser")
		assertString(t, "secret", config.AdminPass, "AdminPass")
	})

	// Case 3: Custom values
	t.Run("Custom values", func(t *testing.T) {
		os.Setenv("PORT", "9090")
		os.Setenv("MC_WORKDIR", "/tmp/mc")
		os.Setenv("JAR_FILE", "paper.jar")
		os.Setenv("RAM", "4096M")
		os.Setenv("ADMIN_USER", "operator")
		os.Setenv("ADMIN_PASS", "topsecret")

		config := Load()
		assertString(t, "9090", config.Port, "Port")
		assertString(t, "/tmp/mc", config.WorkDir, "WorkDir")
		assertString(t, "paper.jar", config.JarFile, "JarFile")
		assertString(t, "4096M", config.RAM, "RAM")
		assertString(t, "operator", config.AdminUser, "AdminUser")
		assertString(t, "topsecret", config.AdminPass, "AdminPass")
	})
}

func assertString(t *testing.T, want, got, field string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}
