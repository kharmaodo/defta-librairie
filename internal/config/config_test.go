package config

import "testing"

func TestGetEnvRemovesWindowsLineEnding(t *testing.T) {
	t.Setenv("DEFTA_TEST_CRLF", "8080\r")
	if value := getEnv("DEFTA_TEST_CRLF", "fallback"); value != "8080" {
		t.Fatalf("expected sanitized port, got %q", value)
	}
}

func TestGetEnvPreservesMeaningfulSpaces(t *testing.T) {
	t.Setenv("DEFTA_TEST_SPACES", "value with spaces")
	if value := getEnv("DEFTA_TEST_SPACES", "fallback"); value != "value with spaces" {
		t.Fatalf("expected spaces to be preserved, got %q", value)
	}
}

func TestLoadCoverConfiguration(t *testing.T) {
	t.Setenv("COVERS_ENABLED", "true")
	t.Setenv("MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MINIO_ACCESS_KEY", "test-access")
	t.Setenv("MINIO_SECRET_KEY", "test-secret")
	t.Setenv("MINIO_USE_SSL", "true")
	t.Setenv("MINIO_BUCKET_COVERS", "covers-test")
	t.Setenv("COVER_MAX_BYTES", "1024")
	t.Setenv("COVER_MAX_PIXELS", "2000")
	t.Setenv("NATS_URL", "nats://nats:4222")
	t.Setenv("NATS_USER", "covers-api")
	t.Setenv("NATS_PASSWORD", "test-password")
	t.Setenv("NATS_COVERS_STREAM", "COVERS_TEST")
	t.Setenv("NATS_COVERS_SUBJECT", "book.covers.test.v1")
	t.Setenv("NATS_COVERS_CONSUMER", "cover-worker-test")
	t.Setenv("COVER_WORKER_MAX_DELIVER", "7")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.CoversEnabled || cfg.MinIOEndpoint != "minio:9000" ||
		cfg.MinIOAccessKey != "test-access" || cfg.MinIOSecretKey != "test-secret" ||
		!cfg.MinIOUseSSL || cfg.MinIOBucketCovers != "covers-test" ||
		cfg.CoverMaxBytes != 1024 || cfg.CoverMaxPixels != 2000 ||
		cfg.NATSURL != "nats://nats:4222" || cfg.NATSUser != "covers-api" ||
		cfg.NATSPassword != "test-password" || cfg.NATSCoversStream != "COVERS_TEST" ||
		cfg.NATSCoversSubject != "book.covers.test.v1" ||
		cfg.NATSCoversConsumer != "cover-worker-test" || cfg.CoverWorkerMaxDeliver != 7 {
		t.Fatalf("unexpected cover config: %+v", cfg)
	}
}

func TestCoverLimitsRejectInvalidValues(t *testing.T) {
	t.Setenv("COVER_MAX_BYTES", "-1")
	t.Setenv("COVER_MAX_PIXELS", "invalid")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.CoverMaxBytes != 5*1024*1024 || cfg.CoverMaxPixels != 24_000_000 {
		t.Fatalf("unexpected cover defaults: bytes=%d pixels=%d", cfg.CoverMaxBytes, cfg.CoverMaxPixels)
	}
}
