package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                      string
	DBPath                    string
	Version                   string
	BuildDate                 string
	PageSize                  int
	JWTSecret                 string
	JWTIssuer                 string
	JWTAudience               string
	JWTAccessTTL              time.Duration
	JWTRefreshTTL             time.Duration
	AuthRateLimit             int
	AuthRateWindow            time.Duration
	AuthCookieSecure          bool
	CoversEnabled             bool
	MinIOEndpoint             string
	MinIOAccessKey            string
	MinIOSecretKey            string
	MinIOUseSSL               bool
	MinIOBucketCovers         string
	MinIOSourceRetentionHours int
	CoverMaxBytes             int64
	CoverMaxPixels            uint64
	NATSURL                   string
	NATSUser                  string
	NATSPassword              string
	NATSCoversStream          string
	NATSCoversSubject         string
	NATSCoversConsumer        string
	CoverWorkerMaxDeliver     int
	NATSSubmissionStream      string
	NATSSubmissionSubject     string
}

func Load() (*Config, error) {
	// Charger .env (optionnel si absent)
	_ = godotenv.Load()

	cfg := &Config{
		Port:                      getEnv("PORT", "8080"),
		DBPath:                    getEnv("DB_PATH", "./data/defta.db"),
		Version:                   getEnv("VERSION", "0.1.0-dev"),
		BuildDate:                 getEnv("BUILD_DATE", "unknown"),
		PageSize:                  getEnvInt("PAGE_SIZE", 30),
		JWTSecret:                 getEnv("JWT_SECRET", ""),
		JWTIssuer:                 getEnv("JWT_ISSUER", "defta-librairie"),
		JWTAudience:               getEnv("JWT_AUDIENCE", "defta-librairie-web"),
		JWTAccessTTL:              time.Duration(getEnvInt("JWT_ACCESS_TTL_SECONDS", 900)) * time.Second,
		JWTRefreshTTL:             time.Duration(getEnvInt("JWT_REFRESH_TTL_SECONDS", 604800)) * time.Second,
		AuthRateLimit:             getEnvInt("AUTH_RATE_LIMIT_REQUESTS", 10),
		AuthRateWindow:            time.Duration(getEnvInt("AUTH_RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
		AuthCookieSecure:          getEnvBool("AUTH_COOKIE_SECURE", false),
		CoversEnabled:             getEnvBool("COVERS_ENABLED", false),
		MinIOEndpoint:             getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:            getEnv("MINIO_ACCESS_KEY", ""),
		MinIOSecretKey:            getEnv("MINIO_SECRET_KEY", ""),
		MinIOUseSSL:               getEnvBool("MINIO_USE_SSL", false),
		MinIOBucketCovers:         getEnv("MINIO_BUCKET_COVERS", "book-covers"),
		MinIOSourceRetentionHours: getEnvInt("MINIO_SOURCE_RETENTION_HOURS", 24),
		CoverMaxBytes:             getEnvPositiveInt64("COVER_MAX_BYTES", 5*1024*1024),
		CoverMaxPixels:            uint64(getEnvPositiveInt64("COVER_MAX_PIXELS", 24_000_000)),
		NATSURL:                   getEnv("NATS_URL", "nats://localhost:4222"),
		NATSUser:                  getEnv("NATS_USER", ""),
		NATSPassword:              getEnv("NATS_PASSWORD", ""),
		NATSCoversStream:          getEnv("NATS_COVERS_STREAM", "BOOK_COVERS"),
		NATSCoversSubject:         getEnv("NATS_COVERS_SUBJECT", "book.covers.process.v1"),
		NATSCoversConsumer:        getEnv("NATS_COVERS_CONSUMER", "cover-worker-v1"),
		CoverWorkerMaxDeliver:     int(getEnvPositiveInt64("COVER_WORKER_MAX_DELIVER", 5)),
		NATSSubmissionStream:      getEnv("NATS_SUBMISSION_STREAM", "BOOK_SUBMISSIONS"),
		NATSSubmissionSubject:     getEnv("NATS_SUBMISSION_SUBJECT", "book.submissions.moderate.v1"),
	}

	return cfg, nil
}

func getEnvBool(key string, fallback bool) bool {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf("Valeur invalide pour %s → utilisation de %t", key, fallback)
		return fallback
	}
	return parsed
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		// Un fichier .env modifié sous Windows peut conserver un retour
		// chariot final. Il rend notamment PORT invalide sous Linux.
		return strings.TrimRight(value, "\r\n")
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

func getEnvPositiveInt64(key string, fallback int64) int64 {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		log.Printf("Valeur invalide pour %s → utilisation de %d", key, fallback)
		return fallback
	}
	return parsed
}
