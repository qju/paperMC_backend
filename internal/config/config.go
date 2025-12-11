package config

import (
	"os"
)

type Config struct {
	Port    string
	WorkDir string
	JarFile string
	RAM     string

	DbName string
}

func Load() *Config {
	return &Config{
		Port:    getEnv("PORT", "8080"),
		WorkDir: getEnv("MC_WORKDIR", "./paperMC"),
		JarFile: getEnv("JAR_FILE", "server.jar"),
		RAM:     getEnv("RAM", "2048M"),
		DbName:  getEnv("DBNAME", "paper.db"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
