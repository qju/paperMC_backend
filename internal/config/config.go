package config

import (
	"os"
)

type Config struct {
	Port    string
	WorkDir string
	JarFile string
	RAM     string

	DBName    string
	AdminUser string
	AdminPass string
}

func Load() *Config {
	return &Config{
		Port:      getEnv("PORT", "8080"),
		WorkDir:   getEnv("MC_WORKDIR", "./paperMS"),
		JarFile:   getEnv("JAR_FILE", "server.jar"),
		RAM:       getEnv("RAM", "8G"),
		DBName:    getEnv("DBNAME", "paper.db"),
		AdminUser: getEnv("ADMIN_USER", "admin"),
		AdminPass: getEnv("ADMIN_PASS", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
