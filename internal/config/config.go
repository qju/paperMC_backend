package config

import (
	"os"
)

type Config struct {
	Port    string
	WorkDir string
	JarFile string
	RAM     string

	AdminUser string
	AdminPass string
}

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

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvNoFallback(key string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	panic("Add Admin password to ENVIRONMENT variable: " + key)
}
