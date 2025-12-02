package config

import (
	"os"
)

// Config holds the configuration values for the server.
type Config struct {
	Port    string
	WorkDir string
	JarFile string
	RAM     string

	AdminUser string
	AdminPass string
}

// Load reads configuration from environment variables and returns a Config struct.
// It sets default values if the environment variables are not present, except for ADMIN_PASS which is required.
func Load() *Config {
	return &Config{
		Port:      getEnv("PORT", "8080"),
		WorkDir:   getEnv("MC_WORKDIR", "./paperMC"),
		JarFile:   getEnv("JAR_FILE", "server.jar"),
		RAM:       getEnv("RAM", "2048M"),
		AdminUser: getEnv("ADMIN_USER", "admin"),
		AdminPass: getEnvNoFallback("ADMIN_PASS"),
	}
}

// getEnv retrieves the value of the environment variable named by the key.
// If the variable is present, the value (which may be empty) is returned.
// Otherwise, the fallback value is returned.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvNoFallback retrieves the value of the environment variable named by the key.
// It panics if the variable is not present.
func getEnvNoFallback(key string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	panic("Add Admin password to ENVIRONMENT variable: " + key)
}
