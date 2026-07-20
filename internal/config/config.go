package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      string
	DBPath    string
	Version   string
	BuildDate string
	PageSize  int
}

func Load() (*Config, error) {
	// Charger .env (optionnel si absent)
	_ = godotenv.Load()

	cfg := &Config{
		Port:      getEnv("PORT", "8080"),
		DBPath:    getEnv("DB_PATH", "./data/defta.db"),
		Version:   getEnv("VERSION", "0.1.0-dev"),
		BuildDate: getEnv("BUILD_DATE", "unknown"),
		PageSize:  getEnvInt("PAGE_SIZE", 30),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	val := getEnv(key, "")
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("Valeur invalide pour %s → utilisation de %d", key, fallback)
		return fallback
	}
	return n
}